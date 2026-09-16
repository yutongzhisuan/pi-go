package masterplanner

import "time"

// TaskSpec defines a task dispatch request (matches Gateway-API AgentRelayService.DispatchTask).
type TaskSpec struct {
	TaskID         string            `json:"task_id"`
	Goal           string            `json:"goal"`
	Model          string            `json:"model,omitempty"`
	Toolsets       []string          `json:"toolsets,omitempty"`
	Params         map[string]any    `json:"params,omitempty"`
	Context        *TaskContext      `json:"context,omitempty"`
	TimeoutSeconds int               `json:"timeout_seconds,omitempty"`
	Priority       int               `json:"priority,omitempty"`
	DependsOn      []string          `json:"depends_on,omitempty"`
	ResumeFromCheckpoint string      `json:"resume_from_checkpoint,omitempty"`
}

// TaskContext carries background information for a task.
// Large contexts (>48 KiB) are gzip+base64 encoded into InlineGzip automatically.
type TaskContext struct {
	Inline     string `json:"inline,omitempty"`
	InlineGzip string `json:"inline_gzip,omitempty"`
}

// TaskResult is the terminal outcome of a task.
type TaskResult struct {
	TaskID              string `json:"task_id"`
	Status              string `json:"status"`
	Summary             string `json:"summary"`
	ResultText          string `json:"result_text"`
	LatestCheckpointID  string `json:"latest_checkpoint_id"`
	Error               string `json:"error,omitempty"`
}

// TaskEvent represents one SSE event from the watch stream.
type TaskEvent struct {
	ID      string `json:"id"`
	Type    string `json:"type"` // progress, terminal, status, checkpoint, aggregate
	TaskID  string `json:"task_id"`
	Data    any    `json:"data"`
}

// TaskStatus enumerates the valid task states.
type TaskStatus string

const (
	TaskStatusPending   TaskStatus = "pending"
	TaskStatusRunning   TaskStatus = "running"
	TaskStatusCompleted TaskStatus = "completed"
	TaskStatusFailed    TaskStatus = "failed"
	TaskStatusLost      TaskStatus = "lost"
	TaskStatusCancelled TaskStatus = "cancelled"
)

// TerminalStatuses is the set of task states that will not change.
var TerminalStatuses = map[TaskStatus]bool{
	TaskStatusCompleted: true,
	TaskStatusFailed:    true,
	TaskStatusLost:      true,
	TaskStatusCancelled: true,
}

// IsTerminal returns true if the status represents a final state.
func (s TaskStatus) IsTerminal() bool {
	return TerminalStatuses[s]
}

// DispatchTaskInput is the input for gateway_dispatch_task.
type DispatchTaskInput struct {
	Goal                 string         `json:"goal"`
	Model                string         `json:"model,omitempty"`
	Toolsets             []string       `json:"toolsets,omitempty"`
	Params               map[string]any `json:"params,omitempty"`
	Context              any            `json:"context,omitempty"`
	TimeoutSeconds       int            `json:"timeout_seconds,omitempty"`
	Priority             int            `json:"priority,omitempty"`
	DependsOn            []string       `json:"depends_on,omitempty"`
	ResumeFromCheckpoint string         `json:"resume_from_checkpoint,omitempty"`
	ResumeSummary        string         `json:"resume_summary,omitempty"`
}

// DispatchTaskOutput is the output for gateway_dispatch_task.
type DispatchTaskOutput struct {
	TaskID        string `json:"task_id"`
	RunID         string `json:"run_id"`
	Status        string `json:"status"`
	IdempotentHit bool   `json:"idempotent_hit"`
	Note          string `json:"note"`
}

// DispatchBatchInput is the input for gateway_dispatch_batch.
type DispatchBatchInput struct {
	Specs      []DispatchTaskInput `json:"specs"`
	JoinPolicy string              `json:"join_policy,omitempty"` // all, any, majority
}

