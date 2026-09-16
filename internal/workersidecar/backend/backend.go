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
	"github.com/dimetron/pi-go/internal/workersidecar/confined"
	"github.com/dimetron/pi-go/internal/workersidecar/options"
	"github.com/dimetron/pi-go/internal/workersidecar/responses"
	"github.com/dimetron/pi-go/internal/workersidecar/sandbox"
)

const executorPreamble = "[sub-agent executor — headless platform sub-task; no master session context]\n" +
	"Execute the goal below. Return concise, concrete findings only.\n"

// Backend manages pi-go agent sessions for Worker sidecar runs.
type Backend struct {
	spawner  *subagent.Spawner
	workRoot string
	cfg      Config
}

// ProgressCallback is called when progress is reported.
type ProgressCallback func(runID, summary string)

// CheckpointCallback is called when a checkpoint is created.
type CheckpointCallback func(runID string, checkpoint *workersidecar.CheckpointInfo)

// Config holds configuration for the backend.
type Config struct {
	WorkRoot           string
	Stateless          bool
	StateRoot          string
	RuntimeBaseURL     string
	Sandbox            *sandbox.Config
	LocalConfined      bool
	ProgressCallback   ProgressCallback
	CheckpointCallback CheckpointCallback
	OnProcess          func(runID string, proc *subagent.Process)
	Sidecar            options.SidecarOptions
}

// New creates a new Backend with the given configuration.
func New(cfg Config) *Backend {
	if cfg.WorkRoot == "" {
		cfg.WorkRoot = os.TempDir()
	}
	return &Backend{
		spawner:  subagent.NewSpawner(""),
		workRoot: cfg.WorkRoot,
		cfg:      cfg,
	}
}

// RunSession executes a pi-go agent session and returns Hermes-shaped results.
func (b *Backend) RunSession(ctx context.Context, params workersidecar.RunParams) workersidecar.RunResult {
	parsed := responses.ParseEnvelope(params.Params, params.Goal, params.Model)
	if parsed.Present && parsed.ErrorCode != "" {
		msg := "invalid responses.v1 envelope"
		return workersidecar.RunResult{
			Status:    "failed",
			Summary:   msg,
			Error:     msg,
			ErrorCode: parsed.ErrorCode,
		}
	}

	workDir, cleanupWork, err := b.prepareWorkDir(params.RunID)
	if err != nil {
		return workersidecar.RunResult{
			Status: "failed",
			Error:  fmt.Sprintf("failed to prepare workdir: %v", err),
		}
	}
	if cleanupWork != nil {
		defer cleanupWork()
	}

	var dockerSession *sandbox.Session
	if b.cfg.Sandbox != nil {
		dockerSession, err = sandbox.StartSession(ctx, *b.cfg.Sandbox, params.RunID, workDir)
		if err != nil {
			return workersidecar.RunResult{
				Status: "failed",
				Error:  fmt.Sprintf("docker sandbox: %v", err),
			}
		}
		defer dockerSession.Destroy(context.Background())
	}

	goal := buildGoal(params)
	if parsed.Present {
		goal = parsed.UserMessage
	}
	timeout := params.TimeoutSeconds
	if timeout == 0 {
		timeout = 600
	}

	env := b.executorEnv(params, params.ResolvedToolsets, dockerSession)
	opts := subagent.SpawnOpts{
		AgentID:     params.RunID,
		Model:       params.Model,
		WorkDir:     workDir,
		Prompt:      goal,
		Instruction: executorPreamble,
		Timeout:     timeout * 1000,
		Env:         env,
	}
	if b.cfg.RuntimeBaseURL != "" && params.Model != "" {
		opts.BaseURL = b.cfg.RuntimeBaseURL
	}

	proc, err := b.spawner.Spawn(ctx, opts)
	if err != nil {
		return workersidecar.RunResult{
			Status: "failed",
			Error:  fmt.Sprintf("failed to spawn agent: %v", err),
		}
	}
	if b.cfg.OnProcess != nil {
		b.cfg.OnProcess(params.RunID, proc)
	}

	taskKey := params.TaskID
	if taskKey == "" {
		taskKey = params.RunID
	}
	result := b.collectResults(ctx, params.RunID, taskKey, proc)
	return responses.WrapRunResult(result, parsed, params.TaskID, params.Model)
}

