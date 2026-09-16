package backend

import (
	"testing"

	"github.com/dimetron/pi-go/internal/workersidecar"
)

func TestMergeAssistantTextPrefersStreamed(t *testing.T) {
	got := mergeAssistantText("OK", "ignored")
	if got != "OK" {
		t.Fatalf("got %q want OK", got)
	}
}

func TestMergeAssistantTextFallsBackToProcessResult(t *testing.T) {
	got := mergeAssistantText("", "OK")
	if got != "OK" {
		t.Fatalf("got %q want OK", got)
	}
}

func TestSummaryForCompletionPrefersAssistantText(t *testing.T) {
	got := summaryForCompletion("OK")
	if got != "OK" {
		t.Fatalf("summary = %q want OK", got)
	}
}

func TestSummaryForCompletionEmptyWhenNoText(t *testing.T) {
	if got := summaryForCompletion(""); got != "" {
		t.Fatalf("summary = %q want empty", got)
	}
}

func TestCompletedRunResultMapsAssistantText(t *testing.T) {
	out := completedRunResult("OK", nil)
	if out.ResultText != "OK" || out.Summary != "OK" {
		t.Fatalf("result=%+v", out)
	}
	if out.Status != "completed" {
		t.Fatalf("status=%q", out.Status)
	}
}

func TestCompletedRunResultFailsWhenAssistantTextEmpty(t *testing.T) {
	out := completedRunResult("", nil)
	if out.Status != "failed" {
		t.Fatalf("status = %q want failed", out.Status)
	}
	if out.ErrorCode != "empty_assistant_output" {
		t.Fatalf("error_code = %q", out.ErrorCode)
	}
	if out.Summary == "Completed in 0s" {
		t.Fatal("must not emit duration-only summary when assistant text is missing")
	}
}

func TestSummaryForCompletionTruncatesLongText(t *testing.T) {
	long := make([]byte, 600)
	for i := range long {
		long[i] = 'a'
	}
	got := summaryForCompletion(string(long))
	if len(got) != summaryMaxLen {
		t.Fatalf("len=%d want %d", len(got), summaryMaxLen)
	}
}

func TestCompletedRunResultUsagePresent(t *testing.T) {
	out := completedRunResult("x", &workersidecar.CheckpointInfo{CheckpointID: "cp-1"})
	if out.Usage == nil {
		t.Fatal("expected usage block")
	}
	if out.Checkpoint == nil || out.Checkpoint.CheckpointID != "cp-1" {
		t.Fatalf("checkpoint=%+v", out.Checkpoint)
	}
}