// DispatchBatchOutput is the output for gateway_dispatch_batch.
type DispatchBatchOutput struct {
	BatchID string   `json:"batch_id"`
	TaskIDs []string `json:"task_ids"`
	RunID   string   `json:"run_id"`
	Count   int      `json:"count"`
	Note    string   `json:"note"`
}

// WatchTaskInput is the input for gateway_watch_task.
type WatchTaskInput struct {
	TaskID       string  `json:"task_id,omitempty"`
	BatchID      string  `json:"batch_id,omitempty"`
	WaitSeconds  float64 `json:"wait_seconds,omitempty"`
	SinceEventID string  `json:"since_event_id,omitempty"`
}

// WatchTaskOutput is the output for gateway_watch_task.
type WatchTaskOutput struct {
	Reason       string                       `json:"reason"` // terminal, timeout, interrupted, error, stream_closed
	Interrupted  bool                         `json:"interrupted"`
	Cursor       string                       `json:"cursor"`
	Progress     map[string]string            `json:"progress"`
	Checkpoints  map[string]string            `json:"checkpoints"`
	Terminal     []TerminalEvent              `json:"terminal"`
	Events       []map[string]any             `json:"events"`
	Error        map[string]any               `json:"error,omitempty"`
	Message      string                       `json:"message,omitempty"`
}

// TerminalEvent represents a terminal task event.
type TerminalEvent struct {
	TaskID  string `json:"task_id"`
	Status  string `json:"status"`
	Summary string `json:"summary"`
}

// GetTaskResultInput is the input for gateway_get_task_result.
type GetTaskResultInput struct {
	TaskID string `json:"task_id"`
}

// GetTaskResultOutput is the output for gateway_get_task_result.
type GetTaskResultOutput struct {
	TaskID             string `json:"task_id"`
	Status             string `json:"status"`
	Summary            string `json:"summary"`
	ResultText         string `json:"result_text"`
	LatestCheckpointID string `json:"latest_checkpoint_id"`
	Error              string `json:"error"`
	Note               string `json:"note"`
}

// ListTasksInput is the input for gateway_list_tasks.
type ListTasksInput struct {
	BatchID string `json:"batch_id,omitempty"`
	Status  string `json:"status,omitempty"`
}

// ListTasksOutput is the output for gateway_list_tasks.
type ListTasksOutput struct {
	MasterSessionID string         `json:"master_session_id"`
	Tasks           []map[string]any `json:"tasks"`
	LocallyOpen     []string       `json:"locally_open"`
	Note            string         `json:"note"`
}

// ListModelsInput is the input for gateway_list_models.
type ListModelsInput struct {
	Region string `json:"region,omitempty"`
}

// ListModelsOutput is the output for gateway_list_models.
type ListModelsOutput struct {
	Models []map[string]any `json:"models"`
	Count  int              `json:"count"`
	Note   string           `json:"note"`
}

// ListWorkersInput is the input for gateway_list_workers.
type ListWorkersInput struct {
	RequireToolsets []string `json:"require_toolsets,omitempty"`
}

// ListWorkersOutput is the output for gateway_list_workers.
type ListWorkersOutput struct {
	Workers           []map[string]any `json:"workers"`
	AvailableToolsets []string         `json:"available_toolsets"`
	Count             int              `json:"count"`
}

// CancelTaskInput is the input for gateway_cancel_task.
type CancelTaskInput struct {
	TaskID  string `json:"task_id,omitempty"`
	BatchID string `json:"batch_id,omitempty"`
	Reason  string `json:"reason,omitempty"`
}

// CancelTaskOutput is the output for gateway_cancel_task.
type CancelTaskOutput struct {
	Cancelled bool           `json:"cancelled"`
	TaskID    string         `json:"task_id"`
	BatchID   string         `json:"batch_id"`
	Response  map[string]any `json:"response"`
}

// TaskRecord is a row from the ledger.
type TaskRecord struct {
	TaskID            string
	RunID             string
	BatchID           string
	Goal              string
	Status            TaskStatus
	CursorEventID     string
	GatewayInstanceID string
	SubmittedAt       time.Time
	UpdatedAt         time.Time
}
