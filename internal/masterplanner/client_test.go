package masterplanner

import (
	"context"
	"net/http"
	"testing"
	"time"
)

func TestCreateTask(t *testing.T) {
	fg := NewFakeGateway()
	defer fg.Close()

	client := NewClient(WithBaseURL(fg.URL()))

	req := &CreateTaskRequest{
		WorkerID: "test-worker-1",
		ModelID:  "test-model-1",
		Prompt:   "Test prompt",
		Priority: TaskPriorityNormal,
		Metadata: map[string]string{"key": "value"},
	}

	resp, err := client.CreateTask(context.Background(), req)
	if err != nil {
		t.Fatalf("CreateTask failed: %v", err)
	}

	if resp.TaskID == "" {
		t.Error("TaskID is empty")
	}
	if resp.Status != TaskStatusPending {
		t.Errorf("Status = %s, want %s", resp.Status, TaskStatusPending)
	}
	if resp.CreatedAt.IsZero() {
		t.Error("CreatedAt is zero")
	}

	task := fg.GetTask(resp.TaskID)
	if task == nil {
		t.Fatal("Task not found in fake gateway")
	}
	if task.WorkerID != req.WorkerID {
		t.Errorf("WorkerID = %s, want %s", task.WorkerID, req.WorkerID)
	}
	if task.ModelID != req.ModelID {
		t.Errorf("ModelID = %s, want %s", task.ModelID, req.ModelID)
	}
	if task.Prompt != req.Prompt {
		t.Errorf("Prompt = %s, want %s", task.Prompt, req.Prompt)
	}
}

func TestBatchCreateTasks(t *testing.T) {
	fg := NewFakeGateway()
	defer fg.Close()

	client := NewClient(WithBaseURL(fg.URL()))

	req := &BatchCreateTasksRequest{
		Tasks: []*CreateTaskRequest{
			{
				WorkerID: "test-worker-1",
				ModelID:  "test-model-1",
				Prompt:   "Prompt 1",
			},
			{
				WorkerID: "test-worker-1",
				ModelID:  "test-model-1",
				Prompt:   "Prompt 2",
			},
		},
	}

	resp, err := client.BatchCreateTasks(context.Background(), req)
	if err != nil {
		t.Fatalf("BatchCreateTasks failed: %v", err)
	}

	if len(resp.Tasks) != 2 {
		t.Fatalf("got %d tasks, want 2", len(resp.Tasks))
	}

	for i, task := range resp.Tasks {
		if task.TaskID == "" {
			t.Errorf("Task %d: TaskID is empty", i)
		}
		if task.Status != TaskStatusPending {
			t.Errorf("Task %d: Status = %s, want %s", i, task.Status, TaskStatusPending)
		}
	}
}

func TestListTasks(t *testing.T) {
	fg := NewFakeGateway()
	defer fg.Close()

	now := time.Now()
	fg.AddTask(&Task{
		TaskID:    "task-1",
		WorkerID:  "worker-1",
		ModelID:   "model-1",
		Status:    TaskStatusPending,
		CreatedAt: now,
	})
	fg.AddTask(&Task{
		TaskID:    "task-2",
		WorkerID:  "worker-2",
		ModelID:   "model-1",
		Status:    TaskStatusRunning,
		CreatedAt: now,
	})

	client := NewClient(WithBaseURL(fg.URL()))

	t.Run("list all tasks", func(t *testing.T) {
		resp, err := client.ListTasks(context.Background(), &ListTasksRequest{})
		if err != nil {
			t.Fatalf("ListTasks failed: %v", err)
		}

		if len(resp.Tasks) != 2 {
			t.Fatalf("got %d tasks, want 2", len(resp.Tasks))
		}
		if resp.TotalCount != 2 {
			t.Errorf("TotalCount = %d, want 2", resp.TotalCount)
		}
	})

	t.Run("filter by worker", func(t *testing.T) {
		resp, err := client.ListTasks(context.Background(), &ListTasksRequest{
			WorkerID: "worker-1",
		})
		if err != nil {
			t.Fatalf("ListTasks failed: %v", err)
		}

		if len(resp.Tasks) != 1 {
			t.Fatalf("got %d tasks, want 1", len(resp.Tasks))
		}
		if resp.Tasks[0].WorkerID != "worker-1" {
			t.Errorf("WorkerID = %s, want worker-1", resp.Tasks[0].WorkerID)
		}
	})

	t.Run("filter by status", func(t *testing.T) {
		resp, err := client.ListTasks(context.Background(), &ListTasksRequest{
			Status: TaskStatusPending,
		})
		if err != nil {
			t.Fatalf("ListTasks failed: %v", err)
		}

		if len(resp.Tasks) != 1 {
			t.Fatalf("got %d tasks, want 1", len(resp.Tasks))
		}
		if resp.Tasks[0].Status != TaskStatusPending {
			t.Errorf("Status = %s, want %s", resp.Tasks[0].Status, TaskStatusPending)
		}
	})
}