func buildGoal(params workersidecar.RunParams) string {
	goal := params.Goal
	if goal == "" {
		goal = "Complete the task"
	}
	if params.ResumeFromCheckpoint != "" {
		prefix := fmt.Sprintf("[Resuming from checkpoint %s]\n", params.ResumeFromCheckpoint)
		if blob := resumeBlobText(params); blob != "" {
			return prefix + blob + "\n" + goal
		}
		return prefix + goal
	}
	return goal
}

func resumeBlobText(params workersidecar.RunParams) string {
	if blob := strings.TrimSpace(params.ResumeBlob); blob != "" {
		return blob
	}
	if params.Params == nil {
		return ""
	}
	if rs, ok := params.Params["resume_summary"].(string); ok {
		return strings.TrimSpace(rs)
	}
	return ""
}

func (b *Backend) executorEnv(params workersidecar.RunParams, resolvedToolsets []string, dockerSession *sandbox.Session) []string {
	if !b.cfg.Stateless && len(resolvedToolsets) == 0 && b.cfg.Sandbox == nil && !b.cfg.LocalConfined {
		return nil
	}
	var env []string
	env = append(env, "PI_ACP_EXECUTOR=1")
	if b.cfg.Stateless {
		env = append(env, "PI_ACP_STATELESS=1")
	}
	if len(resolvedToolsets) > 0 {
		env = append(env, "PI_ACP_RESOLVED_TOOLSETS="+strings.Join(resolvedToolsets, ","))
	}
	if b.cfg.LocalConfined {
		extra := b.cfg.Sidecar.LocalConfinedExtraDeny
		if raw, err := confined.DenyRulesJSON(extra); err == nil {
			env = append(env, "PI_ACP_DENY_RULES="+raw)
		}
		env = append(env, "PI_ACP_LOCAL_CONFINED=1")
	}
	if params.ResumeFromCheckpoint != "" {
		env = append(env, "PI_ACP_RESUME_CHECKPOINT="+params.ResumeFromCheckpoint)
	}
	if b.cfg.Sandbox != nil {
		env = append(env, "PI_WORKER_SANDBOX_DOCKER=1")
	}
	if dockerSession != nil {
		if bin, err := sandbox.FindDocker(); err == nil {
			env = append(env, "PI_WORKER_DOCKER_BIN="+bin)
		}
		env = append(env, "PI_WORKER_DOCKER_CONTAINER="+dockerSession.ContainerID())
		env = append(env, "PI_WORKER_DOCKER_WORKDIR="+sandbox.ContainerWorkdir)
	} else if b.cfg.Sandbox != nil {
		env = append(env, b.cfg.Sandbox.EnvPairs()...)
	}
	return env
}

func (b *Backend) prepareWorkDir(runID string) (string, func(), error) {
	workDir := filepath.Join(b.workRoot, "acp-runs", runID)
	if err := os.MkdirAll(workDir, 0o755); err != nil {
		return "", nil, err
	}
	var cleanup func()
	if b.cfg.Stateless {
		cleanup = func() { _ = os.RemoveAll(workDir) }
	}
	if b.cfg.StateRoot != "" {
		stateDir := filepath.Join(b.cfg.StateRoot, runID)
		if err := os.MkdirAll(stateDir, 0o700); err != nil {
			return "", nil, err
		}
		prev := cleanup
		cleanup = func() {
			if prev != nil {
				prev()
			}
			_ = os.RemoveAll(stateDir)
		}
	}
	return workDir, cleanup, nil
}

