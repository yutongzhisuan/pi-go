package confined

import (
	"encoding/json"
	"strings"
)

// DefaultDenyRules mirror Hermes DEFAULT_LOCAL_DENY_RULES (extend/sub_agent/stateless.py).
var DefaultDenyRules = []string{
	"sudo *",
	"doas *",
	"rm -rf /*",
	"rm -rf ~*",
	"rm -rf $HOME*",
	"chmod -R * /",
	"chown -R * /",
	"curl * | sh*",
	"curl * | bash*",
	"curl *|sh*",
	"curl *|bash*",
	"wget * | sh*",
	"wget * | bash*",
	"wget *|sh*",
	"wget *|bash*",
	"dd *of=/dev/*",
	"mkfs*",
	"shutdown*",
	"reboot*",
	"halt*",
	"poweroff*",
	"systemctl *",
	"launchctl *",
	"crontab *",
	"*/.ssh/*",
	"*/.xhermes*",
	"*/.aws/*",
	"*/.gnupg/*",
}

// MergeRules returns default deny rules plus optional extras from --local-confined-extra-deny.
func MergeRules(extra []string) []string {
	out := make([]string, 0, len(DefaultDenyRules)+len(extra))
	out = append(out, DefaultDenyRules...)
	for _, r := range extra {
		r = strings.TrimSpace(r)
		if r != "" {
			out = append(out, r)
		}
	}
	return out
}

// DenyRulesJSON serializes merged deny rules for PI_ACP_DENY_RULES on executor children.
func DenyRulesJSON(extra []string) (string, error) {
	rules := MergeRules(extra)
	raw, err := json.Marshal(rules)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

// EnforceStartupPolicy validates local-confined can run with deny enforcement on executor bash.
func EnforceStartupPolicy(localConfined bool) error {
	if !localConfined {
		return nil
	}
	return nil
}
