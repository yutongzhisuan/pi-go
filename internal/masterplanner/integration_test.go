//go:build integration

package masterplanner

import (
	"context"
	"os"
	"testing"
	"time"
)

func getIntegrationClient(t *testing.T) *Client {
	baseURL := os.Getenv("INFA_GATEWAY_BASE_URL")
	if baseURL == "" {
		baseURL = "http://localhost:18082"
	}

	apiKey := os.Getenv("INFA_GATEWAY_API_KEY")

	return NewClient(
		WithBaseURL(baseURL),
		WithAPIKey(apiKey),
	)
}

func TestIntegrationCreateTask(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	client := getIntegrationClient(t)
	ctx := context.Background()

	req := &CreateTaskRequest{
		WorkerID: "test-worker",
		ModelID:  "test-model",
		Prompt:   "Integration test task",
		Priority: TaskPriorityNormal,
		Metadata: map[string]string{
			"test": "integration",
		},
	}

	resp, err := client.CreateTask(ctx, req)
	if err != nil {
		t.Fatalf("CreateTask failed: %v", err)
	}

	if resp.TaskID == "" {
		t.Error("TaskID is empty")
	}
	if resp.Status != TaskStatusPending {
		t.Errorf("Status = %s, want %s", resp.Status, TaskStatusPending)
	}

	t.Logf("Created task: %s", resp.TaskID)
}

func TestIntegrationListModels(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	client := getIntegrationClient(t)
	ctx := context.Background()

	resp, err := client.ListModels(ctx, &ListModelsRequest{
		PageSize: 10,
	})
	if err != nil {
		t.Fatalf("ListModels failed: %v", err)
	}

	if len(resp.Models) == 0 {
		t.Error("No models returned")
	}

	for i, model := range resp.Models {
		t.Logf("Model %d: %s (%s)", i+1, model.ModelID, model.Provider)
	}
}

func TestIntegrationListWorkers(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	client := getIntegrationClient(t)
	ctx := context.Background()

	resp, err := client.ListWorkers(ctx, &ListWorkersRequest{
		PageSize: 10,
	})
	if err != nil {
		t.Fatalf("ListWorkers failed: %v", err)
	}

	t.Logf("Found %d workers", len(resp.Workers))
	for i, worker := range resp.Workers {
		t.Logf("Worker %d: %s (%s)", i+1, worker.WorkerID, worker.Status)
	}
}

func TestIntegrationTaskLifecycle(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	client := getIntegrationClient(t)
	ctx := context.Background()

	createResp, err := client.CreateTask(ctx, &CreateTaskRequest{
		ModelID:  "test-model",
		Prompt:   "Lifecycle test",
		Priority: TaskPriorityNormal,
	})
	if err != nil {
		t.Fatalf("CreateTask failed: %v", err)
	}

	taskID := createResp.TaskID
	t.Logf("Created task: %s", taskID)

	listResp, err := client.ListTasks(ctx, &ListTasksRequest{})
	if err != nil {
		t.Fatalf("ListTasks failed: %v", err)
	}

	found := false
	for _, task := range listResp.Tasks {
		if task.TaskID == taskID {
			found = true
			break
		}
	}
	if !found {
		t.Error("Created task not found in list")
	}

	submitResp, err := client.SubmitTaskResult(ctx, taskID, &SubmitTaskResultRequest{
		TaskID: taskID,
		Result: "Task completed successfully",
	})
	if err != nil {
		t.Fatalf("SubmitTaskResult failed: %v", err)
	}

	if submitResp.Status != TaskStatusCompleted {
		t.Errorf("Status = %s, want %s", submitResp.Status, TaskStatusCompleted)
	}

	t.Logf("Task completed: %s", taskID)
}

