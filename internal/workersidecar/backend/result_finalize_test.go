package backend

import (
	"testing"
	"time"

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
	got := summaryForCompletion("OK", time.Second)
	if got != "OK" {
		t.Fatalf("summary = %q want OK", got)
	}
}

func TestSummaryForCompletionDurationFallback(t *testing.T) {
	got := summaryForCompletion("", 2*time.Second)
	if got != "Completed in 2s" {
		t.Fatalf("summary = %q", got)
	}
}

func TestCompletedRunResultMapsAssistantText(t *testing.T) {
	out := completedRunResult("OK", time.Millisecond, nil)
	if out.ResultText != "OK" || out.Summary != "OK" {
		t.Fatalf("result=%+v", out)
	}
	if out.Status != "completed" {
		t.Fatalf("status=%q", out.Status)
	}
}

func TestCompletedRunResultNotDurationSummaryWhenTextPresent(t *testing.T) {
	out := completedRunResult("OK", 0, nil)
	if out.Summary == "Completed in 0s" {
		t.Fatal("summary must not be duration-only when assistant text exists")
	}
}

func TestSummaryForCompletionTruncatesLongText(t *testing.T) {
	long := make([]byte, 600)
	for i := range long {
		long[i] = 'a'
	}
	got := summaryForCompletion(string(long), time.Second)
	if len(got) != summaryMaxLen {
		t.Fatalf("len=%d want %d", len(got), summaryMaxLen)
	}
}

func TestCompletedRunResultUsagePresent(t *testing.T) {
	out := completedRunResult("x", time.Second, &workersidecar.CheckpointInfo{CheckpointID: "cp-1"})
	if out.Usage == nil {
		t.Fatal("expected usage block")
	}
	if out.Checkpoint == nil || out.Checkpoint.CheckpointID != "cp-1" {
		t.Fatalf("checkpoint=%+v", out.Checkpoint)
	}
}
