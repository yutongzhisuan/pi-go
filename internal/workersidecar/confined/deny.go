package confined

import (
	"regexp"
	"strings"

	"golang.org/x/text/unicode/norm"
)

var (
	reLineContinuation = regexp.MustCompile(`\\\r?\n`)
	reBackslashEscape  = regexp.MustCompile(`\\([^\n])`)
	reEmptyQuotes      = regexp.MustCompile(`''|""`)
	reIFS              = regexp.MustCompile(`\$\{IFS\b[^}]*\}|\$IFS\b`)
)

// MatchDeny returns the first matching deny glob for command (Hermes approvals.deny subset).
// Matching is case-insensitive fnmatch over normalized command variants.
func MatchDeny(command string, rules []string) (pattern string, blocked bool) {
	if command == "" || len(rules) == 0 {
		return "", false
	}
	globs := make([]string, 0, len(rules))
	for _, p := range rules {
		p = strings.TrimSpace(p)
		if p != "" {
			globs = append(globs, strings.ToLower(p))
		}
	}
	if len(globs) == 0 {
		return "", false
	}
	for _, variant := range commandDetectionVariants(command) {
		candidate := strings.ToLower(strings.TrimSpace(variant))
		for i, g := range globs {
			if fnmatchCase(g, candidate) {
				return rules[i], true
			}
		}
	}
	return "", false
}

func commandDetectionVariants(command string) []string {
	normalized := normalizeCommandForDetection(command)
	seen := map[string]struct{}{normalized: {}}
	out := []string{normalized}
	for _, part := range splitCommandSegments(normalized) {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if _, ok := seen[part]; ok {
			continue
		}
		seen[part] = struct{}{}
		out = append(out, part)
	}
	return out
}

func splitCommandSegments(command string) []string {
	seps := func(r rune) bool {
		return r == ';' || r == '|' || r == '&'
	}
	parts := strings.FieldsFunc(command, seps)
	if len(parts) <= 1 {
		return nil
	}
	return parts
}

func fnmatchCase(pattern, s string) bool {
	return fnmatchRunes([]rune(pattern), []rune(s))
}

func fnmatchRunes(p, n []rune) bool {
	for len(p) > 0 {
		switch p[0] {
		case '*':
			if len(p) == 1 {
				return true
			}
			for i := 0; i <= len(n); i++ {
				if fnmatchRunes(p[1:], n[i:]) {
					return true
				}
			}
			return false
		case '?':
			if len(n) == 0 {
				return false
			}
			p = p[1:]
			n = n[1:]
		default:
			if len(n) == 0 || p[0] != n[0] {
				return false
			}
			p = p[1:]
			n = n[1:]
		}
	}
	return len(n) == 0
}

func normalizeCommandForDetection(command string) string {
	command = strings.ReplaceAll(command, "\x00", "")
	command = norm.NFKC.String(command)
	command = reLineContinuation.ReplaceAllString(command, "")
	command = reBackslashEscape.ReplaceAllString(command, "$1")
	command = reEmptyQuotes.ReplaceAllString(command, "")
	command = reIFS.ReplaceAllString(command, " ")
	return strings.TrimSpace(command)
}
