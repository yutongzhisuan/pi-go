package confined

import (
	"regexp"
	"strings"
)

// cmdPos anchors patterns to shell command positions (Hermes approval.py _CMDPOS subset).
const cmdPos = `(?:^|[;&|\n` + "`" + `]|\$\()\s*(?:sudo\s+(?:-[^\s]+\s+)*)?(?:env\s+(?:\w+=\S*\s+)*)?(?:(?:exec|nohup|setsid|time)\s+)*\s*`

var hardlinePatterns = []struct {
	re   *regexp.Regexp
	desc string
}{
	{regexp.MustCompile(cmdPos + `rm\s+(-[^\s]*\s+)*` + rmHardlinePath(`/(?:(?:\.\.?)?/)*(?:\.\.?)?\**`)), "recursive delete of root filesystem"},
	{regexp.MustCompile(cmdPos + `rm\s+(-[^\s]*\s+)*` + rmHardlinePath(`(?:/home|/root|/etc|/usr|/var|/bin|/sbin|/boot|/lib)(?:/\*|\*)?`)), "recursive delete of system directory"},
	{regexp.MustCompile(cmdPos + `rm\s+(-[^\s]*\s+)*` + rmHardlinePath(`(?:~|\$\{?HOME\}?)(?:/?|/\*)?`)), "recursive delete of home directory"},
	{regexp.MustCompile(`\bmkfs(\.[a-z0-9]+)?\b`), "format filesystem (mkfs)"},
	{regexp.MustCompile(`\bdd\b[^\n]*\bof=/dev/(sd|nvme|hd|mmcblk|vd|xvd)[a-z0-9]*`), "dd to raw block device"},
	{regexp.MustCompile(`>\s*/dev/(sd|nvme|hd|mmcblk|vd|xvd)[a-z0-9]*\b`), "redirect to raw block device"},
	{regexp.MustCompile(`:\(\)\s*\{\s*:\s*\|\s*:\s*&\s*\}\s*;\s*:`), "fork bomb"},
	{regexp.MustCompile(`\bkill\s+(-[^\s]+\s+)*-1\b`), "kill all processes"},
	{regexp.MustCompile(cmdPos + `(shutdown|reboot|halt|poweroff)\b`), "system shutdown/reboot"},
	{regexp.MustCompile(cmdPos + `init\s+[06]\b`), "init 0/6 (shutdown/reboot)"},
	{regexp.MustCompile(cmdPos + `systemctl\s+(poweroff|reboot|halt|kexec)\b`), "systemctl poweroff/reboot"},
	{regexp.MustCompile(cmdPos + `telinit\s+[06]\b`), "telinit 0/6 (shutdown/reboot)"},
}

func rmHardlinePath(pathAlt string) string {
	tail := `(?:\s|$|[)` + "`" + `;|&])`
	return `(?:["'](?:` + pathAlt + `)["']|(?:` + pathAlt + `)` + tail + `)`
}

// MatchHardline returns a description when command matches portable Hermes hardline rules.
func MatchHardline(command string) (description string, blocked bool) {
	if strings.TrimSpace(command) == "" {
		return "", false
	}
	for _, variant := range commandDetectionVariants(command) {
		candidate := strings.ToLower(strings.TrimSpace(variant))
		for _, hp := range hardlinePatterns {
			if hp.re.MatchString(candidate) {
				return hp.desc, true
			}
		}
	}
	return "", false
}
