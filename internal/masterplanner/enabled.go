package masterplanner

import (
	"os"
	"strings"
)

const envEnable = "PI_MASTER_PLANNER"

// Enabled reports whether gateway_* master planner tools and prompt should load.
// Set PI_MASTER_PLANNER=1|true|yes or pass --master-planner on the pi CLI.
func Enabled() bool {
	if cliEnabled {
		return true
	}
	v := strings.TrimSpace(strings.ToLower(os.Getenv(envEnable)))
	switch v {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

var cliEnabled bool

// SetCLIEnabled is called from the pi CLI when --master-planner is passed.
func SetCLIEnabled(on bool) {
	cliEnabled = on
}
