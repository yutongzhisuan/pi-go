package backend

import (
	"context"
	"testing"

	"github.com/dimetron/pi-go/internal/workersidecar"
)

func TestPrepareWorkDir(t *testing.T) {
	b := New(Config{
		WorkRoot:  t.TempDir(),
		Stateless: true,
	})

	workDir, cleanup, err := b.prepareWorkDir("test-run-123")
	if err != nil {
		t.Fatalf("prepareWorkDir() failed: %v", err)
	}
	if cleanup != nil {
		defer cleanup()
	}

	if workDir == "" {
		t.Error("prepareWorkDir() returned empty path")
	}
}

func TestRunSession_MissingGoal(t *testing.T) {
	b := New(Config{
		WorkRoot:  t.TempDir(),
		Stateless: true,
	})

	params := workersidecar.RunParams{
		RunID:          "test-run",
		TimeoutSeconds: 1,
	}

	result := b.RunSession(context.Background(), params)
	if result.Status == "" {
		t.Error("RunSession() should return a status")
	}
}

func TestBackendConfig(t *testing.T) {
	cfg := Config{
		WorkRoot:  "/tmp/test",
		Stateless: true,
	}

	b := New(cfg)
	if b == nil {
		t.Fatal("New() returned nil")
	}

	if b.workRoot != cfg.WorkRoot {
		t.Errorf("Backend.workRoot = %q, want %q", b.workRoot, cfg.WorkRoot)
	}

	if b.cfg.Stateless != cfg.Stateless {
		t.Errorf("Backend.cfg.Stateless = %v, want %v", b.cfg.Stateless, cfg.Stateless)
	}
}
