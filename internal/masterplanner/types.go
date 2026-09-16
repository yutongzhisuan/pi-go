package masterplanner

import "time"

type TaskStatus string

const (
	TaskStatusPending   TaskStatus = "PENDING"
	TaskStatusRunning   TaskStatus = "RUNNING"
	TaskStatusCompleted TaskStatus = "COMPLETED"
	TaskStatusFailed    TaskStatus = "FAILED"
	TaskStatusCancelled TaskStatus = "CANCELLED"
)

type TaskEventKind string

const (
	TaskEventKindCreated    TaskEventKind = "CREATED"
	TaskEventKindStarted    TaskEventKind = "STARTED"
	TaskEventKindProgress   TaskEventKind = "PROGRESS"
	TaskEventKindCompleted  TaskEventKind = "COMPLETED"
	TaskEventKindFailed     TaskEventKind = "FAILED"
	TaskEventKindCancelled  TaskEventKind = "CANCELLED"
	TaskEventKindHeartbeat  TaskEventKind = "HEARTBEAT"
)

type TaskPriority int32

const (
	TaskPriorityLow    TaskPriority = 0
	TaskPriorityNormal TaskPriority = 1
	TaskPriorityHigh   TaskPriority = 2
)

type CreateTaskRequest struct {
	WorkerID    string                 `json:"worker_id,omitempty"`
	ModelID     string                 `json:"model_id"`
	Prompt      string                 `json:"prompt"`
	Priority    TaskPriority           `json:"priority,omitempty"`
	Metadata    map[string]string      `json:"metadata,omitempty"`
	Timeout     int64                  `json:"timeout,omitempty"`
	MaxRetries  int32                  `json:"max_retries,omitempty"`
	Context     map[string]interface{} `json:"context,omitempty"`
}

type CreateTaskResponse struct {
	TaskID    string     `json:"task_id"`
	Status    TaskStatus `json:"status"`
	CreatedAt time.Time  `json:"created_at"`
}

type BatchCreateTasksRequest struct {
	Tasks []*CreateTaskRequest `json:"tasks"`
}

type BatchCreateTasksResponse struct {
	Tasks []*CreateTaskResponse `json:"tasks"`
}

type ListTasksRequest struct {
	WorkerID   string     `json:"worker_id,omitempty"`
	Status     TaskStatus `json:"status,omitempty"`
	PageSize   int32      `json:"page_size,omitempty"`
	PageToken  string     `json:"page_token,omitempty"`
}

type Task struct {
	TaskID      string                 `json:"task_id"`
	WorkerID    string                 `json:"worker_id"`
	ModelID     string                 `json:"model_id"`
	Prompt      string                 `json:"prompt"`
	Status      TaskStatus             `json:"status"`
	Priority    TaskPriority           `json:"priority"`
	Result      string                 `json:"result,omitempty"`
	Error       string                 `json:"error,omitempty"`
	Metadata    map[string]string      `json:"metadata,omitempty"`
	Context     map[string]interface{} `json:"context,omitempty"`
	CreatedAt   time.Time              `json:"created_at"`
	StartedAt   *time.Time             `json:"started_at,omitempty"`
	CompletedAt *time.Time             `json:"completed_at,omitempty"`
	RetryCount  int32                  `json:"retry_count"`
}

type ListTasksResponse struct {
	Tasks         []*Task `json:"tasks"`
	NextPageToken string  `json:"next_page_token,omitempty"`
	TotalCount    int64   `json:"total_count,omitempty"`
}

type WatchTasksRequest struct {
	WorkerID string     `json:"worker_id,omitempty"`
	Status   TaskStatus `json:"status,omitempty"`
}

type TaskEvent struct {
	EventID    string        `json:"event_id"`
	TaskID     string        `json:"task_id"`
	Kind       TaskEventKind `json:"kind"`
	Message    string        `json:"message,omitempty"`
	Data       string        `json:"data,omitempty"`
	Timestamp  time.Time     `json:"timestamp"`
	SequenceID int64         `json:"sequence_id"`
}

type SubmitTaskResultRequest struct {
	TaskID string `json:"task_id"`
	Result string `json:"result"`
	Error  string `json:"error,omitempty"`
}

type SubmitTaskResultResponse struct {
	TaskID      string     `json:"task_id"`
	Status      TaskStatus `json:"status"`
	CompletedAt time.Time  `json:"completed_at"`
}

type CancelTaskRequest struct {
	TaskID string `json:"task_id"`
	Reason string `json:"reason,omitempty"`
}

type CancelTaskResponse struct {
	TaskID      string     `json:"task_id"`
	Status      TaskStatus `json:"status"`
	CancelledAt time.Time  `json:"cancelled_at"`
}

type ListWorkersRequest struct {
	Status    string `json:"status,omitempty"`
	PageSize  int32  `json:"page_size,omitempty"`
	PageToken string `json:"page_token,omitempty"`
}

type Worker struct {
	WorkerID     string            `json:"worker_id"`
	Status       string            `json:"status"`
	Capabilities []string          `json:"capabilities,omitempty"`
	Metadata     map[string]string `json:"metadata,omitempty"`
	LastSeenAt   time.Time         `json:"last_seen_at"`
	RegisteredAt time.Time         `json:"registered_at"`
}

type ListWorkersResponse struct {
	Workers       []*Worker `json:"workers"`
	NextPageToken string    `json:"next_page_token,omitempty"`
	TotalCount    int64     `json:"total_count,omitempty"`
}

type ListModelsRequest struct {
	Provider  string `json:"provider,omitempty"`
	PageSize  int32  `json:"page_size,omitempty"`
	PageToken string `json:"page_token,omitempty"`
}

type Model struct {
	ModelID       string            `json:"model_id"`
	Provider      string            `json:"provider"`
	DisplayName   string            `json:"display_name"`
	Capabilities  []string          `json:"capabilities,omitempty"`
	ContextWindow int64             `json:"context_window,omitempty"`
	Metadata      map[string]string `json:"metadata,omitempty"`
}

type ListModelsResponse struct {
	Models        []*Model `json:"models"`
	NextPageToken string   `json:"next_page_token,omitempty"`
	TotalCount    int64    `json:"total_count,omitempty"`
}

type SSEError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

const (
	SSEErrorCodeCursorOutOfRange SSEErrorCode = "cursor_out_of_range"
	SSEErrorCodeSlowConsumer     SSEErrorCode = "slow_consumer"
	SSEErrorCodeUnknown          SSEErrorCode = "unknown"
)

type SSEErrorCode string
