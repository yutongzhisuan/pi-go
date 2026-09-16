package backend

import (
	"fmt"
	"strings"
	"time"

	"github.com/dimetron/pi-go/internal/workersidecar"
)

const summaryMaxLen = 500

// mergeAssistantText prefers streamed deltas but falls back to the child pi
// process accumulated stdout result (available after Wait).
func mergeAssistantText(streamed, fromProcess string) string {
	streamed = strings.TrimSpace(streamed)
	if streamed != "" {
		return streamed
	}
	return strings.TrimSpace(fromProcess)
}

// summaryForCompletion uses assistant text when present; duration is only a fallback.
func summaryForCompletion(assistantText string, duration time.Duration) string {
	text := strings.TrimSpace(assistantText)
	if text == "" {
		return fmt.Sprintf("Completed in %v", duration.Round(time.Second))
	}
	if len(text) > summaryMaxLen {
		return text[:summaryMaxLen]
	}
	return text
}

func completedRunResult(assistantText string, duration time.Duration, checkpoint *workersidecar.CheckpointInfo) workersidecar.RunResult {
	text := strings.TrimSpace(assistantText)
	return workersidecar.RunResult{
		Status:     "completed",
		Summary:    summaryForCompletion(text, duration),
		ResultText: text,
		Usage:      &workersidecar.UsageInfo{},
		Checkpoint: checkpoint,
	}
}
