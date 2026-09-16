package masterplanner_test

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"github.com/dimetron/pi-go/internal/masterplanner"
	"github.com/dimetron/pi-go/internal/masterplanner/fake"
)

// TestIntegration_FullWorkflow exercises the complete PLAN → DISPATCH → WATCH → JOIN → ANSWER loop
// against a fake Gateway that mimics Hermes wire behavior.
func TestIntegration_FullWorkflow(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Setup fake Gateway
	gw := fake.NewGateway()
	defer gw.Close()

	// Setup client and ledger
	client := masterplanner.NewGatewayClientWithConfig(gw.URL(), "test-key", 30)
	ledger, err := masterplanner.NewLedger(":memory:")
	if err != nil {
		t.Fatalf("NewLedger: %v", err)
	}
	defer ledger.Close()

	tools, err := masterplanner.MasterPlannerTools(
		masterplanner.WithClient(client),
		masterplanner.WithLedger(ledger),
	)
	if err != nil {
		t.Fatalf("MasterPlannerTools: %v", err)
	}

	if len(tools) != 8 {
		t.Fatalf("expected 8 tools, got %d", len(tools))
	}

	// Verify all expected tools are present
	expectedTools := []string{
		"gateway_dispatch_task",
		"gateway_dispatch_batch",
		"gateway_watch_task",
		"gateway_get_task_result",
		"gateway_list_tasks",
		"gateway_list_models",
		"gateway_list_workers",
		"gateway_cancel_task",
	}

	toolNames := make(map[string]bool)
	for _, tool := range tools {
		toolNames[tool.Name()] = true
	}

	for _, expected := range expectedTools {
		if !toolNames[expected] {
			t.Errorf("missing tool: %s", expected)
		}
	}

	ctx := context.Background()

	// 1. PLAN: List models (verify Hermes-shaped response)
	t.Run("list_models", func(t *testing.T) {
		resp, err := client.ListModels(ctx, "")
		if err != nil {
			t.Fatalf("ListModels: %v", err)
		}

		models, ok := resp["models"].([]any)
		if !ok || len(models) == 0 {
			t.Fatal("expected models array")
		}

		// Verify Hermes field names
		model := models[0].(map[string]any)
		requiredFields := []string{"model_version_id", "display_name", "node_count", "available_slots", "regions"}
		for _, field := range requiredFields {
			if _, ok := model[field]; !ok {
				t.Errorf("missing field in model: %s", field)
			}
		}
	})

	// 2. DISPATCH: Single task (verify Hermes request/response shape)
	var taskID string
	t.Run("dispatch_task", func(t *testing.T) {
		spec := masterplanner.TaskSpec{
			TaskID: "test-run-1",
			Goal:   "Research quantum computing",
			Model:  "gpt-4-turbo",
		}

		resp, err := client.DispatchTask(ctx, spec, "session-1")
		if err != nil {
			t.Fatalf("DispatchTask: %v", err)
		}

		// Verify Hermes response fields
		requiredFields := []string{"task_id", "status"}
		for _, field := range requiredFields {
			if _, ok := resp[field]; !ok {
				t.Errorf("missing field in dispatch response: %s", field)
			}
		}

		taskID = resp["task_id"].(string)
		if taskID != "test-run-1" {
			t.Errorf("task_id = %s, want test-run-1", taskID)
		}
	})

	// 3. WATCH: Add events and watch (verify SSE stream behavior)
	t.Run("watch_task_with_events", func(t *testing.T) {
		// Simulate task progression
		gw.AddProgressEvent(taskID, "Starting research on quantum computing trends")
		gw.AddCheckpointEvent(taskID, "cp-1", "Gathered 50 papers on quantum algorithms")
		gw.SetTaskStatus(taskID, "completed", "Research complete: found 3 major trends")

		// Watch for events
		result, err := client.Watch(ctx, masterplanner.WatchOptions{
			TaskID:      taskID,
			WaitSeconds: 5,
		})
		if err != nil {
			t.Fatalf("Watch: %v", err)
		}

		// Verify we got events
		if len(result.Events) == 0 {
			t.Fatal("expected events from watch stream")
		}

		// Verify event structure matches Hermes
		hasProgress := false
		hasCheckpoint := false
		hasTerminal := false

		for _, event := range result.Events {
			eventType := event["type"].(string)
			switch eventType {
			case "progress":
				hasProgress = true
			case "checkpoint":
				hasCheckpoint = true
			case "terminal":
				hasTerminal = true
			}
		}

		if !hasProgress {
			t.Error("expected progress event")
		}
		if !hasCheckpoint {
			t.Error("expected checkpoint event")
		}
		if !hasTerminal {
			t.Error("expected terminal event")
		}

		if result.Cursor == "" {
			t.Error("expected non-empty cursor")
		}

		if result.Reason != "terminal" {
			t.Errorf("reason = %s, want terminal", result.Reason)
		}
	})

	// 4. JOIN: Get final result (verify Hermes result shape)
	t.Run("get_task_result", func(t *testing.T) {
		// Set result on task via GetTask
		task := gw.GetTask(taskID)
		if task == nil {
			t.Fatal("task not found")
		}
		task.ResultText = "Quantum computing trends: 1) Topological qubits 2) Error correction 3) Quantum ML"

		resp, err := client.GetTaskResult(ctx, taskID)
		if err != nil {
			t.Fatalf("GetTaskResult: %v", err)
		}

		// Verify Hermes result fields
		requiredFields := []string{"task_id", "status", "summary", "result_text", "latest_checkpoint_id"}
		for _, field := range requiredFields {
			if _, ok := resp[field]; !ok {
				t.Errorf("missing field in result: %s", field)
			}
		}

		if resp["task_id"] != taskID {
			t.Errorf("task_id mismatch")
		}

		resultText := resp["result_text"].(string)
		if resultText == "" {
			t.Error("expected non-empty result_text")
		}
	})

	// 5. List tasks (verify Hermes list shape)
	t.Run("list_tasks", func(t *testing.T) {
		resp, err := client.ListTasks(ctx, masterplanner.ListTasksOptions{
			MasterSessionID: "session-1",
		})
		if err != nil {
			t.Fatalf("ListTasks: %v", err)
		}

		tasks, ok := resp["tasks"].([]any)
		if !ok || len(tasks) == 0 {
			t.Fatal("expected tasks array")
		}

		// Verify Hermes task list fields
		task := tasks[0].(map[string]any)
		requiredFields := []string{"task_id", "status"}
		for _, field := range requiredFields {
			if _, ok := task[field]; !ok {
				t.Errorf("missing field in task: %s", field)
			}
		}
	})

	// 6. List workers (verify Hermes workers shape)
	t.Run("list_workers", func(t *testing.T) {
		resp, err := client.ListWorkers(ctx, nil)
		if err != nil {
			t.Fatalf("ListWorkers: %v", err)
		}

		workers, ok := resp["workers"].([]any)
		if !ok || len(workers) == 0 {
			t.Fatal("expected workers array")
		}

		// Verify Hermes worker fields
		worker := workers[0].(map[string]any)
		requiredFields := []string{"worker_id", "toolsets"}
		for _, field := range requiredFields {
			if _, ok := worker[field]; !ok {
				t.Errorf("missing field in worker: %s", field)
			}
		}
	})

	// 7. Cancel task (verify Hermes cancel behavior)
	t.Run("cancel_task", func(t *testing.T) {
		// Dispatch a new task to cancel
		cancelSpec := masterplanner.TaskSpec{
			TaskID: "test-run-cancel",
			Goal:   "Task to cancel",
		}
		client.DispatchTask(ctx, cancelSpec, "session-1")

		resp, err := client.CancelTask(ctx, "test-run-cancel", "", "test cancellation")
		if err != nil {
			t.Fatalf("CancelTask: %v", err)
		}

		cancelled, ok := resp["cancelled"].(bool)
		if !ok || !cancelled {
			t.Error("expected cancelled=true")
		}
	})
}

