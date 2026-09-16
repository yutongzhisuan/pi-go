package fake

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"time"
)

// Gateway is a fake Gateway-API AgentRelayService for testing.
// It mimics Hermes wire behavior from extend/master_planner.
type Gateway struct {
	mu     sync.Mutex
	tasks  map[string]*Task
	models []Model
	server *httptest.Server
}

// Task represents a fake task on the Gateway.
type Task struct {
	TaskID             string                 `json:"task_id"`
	Goal               string                 `json:"goal"`
	Status             string                 `json:"status"`
	Summary            string                 `json:"summary"`
	ResultText         string                 `json:"result_text"`
	LatestCheckpointID string                 `json:"latest_checkpoint_id"`
	Error              string                 `json:"error,omitempty"`
	BatchID            string                 `json:"batch_id,omitempty"`
	ProgressSummary    string                 `json:"progress_summary,omitempty"`
	Events             []Event                `json:"-"`
	Metadata           map[string]interface{} `json:"-"`
}

// Event represents an SSE event.
type Event struct {
	EventID string                 `json:"event_id"`
	Kind    string                 `json:"kind"`
	TaskID  string                 `json:"task_id"`
	Data    map[string]interface{} `json:"data"`
}

// Model represents a schedulable model.
type Model struct {
	ModelVersionID string   `json:"model_version_id"`
	DisplayName    string   `json:"display_name"`
	NodeCount      int      `json:"node_count"`
	AvailableSlots int      `json:"available_slots"`
	Regions        []string `json:"regions"`
}

// NewGateway creates a new fake Gateway server.
func NewGateway() *Gateway {
	g := &Gateway{
		tasks: make(map[string]*Task),
		models: []Model{
			{
				ModelVersionID: "gpt-4-turbo",
				DisplayName:    "GPT-4 Turbo",
				NodeCount:      10,
				AvailableSlots: 50,
				Regions:        []string{"us-west", "eu-central"},
			},
			{
				ModelVersionID: "claude-opus-4",
				DisplayName:    "Claude Opus 4",
				NodeCount:      5,
				AvailableSlots: 25,
				Regions:        []string{"us-west"},
			},
		},
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/v1/agent/tasks", g.handleTasks)
	mux.HandleFunc("/v1/agent/tasks:batch", g.handleBatch)
	mux.HandleFunc("/v1/agent/tasks:watch", g.handleWatch)
	mux.HandleFunc("/v1/agent/models:list", g.handleListModels)
	mux.HandleFunc("/v1/agent/workers:list", g.handleListWorkers)

	// Handle task-specific routes with path matching
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, ":result") {
			g.handleGetResult(w, r)
		} else if strings.Contains(r.URL.Path, ":cancel") {
			g.handleCancel(w, r)
		} else {
			http.NotFound(w, r)
		}
	})

	g.server = httptest.NewServer(mux)
	return g
}

// URL returns the fake Gateway's base URL.
func (g *Gateway) URL() string {
	return g.server.URL
}

// Close shuts down the fake Gateway.
func (g *Gateway) Close() {
	g.server.Close()
}

// SetTaskStatus updates a task's status and adds a terminal event.
func (g *Gateway) SetTaskStatus(taskID, status, summary string) {
	g.mu.Lock()
	defer g.mu.Unlock()

	task, ok := g.tasks[taskID]
	if !ok {
		return
	}

	task.Status = status
	task.Summary = summary

	// Add terminal event
	eventID := fmt.Sprintf("%d", len(task.Events)+1)
	task.Events = append(task.Events, Event{
		EventID: eventID,
		Kind:    "TASK_EVENT_KIND_TERMINAL",
		TaskID:  taskID,
		Data: map[string]interface{}{
			"task_id": taskID,
			"result": map[string]interface{}{
				"status":  "TASK_STATUS_" + strings.ToUpper(status),
				"summary": summary,
			},
		},
	})
}

// AddProgressEvent adds a progress event to a task.
func (g *Gateway) AddProgressEvent(taskID, progressSummary string) {
	g.mu.Lock()
	defer g.mu.Unlock()

	task, ok := g.tasks[taskID]
	if !ok {
		return
	}

	task.ProgressSummary = progressSummary
	eventID := fmt.Sprintf("%d", len(task.Events)+1)
	task.Events = append(task.Events, Event{
		EventID: eventID,
		Kind:    "TASK_EVENT_KIND_PROGRESS",
		TaskID:  taskID,
		Data: map[string]interface{}{
			"task_id":          taskID,
			"progress_summary": progressSummary,
		},
	})
}

// AddCheckpointEvent adds a checkpoint event to a task.
func (g *Gateway) AddCheckpointEvent(taskID, checkpointID, summary string) {
	g.mu.Lock()
	defer g.mu.Unlock()

	task, ok := g.tasks[taskID]
	if !ok {
		return
	}

	task.LatestCheckpointID = checkpointID
	eventID := fmt.Sprintf("%d", len(task.Events)+1)
	task.Events = append(task.Events, Event{
		EventID: eventID,
		Kind:    "TASK_EVENT_KIND_CHECKPOINT",
		TaskID:  taskID,
		Data: map[string]interface{}{
			"task_id": taskID,
			"checkpoint": map[string]interface{}{
				"checkpoint_id": checkpointID,
				"summary":       summary,
			},
		},
	})
}

// GetTask returns a task by ID (for testing).
func (g *Gateway) GetTask(taskID string) *Task {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.tasks[taskID]
}

