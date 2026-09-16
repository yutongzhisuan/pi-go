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

func TestSalvageCheckpointShape(t *testing.T) {
	cp := salvageCheckpoint("task-1", 3, nil, "partial output")
	if cp == nil {
		t.Fatal("expected checkpoint")
	}
	if cp.CheckpointID == "" || cp.ResumeBlob != "partial output" {
		t.Fatalf("unexpected checkpoint: %+v", cp)
	}
	if step, ok := cp.Fields["step"].(int); !ok || step != 3 {
		t.Fatalf("fields step: %+v", cp.Fields)
	}
}

func TestSalvageCheckpointFillsResumeBlob(t *testing.T) {
	last := &workersidecar.CheckpointInfo{
		CheckpointID: "cp-task-1-1",
		Summary:      "step 2 milestone",
		Fields:       map[string]interface{}{"step": 2},
	}
	cp := salvageCheckpoint("task-1", 2, last, "saved text")
	if cp.ResumeBlob != "saved text" {
		t.Fatalf("resume_blob=%q", cp.ResumeBlob)
	}
}
