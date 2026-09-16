package tools

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/dimetron/pi-go/internal/workersidecar/confined"
)

const envACPDenyRules = "PI_ACP_DENY_RULES"

func acpDenyRulesFromEnv() []string {
	raw := strings.TrimSpace(os.Getenv(envACPDenyRules))
	if raw == "" {
		return nil
	}
	var rules []string
	if err := json.Unmarshal([]byte(raw), &rules); err != nil {
		return nil
	}
	return rules
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
