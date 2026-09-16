package tools

import (
	"strings"
	"testing"

	"google.golang.org/adk/v2/tool"

	"github.com/dimetron/pi-go/internal/config"
	"github.com/dimetron/pi-go/internal/subagent"
)

func TestResolveSpawnRole_OrchestratorDegradesAtDepthOne(t *testing.T) {
	var logs []string
	DelegationNoticeHook = func(msg string) { logs = append(logs, msg) }
	t.Cleanup(func() { DelegationNoticeHook = nil })

	res := ResolvedDelegation{MaxSpawnDepth: 1, OrchestratorEnabled: true}
	got := ResolveSpawnRole("orchestrator", 0, res)
	if got != delegateRoleLeaf {
		t.Fatalf("role = %q, want leaf", got)
	}
	if len(logs) == 0 || !strings.Contains(logs[0], "degraded") {
		t.Fatalf("expected degrade log, got %v", logs)
	}
}

func TestCanDelegateAtCurrentDepth_OrchestratorChild(t *testing.T) {
	t.Setenv(EnvDelegatedChild, "1")
	t.Setenv(EnvDelegateDepth, "1")
	t.Setenv(EnvDelegateRole, "orchestrator")
	t.Cleanup(func() {
		t.Setenv(EnvDelegatedChild, "")
		t.Setenv(EnvDelegateDepth, "")
		t.Setenv(EnvDelegateRole, "")
	})

	res := ResolvedDelegation{MaxSpawnDepth: 2}
	if !CanDelegateAtCurrentDepth(res) {
		t.Fatal("orchestrator child should delegate when max_spawn_depth=2")
	}

	t.Setenv(EnvDelegateRole, "leaf")
	if CanDelegateAtCurrentDepth(res) {
		t.Fatal("leaf child should not delegate")
	}
}

func TestDelegateTask_ListSteerInterrupt(t *testing.T) {
	cfg := config.Defaults()
	orch := subagent.NewOrchestrator(&cfg, "", nil)
	if !orch.SetStatusForTest("child-a", "running") {
		t.Fatal("SetStatusForTest failed")
	}

	listOut, err := delegateTaskHandler(nil, orch, DelegateTaskInput{Action: "list"})
	if err != nil {
		t.Fatal(err)
	}
	if len(listOut.Children) != 1 || listOut.Children[0].ID != "child-a" {
		t.Fatalf("list = %+v", listOut.Children)
	}

	steerOut, err := delegateTaskHandler(nil, orch, DelegateTaskInput{
		Action:  "steer",
		AgentID: "child-a",
		Message: "focus on tests",
	})
	if err != nil {
		t.Fatal(err)
	}
	if steerOut.Summary == "" {
		t.Fatal("expected steer summary")
	}
	q, ok := orch.SteerQueue("child-a")
	if !ok || len(q) != 1 || q[0] != "focus on tests" {
		t.Fatalf("steer queue = %v ok=%v", q, ok)
	}

	_, err = delegateTaskHandler(nil, orch, DelegateTaskInput{
		Action:  "interrupt",
		AgentID: "child-a",
	})
	if err != nil {
		t.Fatal(err)
	}
	st, ok := orch.Get("child-a")
	if !ok || st.Status != "canceled" {
		t.Fatalf("after interrupt status = %+v ok=%v", st, ok)
	}
}

func TestResolveDelegation_ConfigAndEnv(t *testing.T) {
	maxC, maxD := 5, 3
	orchOn := true
	cfg := config.Defaults()
	cfg.Delegation = &config.DelegationConfig{
		MaxConcurrentChildren: &maxC,
		MaxSpawnDepth:         &maxD,
		OrchestratorEnabled:   &orchOn,
	}
	res := ResolveDelegation(&cfg)
	if res.MaxConcurrentChildren != 5 || res.MaxSpawnDepth != 3 || !res.OrchestratorEnabled {
		t.Fatalf("config merge = %+v", res)
	}

	t.Setenv(EnvDelegateMaxSpawnDepth, "2")
	t.Cleanup(func() { t.Setenv(EnvDelegateMaxSpawnDepth, "") })
	res = ResolveDelegation(&cfg)
	if res.MaxSpawnDepth != 2 {
		t.Fatalf("env override = %d", res.MaxSpawnDepth)
	}
}

func TestFilterToolsForDelegateChild_OrchestratorKeepsDelegateTask(t *testing.T) {
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

	out := FilterToolsForDelegateChild([]tool.Tool{
		stubTool{name: "delegate_task"},
		stubTool{name: "bash"},
	})
	if len(out) != 2 {
		t.Fatalf("tools = %v", delegateToolNames(out))
	}
}
