package tools

import (
	"strings"
	"testing"

	"google.golang.org/adk/v2/tool"
)

type stubTool struct{ name string }

func (s stubTool) Name() string { return s.name }

func (s stubTool) Description() string { return s.name }

func (s stubTool) IsLongRunning() bool { return false }

func TestIsDelegateBlockedTool(t *testing.T) {
	cases := []struct {
		name  string
		want  bool
	}{
		{"delegate_task", false},
		{"clarify", true},
		{"gateway_dispatch_task", true},
		{"gateway_list_models", true},
		{"memory", true},
		{"mem-search", true},
		{"send_message", true},
		{"cronjob", true},
		{"bash", false},
		{"subagent", false},
	}
	for _, tc := range cases {
		if got := IsDelegateBlockedTool(tc.name); got != tc.want {
			t.Errorf("IsDelegateBlockedTool(%q) = %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestFilterToolsForDelegateChild(t *testing.T) {
	t.Setenv(EnvDelegatedChild, "1")
	t.Cleanup(func() { t.Setenv(EnvDelegatedChild, "") })

	in := []tool.Tool{
		stubTool{name: "bash"},
		stubTool{name: "delegate_task"},
		stubTool{name: "gateway_watch_task"},
		stubTool{name: "mem-search"},
	}
	out := FilterToolsForDelegateChild(in)
	if len(out) != 1 || out[0].Name() != "bash" {
		names := delegateToolNames(out)
		t.Fatalf("filtered tools = %v, want [bash]", names)
	}
}

func TestFilterToolsForDelegateChild_ParentUnfiltered(t *testing.T) {
	t.Setenv(EnvDelegatedChild, "")
	in := []tool.Tool{stubTool{name: "delegate_task"}, stubTool{name: "bash"}}
	out := FilterToolsForDelegateChild(in)
	if len(out) != 2 {
		t.Fatalf("parent should keep all tools, got %d", len(out))
	}
}

func TestMaxConcurrentDelegateChildren(t *testing.T) {
	t.Setenv(EnvDelegateMaxConcurrent, "")
	if got := MaxConcurrentDelegateChildren(nil); got != 3 {
		t.Fatalf("default = %d, want 3", got)
	}
	t.Setenv(EnvDelegateMaxConcurrent, "5")
	if got := MaxConcurrentDelegateChildren(nil); got != 5 {
		t.Fatalf("override = %d, want 5", got)
	}
}

func TestIsDelegateBlockedTool_LeafChildBlocksDelegateTask(t *testing.T) {
	t.Setenv(EnvDelegatedChild, "1")
	t.Setenv(EnvDelegateRole, "leaf")
	t.Cleanup(func() {
		t.Setenv(EnvDelegatedChild, "")
		t.Setenv(EnvDelegateRole, "")
	})
	if !IsDelegateBlockedTool("delegate_task") {
		t.Fatal("leaf delegate child should not get delegate_task")
	}
}

func delegateToolNames(ts []tool.Tool) []string {
	names := make([]string, 0, len(ts))
	for _, t := range ts {
		if t != nil {
			names = append(names, t.Name())
		}
	}
	return names
}

func TestBuildDelegatePrompt(t *testing.T) {
	got := buildDelegatePrompt("Do the thing", "extra facts")
	if !strings.Contains(got, "Do the thing") || !strings.Contains(got, "extra facts") {
		t.Fatalf("prompt = %q", got)
	}
}
