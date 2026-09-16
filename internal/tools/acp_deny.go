package tools

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/dimetron/pi-go/internal/workersidecar/confined"
)

const (
	envACPLocalConfined = "PI_ACP_LOCAL_CONFINED"
	envACPDenyRules     = "PI_ACP_DENY_RULES"
)

func acpDenyRulesFromEnv() []string {
	if !acpLocalConfinedActive() {
		return nil
	}
	raw := strings.TrimSpace(os.Getenv(envACPDenyRules))
	if raw == "" {
		return confined.DefaultDenyRules
	}
	var rules []string
	if err := json.Unmarshal([]byte(raw), &rules); err != nil {
		return confined.DefaultDenyRules
	}
	return rules
}

func acpLocalConfinedActive() bool {
	v := strings.TrimSpace(strings.ToLower(os.Getenv(envACPLocalConfined)))
	switch v {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func checkACPDeny(command string) error {
	rules := acpDenyRulesFromEnv()
	if len(rules) == 0 {
		return nil
	}
	if pattern, blocked := confined.MatchDeny(command, rules); blocked {
		return fmt.Errorf(
			"BLOCKED: command matches deny rule %q (local-confined worker sidecar policy)",
			pattern,
		)
	}
	return nil
}