func TestWatchTasks(t *testing.T) {
	fg := NewFakeGateway()
	defer fg.Close()

	now := time.Now()
	fg.AddTask(&Task{
		TaskID:    "task-1",
		WorkerID:  "worker-1",
		Status:    TaskStatusPending,
		CreatedAt: now,
	})

	client := NewClient(WithBaseURL(fg.URL()))

	stream, err := client.WatchTasks(context.Background(), &WatchTasksRequest{})
	if err != nil {
		t.Fatalf("WatchTasks failed: %v", err)
	}
	defer stream.Close()

	timeout := time.After(5 * time.Second)
	eventCount := 0

	for {
		select {
		case event := <-stream.Events:
			if event == nil {
				if eventCount == 0 {
					t.Error("No events received")
				}
				return
			}
			eventCount++
			if event.TaskID != "task-1" {
				t.Errorf("TaskID = %s, want task-1", event.TaskID)
			}
			if event.Kind != TaskEventKindCreated {
				t.Errorf("Kind = %s, want %s", event.Kind, TaskEventKindCreated)
			}
			return

		case err := <-stream.Errors:
			if err != nil {
				t.Fatalf("Stream error: %v", err)
			}
			return

		case <-timeout:
			t.Fatal("Timeout waiting for events")
		}
	}
}

func TestSubmitTaskResult(t *testing.T) {
	fg := NewFakeGateway()
	defer fg.Close()

	now := time.Now()
	fg.AddTask(&Task{
		TaskID:    "task-1",
		WorkerID:  "worker-1",
		Status:    TaskStatusRunning,
		CreatedAt: now,
	})

	client := NewClient(WithBaseURL(fg.URL()))

	req := &SubmitTaskResultRequest{
		TaskID: "task-1",
		Result: "Task completed successfully",
	}

	resp, err := client.SubmitTaskResult(context.Background(), "task-1", req)
	if err != nil {
		t.Fatalf("SubmitTaskResult failed: %v", err)
	}

	if resp.TaskID != "task-1" {
		t.Errorf("TaskID = %s, want task-1", resp.TaskID)
	}
	if resp.Status != TaskStatusCompleted {
		t.Errorf("Status = %s, want %s", resp.Status, TaskStatusCompleted)
	}
	if resp.CompletedAt.IsZero() {
		t.Error("CompletedAt is zero")
	}

	task := fg.GetTask("task-1")
	if task.Status != TaskStatusCompleted {
		t.Errorf("Task status = %s, want %s", task.Status, TaskStatusCompleted)
	}
	if task.Result != req.Result {
		t.Errorf("Task result = %s, want %s", task.Result, req.Result)
	}
}

func TestCancelTask(t *testing.T) {
	fg := NewFakeGateway()
	defer fg.Close()

	now := time.Now()
	fg.AddTask(&Task{
		TaskID:    "task-1",
		WorkerID:  "worker-1",
		Status:    TaskStatusRunning,
		CreatedAt: now,
	})

	client := NewClient(WithBaseURL(fg.URL()))

	req := &CancelTaskRequest{
		TaskID: "task-1",
		Reason: "User requested cancellation",
	}

	resp, err := client.CancelTask(context.Background(), "task-1", req)
	if err != nil {
		t.Fatalf("CancelTask failed: %v", err)
	}

	if resp.TaskID != "task-1" {
		t.Errorf("TaskID = %s, want task-1", resp.TaskID)
	}
	if resp.Status != TaskStatusCancelled {
		t.Errorf("Status = %s, want %s", resp.Status, TaskStatusCancelled)
	}
	if resp.CancelledAt.IsZero() {
		t.Error("CancelledAt is zero")
	}

	task := fg.GetTask("task-1")
	if task.Status != TaskStatusCancelled {
		t.Errorf("Task status = %s, want %s", task.Status, TaskStatusCancelled)
	}
}

