package backend

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/dimetron/pi-go/internal/subagent"
	"github.com/dimetron/pi-go/internal/workersidecar"
)

// Backend manages pi-go agent sessions for Worker sidecar runs.
type Backend struct {
	spawner  *subagent.Spawner
	workRoot string
	stateless bool
	progressCallback ProgressCallback
	checkpointCallback CheckpointCallback
}

// ProgressCallback is called when progress is reported.
type ProgressCallback func(runID, summary string)

// CheckpointCallback is called when a checkpoint is created.
type CheckpointCallback func(runID string, checkpoint *workersidecar.CheckpointInfo)

// Config holds configuration for the backend.
type Config struct {
	WorkRoot           string
	Stateless          bool
	ProgressCallback   ProgressCallback
	CheckpointCallback CheckpointCallback
}

// New creates a new Backend with the given configuration.
func New(cfg Config) *Backend {
	if cfg.WorkRoot == "" {
		cfg.WorkRoot = os.TempDir()
	}
	return &Backend{
		spawner:            subagent.NewSpawner(""),
		workRoot:           cfg.WorkRoot,
		stateless:          cfg.Stateless,
		progressCallback:   cfg.ProgressCallback,
		checkpointCallback: cfg.CheckpointCallback,
	}
}

// RunSession executes a pi-go agent session and returns Hermes-shaped results.
func (b *Backend) RunSession(ctx context.Context, params workersidecar.RunParams) workersidecar.RunResult {
	workDir, err := b.prepareWorkDir(params.RunID)
	if err != nil {
		return workersidecar.RunResult{
			Status: "failed",
			Error:  fmt.Sprintf("failed to prepare workdir: %v", err),
		}
	}

	if b.stateless {
		defer os.RemoveAll(workDir)
	}

	goal := params.Goal
	if goal == "" {
		goal = "Complete the task"
	}

	timeout := params.TimeoutSeconds
	if timeout == 0 {
		timeout = 600
	}

	opts := subagent.SpawnOpts{
		AgentID:     params.RunID,
		Model:       params.Model,
		WorkDir:     workDir,
		Prompt:      goal,
		Instruction: "",
		Timeout:     timeout * 1000,
	}

	proc, err := b.spawner.Spawn(ctx, opts)
	if err != nil {
		return workersidecar.RunResult{
			Status: "failed",
			Error:  fmt.Sprintf("failed to spawn agent: %v", err),
		}
	}

	return b.collectResults(params.RunID, proc)
}

// prepareWorkDir creates a disposable working directory for a run.
func (b *Backend) prepareWorkDir(runID string) (string, error) {
	workDir := filepath.Join(b.workRoot, "acp-runs", runID)
	if err := os.MkdirAll(workDir, 0o755); err != nil {
		return "", err
	}
	return workDir, nil
}

// collectResults consumes events from a pi process and builds the result.
func (b *Backend) collectResults(runID string, proc *subagent.Process) workersidecar.RunResult {
	var resultText strings.Builder
	var lastCheckpoint *workersidecar.CheckpointInfo
	startTime := time.Now()

	for ev := range proc.Events() {
		switch ev.Type {
		case "text_delta":
			resultText.WriteString(ev.Content)
			if b.progressCallback != nil && ev.Content != "" {
				b.progressCallback(runID, strings.TrimSpace(ev.Content))
			}

		case "error":
			return workersidecar.RunResult{
				Status:     "failed",
				Summary:    "Agent encountered an error",
				ResultText: resultText.String(),
				Error:      ev.Error,
			}

		case "message_end":
			duration := time.Since(startTime)
			return workersidecar.RunResult{
				Status:     "completed",
				Summary:    fmt.Sprintf("Completed in %v", duration.Round(time.Second)),
				ResultText: resultText.String(),
				Usage: &workersidecar.UsageInfo{
					TotalTokens: 0,
				},
				Checkpoint: lastCheckpoint,
			}
		}
	}

	finalResult, err := proc.Wait()
	if err != nil {
		return workersidecar.RunResult{
			Status:     "failed",
			Summary:    "Agent failed",
			ResultText: resultText.String(),
			Error:      err.Error(),
		}
	}

	if finalResult != "" {
		resultText.WriteString(finalResult)
	}

	return workersidecar.RunResult{
		Status:     "completed",
		Summary:    "Task completed",
		ResultText: resultText.String(),
		Checkpoint: lastCheckpoint,
	}
}

// Cancel attempts to cancel a running session.
func (b *Backend) Cancel(runID string, proc *subagent.Process) error {
	if proc == nil {
		return fmt.Errorf("no process found for run %s", runID)
	}
	proc.Cancel()
	return nil
}

// PiBinaryPath returns the path to the pi binary used for spawning.
func (b *Backend) PiBinaryPath() (string, error) {
	return exec.LookPath("pi")
}
