package backend

import (
	"context"
	"testing"

	"github.com/dimetron/pi-go/internal/subagent"
	"github.com/dimetron/pi-go/internal/workersidecar/options"
)

// TestCollectResultsMergesProcessResult verifies that when the event stream
// delivers message_end without text_delta (e.g. dropped deltas), collectResults
// still maps the child Wait() accumulated stdout into result_text/summary.
func TestCollectResultsMergesProcessResult(t *testing.T) {
	proc := subagent.NewFixedProcess(
		[]subagent.Event{{Type: "message_end"}},
		"OK",
		nil,
	)

	b := New(Config{
		Sidecar: options.SidecarOptions{},
	})
	result := b.collectResults(context.Background(), "run-1", "task-1", proc)
	if result.ResultText != "OK" {
		t.Fatalf("result_text = %q want OK", result.ResultText)
	}
	if result.Summary != "OK" {
		t.Fatalf("summary = %q want OK", result.Summary)
	}
}

func TestCollectResultsStreamedTextWinsOverWait(t *testing.T) {
	proc := subagent.NewFixedProcess(
		[]subagent.Event{
			{Type: "text_delta", Content: "streamed"},
			{Type: "message_end"},
		},
		"from-wait",
		nil,
	)
	b := New(Config{Sidecar: options.SidecarOptions{}})
	result := b.collectResults(context.Background(), "run-1", "task-1", proc)
	if result.ResultText != "streamed" {
		t.Fatalf("result_text = %q want streamed", result.ResultText)
	}
}

func TestCollectResultsFailsWhenNoAssistantText(t *testing.T) {
	proc := subagent.NewFixedProcess(
		[]subagent.Event{{Type: "message_start"}, {Type: "message_end"}},
		"",
		nil,
	)
	b := New(Config{Sidecar: options.SidecarOptions{}})
	result := b.collectResults(context.Background(), "run-empty", "task-1", proc)
	if result.Status != "failed" || result.ErrorCode != "empty_assistant_output" {
		t.Fatalf("result = %+v", result)
	}
}
