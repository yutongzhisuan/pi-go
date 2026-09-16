package confined

import (
	"fmt"
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

// EnforceStartupPolicy returns an error when local-confined cannot be enforced yet.
// pi-go bash does not yet implement Hermes-style approvals.deny glob matching on the sidecar path.
func EnforceStartupPolicy(localConfined bool) error {
	if !localConfined {
		return nil
	}
	return fmt.Errorf("--local-confined requires approval deny enforcement (not available on pi-go worker sidecar yet); use --sandbox docker for untrusted tasks or run without --local-confined")
}
