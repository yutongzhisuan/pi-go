package backend

import (
	"strings"
	"testing"

	"github.com/dimetron/pi-go/internal/workersidecar"
)

func TestBuildGoalResumeBlob(t *testing.T) {
	got := buildGoal(workersidecar.RunParams{
		ResumeFromCheckpoint: "cp-1",
		ResumeBlob:           "partial state",
		Goal:                   "finish",
	})
	if !strings.Contains(got, "cp-1") || !strings.Contains(got, "partial state") || !strings.Contains(got, "finish") {
		t.Fatalf("unexpected goal: %q", got)
	}
}
