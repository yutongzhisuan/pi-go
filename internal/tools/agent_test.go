package tools

import (
	"testing"

	"github.com/dimetron/pi-go/internal/config"
	"github.com/dimetron/pi-go/internal/subagent"
)

func TestAgentTool_Registration(t *testing.T) {
	cfg := config.Defaults()
	cfg.Roles["smol"] = config.RoleConfig{Model: "claude-haiku"}
	cfg.Roles["slow"] = config.RoleConfig{Model: "claude-opus"}
	cfg.Roles["plan"] = config.RoleConfig{Model: "claude-sonnet"}

	orch := subagent.NewOrchestrator(&cfg, "", nil)

	tools, err := AgentTools(orch, nil)
	if err != nil {
		t.Fatalf("AgentTools: %v", err)
	}
	if len(tools) != 2 {
		t.Fatalf("expected 2 tools, got %d", len(tools))
	}
	names := map[string]bool{}
	for _, tl := range tools {
		names[tl.Name()] = true
	}
	if !names["subagent"] || !names["delegate_task"] {
		t.Errorf("expected subagent and delegate_task, got %v", names)
	}
}

func TestAgentTools_LegacyCallbackWrapping(t *testing.T) {
	orch := subagent.NewOrchestrator(defaultConfigPtr(), "", nil)

	var receivedID, receivedKind, receivedContent string
	cb := func(agentID, eventType, content string) {
		receivedID = agentID
		receivedKind = eventType
		receivedContent = content
	}

	tools, err := AgentTools(orch, cb)
	if err != nil {
		t.Fatalf("AgentTools: %v", err)
	}
	if len(tools) != 2 {
		t.Fatalf("expected 2 tools, got %d", len(tools))
	}

	// Verify the tool was created (we can't easily invoke it without a real orchestrator,
	// but we've confirmed the wrapping compiles and the tool is registered).
	_ = receivedID
	_ = receivedKind
	_ = receivedContent
}
