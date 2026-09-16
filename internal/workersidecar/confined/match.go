package confined

import (
	"regexp"
	"strings"
)

// MatchDeny returns the first matching deny glob for command (Hermes approvals.deny subset).
// Matching is case-insensitive; patterns use * and ? like Python fnmatch.
func MatchDeny(command string, rules []string) (pattern string, blocked bool) {
	if command == "" || len(rules) == 0 {
		return "", false
	}
	for _, variant := range commandVariants(command) {
		candidate := strings.ToLower(strings.TrimSpace(variant))
		if candidate == "" {
			continue
		}
		for _, rule := range rules {
			rule = strings.TrimSpace(rule)
			if rule == "" {
				continue
			}
			re, err := globToRegexp(rule)
			if err != nil {
				continue
			}
			if re.MatchString(candidate) {
				return rule, true
			}
		}
	}
	return "", false
}

func commandVariants(command string) []string {
	trimmed := strings.TrimSpace(command)
	collapsed := strings.Join(strings.Fields(trimmed), " ")
	if collapsed == trimmed {
		return []string{trimmed}
	}
	return []string{trimmed, collapsed}
}

func globToRegexp(pattern string) (*regexp.Regexp, error) {
	var b strings.Builder
	b.WriteString("(?i)^")
	for i := 0; i < len(pattern); i++ {
		c := pattern[i]
		switch c {
		case '*':
			b.WriteString(".*")
		case '?':
			b.WriteString(".")
		case '.', '+', '(', ')', '|', '^', '$', '[', ']', '{', '}', '\\':
			b.WriteString(`\`)
			b.WriteByte(c)
		default:
			b.WriteByte(c)
		}
	}
	b.WriteString("$")
	return regexp.Compile(b.String())
}
