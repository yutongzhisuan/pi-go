package responses

import (
	"encoding/json"
	"testing"
)

func TestSplitReplayMessages_empty(t *testing.T) {
	split := SplitReplayMessages(nil)
	if split.Structured || len(split.History) != 0 || split.CurrentUser != nil {
		t.Fatalf("split = %+v", split)
	}
}

func TestSplitReplayMessages_trailingUser(t *testing.T) {
	input := []interface{}{
		map[string]interface{}{"role": "user", "content": "first"},
		map[string]interface{}{"role": "assistant", "content": "ok"},
		map[string]interface{}{"role": "user", "content": "second"},
	}
	split := SplitReplayMessages(input)
	if !split.Structured {
		t.Fatal("expected structured split")
	}
	if len(split.History) != 2 {
		t.Fatalf("history len = %d", len(split.History))
	}
	if userText(split.CurrentUser) != "second" {
		t.Fatalf("current user = %q", userText(split.CurrentUser))
	}
}

func TestSplitReplayMessages_responsesMessageShape(t *testing.T) {
	input := []interface{}{
		map[string]interface{}{
			"type": "message", "role": "user",
			"content": []interface{}{
				map[string]interface{}{"type": "input_text", "text": "hello"},
			},
		},
		map[string]interface{}{
			"type": "message", "role": "assistant",
			"content": []interface{}{
				map[string]interface{}{"type": "output_text", "text": "hi"},
			},
		},
		map[string]interface{}{
			"type": "message", "role": "user",
			"content": []interface{}{
				map[string]interface{}{"type": "input_text", "text": "follow up"},
			},
		},
	}
	split := SplitReplayMessages(input)
	if !split.Structured || userText(split.CurrentUser) != "follow up" {
		t.Fatalf("split = %+v", split)
	}
}

func TestSplitReplayMessages_noTrailingUserFlatten(t *testing.T) {
	input := []interface{}{
		map[string]interface{}{"role": "user", "content": "only user"},
		map[string]interface{}{"role": "assistant", "content": "reply"},
	}
	split := SplitReplayMessages(input)
	if split.Structured {
		t.Fatalf("expected flatten path, got %+v", split)
	}
}

func TestSplitReplayMessages_stringInputNotStructured(t *testing.T) {
	split := SplitReplayMessages("plain text")
	if split.Structured {
		t.Fatal("string input must not structured-split")
	}
}

func TestTurnOutputItems_messageAndTools(t *testing.T) {
	items := TurnOutputItems("task-1", TurnAccum{
		AssistantText: "done",
		ToolCalls: []TurnToolCall{
			{Name: "bash", Arguments: `{"cmd":"ls"}`, CallID: "call_1"},
		},
	})
	if len(items) != 2 {
		t.Fatalf("items = %d", len(items))
	}
	if items[0]["type"] != "message" {
		t.Fatalf("first item type = %v", items[0]["type"])
	}
	if items[1]["type"] != "function_call" {
		t.Fatalf("second item type = %v", items[1]["type"])
	}
	if items[1]["call_id"] != "call_1" {
		t.Fatalf("call_id = %v", items[1]["call_id"])
	}
}

func TestTurnOutputItems_reasoningBeforeMessage(t *testing.T) {
	items := TurnOutputItems("t", TurnAccum{
		ReasoningText: "think",
		AssistantText: "answer",
	})
	if len(items) != 2 {
		t.Fatalf("items = %d", len(items))
	}
	if items[0]["type"] != "reasoning" {
		t.Fatalf("first = %v", items[0]["type"])
	}
	if items[1]["type"] != "message" {
		t.Fatalf("second = %v", items[1]["type"])
	}
}

func TestOutputItemsAfterLastUser(t *testing.T) {
	input := []interface{}{
		map[string]interface{}{"role": "user", "content": "u1"},
		map[string]interface{}{"role": "assistant", "content": "a1"},
		map[string]interface{}{"role": "user", "content": "u2"},
	}
	prior := []map[string]interface{}{{"type": "message", "role": "assistant", "content": "stale"}}
	current := []map[string]interface{}{
		{"type": "message", "role": "assistant", "content": "fresh"},
		{"type": "function_call", "name": "bash", "call_id": "c1"},
	}
	out := OutputItemsAfterLastUser(input, append(prior, current...))
	if len(out) != 2 {
		t.Fatalf("out len = %d", len(out))
	}
}

func TestResponseOutputItemAddedEvents(t *testing.T) {
	items := TurnOutputItems("tid", TurnAccum{AssistantText: "x"})
	events := ResponseOutputItemAddedEvents(items)
	if len(events) != 1 {
		t.Fatalf("events = %d", len(events))
	}
	if events[0]["type"] != "response.output_item.added" {
		t.Fatalf("type = %v", events[0]["type"])
	}
	raw, _ := json.Marshal(events[0])
	if len(raw) == 0 {
		t.Fatal("empty event json")
	}
}

func userText(item map[string]interface{}) string {
	if item == nil {
		return ""
	}
	return contentText(item["content"])
}
