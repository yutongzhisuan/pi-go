package tools

import (
	"fmt"
	"strings"

	"github.com/dimetron/pi-go/internal/subagent"
)

// DelegateChildEntry is one active delegated child (action=list).
type DelegateChildEntry struct {
	ID     string `json:"id"`
	Goal   string `json:"goal"`
	Status string `json:"status"`
}

func delegateTaskList(orch *subagent.Orchestrator) (DelegateTaskOutput, error) {
	if orch == nil {
		return DelegateTaskOutput{}, fmt.Errorf("delegate_task list: orchestrator unavailable")
	}
	entries := make([]DelegateChildEntry, 0)
	for _, st := range orch.List() {
		if st.Status != "running" {
			continue
		}
		entries = append(entries, DelegateChildEntry{
			ID:     st.AgentID,
			Goal:   truncateDelegateGoal(st.Prompt),
			Status: st.Status,
		})
	}
	return DelegateTaskOutput{
		Children: entries,
		Summary:  fmt.Sprintf("delegate: %d active child(ren)", len(entries)),
	}, nil
}

func delegateTaskSteer(orch *subagent.Orchestrator, agentID, message string) (DelegateTaskOutput, error) {
	agentID = strings.TrimSpace(agentID)
	if agentID == "" {
		return DelegateTaskOutput{}, fmt.Errorf("agent_id is required for action=steer")
	}
	if orch == nil {
		return DelegateTaskOutput{}, fmt.Errorf("delegate_task steer: orchestrator unavailable")
	}
	if err := orch.Steer(agentID, message); err != nil {
		return DelegateTaskOutput{}, err
	}
	return DelegateTaskOutput{
		Summary: fmt.Sprintf("delegate: steer queued for %s", agentID),
	}, nil
}

func delegateTaskInterrupt(orch *subagent.Orchestrator, agentID string) (DelegateTaskOutput, error) {
	agentID = strings.TrimSpace(agentID)
	if agentID == "" {
		return DelegateTaskOutput{}, fmt.Errorf("agent_id is required for action=interrupt")
	}
	if orch == nil {
		return DelegateTaskOutput{}, fmt.Errorf("delegate_task interrupt: orchestrator unavailable")
	}
	if err := orch.Cancel(agentID); err != nil {
		return DelegateTaskOutput{}, err
	}
	return DelegateTaskOutput{
		Summary: fmt.Sprintf("delegate: interrupt sent to %s", agentID),
	}, nil
}

func truncateDelegateGoal(prompt string) string {
	prompt = strings.TrimSpace(prompt)
	const max = 240
	if len(prompt) <= max {
		return prompt
	}
	return prompt[:max] + "…"
}
