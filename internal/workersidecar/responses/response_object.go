package responses

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/dimetron/pi-go/internal/workersidecar"
)

const defaultMaxResultBytes = 262144

// WrapRunResult replaces result_text with a Responses API object when an envelope was present.
func WrapRunResult(result workersidecar.RunResult, parsed ParsedEnvelope, taskID, boundModel string) workersidecar.RunResult {
	if !parsed.Present || parsed.ErrorCode != "" {
		return result
	}
	respStatus := "completed"
	switch result.Status {
	case "failed":
		respStatus = "failed"
	case "cancelled":
		respStatus = "cancelled"
	}
	text := result.ResultText
	if text == "" && result.Summary != "" && !durationOnlySummary(result.Summary) {
		text = result.Summary
	}
	var tools []interface{}
	if parsed.RequestEcho != nil {
		if raw, ok := parsed.RequestEcho["tools"]; ok {
			if list, ok := raw.([]interface{}); ok {
				tools = list
			}
		}
	}
	var usage map[string]interface{}
	if result.Usage != nil {
		usage = map[string]interface{}{
			"input_tokens":  result.Usage.InputTokens,
			"output_tokens": result.Usage.OutputTokens,
			"total_tokens":  result.Usage.TotalTokens,
		}
	}
	var errText *string
	if result.Status != "completed" && result.Error != "" {
		errText = &result.Error
	}
	obj := BuildResponseObject(
		parsed.ResponseID,
		parsed.Model,
		boundModel,
		respStatus,
		text,
		taskID,
		parsed.Instructions,
		tools,
		usage,
		errText,
	)
	maxBytes := parsed.MaxResultBytes
	if maxBytes <= 0 {
		maxBytes = defaultMaxResultBytes
	}
	result.ResultText = SerializeResponse(obj, maxBytes)
	if result.Summary == "" && text != "" {
		if len(text) > 500 {
			result.Summary = text[:500]
		} else {
			result.Summary = text
		}
	}
	return result
}

// BuildResponseObject builds an OpenAI Responses object (Hermes build_response_object subset).
func BuildResponseObject(responseID, envelopeModel, boundModel, status, outputText, taskID, instructions string, tools []interface{}, usage map[string]interface{}, errText *string) map[string]interface{} {
	rid := strings.TrimSpace(responseID)
	if rid == "" {
		rid = "resp-" + taskID
	}
	model := strings.TrimSpace(envelopeModel)
	if model == "" {
		model = strings.TrimSpace(boundModel)
	}
	text := outputText
	if text == "" && errText != nil {
		text = *errText
	}
	if text == "" && status != "completed" {
		text = status
	}
	message := map[string]interface{}{
		"id":     "msg-" + taskID,
		"type":   "message",
		"role":   "assistant",
		"status": status,
		"content": []map[string]interface{}{
			{
				"type":        "output_text",
				"text":        text,
				"annotations": []interface{}{},
				"logprobs":    []interface{}{},
			},
		},
	}
	var errField interface{}
	if errText != nil {
		errField = *errText
	}
	return map[string]interface{}{
		"id":                   rid,
		"object":               "response",
		"created_at":           time.Now().Unix(),
		"status":               status,
		"model":                model,
		"output":               []interface{}{message},
		"store":                false,
		"previous_response_id": nil,
		"parallel_tool_calls":  false,
		"instructions":         instructions,
		"tools":                toolsOrEmpty(tools),
		"tool_choice":          "auto",
		"truncation":           "disabled",
		"reasoning":            map[string]interface{}{"effort": nil, "summary": nil},
		"usage":                usageOrEmpty(usage),
		"incomplete_details":   nil,
		"error":                errField,
		"metadata":             map[string]interface{}{"task_id": taskID, "truncated": false},
	}
}

func durationOnlySummary(summary string) bool {
	s := strings.TrimSpace(summary)
	if !strings.HasPrefix(s, "Completed in ") {
		return false
	}
	rest := strings.TrimPrefix(s, "Completed in ")
	return strings.HasSuffix(rest, "s") || strings.HasSuffix(rest, "m") || strings.HasSuffix(rest, "h")
}

func toolsOrEmpty(tools []interface{}) []interface{} {
	if tools == nil {
		return []interface{}{}
	}
	return tools
}

func usageOrEmpty(usage map[string]interface{}) map[string]interface{} {
	if usage == nil {
		return map[string]interface{}{}
	}
	return usage
}

// SerializeResponse JSON-encodes the response object, trimming output_text to maxBytes when needed.
func SerializeResponse(obj map[string]interface{}, maxBytes int) string {
	if maxBytes <= 0 {
		maxBytes = defaultMaxResultBytes
	}
	encoded, _ := json.Marshal(obj)
	if len(encoded) <= maxBytes {
		return string(encoded)
	}
	trimmed := deepCopyMap(obj)
	text := firstOutputText(trimmed)
	if text == "" {
		return string(encoded)
	}
	lo, hi := 0, len(text)
	best := string(encoded)
	for lo <= hi {
		mid := (lo + hi) / 2
		setFirstOutputText(trimmed, text[:mid])
		setMetadataTruncated(trimmed, true)
		candidate, _ := json.Marshal(trimmed)
		if len(candidate) <= maxBytes {
			best = string(candidate)
			lo = mid + 1
		} else {
			hi = mid - 1
		}
	}
	return best
}

func deepCopyMap(in map[string]interface{}) map[string]interface{} {
	raw, _ := json.Marshal(in)
	var out map[string]interface{}
	_ = json.Unmarshal(raw, &out)
	return out
}

func firstOutputText(obj map[string]interface{}) string {
	output, ok := obj["output"].([]interface{})
	if !ok {
		return ""
	}
	for _, item := range output {
		m, ok := item.(map[string]interface{})
		if !ok || m["type"] != "message" {
			continue
		}
		content, ok := m["content"].([]interface{})
		if !ok || len(content) == 0 {
			continue
		}
		part, ok := content[0].(map[string]interface{})
		if !ok {
			continue
		}
		if t, ok := part["text"].(string); ok {
			return t
		}
	}
	return ""
}

func setFirstOutputText(obj map[string]interface{}, text string) {
	output, ok := obj["output"].([]interface{})
	if !ok || len(output) == 0 {
		return
	}
	m, ok := output[0].(map[string]interface{})
	if !ok {
		return
	}
	content, ok := m["content"].([]interface{})
	if !ok || len(content) == 0 {
		return
	}
	part, ok := content[0].(map[string]interface{})
	if !ok {
		return
	}
	part["text"] = text
}

func setMetadataTruncated(obj map[string]interface{}, truncated bool) {
	meta, ok := obj["metadata"].(map[string]interface{})
	if !ok {
		meta = map[string]interface{}{}
		obj["metadata"] = meta
	}
	meta["truncated"] = truncated
}