func TestIntegrationWatchTasks(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	client := getIntegrationClient(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	stream, err := client.WatchTasks(ctx, &WatchTasksRequest{})
	if err != nil {
		t.Fatalf("WatchTasks failed: %v", err)
	}
	defer stream.Close()

	eventCount := 0
	timeout := time.After(5 * time.Second)

	for {
		select {
		case event := <-stream.Events:
			if event == nil {
				t.Logf("Received %d events", eventCount)
				return
			}
			eventCount++
			t.Logf("Event %d: %s - %s", eventCount, event.Kind, event.TaskID)

		case err := <-stream.Errors:
			if err != nil {
				t.Fatalf("Stream error: %v", err)
			}
			t.Logf("Received %d events", eventCount)
			return

		case <-timeout:
			t.Logf("Received %d events (timeout)", eventCount)
			return
		}
	}
}

func TestIntegrationBatchCreateTasks(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	client := getIntegrationClient(t)
	ctx := context.Background()

	req := &BatchCreateTasksRequest{
		Tasks: []*CreateTaskRequest{
			{ModelID: "test-model", Prompt: "Batch task 1"},
			{ModelID: "test-model", Prompt: "Batch task 2"},
			{ModelID: "test-model", Prompt: "Batch task 3"},
		},
	}

	resp, err := client.BatchCreateTasks(ctx, req)
	if err != nil {
		t.Fatalf("BatchCreateTasks failed: %v", err)
	}

	if len(resp.Tasks) != 3 {
		t.Fatalf("got %d tasks, want 3", len(resp.Tasks))
	}

	for i, task := range resp.Tasks {
		t.Logf("Task %d: %s (%s)", i+1, task.TaskID, task.Status)
	}
}

func TestIntegrationCancelTask(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	client := getIntegrationClient(t)
	ctx := context.Background()

	createResp, err := client.CreateTask(ctx, &CreateTaskRequest{
		ModelID: "test-model",
		Prompt:  "Task to be cancelled",
	})
	if err != nil {
		t.Fatalf("CreateTask failed: %v", err)
	}

	taskID := createResp.TaskID
	t.Logf("Created task: %s", taskID)

	cancelResp, err := client.CancelTask(ctx, taskID, &CancelTaskRequest{
		TaskID: taskID,
		Reason: "Integration test cancellation",
	})
	if err != nil {
		t.Fatalf("CancelTask failed: %v", err)
	}

	if cancelResp.Status != TaskStatusCancelled {
		t.Errorf("Status = %s, want %s", cancelResp.Status, TaskStatusCancelled)
	}

	t.Logf("Task cancelled: %s", taskID)
}

func TestIntegrationPagination(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	client := getIntegrationClient(t)
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		_, err := client.CreateTask(ctx, &CreateTaskRequest{
			ModelID: "test-model",
			Prompt:  "Pagination test",
		})
		if err != nil {
			t.Fatalf("CreateTask %d failed: %v", i, err)
		}
	}

	resp, err := client.ListTasks(ctx, &ListTasksRequest{
		PageSize: 2,
	})
	if err != nil {
		t.Fatalf("ListTasks failed: %v", err)
	}

	if len(resp.Tasks) > 2 {
		t.Errorf("got %d tasks, want at most 2", len(resp.Tasks))
	}

	if resp.NextPageToken != "" {
		t.Logf("Next page token: %s", resp.NextPageToken)
	}
}

func TestIntegrationFieldMarshaling(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	client := getIntegrationClient(t)
	ctx := context.Background()

	req := &CreateTaskRequest{
		WorkerID: "test_worker_id",
		ModelID:  "test_model_id",
		Prompt:   "Field marshaling test",
		Metadata: map[string]string{
			"test_key":    "test_value",
			"another_key": "another_value",
		},
		MaxRetries: 5,
		Timeout:    7200,
	}

	resp, err := client.CreateTask(ctx, req)
	if err != nil {
		t.Fatalf("CreateTask failed: %v", err)
	}

	listResp, err := client.ListTasks(ctx, &ListTasksRequest{})
	if err != nil {
		t.Fatalf("ListTasks failed: %v", err)
	}

	var task *Task
	for _, t := range listResp.Tasks {
		if t.TaskID == resp.TaskID {
			task = t
			break
		}
	}

	if task == nil {
		t.Fatal("Task not found")
	}

	if task.WorkerID != req.WorkerID {
		t.Errorf("WorkerID = %s, want %s", task.WorkerID, req.WorkerID)
	}
	if task.ModelID != req.ModelID {
		t.Errorf("ModelID = %s, want %s", task.ModelID, req.ModelID)
	}
	if task.Metadata["test_key"] != "test_value" {
		t.Error("Metadata not preserved")
	}
}