func (g *Gateway) handleTasks(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		g.handleListTasks(w, r)
		return
	}

	if r.Method != "POST" {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Spec struct {
			TaskID  string `json:"task_id"`
			Goal    string `json:"goal"`
			Model   string `json:"model"`
			BatchID string `json:"batch_id"`
		} `json:"spec"`
		MasterSessionID string `json:"master_session_id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	g.mu.Lock()
	defer g.mu.Unlock()

	task := &Task{
		TaskID:  req.Spec.TaskID,
		Goal:    req.Spec.Goal,
		Status:  "pending",
		BatchID: req.Spec.BatchID,
		Events:  []Event{},
	}

	g.tasks[req.Spec.TaskID] = task

	resp := map[string]interface{}{
		"task_id":        task.TaskID,
		"status":         "TASK_STATUS_PENDING",
		"idempotent_hit": false,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (g *Gateway) handleBatch(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Specs           []map[string]interface{} `json:"specs"`
		BatchID         string                   `json:"batch_id"`
		MasterSessionID string                   `json:"master_session_id"`
		Policy          map[string]interface{}   `json:"policy"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	g.mu.Lock()
	defer g.mu.Unlock()

	for _, spec := range req.Specs {
		taskID := spec["task_id"].(string)
		goal := spec["goal"].(string)

		task := &Task{
			TaskID:  taskID,
			Goal:    goal,
			Status:  "pending",
			BatchID: req.BatchID,
			Events:  []Event{},
		}
		g.tasks[taskID] = task
	}

	resp := map[string]interface{}{
		"batch_id": req.BatchID,
		"count":    len(req.Specs),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (g *Gateway) handleWatch(w http.ResponseWriter, r *http.Request) {
	var req struct {
		TaskID       string `json:"task_id"`
		BatchID      string `json:"batch_id"`
		SinceEventID string `json:"since_event_id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	g.mu.Lock()
	var tasksToWatch []*Task
	if req.TaskID != "" {
		if task := g.tasks[req.TaskID]; task != nil {
			tasksToWatch = append(tasksToWatch, task)
		}
	} else if req.BatchID != "" {
		// Find all tasks in batch
		for _, t := range g.tasks {
			if t.BatchID == req.BatchID {
				tasksToWatch = append(tasksToWatch, t)
			}
		}
	}
	g.mu.Unlock()

	if len(tasksToWatch) == 0 {
		return
	}

	// Send all events from all watched tasks
	for _, task := range tasksToWatch {
		g.mu.Lock()
		events := task.Events
		g.mu.Unlock()

		for _, event := range events {
			// Build full event data structure matching Hermes format
			fullData := map[string]interface{}{
				"event_id": event.EventID,
				"kind":     event.Kind,
			}
			// Merge event.Data into fullData
			for k, v := range event.Data {
				fullData[k] = v
			}
			
			data, _ := json.Marshal(fullData)
			fmt.Fprintf(w, "event: message\n")
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
			time.Sleep(5 * time.Millisecond)
		}
	}
}

func (g *Gateway) handleGetResult(w http.ResponseWriter, r *http.Request) {
	// Extract task_id from path like /v1/agent/tasks/{task_id}:result
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) < 5 {
		http.Error(w, "invalid path", http.StatusBadRequest)
		return
	}
	taskIDPart := parts[4]
	taskID := strings.TrimSuffix(taskIDPart, ":result")

	g.mu.Lock()
	task, ok := g.tasks[taskID]
	g.mu.Unlock()

	if !ok {
		http.Error(w, "task not found", http.StatusNotFound)
		return
	}

	resp := map[string]interface{}{
		"task_id":              task.TaskID,
		"status":               "TASK_STATUS_" + strings.ToUpper(task.Status),
		"summary":              task.Summary,
		"result_text":          task.ResultText,
		"latest_checkpoint_id": task.LatestCheckpointID,
		"error":                task.Error,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (g *Gateway) handleListTasks(w http.ResponseWriter, r *http.Request) {
	g.mu.Lock()
	defer g.mu.Unlock()

	var tasks []map[string]interface{}
	for _, task := range g.tasks {
		tasks = append(tasks, map[string]interface{}{
			"task_id": task.TaskID,
			"status":  "TASK_STATUS_" + strings.ToUpper(task.Status),
			"goal":    task.Goal,
		})
	}

	resp := map[string]interface{}{
		"tasks": tasks,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (g *Gateway) handleListModels(w http.ResponseWriter, r *http.Request) {
	resp := map[string]interface{}{
		"models": g.models,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (g *Gateway) handleListWorkers(w http.ResponseWriter, r *http.Request) {
	workers := []map[string]interface{}{
		{
			"worker_id": "worker-1",
			"toolsets":  []string{"python", "bash", "web"},
			"status":    "ready",
		},
		{
			"worker_id": "worker-2",
			"toolsets":  []string{"python", "nodejs"},
			"status":    "ready",
		},
	}

	resp := map[string]interface{}{
		"workers": workers,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (g *Gateway) handleCancel(w http.ResponseWriter, r *http.Request) {
	// Extract task_id from path
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) < 5 {
		http.Error(w, "invalid path", http.StatusBadRequest)
		return
	}
	taskIDPart := parts[4]
	taskID := strings.TrimSuffix(taskIDPart, ":cancel")

	var req struct {
		BatchID string `json:"batch_id"`
		Reason  string `json:"reason"`
	}
	json.NewDecoder(r.Body).Decode(&req)

	g.mu.Lock()
	defer g.mu.Unlock()

	if req.BatchID != "" {
		// Cancel all tasks in batch
		for _, task := range g.tasks {
			if task.BatchID == req.BatchID && task.Status != "completed" {
				task.Status = "cancelled"
			}
		}
	} else {
		// Cancel single task
		if task, ok := g.tasks[taskID]; ok {
			task.Status = "cancelled"
		}
	}

	resp := map[string]interface{}{
		"cancelled": true,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
