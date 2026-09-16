package tools

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/tool"

	"github.com/dimetron/pi-go/internal/subagent"
)

// DelegateTaskInput is the Hermes-compatible delegate_task surface (PR2 subset).
type DelegateTaskInput struct {
	Goal    string             `json:"goal,omitempty"`
	Context string             `json:"context,omitempty"`
	Tasks   []DelegateTaskItem `json:"tasks,omitempty"`
}

// DelegateTaskItem is one parallel delegated subtask.
type DelegateTaskItem struct {
	Goal    string `json:"goal"`
	Context string `json:"context,omitempty"`
}

// DelegateTaskOutput aggregates child summaries for the parent planner.
type DelegateTaskOutput struct {
	Results []AgentResult `json:"results"`
	Summary string        `json:"summary"`
}

type delegateWorkItem struct {
	Goal string
}

// NewDelegateTaskTool creates the Hermes delegate_task tool wired to an Orchestrator.
func NewDelegateTaskTool(orch *subagent.Orchestrator) (tool.Tool, error) {
	desc := `Delegate work to isolated child agents (Hermes-compatible). Provide either goal or tasks[] (each with goal and optional context); optional top-level context applies to a single goal. Children run with a fresh session and return summaries only — not intermediate tool traces. Over max_concurrent_children returns an error.`

	return newTool("delegate_task", desc,
		func(ctx agent.Context, input DelegateTaskInput) (DelegateTaskOutput, error) {
			return delegateTaskHandler(ctx, orch, input)
		},
		map[string]string{
			"tasks":   "tasks",
			"message": "goal",
			"prompt":  "goal",
		},
	)
}

func delegateTaskHandler(ctx agent.Context, orch *subagent.Orchestrator, input DelegateTaskInput) (DelegateTaskOutput, error) {
	if IsDelegateChild() {
		return DelegateTaskOutput{}, fmt.Errorf(
			"delegate_task refused: max_spawn_depth=1 (delegated children cannot delegate again)",
		)
	}

	items, err := normalizeDelegateWork(input)
	if err != nil {
		return DelegateTaskOutput{}, err
	}

	max := MaxConcurrentDelegateChildren()
	if len(items) > max {
		return DelegateTaskOutput{}, fmt.Errorf(
			"too many delegated tasks: %d exceeds max_concurrent_children (%d)",
			len(items), max,
		)
	}

	agentCfg, err := resolveDelegateAgent(orch)
	if err != nil {
		return DelegateTaskOutput{}, err
	}

	start := time.Now()
	spawnCtx := resolveContext(ctx)

	if len(items) == 1 {
		result := runDelegateStep(spawnCtx, orch, agentCfg, items[0].Goal)
		return DelegateTaskOutput{
			Results: []AgentResult{result},
			Summary: fmt.Sprintf("delegate: 1 task, %s in %s", result.Status, result.Duration),
		}, nil
	}

	results := make([]AgentResult, len(items))
	var wg sync.WaitGroup
	for i, item := range items {
		wg.Add(1)
		go func(idx int, goal string) {
			defer wg.Done()
			results[idx] = runDelegateStep(spawnCtx, orch, agentCfg, goal)
		}(i, item.Goal)
	}
	wg.Wait()

	duration := time.Since(start).Truncate(time.Millisecond).String()
	return DelegateTaskOutput{
		Results: results,
		Summary: buildParallelSummary(results, len(items), duration),
	}, nil
}

func normalizeDelegateWork(in DelegateTaskInput) ([]delegateWorkItem, error) {
	hasGoal := strings.TrimSpace(in.Goal) != ""
	hasTasks := len(in.Tasks) > 0
	if hasGoal && hasTasks {
		return nil, fmt.Errorf("provide either goal or tasks[], not both")
	}
	if !hasGoal && !hasTasks {
		return nil, fmt.Errorf("goal or tasks[] is required")
	}

	if hasTasks {
		out := make([]delegateWorkItem, 0, len(in.Tasks))
		top := strings.TrimSpace(in.Context)
		for i, t := range in.Tasks {
			goal := strings.TrimSpace(t.Goal)
			if goal == "" {
				return nil, fmt.Errorf("tasks[%d].goal is required", i)
			}
			prompt := buildDelegatePrompt(goal, mergeDelegateContext(top, t.Context))
			out = append(out, delegateWorkItem{Goal: prompt})
		}
		return out, nil
	}

	goal := strings.TrimSpace(in.Goal)
	prompt := buildDelegatePrompt(goal, strings.TrimSpace(in.Context))
	return []delegateWorkItem{{Goal: prompt}}, nil
}

func mergeDelegateContext(top, task string) string {
	top = strings.TrimSpace(top)
	task = strings.TrimSpace(task)
	switch {
	case top == "":
		return task
	case task == "":
		return top
	default:
		return top + "\n\n" + task
	}
}

func buildDelegatePrompt(goal, context string) string {
	goal = strings.TrimSpace(goal)
	context = strings.TrimSpace(context)
	if context == "" {
		return goal
	}
	if goal == "" {
		return context
	}
	return goal + "\n\n[Context]\n" + context
}

func resolveDelegateAgent(orch *subagent.Orchestrator) (subagent.AgentConfig, error) {
	if ac, err := orch.LookupAgent(DefaultDelegateAgent); err == nil {
		return ac, nil
	}
	if ac, err := orch.LookupAgent("task"); err == nil {
		return ac, nil
	}
	return subagent.AgentConfig{}, fmt.Errorf(
		"no delegate agent: %q (or task) is not registered",
		DefaultDelegateAgent,
	)
}

func runDelegateStep(ctx context.Context, orch *subagent.Orchestrator, agentCfg subagent.AgentConfig, prompt string) AgentResult {
	stepStart := time.Now()

	events, agentID, err := orch.Spawn(ctx, subagent.SpawnInput{
		Agent:  agentCfg,
		Prompt: prompt,
		Env:    DelegateChildEnv(),
	})
	if err != nil {
		return AgentResult{
			Agent:    agentCfg.Name,
			Status:   "failed",
			Error:    err.Error(),
			Duration: time.Since(stepStart).Truncate(time.Millisecond).String(),
		}
	}

	resultText, status, errMsg, sessID := consumeDelegateEvents(events)
	return AgentResult{
		Agent:     agentCfg.Name,
		AgentID:   agentID,
		Status:    status,
		Result:    resultText,
		Error:     errMsg,
		Duration:  time.Since(stepStart).Truncate(time.Millisecond).String(),
		SessionID: sessID,
	}
}

// consumeDelegateEvents collects assistant text only — child tool_call/tool_result events are not surfaced to the parent.
func consumeDelegateEvents(events <-chan subagent.Event) (resultText, status, errMsg, sessID string) {
	var result strings.Builder
	status = "completed"
	for ev := range events {
		switch ev.Type {
		case "text_delta":
			result.WriteString(ev.Content)
		case "error":
			status = "failed"
			errMsg = ev.Error
		case "message_start":
			if ev.SessionID != "" {
				sessID = ev.SessionID
			}
		case "run_done":
			if ev.Status == "timeout" {
				status = "timeout"
			}
		}
	}
	return truncateOutput(result.String()), status, errMsg, sessID
}