func TestListWorkers(t *testing.T) {
	fg := NewFakeGateway()
	defer fg.Close()

	client := NewClient(WithBaseURL(fg.URL()))

	resp, err := client.ListWorkers(context.Background(), &ListWorkersRequest{})
	if err != nil {
		t.Fatalf("ListWorkers failed: %v", err)
	}

	if len(resp.Workers) == 0 {
		t.Error("No workers returned")
	}
	if resp.TotalCount == 0 {
		t.Error("TotalCount is zero")
	}

	worker := resp.Workers[0]
	if worker.WorkerID == "" {
		t.Error("WorkerID is empty")
	}
	if worker.Status == "" {
		t.Error("Status is empty")
	}
}

func TestListModels(t *testing.T) {
	fg := NewFakeGateway()
	defer fg.Close()

	client := NewClient(WithBaseURL(fg.URL()))

	resp, err := client.ListModels(context.Background(), &ListModelsRequest{})
	if err != nil {
		t.Fatalf("ListModels failed: %v", err)
	}

	if len(resp.Models) == 0 {
		t.Error("No models returned")
	}
	if resp.TotalCount == 0 {
		t.Error("TotalCount is zero")
	}

	model := resp.Models[0]
	if model.ModelID == "" {
		t.Error("ModelID is empty")
	}
	if model.Provider == "" {
		t.Error("Provider is empty")
	}
}

func TestJSONFieldNaming(t *testing.T) {
	fg := NewFakeGateway()
	defer fg.Close()

	client := NewClient(WithBaseURL(fg.URL()))

	req := &CreateTaskRequest{
		WorkerID: "test-worker",
		ModelID:  "test-model",
		Prompt:   "test",
		Metadata: map[string]string{"test_key": "test_value"},
	}

	resp, err := client.CreateTask(context.Background(), req)
	if err != nil {
		t.Fatalf("CreateTask failed: %v", err)
	}

	task := fg.GetTask(resp.TaskID)
	if task == nil {
		t.Fatal("Task not found")
	}

	if task.Metadata["test_key"] != "test_value" {
		t.Error("Metadata not properly preserved through JSON encoding")
	}
}

func TestClientTimeout(t *testing.T) {
	fg := NewFakeGateway()
	defer fg.Close()

	client := NewClient(
		WithBaseURL(fg.URL()),
		WithHTTPClient(&http.Client{Timeout: 1 * time.Millisecond}),
	)

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()

	time.Sleep(2 * time.Millisecond)

	_, err := client.CreateTask(ctx, &CreateTaskRequest{
		ModelID: "test",
		Prompt:  "test",
	})

	if err == nil {
		t.Error("Expected timeout error, got nil")
	}
}

func TestTaskStatusEnum(t *testing.T) {
	statuses := []TaskStatus{
		TaskStatusPending,
		TaskStatusRunning,
		TaskStatusCompleted,
		TaskStatusFailed,
		TaskStatusCancelled,
	}

	for _, status := range statuses {
		if status == "" {
			t.Errorf("Status should not be empty")
		}
	}
}

func TestTaskEventKindEnum(t *testing.T) {
	kinds := []TaskEventKind{
		TaskEventKindCreated,
		TaskEventKindStarted,
		TaskEventKindProgress,
		TaskEventKindCompleted,
		TaskEventKindFailed,
		TaskEventKindCancelled,
		TaskEventKindHeartbeat,
	}

	for _, kind := range kinds {
		if kind == "" {
			t.Errorf("Kind should not be empty")
		}
	}
}

func TestSSEErrorCodes(t *testing.T) {
	codes := []SSEErrorCode{
		SSEErrorCodeCursorOutOfRange,
		SSEErrorCodeSlowConsumer,
		SSEErrorCodeUnknown,
	}

	for _, code := range codes {
		if code == "" {
			t.Errorf("Error code should not be empty")
		}
	}

	if SSEErrorCodeCursorOutOfRange != "cursor_out_of_range" {
		t.Errorf("cursor_out_of_range = %s", SSEErrorCodeCursorOutOfRange)
	}
	if SSEErrorCodeSlowConsumer != "slow_consumer" {
		t.Errorf("slow_consumer = %s", SSEErrorCodeSlowConsumer)
	}
}
