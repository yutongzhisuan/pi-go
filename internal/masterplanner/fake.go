package masterplanner

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"time"
)

type FakeGateway struct {
	srv       *httptest.Server
	mu        sync.RWMutex
	tasks     map[string]*Task
	workers   map[string]*Worker
	models    map[string]*Model
	events    map[string][]*TaskEvent
	nextID    int
	streaming bool
}

func NewFakeGateway() *FakeGateway {
	fg := &FakeGateway{
		tasks:   make(map[string]*Task),
		workers: make(map[string]*Worker),
		models:  make(map[string]*Model),
		events:  make(map[string][]*TaskEvent),
	}

	fg.srv = httptest.NewServer(http.HandlerFunc(fg.handler))

	fg.models["test-model-1"] = &Model{
		ModelID:       "test-model-1",
		Provider:      "test-provider",
		DisplayName:   "Test Model 1",
		Capabilities:  []string{"chat", "completion"},
		ContextWindow: 8192,
	}

	fg.workers["test-worker-1"] = &Worker{
		WorkerID:     "test-worker-1",
		Status:       "active",
		Capabilities: []string{"chat", "completion"},
		LastSeenAt:   time.Now(),
		RegisteredAt: time.Now().Add(-1 * time.Hour),
	}

	return fg
}

func (fg *FakeGateway) Close() {
	fg.srv.Close()
}

func (fg *FakeGateway) URL() string {
	return fg.srv.URL
}

func (fg *FakeGateway) handler(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.Method == http.MethodPost && r.URL.Path == "/v1/agent/tasks":
		fg.handleCreateTask(w, r)
	case r.Method == http.MethodPost && r.URL.Path == "/v1/agent/tasks:batch":
		fg.handleBatchCreateTasks(w, r)
	case r.Method == http.MethodGet && r.URL.Path == "/v1/agent/tasks":
		fg.handleListTasks(w, r)
	case r.Method == http.MethodPost && r.URL.Path == "/v1/agent/tasks:watch":
		fg.handleWatchTasks(w, r)
	case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, ":result"):
		fg.handleSubmitTaskResult(w, r)
	case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, ":cancel"):
		fg.handleCancelTask(w, r)
	case r.Method == http.MethodPost && r.URL.Path == "/v1/agent/workers:list":
		fg.handleListWorkers(w, r)
	case r.Method == http.MethodPost && r.URL.Path == "/v1/agent/models:list":
		fg.handleListModels(w, r)
	default:
		http.NotFound(w, r)
	}
}

