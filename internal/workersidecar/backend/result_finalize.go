package backend

import (
	"strings"

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

const emptyAssistantSummary = "Executor completed without assistant text"

// summaryForCompletion uses assistant text when present.
func summaryForCompletion(assistantText string) string {
	text := strings.TrimSpace(assistantText)
	if text == "" {
		return ""
	}
	if len(text) > summaryMaxLen {
		return text[:summaryMaxLen]
	}
	return text
}

func completedRunResult(assistantText string, checkpoint *workersidecar.CheckpointInfo) workersidecar.RunResult {
	text := strings.TrimSpace(assistantText)
	if text == "" {
		return workersidecar.RunResult{
			Status:     "failed",
			Summary:    emptyAssistantSummary,
			Error:      "executor produced no assistant text",
			ErrorCode:  "empty_assistant_output",
			Checkpoint: checkpoint,
		}
	}
	return workersidecar.RunResult{
		Status:     "completed",
		Summary:    summaryForCompletion(text),
		ResultText: text,
		Usage:      &workersidecar.UsageInfo{},
		Checkpoint: checkpoint,
	}
}
