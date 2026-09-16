package cli

import (
	"encoding/json"
	"os"
	"strings"

	adktool "google.golang.org/adk/v2/tool"

	"github.com/dimetron/pi-go/internal/tools"
	"github.com/dimetron/pi-go/internal/workersidecar/responses"
)

const (
	envACPExecutor         = "PI_ACP_EXECUTOR"
	envACPResolvedToolsets = "PI_ACP_RESOLVED_TOOLSETS"
	envACPStateless        = "PI_ACP_STATELESS"
	envACPReplayHistory    = "PI_ACP_REPLAY_HISTORY"
)

func acpExecutorActive() bool {
	v := strings.TrimSpace(strings.ToLower(os.Getenv(envACPExecutor)))
	switch v {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func acpStatelessChild() bool {
	v := strings.TrimSpace(strings.ToLower(os.Getenv(envACPStateless)))
	switch v {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

// adjustToolsForACPExecutor applies worker-sidecar child constraints when spawned via acp.run.
func adjustToolsForACPExecutor(coreTools []adktool.Tool) []adktool.Tool {
	if !acpExecutorActive() {
		return coreTools
	}
	raw := os.Getenv(envACPResolvedToolsets)
	if raw == "" {
		return nil
	}
	return tools.FilterByExecutorToolsets(coreTools, tools.ParseResolvedExecutorToolsets(raw))
}

// applyACPReplayHistory prepends structured replay context for Responses L2 executor children.
func applyACPReplayHistory(prompt string) string {
	if !acpExecutorActive() {
		return prompt
	}
	raw := strings.TrimSpace(os.Getenv(envACPReplayHistory))
	if raw == "" {
		return prompt
	}
	var history []map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &history); err != nil || len(history) == 0 {
		return prompt
	}
	block := responses.FormatReplayHistoryBlock(history)
	if block == "" {
		return prompt
	}
	return "[Prior conversation context]\n" + block + "\n\n[Current task]\n" + prompt
}
