package options

import (
	"os"
	"strconv"
	"strings"
)

const (
	ProgressModeMinimal = "minimal"
	ProgressModeTools   = "tools"
	ProgressModeOff     = "off"

	envProgressMode         = "ACP_PROGRESS_MODE"
	envCheckpointEverySteps = "ACP_CHECKPOINT_EVERY_STEPS"
)

// SidecarOptions mirrors Hermes SubAgentRuntimeOptions / CLI flags for a sidecar process.
type SidecarOptions struct {
	ProgressMode          string
	CheckpointEverySteps  int
	ProgressIntervalSec   float64
	StatelessToolsets     []string
	StateRoot             string
	LocalConfined         bool
	LocalConfinedExtraDeny []string
}

// ParseSidecarOptions builds options from CLI flag values with env fallbacks.
func ParseSidecarOptions(progressMode string, checkpointEvery int, progressInterval float64, statelessToolsets string, stateRoot string, localConfined bool, extraDeny string) SidecarOptions {
	o := SidecarOptions{
		ProgressMode:         strings.TrimSpace(progressMode),
		CheckpointEverySteps: checkpointEvery,
		ProgressIntervalSec:  progressInterval,
		StateRoot:            strings.TrimSpace(stateRoot),
		LocalConfined:        localConfined,
	}
	if o.ProgressMode == "" {
		o.ProgressMode = strings.TrimSpace(os.Getenv(envProgressMode))
	}
	if o.CheckpointEverySteps == 0 {
		if v := strings.TrimSpace(os.Getenv(envCheckpointEverySteps)); v != "" {
			if n, err := strconv.Atoi(v); err == nil {
				o.CheckpointEverySteps = n
			}
		}
	}
	if raw := strings.TrimSpace(statelessToolsets); raw != "" {
		o.StatelessToolsets = splitCSV(raw)
	}
	if raw := strings.TrimSpace(extraDeny); raw != "" {
		o.LocalConfinedExtraDeny = splitCSV(raw)
	}
	return o
}

func (o SidecarOptions) ProgressEnabled() bool {
	return strings.ToLower(o.ProgressMode) != ProgressModeOff
}

func splitCSV(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
