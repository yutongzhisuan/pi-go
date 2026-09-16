package backend

import (
	"testing"

	"github.com/dimetron/pi-go/internal/workersidecar"
	"github.com/dimetron/pi-go/internal/workersidecar/responses"
)

func TestBuildExecutorRun_replayHistoryEnv(t *testing.T) {
	params := workersidecar.RunParams{Goal: "fallback"}
	parsed := responses.ParseEnvelope(map[string]interface{}{
		responses.EnvelopeKey: `{"request":{"input":[
			{"role":"user","content":"first"},
			{"role":"assistant","content":"ok"},
			{"role":"user","content":"second"}
		],"instructions":"sys"}}`,
	}, params.Goal, "m")

	goal, extra := buildExecutorRun(params, parsed)
	if goal != "sys\n\nsecond" {
		t.Fatalf("goal = %q", goal)
	}
	if len(extra) != 1 {
		t.Fatalf("extra env = %v", extra)
	}
	if extra[0] == envACPReplayHistory+"=[]" {
		t.Fatalf("unexpected empty history env")
	}
}

func TestBuildExecutorRun_noReplayWhenFlatten(t *testing.T) {
	parsed := responses.ParseEnvelope(map[string]interface{}{
		responses.EnvelopeKey: `{"request":{"input":[
			{"role":"user","content":"u"},
			{"role":"assistant","content":"a"}
		]}}`,
	}, "g", "m")
	_, extra := buildExecutorRun(workersidecar.RunParams{}, parsed)
	if len(extra) != 0 {
		t.Fatalf("expected no replay env, got %v", extra)
	}
}
