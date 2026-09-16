package masterplanner

import (
	"os"
	"strings"
)

const envDelegatedChild = "PI_DELEGATED_CHILD"

// DelegationRefusalJSON returns a Hermes-shaped refusal payload when running
// inside a delegate_task child context, or empty when allowed.
// pi-go sets PI_DELEGATED_CHILD=1 on delegate_task children when that flow lands.
func DelegationRefusalJSON() string {
	v := strings.TrimSpace(strings.ToLower(os.Getenv(envDelegatedChild)))
	switch v {
	case "1", "true", "yes", "on":
		return delegationRefusalPayload
	default:
		return ""
	}
}

const delegationRefusalPayload = `{"error":"delegated_child_refused","message":"gateway_* tools are refused inside a delegate_task child agent. Only the main planner agent may dispatch/watch/cancel platform tasks — return your findings to the parent planner and let it decide whether to dispatch."}`

// SetDelegatedChild marks the current process as a delegate_task child (tests / future delegate wiring).
func SetDelegatedChild(on bool) {
	if on {
		_ = os.Setenv(envDelegatedChild, "1")
		return
	}
	_ = os.Unsetenv(envDelegatedChild)
}
