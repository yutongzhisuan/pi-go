package tools

import (
	"strings"
	"testing"

	"google.golang.org/adk/v2/tool"

	"github.com/dimetron/pi-go/internal/config"
	"github.com/dimetron/pi-go/internal/subagent"
)

func TestNormalizeDelegateWork_SingleGoal(t *testing.T) {
	items, err := normalizeDelegateWork(DelegateTaskInput{
		Goal:    "research X",
		Context: "constraint Y",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || !strings.Contains(items[0].Goal, "research X") {
		t.Fatalf("items = %+v", items)
	}
}

func TestNormalizeDelegateWork_Tasks(t *testing.T) {
	items, err := normalizeDelegateWork(DelegateTaskInput{
		Context: "shared",
		Tasks: []DelegateTaskItem{
			{Goal: "a", Context: "ctx-a"},
			{Goal: "b"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 {
		t.Fatalf("len = %d", len(items))
	}
}

func TestNormalizeDelegateWork_Conflict(t *testing.T) {
	_, err := normalizeDelegateWork(DelegateTaskInput{
		Goal:  "x",
		Tasks: []DelegateTaskItem{{Goal: "y"}},
	})
	if err == nil {
		t.Fatal("expected error for goal+tasks")
	}
}

func TestDelegateTask_OverConcurrencyError(t *testing.T) {
	t.Setenv(EnvDelegateMaxConcurrent, "2")
	t.Cleanup(func() { t.Setenv(EnvDelegateMaxConcurrent, "") })

	cfg := config.Defaults()
	cfg.Roles["smol"] = config.RoleConfig{Model: "claude-haiku"}
	orch := subagent.NewOrchestrator(&cfg, "", nil)

	_, err := delegateTaskHandler(nil, orch, DelegateTaskInput{
		Tasks: []DelegateTaskItem{
			{Goal: "one"},
			{Goal: "two"},
			{Goal: "three"},
		},
	})
	if err == nil || !strings.Contains(err.Error(), "max_concurrent_children") {
		t.Fatalf("expected concurrency tool error, got %v", err)
	}
}

func TestDelegateTask_NestedDelegateRefused(t *testing.T) {
	t.Setenv(EnvDelegatedChild, "1")
	t.Cleanup(func() { t.Setenv(EnvDelegatedChild, "") })

	cfg := config.Defaults()
	cfg.Roles["smol"] = config.RoleConfig{Model: "claude-haiku"}
	orch := subagent.NewOrchestrator(&cfg, "", nil)

	_, err := delegateTaskHandler(nil, orch, DelegateTaskInput{Goal: "nested"})
	if err == nil || !strings.Contains(err.Error(), "max_spawn_depth") {
		t.Fatalf("expected depth error, got %v", err)
	}
}

func TestDelegateTask_NestedOrchestratorAllowed(t *testing.T) {
	t.Setenv(EnvDelegatedChild, "1")
	t.Setenv(EnvDelegateDepth, "1")
	t.Setenv(EnvDelegateRole, "orchestrator")
	t.Setenv(EnvDelegateMaxSpawnDepth, "2")
	t.Cleanup(func() {
		t.Setenv(EnvDelegatedChild, "")
		t.Setenv(EnvDelegateDepth, "")
		t.Setenv(EnvDelegateRole, "")
		t.Setenv(EnvDelegateMaxSpawnDepth, "")
	})

	res := ResolvedDelegation{MaxSpawnDepth: 2}
	if !CanDelegateAtCurrentDepth(res) {
		t.Fatal("orchestrator child should pass depth gate when max_spawn_depth=2")
	}
}

func TestDelegateTask_GatewayRefusalInChild(t *testing.T) {
	t.Setenv(EnvDelegatedChild, "1")
	t.Setenv(EnvDelegateRole, "leaf")
	t.Cleanup(func() {
		t.Setenv(EnvDelegatedChild, "")
		t.Setenv(EnvDelegateRole, "")
	})

	filtered := FilterToolsForDelegateChild([]tool.Tool{
		stubTool{name: "gateway_dispatch_task"},
		stubTool{name: "read"},
		stubTool{name: "delegate_task"},
	})
	for _, tl := range filtered {
		if strings.HasPrefix(tl.Name(), "gateway_") {
			t.Fatalf("gateway tool %q should be filtered", tl.Name())
		}
	}
	if len(filtered) != 1 || filtered[0].Name() != "read" {
		t.Fatalf("filtered = %v", delegateToolNames(filtered))
	}
}