func (b *Backend) collectResults(ctx context.Context, runID, taskKey string, proc *subagent.Process) workersidecar.RunResult {
	var resultText strings.Builder
	var lastCheckpoint *workersidecar.CheckpointInfo
	startTime := time.Now()
	stepCount := 0
	checkpointSeq := 0
	every := b.cfg.Sidecar.CheckpointEverySteps
	progressMode := strings.ToLower(b.cfg.Sidecar.ProgressMode)
	minInterval := time.Duration(b.cfg.Sidecar.ProgressIntervalSec * float64(time.Second))
	if minInterval <= 0 {
		minInterval = 5 * time.Second
	}
	var lastProgress time.Time

	emitProgress := func(summary string) {
		if b.cfg.ProgressCallback == nil || !b.cfg.Sidecar.ProgressEnabled() {
			return
		}
		if progressMode == options.ProgressModeTools {
			return
		}
		text := strings.TrimSpace(summary)
		if text == "" {
			return
		}
		now := time.Now()
		if !lastProgress.IsZero() && now.Sub(lastProgress) < minInterval {
			return
		}
		lastProgress = now
		b.cfg.ProgressCallback(runID, text)
	}

	emitToolProgress := func(name string) {
		if progressMode != options.ProgressModeTools {
			return
		}
		emitProgress("tool: " + name)
	}

	maybeCheckpoint := func(step int) {
		if every <= 0 || step <= 0 || step%every != 0 {
			return
		}
		checkpointSeq++
		summary := fmt.Sprintf("step %d milestone", step)
		cp := &workersidecar.CheckpointInfo{
			CheckpointID: fmt.Sprintf("cp-%s-%d", taskKey, checkpointSeq),
			Summary:      summary,
			Fields:       map[string]interface{}{"step": step},
			ResumeBlob:   "",
		}
		lastCheckpoint = cp
		if b.cfg.CheckpointCallback != nil {
			b.cfg.CheckpointCallback(runID, cp)
		}
	}

	for {
		select {
		case <-ctx.Done():
			proc.Cancel()
			return workersidecar.RunResult{
				Status:     "cancelled",
				Summary:    "Run cancelled",
				ResultText: resultText.String(),
				Error:      ctx.Err().Error(),
				Checkpoint: salvageCheckpoint(taskKey, stepCount, lastCheckpoint, resultText.String()),
			}
		case ev, ok := <-proc.Events():
			if !ok {
				goto done
			}
			switch ev.Type {
			case "text_delta":
				resultText.WriteString(ev.Content)
				emitProgress(ev.Content)
			case "tool_call":
				stepCount++
				emitToolProgress(ev.Content)
				maybeCheckpoint(stepCount)
			case "error":
				return workersidecar.RunResult{
					Status:     "failed",
					Summary:    "Agent encountered an error",
					ResultText: resultText.String(),
					Error:      ev.Error,
					Checkpoint: salvageCheckpoint(taskKey, stepCount, lastCheckpoint, resultText.String()),
				}
			case "message_end":
				duration := time.Since(startTime)
				return workersidecar.RunResult{
					Status:     "completed",
					Summary:    fmt.Sprintf("Completed in %v", duration.Round(time.Second)),
					ResultText: resultText.String(),
					Usage:      &workersidecar.UsageInfo{},
					Checkpoint: lastCheckpoint,
				}
			}
		}
	}

done:
	finalResult, err := proc.Wait()
	if err != nil {
		if ctx.Err() != nil {
			return workersidecar.RunResult{
				Status:     "cancelled",
				Summary:    "Run cancelled",
				ResultText: resultText.String(),
				Error:      ctx.Err().Error(),
				Checkpoint: salvageCheckpoint(taskKey, stepCount, lastCheckpoint, resultText.String()),
			}
		}
		return workersidecar.RunResult{
			Status:     "failed",
			Summary:    "Agent failed",
			ResultText: resultText.String(),
			Error:      err.Error(),
			Checkpoint: salvageCheckpoint(taskKey, stepCount, lastCheckpoint, resultText.String()),
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

// PiBinaryPath returns the path to the pi binary used for spawning.
func (b *Backend) PiBinaryPath() (string, error) {
	return exec.LookPath("pi")
}

// salvageCheckpoint ensures terminal runs expose a Hub-compatible checkpoint with resume_blob when possible.
func salvageCheckpoint(taskKey string, stepCount int, last *workersidecar.CheckpointInfo, partial string) *workersidecar.CheckpointInfo {
	if last != nil {
		cp := *last
		if cp.ResumeBlob == "" && strings.TrimSpace(partial) != "" {
			cp.ResumeBlob = partial
		}
		if cp.Fields == nil && stepCount > 0 {
			cp.Fields = map[string]interface{}{"step": stepCount}
		}
		return &cp
	}
	if strings.TrimSpace(partial) == "" {
		return nil
	}
	return &workersidecar.CheckpointInfo{
		CheckpointID: fmt.Sprintf("cp-%s-salvage", taskKey),
		Summary:      "partial progress",
		Fields:       map[string]interface{}{"step": stepCount},
		ResumeBlob:   partial,
	}
}