func (fg *FakeGateway) handleCreateTask(w http.ResponseWriter, r *http.Request) {
	var req CreateTaskRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	fg.mu.Lock()
	defer fg.mu.Unlock()

	fg.nextID++
	taskID := fmt.Sprintf("task-%d", fg.nextID)
	now := time.Now()

	task := &Task{
		TaskID:     taskID,
		WorkerID:   req.WorkerID,
		ModelID:    req.ModelID,
		Prompt:     req.Prompt,
		Status:     TaskStatusPending,
		Priority:   req.Priority,
		Metadata:   req.Metadata,
		Context:    req.Context,
		CreatedAt:  now,
		RetryCount: 0,
	}

	fg.tasks[taskID] = task

	event := &TaskEvent{
		EventID:    fmt.Sprintf("evt-%d-1", fg.nextID),
		TaskID:     taskID,
		Kind:       TaskEventKindCreated,
		Message:    "Task created",
		Timestamp:  now,
		SequenceID: 1,
	}
	fg.events[taskID] = append(fg.events[taskID], event)

	resp := &CreateTaskResponse{
		TaskID:    taskID,
		Status:    TaskStatusPending,
		CreatedAt: now,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (fg *FakeGateway) handleBatchCreateTasks(w http.ResponseWriter, r *http.Request) {
	var req BatchCreateTasksRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	fg.mu.Lock()
	defer fg.mu.Unlock()

	var tasks []*CreateTaskResponse
	for _, taskReq := range req.Tasks {
		fg.nextID++
		taskID := fmt.Sprintf("task-%d", fg.nextID)
		now := time.Now()

		task := &Task{
			TaskID:     taskID,
			WorkerID:   taskReq.WorkerID,
			ModelID:    taskReq.ModelID,
			Prompt:     taskReq.Prompt,
			Status:     TaskStatusPending,
			Priority:   taskReq.Priority,
			Metadata:   taskReq.Metadata,
			Context:    taskReq.Context,
			CreatedAt:  now,
			RetryCount: 0,
		}

		fg.tasks[taskID] = task

		event := &TaskEvent{
			EventID:    fmt.Sprintf("evt-%d-1", fg.nextID),
			TaskID:     taskID,
			Kind:       TaskEventKindCreated,
			Message:    "Task created",
			Timestamp:  now,
			SequenceID: 1,
		}
		fg.events[taskID] = append(fg.events[taskID], event)

		tasks = append(tasks, &CreateTaskResponse{
			TaskID:    taskID,
			Status:    TaskStatusPending,
			CreatedAt: now,
		})
	}

	resp := &BatchCreateTasksResponse{
		Tasks: tasks,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (fg *FakeGateway) handleListTasks(w http.ResponseWriter, r *http.Request) {
	fg.mu.RLock()
	defer fg.mu.RUnlock()

	workerID := r.URL.Query().Get("worker_id")
	statusStr := r.URL.Query().Get("status")

	var tasks []*Task
	for _, task := range fg.tasks {
		if workerID != "" && task.WorkerID != workerID {
			continue
		}
		if statusStr != "" && string(task.Status) != statusStr {
			continue
		}
		tasks = append(tasks, task)
	}

	resp := &ListTasksResponse{
		Tasks:      tasks,
		TotalCount: int64(len(tasks)),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (fg *FakeGateway) handleWatchTasks(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "SSE not supported", http.StatusInternalServerError)
		return
	}

	fg.mu.RLock()
	var allEvents []*TaskEvent
	for _, events := range fg.events {
		allEvents = append(allEvents, events...)
	}
	fg.mu.RUnlock()

	for _, event := range allEvents {
		data, err := json.Marshal(event)
		if err != nil {
			continue
		}

		fmt.Fprintf(w, "event: task_event\n")
		fmt.Fprintf(w, "data: %s\n\n", data)
		flusher.Flush()

		if fg.streaming {
			time.Sleep(100 * time.Millisecond)
		}
	}
}

func (fg *FakeGateway) handleSubmitTaskResult(w http.ResponseWriter, r *http.Request) {
	taskID := extractTaskID(r.URL.Path, ":result")
	if taskID == "" {
		http.Error(w, "invalid task ID", http.StatusBadRequest)
		return
	}

	var req SubmitTaskResultRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	fg.mu.Lock()
	defer fg.mu.Unlock()

	task, exists := fg.tasks[taskID]
	if !exists {
		http.Error(w, "task not found", http.StatusNotFound)
		return
	}

	now := time.Now()
	task.Status = TaskStatusCompleted
	task.Result = req.Result
	task.Error = req.Error
	task.CompletedAt = &now

	event := &TaskEvent{
		EventID:    fmt.Sprintf("evt-%s-complete", taskID),
		TaskID:     taskID,
		Kind:       TaskEventKindCompleted,
		Message:    "Task completed",
		Data:       req.Result,
		Timestamp:  now,
		SequenceID: int64(len(fg.events[taskID]) + 1),
	}
	fg.events[taskID] = append(fg.events[taskID], event)

	resp := &SubmitTaskResultResponse{
		TaskID:      taskID,
		Status:      TaskStatusCompleted,
		CompletedAt: now,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (fg *FakeGateway) handleCancelTask(w http.ResponseWriter, r *http.Request) {
	taskID := extractTaskID(r.URL.Path, ":cancel")
	if taskID == "" {
		http.Error(w, "invalid task ID", http.StatusBadRequest)
		return
	}

	var req CancelTaskRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	fg.mu.Lock()
	defer fg.mu.Unlock()

	task, exists := fg.tasks[taskID]
	if !exists {
		http.Error(w, "task not found", http.StatusNotFound)
		return
	}

	now := time.Now()
	task.Status = TaskStatusCancelled

	event := &TaskEvent{
		EventID:    fmt.Sprintf("evt-%s-cancel", taskID),
		TaskID:     taskID,
		Kind:       TaskEventKindCancelled,
		Message:    req.Reason,
		Timestamp:  now,
		SequenceID: int64(len(fg.events[taskID]) + 1),
	}
	fg.events[taskID] = append(fg.events[taskID], event)

	resp := &CancelTaskResponse{
		TaskID:      taskID,
		Status:      TaskStatusCancelled,
		CancelledAt: now,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (fg *FakeGateway) handleListWorkers(w http.ResponseWriter, r *http.Request) {
	fg.mu.RLock()
	defer fg.mu.RUnlock()

	var workers []*Worker
	for _, worker := range fg.workers {
		workers = append(workers, worker)
	}

	resp := &ListWorkersResponse{
		Workers:    workers,
		TotalCount: int64(len(workers)),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (fg *FakeGateway) handleListModels(w http.ResponseWriter, r *http.Request) {
	fg.mu.RLock()
	defer fg.mu.RUnlock()

	var models []*Model
	for _, model := range fg.models {
		models = append(models, model)
	}

	resp := &ListModelsResponse{
		Models:     models,
		TotalCount: int64(len(models)),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func extractTaskID(path, suffix string) string {
	path = strings.TrimSuffix(path, suffix)
	parts := strings.Split(path, "/")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return ""
}

func (fg *FakeGateway) AddTask(task *Task) {
	fg.mu.Lock()
	defer fg.mu.Unlock()
	fg.tasks[task.TaskID] = task
}

func (fg *FakeGateway) GetTask(taskID string) *Task {
	fg.mu.RLock()
	defer fg.mu.RUnlock()
	return fg.tasks[taskID]
}

func (fg *FakeGateway) AddWorker(worker *Worker) {
	fg.mu.Lock()
	defer fg.mu.Unlock()
	fg.workers[worker.WorkerID] = worker
}

func (fg *FakeGateway) AddModel(model *Model) {
	fg.mu.Lock()
	defer fg.mu.Unlock()
	fg.models[model.ModelID] = model
}

func (fg *FakeGateway) SetStreaming(enabled bool) {
	fg.mu.Lock()
	defer fg.mu.Unlock()
	fg.streaming = enabled
}
