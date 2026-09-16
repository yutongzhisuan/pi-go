package cli

import (
	"os"
	"strings"

	adktool "google.golang.org/adk/v2/tool"

	"github.com/dimetron/pi-go/internal/tools"
)

const (
	envACPExecutor         = "PI_ACP_EXECUTOR"
	envACPResolvedToolsets = "PI_ACP_RESOLVED_TOOLSETS"
	envACPStateless        = "PI_ACP_STATELESS"
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
