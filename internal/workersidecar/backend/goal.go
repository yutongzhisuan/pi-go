package backend

import (
	"encoding/json"

	"github.com/dimetron/pi-go/internal/workersidecar"
	"github.com/dimetron/pi-go/internal/workersidecar/responses"
)

// buildExecutorRun derives the child goal and optional replay env from run params.
func buildExecutorRun(params workersidecar.RunParams, parsed responses.ParsedEnvelope) (goal string, extraEnv []string) {
	goal = buildGoal(params)
	if !parsed.Present || parsed.ErrorCode != "" {
		return goal, nil
	}
	goal = parsed.UserMessage
	if parsed.Replay.Structured && len(parsed.Replay.History) > 0 {
		raw, err := json.Marshal(parsed.Replay.History)
		if err == nil && len(raw) > 0 && string(raw) != "null" {
			extraEnv = append(extraEnv, envACPReplayHistory+"="+string(raw))
		}
	}
	return goal, extraEnv
}

const envACPReplayHistory = "PI_ACP_REPLAY_HISTORY"
