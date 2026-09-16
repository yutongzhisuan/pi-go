package responses

import (
	"fmt"
	"strings"
)

// ReplaySplit is the result of SplitReplayMessages (Hermes split_replay_messages subset).
type ReplaySplit struct {
	Structured  bool
	History     []map[string]interface{}
	CurrentUser map[string]interface{}
}

// TurnToolCall is a tool invocation emitted as a Responses function_call item.
type TurnToolCall struct {
	Name      string
	Arguments string
	CallID    string
}

// TurnAccum collects one executor turn for TurnOutputItems.
type TurnAccum struct {
	AssistantText string
	ReasoningText string
	ToolCalls     []TurnToolCall
}

// SplitReplayMessages splits Responses input into replay history and the trailing user turn.
// When the transcript does not end with a user message, Structured is false and callers flatten.
func SplitReplayMessages(rawInput interface{}) ReplaySplit {
	items, ok := inputItems(rawInput)
	if !ok || len(items) == 0 {
		return ReplaySplit{}
	}
	if len(items) == 0 {
		return ReplaySplit{}
	}
	if !isUserItem(items[len(items)-1]) {
		return ReplaySplit{}
	}
	lastUser := len(items) - 1
	history := make([]map[string]interface{}, 0, lastUser)
	for i := 0; i < lastUser; i++ {
		history = append(history, items[i])
	}
	return ReplaySplit{
		Structured:  true,
		History:     history,
		CurrentUser: items[lastUser],
	}
}

// OutputItemsAfterLastUser returns output items belonging to the turn after the last user input.
func OutputItemsAfterLastUser(rawInput interface{}, output []map[string]interface{}) []map[string]interface{} {
	split := SplitReplayMessages(rawInput)
	if !split.Structured || len(output) == 0 {
		return output
	}
	skip := priorTurnOutputCount(split.History)
	if skip <= 0 {
		return output
	}
	idx := 0
	for idx < len(output) && skip > 0 {
		if isAssistantOrToolOutput(output[idx]) {
			skip--
		}
		idx++
	}
	if idx >= len(output) {
		return nil
	}
	return output[idx:]
}

func priorTurnOutputCount(history []map[string]interface{}) int {
	n := 0
	for _, item := range history {
		t := strings.ToLower(stringField(item["type"]))
		switch t {
		case "function_call", "function_call_output", "reasoning":
			n++
		case "message", "":
			if strings.ToLower(stringField(item["role"])) == "assistant" {
				n++
			}
		}
	}
	return n
}

// TurnOutputItems builds Responses output item maps for SSE (Hermes turn_output_items subset).
func TurnOutputItems(taskID string, acc TurnAccum) []map[string]interface{} {
	var out []map[string]interface{}
	idx := 0
	if strings.TrimSpace(acc.ReasoningText) != "" {
		out = append(out, map[string]interface{}{
			"id":     fmt.Sprintf("rsn_%s_%d", taskID, idx),
			"type":   "reasoning",
			"status": "completed",
			"summary": []interface{}{
				map[string]interface{}{"type": "summary_text", "text": acc.ReasoningText},
			},
		})
		idx++
	}
	if text := strings.TrimSpace(acc.AssistantText); text != "" || len(acc.ToolCalls) == 0 {
		out = append(out, map[string]interface{}{
			"id":     fmt.Sprintf("msg_%s_%d", taskID, idx),
			"type":   "message",
			"role":   "assistant",
			"status": "completed",
			"content": []interface{}{
				map[string]interface{}{
					"type": "output_text",
					"text": text,
				},
			},
		})
		idx++
	}
	for i, tc := range acc.ToolCalls {
		callID := strings.TrimSpace(tc.CallID)
		if callID == "" {
			callID = fmt.Sprintf("call_%s_%d", taskID, i)
		}
		out = append(out, map[string]interface{}{
			"id":        fmt.Sprintf("fc_%s_%d", taskID, idx+i),
			"type":      "function_call",
			"call_id":   callID,
			"name":      tc.Name,
			"arguments": tc.Arguments,
			"status":    "completed",
		})
	}
	return out
}

// ResponseOutputItemAddedEvents wraps output items as response.output_item.added SSE payloads.
func ResponseOutputItemAddedEvents(items []map[string]interface{}) []map[string]interface{} {
	events := make([]map[string]interface{}, len(items))
	for i, item := range items {
		events[i] = map[string]interface{}{
			"type":         "response.output_item.added",
			"output_index": i,
			"item":         item,
		}
	}
	return events
}

// UserMessageFromSplit formats the current user turn with optional instructions prefix.
func UserMessageFromSplit(split ReplaySplit, instructions, goal string) string {
	if !split.Structured || split.CurrentUser == nil {
		return ""
	}
	text := contentText(split.CurrentUser["content"])
	if strings.TrimSpace(text) == "" {
		text = goal
	}
	if instructions != "" {
		return instructions + "\n\n" + text
	}
	return text
}

// FormatReplayHistoryBlock renders structured history for executor injection.
func FormatReplayHistoryBlock(history []map[string]interface{}) string {
	if len(history) == 0 {
		return ""
	}
	var lines []string
	for _, item := range history {
		lines = append(lines, replayHistoryLine(item))
	}
	return strings.Join(lines, "\n")
}

func replayHistoryLine(item map[string]interface{}) string {
	if item == nil {
		return ""
	}
	itemType := strings.ToLower(stringField(item["type"]))
	if itemType == "function_call" {
		return fmt.Sprintf("tool_call: %s(%s)", stringField(item["name"]), stringField(item["arguments"]))
	}
	if itemType == "function_call_output" {
		return fmt.Sprintf("tool_output(%s): %s", stringField(item["call_id"]), stringField(item["output"]))
	}
	role := strings.ToLower(stringField(item["role"]))
	if role == "" {
		role = "user"
	}
	body := contentText(item["content"])
	if body == "" {
		body = stringField(item["output"])
	}
	return fmt.Sprintf("%s: %s", role, body)
}

func inputItems(rawInput interface{}) ([]map[string]interface{}, bool) {
	switch v := rawInput.(type) {
	case nil:
		return nil, false
	case string:
		return nil, false
	case []interface{}:
		out := make([]map[string]interface{}, 0, len(v))
		for _, el := range v {
			m, ok := el.(map[string]interface{})
			if !ok {
				continue
			}
			out = append(out, m)
		}
		return out, true
	case []map[string]interface{}:
		return v, true
	default:
		return nil, false
	}
}

func isUserItem(item map[string]interface{}) bool {
	if item == nil {
		return false
	}
	itemType := strings.ToLower(stringField(item["type"]))
	if itemType != "" && itemType != "message" {
		return false
	}
	return strings.ToLower(stringField(item["role"])) == "user"
}

func isAssistantOrToolOutput(item map[string]interface{}) bool {
	if item == nil {
		return false
	}
	switch strings.ToLower(stringField(item["type"])) {
	case "message", "function_call", "reasoning":
		return true
	default:
		return false
	}
}
