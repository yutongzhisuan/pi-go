package tools

import (
	"os"
	"strconv"
	"strings"

	"google.golang.org/adk/v2/tool"
)

const (
	// EnvDelegatedChild marks a pi process spawned as a delegate_task child.
	EnvDelegatedChild = "PI_DELEGATED_CHILD"
	// EnvDelegateMaxConcurrent overrides the per-call delegate_task fan-out limit.
	EnvDelegateMaxConcurrent = "PI_DELEGATE_MAX_CONCURRENT"
)

const defaultDelegateMaxConcurrent = 3

// DefaultDelegateAgent is the bundled subagent type used for Hermes-style delegation.
const DefaultDelegateAgent = "worker"

// delegateChildBlocklistExact is the Hermes PR2 blocklist (gateway_* handled by prefix).
var delegateChildBlocklistExact = map[string]bool{
	"delegate_task": true,
	"clarify":       true,
	"memory":        true,
	"mem-search":    true,
	"mem-timeline":  true,
	"mem-get":       true,
	"send_message":  true,
	"cronjob":       true,
}

// IsDelegateChild reports whether this process is a delegate_task child.
func IsDelegateChild() bool {
	v := strings.TrimSpace(strings.ToLower(os.Getenv(EnvDelegatedChild)))
	switch v {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

// DelegateChildEnv returns env entries injected into delegate_task child processes.
func DelegateChildEnv() []string {
	return []string{EnvDelegatedChild + "=1"}
}

// MaxConcurrentDelegateChildren returns the Hermes max_concurrent_children limit.
func MaxConcurrentDelegateChildren() int {
	raw := strings.TrimSpace(os.Getenv(EnvDelegateMaxConcurrent))
	if raw == "" {
		return defaultDelegateMaxConcurrent
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 1 {
		return defaultDelegateMaxConcurrent
	}
	return n
}

// IsDelegateBlockedTool reports whether a tool must not be exposed to delegate children.
func IsDelegateBlockedTool(name string) bool {
	name = strings.TrimSpace(name)
	if name == "" {
		return false
	}
	if strings.HasPrefix(name, "gateway_") {
		return true
	}
	return delegateChildBlocklistExact[name]
}

// FilterToolsForDelegateChild removes blocklisted tools when PI_DELEGATED_CHILD is set.
func FilterToolsForDelegateChild(all []tool.Tool) []tool.Tool {
	if !IsDelegateChild() {
		return all
	}
	out := make([]tool.Tool, 0, len(all))
	for _, t := range all {
		if t == nil {
			continue
		}
		if IsDelegateBlockedTool(t.Name()) {
			continue
		}
		out = append(out, t)
	}
	return out
}

// AdjustToolsForDelegateChild applies delegate-child tool filtering in-process.
func AdjustToolsForDelegateChild(all []tool.Tool) []tool.Tool {
	return FilterToolsForDelegateChild(all)
}
