package tools

import (
	"os"
	"strconv"
	"strings"

	"google.golang.org/adk/v2/tool"

	"github.com/dimetron/pi-go/internal/config"
)

const (
	EnvDelegatedChild      = "PI_DELEGATED_CHILD"
	EnvDelegateMaxConcurrent = "PI_DELEGATE_MAX_CONCURRENT"
	EnvDelegateMaxSpawnDepth = "PI_DELEGATE_MAX_SPAWN_DEPTH"
	EnvDelegateOrchestrator  = "PI_DELEGATE_ORCHESTRATOR_ENABLED"
	EnvDelegateDepth         = "PI_DELEGATE_DEPTH"
	EnvDelegateRole          = "PI_DELEGATE_ROLE"
	EnvSubagentAutoApprove   = "PI_SUBAGENT_AUTO_APPROVE"
)

const (
	defaultDelegateMaxConcurrent = 3
	defaultDelegateMaxSpawnDepth = 1
)

const (
	delegateRoleLeaf         = "leaf"
	delegateRoleOrchestrator = "orchestrator"
)

// DefaultDelegateAgent is the bundled subagent type used for Hermes-style delegation.
const DefaultDelegateAgent = "worker"

var delegateChildBlocklistExact = map[string]bool{
	"clarify":      true,
	"memory":       true,
	"mem-search":   true,
	"mem-timeline": true,
	"mem-get":      true,
	"send_message": true,
	"cronjob":      true,
}

// DelegationNoticeHook receives non-fatal delegation messages (e.g. role degrade).
var DelegationNoticeHook func(string)

func delegationLog(msg string) {
	if DelegationNoticeHook != nil {
		DelegationNoticeHook(msg)
	}
}

// ResolvedDelegation is the effective delegate_task policy for this process.
type ResolvedDelegation struct {
	MaxConcurrentChildren int
	MaxSpawnDepth         int
	OrchestratorEnabled   bool
	SubagentAutoApprove   bool
}

// ResolveDelegation merges config file values with env overrides.
func ResolveDelegation(cfg *config.Config) ResolvedDelegation {
	out := ResolvedDelegation{
		MaxConcurrentChildren: defaultDelegateMaxConcurrent,
		MaxSpawnDepth:         defaultDelegateMaxSpawnDepth,
		OrchestratorEnabled:   false,
		SubagentAutoApprove:   false,
	}
	if cfg != nil && cfg.Delegation != nil {
		d := cfg.Delegation
		if d.MaxConcurrentChildren != nil && *d.MaxConcurrentChildren > 0 {
			out.MaxConcurrentChildren = *d.MaxConcurrentChildren
		}
		if d.MaxSpawnDepth != nil && *d.MaxSpawnDepth > 0 {
			out.MaxSpawnDepth = *d.MaxSpawnDepth
		}
		if d.OrchestratorEnabled != nil {
			out.OrchestratorEnabled = *d.OrchestratorEnabled
		}
		if d.SubagentAutoApprove != nil {
			out.SubagentAutoApprove = *d.SubagentAutoApprove
		}
	}
	if v := envInt(EnvDelegateMaxConcurrent); v > 0 {
		out.MaxConcurrentChildren = v
	}
	if v := envInt(EnvDelegateMaxSpawnDepth); v > 0 {
		out.MaxSpawnDepth = v
	}
	if v, ok := envBool(EnvDelegateOrchestrator); ok {
		out.OrchestratorEnabled = v
	}
	if v, ok := envBool(EnvSubagentAutoApprove); ok {
		out.SubagentAutoApprove = v
	}
	return out
}

// MaxConcurrentDelegateChildren returns the Hermes max_concurrent_children limit.
func MaxConcurrentDelegateChildren(cfg *config.Config) int {
	return ResolveDelegation(cfg).MaxConcurrentChildren
}

// SubagentAutoApprove reports whether subagent tool approvals default to allow.
func SubagentAutoApprove(cfg *config.Config) bool {
	return ResolveDelegation(cfg).SubagentAutoApprove
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

// DelegateDepth returns the delegate spawn depth of this process (0 = root planner).
func DelegateDepth() int {
	if !IsDelegateChild() {
		return 0
	}
	if d := envInt(EnvDelegateDepth); d > 0 {
		return d
	}
	return 1
}

// DelegateRole returns leaf or orchestrator for the current delegate child.
func DelegateRole() string {
	r := strings.TrimSpace(strings.ToLower(os.Getenv(EnvDelegateRole)))
	if r == delegateRoleOrchestrator {
		return delegateRoleOrchestrator
	}
	return delegateRoleLeaf
}

// CanDelegateAtCurrentDepth reports whether this agent may invoke delegate_task spawn.
func CanDelegateAtCurrentDepth(res ResolvedDelegation) bool {
	depth := DelegateDepth()
	if depth >= res.MaxSpawnDepth {
		return false
	}
	if depth >= 1 && DelegateRole() != delegateRoleOrchestrator {
		return false
	}
	return true
}

// ResolveSpawnRole picks the effective child role for a spawn at parentDepth.
func ResolveSpawnRole(requested string, parentDepth int, res ResolvedDelegation) string {
	role := strings.TrimSpace(strings.ToLower(requested))
	if role == "" {
		role = delegateRoleLeaf
	}
	if parentDepth >= 1 {
		return delegateRoleLeaf
	}
	if role != delegateRoleOrchestrator {
		return delegateRoleLeaf
	}
	if !res.OrchestratorEnabled {
		delegationLog("delegate_task: orchestrator role requested but orchestrator_enabled=false; using leaf")
		return delegateRoleLeaf
	}
	if res.MaxSpawnDepth <= 1 {
		delegationLog("delegate_task: orchestrator role degraded to leaf (max_spawn_depth=1)")
		return delegateRoleLeaf
	}
	if parentDepth+1 >= res.MaxSpawnDepth {
		return delegateRoleLeaf
	}
	return delegateRoleOrchestrator
}

// DelegateChildEnv returns env entries injected into delegate_task child processes.
func DelegateChildEnv(parentDepth int, childRole string) []string {
	childDepth := parentDepth + 1
	if childDepth < 1 {
		childDepth = 1
	}
	childRole = strings.TrimSpace(strings.ToLower(childRole))
	if childRole != delegateRoleOrchestrator {
		childRole = delegateRoleLeaf
	}
	return []string{
		EnvDelegatedChild + "=1",
		EnvDelegateDepth + "=" + strconv.Itoa(childDepth),
		EnvDelegateRole + "=" + childRole,
	}
}

// IsDelegateBlockedTool reports whether a tool must not be exposed to delegate children.
func IsDelegateBlockedTool(name string) bool {
	name = strings.TrimSpace(name)
	if name == "" {
		return false
	}
	if name == "delegate_task" {
		return !canExposeDelegateTaskToChild()
	}
	if strings.HasPrefix(name, "gateway_") {
		return true
	}
	return delegateChildBlocklistExact[name]
}

func canExposeDelegateTaskToChild() bool {
	if !IsDelegateChild() {
		return true
	}
	res := ResolveDelegation(nil)
	if DelegateDepth() >= res.MaxSpawnDepth {
		return false
	}
	return DelegateRole() == delegateRoleOrchestrator
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

func envInt(key string) int {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return 0
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return 0
	}
	return n
}

func envBool(key string) (bool, bool) {
	raw := strings.TrimSpace(strings.ToLower(os.Getenv(key)))
	switch raw {
	case "1", "true", "yes", "on":
		return true, true
	case "0", "false", "no", "off":
		return false, true
	default:
		return false, false
	}
}
