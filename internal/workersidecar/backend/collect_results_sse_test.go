package backend

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/dimetron/pi-go/internal/subagent"
	"github.com/dimetron/pi-go/internal/workersidecar/options"
	"github.com/dimetron/pi-go/internal/workersidecar/responses"
)

func TestCollectResults_emitsResponseOutputItems(t *testing.T) {
	var events []map[string]interface{}
	proc := subagent.NewFixedProcess(
		[]subagent.Event{
			{Type: "text_delta", Content: "hello"},
			{Type: "tool_call", Content: "bash", ToolArgs: map[string]interface{}{"cmd": "ls"}},
			{Type: "message_end"},
		},
		"hello",
		nil,
	)
	b := New(Config{
		Sidecar: options.SidecarOptions{},
		ResponseEventCallback: func(_ string, event map[string]interface{}) {
			events = append(events, event)
		},
	})
	parsed := responses.ParsedEnvelope{Present: true, RawInput: []interface{}{
		map[string]interface{}{"role": "user", "content": "go"},
	}}
	_ = b.collectResults(context.Background(), "run-1", "task-1", proc, parsed)
	if len(events) == 0 {
		t.Fatal("expected response output_item events")
	}
	foundMessage := false
	foundTool := false
	for _, ev := range events {
		if ev["type"] != "response.output_item.added" {
			continue
		}
		item, _ := ev["item"].(map[string]interface{})
		if item == nil {
			continue
		}
		switch item["type"] {
		case "message":
			foundMessage = true
		case "function_call":
			foundTool = true
		}
	}
	if !foundMessage || !foundTool {
		t.Fatalf("events = %s", mustJSON(events))
	}
}

func mustJSON(v any) string {
	raw, _ := json.Marshal(v)
	return string(raw)
}