// TestIntegration_BatchWorkflow tests batch dispatch and watch.
func TestIntegration_BatchWorkflow(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	gw := fake.NewGateway()
	defer gw.Close()

	client := masterplanner.NewGatewayClientWithConfig(gw.URL(), "test-key", 30)
	ctx := context.Background()

	// Dispatch batch
	specs := []masterplanner.TaskSpec{
		{TaskID: "batch-1-1", Goal: "Research AI safety"},
		{TaskID: "batch-1-2", Goal: "Research quantum algorithms"},
		{TaskID: "batch-1-3", Goal: "Research biotech advances"},
	}

	resp, err := client.DispatchBatch(ctx, specs, "batch-1", "session-1", "all")
	if err != nil {
		t.Fatalf("DispatchBatch: %v", err)
	}

	// Verify Hermes batch response
	batchID, ok := resp["batch_id"].(string)
	if !ok || batchID == "" {
		t.Fatal("expected batch_id in response")
	}

	count, ok := resp["count"].(float64)
	if !ok || int(count) != 3 {
		t.Errorf("count = %v, want 3", resp["count"])
	}

	// Complete all tasks in batch
	for _, spec := range specs {
		gw.SetTaskStatus(spec.TaskID, "completed", "Done: "+spec.Goal)
	}

	// Watch batch
	result, err := client.Watch(ctx, masterplanner.WatchOptions{
		BatchID:     batchID,
		WaitSeconds: 5,
	})
	if err != nil {
		t.Fatalf("Watch batch: %v", err)
	}

	// Should have terminal events for all tasks
	if len(result.Events) < 3 {
		t.Errorf("expected at least 3 events, got %d", len(result.Events))
	}
}

// TestIntegration_LedgerRecovery tests ledger-based recovery.
func TestIntegration_LedgerRecovery(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	gw := fake.NewGateway()
	defer gw.Close()

	client := masterplanner.NewGatewayClientWithConfig(gw.URL(), "test-key", 30)
	ledger, err := masterplanner.NewLedger(":memory:")
	if err != nil {
		t.Fatalf("NewLedger: %v", err)
	}
	defer ledger.Close()

	ctx := context.Background()

	// Dispatch task and record in ledger
	spec := masterplanner.TaskSpec{
		TaskID: "recovery-1",
		Goal:   "Test recovery",
	}
	client.DispatchTask(ctx, spec, "session-1")

	runID := "test-run"
	if err := ledger.Record(runID, spec.TaskID, spec.Goal); err != nil {
		t.Fatalf("Record: %v", err)
	}

	// Verify ledger can retrieve task
	rec, err := ledger.Get(spec.TaskID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if rec == nil {
		t.Fatal("expected task record")
	}
	if rec.TaskID != spec.TaskID {
		t.Errorf("task_id = %s, want %s", rec.TaskID, spec.TaskID)
	}

	// Simulate cursor update
	if err := ledger.UpdateCursor(spec.TaskID, "cursor-123", "gateway-instance-1"); err != nil {
		t.Fatalf("UpdateCursor: %v", err)
	}

	rec, _ = ledger.Get(spec.TaskID)
	if rec.CursorEventID != "cursor-123" {
		t.Errorf("cursor = %s, want cursor-123", rec.CursorEventID)
	}

	// Simulate status update
	if err := ledger.UpdateStatus(spec.TaskID, masterplanner.TaskStatusCompleted); err != nil {
		t.Fatalf("UpdateStatus: %v", err)
	}

	rec, _ = ledger.Get(spec.TaskID)
	if rec.Status != masterplanner.TaskStatusCompleted {
		t.Errorf("status = %s, want completed", rec.Status)
	}
}

// TestIntegration_HermesFieldCompatibility verifies all tool param and result fields
// match Hermes tools.py wire contracts.
func TestIntegration_HermesFieldCompatibility(t *testing.T) {
	// This test documents the Hermes wire contract and verifies our types match.
	
	t.Run("dispatch_task_params", func(t *testing.T) {
		// From Hermes tools.py gateway_dispatch_task parameters
		expected := []string{
			"goal",                   // required
			"model",                  // optional
			"toolsets",               // optional
			"params",                 // optional
			"context",                // optional
			"timeout_seconds",        // optional
			"priority",               // optional
			"depends_on",             // optional
			"resume_from_checkpoint", // optional
			"resume_summary",         // optional
		}

		input := masterplanner.DispatchTaskInput{
			Goal:                 "test",
			Model:                "gpt-4",
			Toolsets:             []string{"python"},
			Params:               map[string]any{"key": "value"},
			Context:              "context",
			TimeoutSeconds:       300,
			Priority:             1,
			DependsOn:            []string{"task-0"},
			ResumeFromCheckpoint: "cp-1",
			ResumeSummary:        "summary",
		}

		data, _ := json.Marshal(input)
		var m map[string]any
		json.Unmarshal(data, &m)

		for _, field := range expected {
			if _, ok := m[field]; !ok {
				t.Errorf("missing Hermes field in DispatchTaskInput: %s", field)
			}
		}
	})

	t.Run("dispatch_task_result", func(t *testing.T) {
		// From Hermes tools.py gateway_dispatch_task result
		expected := []string{"task_id", "run_id", "status", "idempotent_hit", "note"}

		output := masterplanner.DispatchTaskOutput{
			TaskID:        "task-1",
			RunID:         "run-1",
			Status:        "submitted",
			IdempotentHit: false,
			Note:          "note",
		}

		data, _ := json.Marshal(output)
		var m map[string]any
		json.Unmarshal(data, &m)

		for _, field := range expected {
			if _, ok := m[field]; !ok {
				t.Errorf("missing Hermes field in DispatchTaskOutput: %s", field)
			}
		}
	})

	t.Run("watch_task_result", func(t *testing.T) {
		// From Hermes tools.py gateway_watch_task result
		expected := []string{"reason", "interrupted", "cursor", "progress", "checkpoints", "terminal", "events"}

		output := masterplanner.WatchTaskOutput{
			Reason:      "terminal",
			Interrupted: false,
			Cursor:      "123",
			Progress:    map[string]string{"task-1": "working"},
			Checkpoints: map[string]string{"task-1": "summary"},
			Terminal:    []masterplanner.TerminalEvent{{TaskID: "task-1", Status: "completed"}},
			Events:      []map[string]any{{"type": "progress"}},
		}

		data, _ := json.Marshal(output)
		var m map[string]any
		json.Unmarshal(data, &m)

		for _, field := range expected {
			if _, ok := m[field]; !ok {
				t.Errorf("missing Hermes field in WatchTaskOutput: %s", field)
			}
		}
	})

	t.Run("get_task_result", func(t *testing.T) {
		// From Hermes tools.py gateway_get_task_result result
		expected := []string{"task_id", "status", "summary", "result_text", "latest_checkpoint_id", "error", "note"}

		output := masterplanner.GetTaskResultOutput{
			TaskID:             "task-1",
			Status:             "completed",
			Summary:            "done",
			ResultText:         "result",
			LatestCheckpointID: "cp-1",
			Error:              "",
			Note:               "note",
		}

		data, _ := json.Marshal(output)
		var m map[string]any
		json.Unmarshal(data, &m)

		for _, field := range expected {
			if _, ok := m[field]; !ok {
				t.Errorf("missing Hermes field in GetTaskResultOutput: %s", field)
			}
		}
	})
}

// TestIntegration_RealGateway documents how to run tests against a real Gateway.
// Set REAL_GATEWAY_TEST=1 and configure environment to run.
func TestIntegration_RealGateway(t *testing.T) {
	if os.Getenv("REAL_GATEWAY_TEST") != "1" {
		t.Skip("Set REAL_GATEWAY_TEST=1 to run against real Gateway")
	}

	// Configuration required:
	// export INFA_GATEWAY_API_KEY="your-api-key"
	// export INFA_GATEWAY_BASE_URL="https://gateway.your-org.com"
	// export REAL_GATEWAY_TEST=1

	apiKey := os.Getenv("INFA_GATEWAY_API_KEY")
	if apiKey == "" {
		t.Fatal("INFA_GATEWAY_API_KEY required for real Gateway test")
	}

	client, err := masterplanner.NewGatewayClient()
	if err != nil {
		t.Fatalf("NewGatewayClient: %v", err)
	}

	ctx := context.Background()

	// List models to verify connectivity
	t.Run("connectivity_check", func(t *testing.T) {
		resp, err := client.ListModels(ctx, "")
		if err != nil {
			t.Fatalf("ListModels: %v", err)
		}

		models, ok := resp["models"].([]any)
		if !ok {
			t.Fatal("expected models array")
		}

		t.Logf("Found %d models on real Gateway", len(models))
		for i, m := range models {
			if model, ok := m.(map[string]any); ok {
				t.Logf("  Model %d: %s", i+1, model["model_version_id"])
			}
		}
	})

	// Dispatch a real task (optional - uncomment to run)
	/*
	t.Run("dispatch_real_task", func(t *testing.T) {
		spec := masterplanner.TaskSpec{
			TaskID: fmt.Sprintf("e2e-test-%d", time.Now().Unix()),
			Goal:   "Echo test: respond with 'Gateway integration test successful'",
			Model:  "gpt-4-turbo", // Use a model from list_models
		}

		resp, err := client.DispatchTask(ctx, spec, "e2e-session")
		if err != nil {
			t.Fatalf("DispatchTask: %v", err)
		}

		taskID := resp["task_id"].(string)
		t.Logf("Dispatched real task: %s", taskID)

		// Watch for completion (with timeout)
		deadline := time.Now().Add(2 * time.Minute)
		for time.Now().Before(deadline) {
			result, err := client.Watch(ctx, masterplanner.WatchOptions{
				TaskID:      taskID,
				WaitSeconds: 30,
			})
			if err != nil {
				t.Fatalf("Watch: %v", err)
			}

			if result.Reason == "terminal" {
				t.Log("Task completed")
				// Get result
				finalResult, err := client.GetTaskResult(ctx, taskID)
				if err != nil {
					t.Fatalf("GetTaskResult: %v", err)
				}
				t.Logf("Result: %s", finalResult["result_text"])
				return
			}

			t.Logf("Still running, reason: %s", result.Reason)
			time.Sleep(5 * time.Second)
		}

		t.Error("Task did not complete within timeout")
	})
	*/
}
