package masterplanner

import "time"

// TaskSpec defines a task dispatch request (matches Gateway-API AgentRelayService.DispatchTask).
type TaskSpec struct {
	TaskID                string                `json:"task_id"`
	Goal                  string                `json:"goal"`
	Params                map[string]any        `json:"params,omitempty"`
	Context               *TaskContext          `json:"context,omitempty"`
	Toolsets              []string              `json:"toolsets,omitempty"`
	TargetWorker          string                `json:"target_worker,omitempty"`
	TimeoutSeconds        int                   `json:"timeout_seconds,omitempty"`
	CallbackTopic         string                `json:"callback_topic,omitempty"`
	Priority              int                   `json:"priority,omitempty"`
	DependsOn             []string              `json:"depends_on,omitempty"`
	AggregateKey          string                `json:"aggregate_key,omitempty"`
	MinResources          *ResourceRequirements `json:"min_resources,omitempty"`
	TraceContext          *TraceContext         `json:"trace_context,omitempty"`
	AllowedWorkerIDs      []string              `json:"allowed_worker_ids,omitempty"`
	DenyWorkerIDs         []string              `json:"deny_worker_ids,omitempty"`
	QueueTimeoutSeconds   int                   `json:"queue_timeout_seconds,omitempty"`
	MaxAttempts           int                   `json:"max_attempts,omitempty"`
	FirstProgressSeconds  int                   `json:"first_progress_seconds,omitempty"`
	Model                 string                `json:"model,omitempty"`
	ResumeFromCheckpoint  string                `json:"resume_from_checkpoint,omitempty"`
}

// ResourceRequirements specifies resource constraints for task scheduling.
type ResourceRequirements struct {
	MinCPUCores              int      `json:"min_cpu_cores,omitempty"`
	MinMemoryGB              int      `json:"min_memory_gb,omitempty"`
	RequiresGPU              bool     `json:"requires_gpu,omitempty"`
	RequiredNetworkProfiles  []string `json:"required_network_profiles,omitempty"`
}

// TraceContext carries distributed tracing metadata.
type TraceContext struct {
	TraceID      string `json:"trace_id,omitempty"`
	SpanID       string `json:"span_id,omitempty"`
	ParentSpanID string `json:"parent_span_id,omitempty"`
	Sampled      bool   `json:"sampled,omitempty"`
}

// TaskContext carries background information for a task.
// Large contexts (>48 KiB) are gzip+base64 encoded into InlineGzip automatically.
type TaskContext struct {
	Inline     string       `json:"inline,omitempty"`
	InlineGzip *InlineGzip  `json:"inline_gzip,omitempty"`
	Ref        *ContextRef  `json:"ref,omitempty"`
}

// InlineGzip represents gzip-compressed context data.
type InlineGzip struct {
	GzipData []byte `json:"gzip_data"`
	SHA256   string `json:"sha256"`
}

// ContextRef represents out-of-band context fetched by the worker.
type ContextRef struct {
	URI             string `json:"uri"`
	SHA256          string `json:"sha256"`
	ContentEncoding string `json:"content_encoding,omitempty"`
	Signature       string `json:"signature,omitempty"`
}

// TaskResult is the terminal outcome of a task.
type TaskResult struct {
	TaskID             string      `json:"task_id"`
	Status             string      `json:"status"`
	Summary            string      `json:"summary"`
	ResultText         string      `json:"result_text"`
	Fields             *TaskFields `json:"fields,omitempty"`
	Error              string      `json:"error,omitempty"`
	Usage              *TaskUsage  `json:"usage,omitempty"`
	StartedAt          int64       `json:"started_at,omitempty"`
	CompletedAt        int64       `json:"completed_at,omitempty"`
	WorkerID           string      `json:"worker_id,omitempty"`
	SchemaVersion      int         `json:"schema_version,omitempty"`
	BatchID            string      `json:"batch_id,omitempty"`
	LatestCheckpointID string      `json:"latest_checkpoint_id,omitempty"`
	Attempt            int         `json:"attempt,omitempty"`
	MaxAttempts        int         `json:"max_attempts,omitempty"`
	ResultTruncated    bool        `json:"result_truncated,omitempty"`
}

// TaskUsage captures resource consumption metrics.
type TaskUsage struct {
	PromptTokens     int     `json:"prompt_tokens,omitempty"`
	CompletionTokens int     `json:"completion_tokens,omitempty"`
	TotalTokens      int     `json:"total_tokens,omitempty"`
	APICalls         int     `json:"api_calls,omitempty"`
	ToolCalls        int     `json:"tool_calls,omitempty"`
	WallSeconds      float64 `json:"wall_seconds,omitempty"`
	CostUSD          float64 `json:"cost_usd,omitempty"`
	Model            string  `json:"model,omitempty"`
}

// TaskFields carries structured task metadata.
type TaskFields struct {
	Version    int                `json:"version,omitempty"`
	Metrics    []Metric           `json:"metrics,omitempty"`
	Tags       []KeyValue         `json:"tags,omitempty"`
	Report     string             `json:"report,omitempty"`
	Extensions map[string][]byte  `json:"extensions,omitempty"`
}

// Metric represents a named numeric measurement.
type Metric struct {
	Name         string  `json:"name"`
	Value        float64 `json:"value"`
	Unit         string  `json:"unit,omitempty"`
	Description  string  `json:"description,omitempty"`
	OriginTaskID string  `json:"origin_task_id,omitempty"`
}

// KeyValue is a generic key-value pair.
type KeyValue struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// TaskEvent represents one SSE event from the watch stream.
type TaskEvent struct {
	EventID          int64            `json:"event_id,omitempty"`
	EventAt          int64            `json:"event_at,omitempty"`
	TaskID           string           `json:"task_id"`
	BatchID          string           `json:"batch_id,omitempty"`
	Kind             string           `json:"kind,omitempty"`
	Result           *TaskResult      `json:"result,omitempty"`
	ProgressSummary  string           `json:"progress_summary,omitempty"`
	Checkpoint       *TaskCheckpoint  `json:"checkpoint,omitempty"`
	Aggregate        *AggregateResult `json:"aggregate,omitempty"`
	TraceContext     *TraceContext    `json:"trace_context,omitempty"`
	ID               string           `json:"id,omitempty"`
	Type             string           `json:"type,omitempty"`
	Data             any              `json:"data,omitempty"`
}

// TaskCheckpoint represents an intermediate task checkpoint.
type TaskCheckpoint struct {
	TaskID       string      `json:"task_id"`
	CheckpointID string      `json:"checkpoint_id"`
	EventID      int64       `json:"event_id,omitempty"`
	CheckpointAt int64       `json:"checkpoint_at,omitempty"`
	Summary      string      `json:"summary,omitempty"`
	Fields       *TaskFields `json:"fields,omitempty"`
	ResumeBlob   []byte      `json:"resume_blob,omitempty"`
	LeaseUntil   int64       `json:"lease_until,omitempty"`
}

// AggregateResult represents a batch aggregation result.
type AggregateResult struct {
	BatchID       string            `json:"batch_id"`
	AggregateKey  string            `json:"aggregate_key"`
	TaskIDs       []string          `json:"task_ids,omitempty"`
	StatusCounts  map[string]int    `json:"status_counts,omitempty"`
	Summary       string            `json:"summary,omitempty"`
	Metrics       []Metric          `json:"metrics,omitempty"`
	SchemaVersion int               `json:"schema_version,omitempty"`
}

// BatchPolicy configures batch completion conditions.
type BatchPolicy struct {
	CompletionMode   string  `json:"completion_mode,omitempty"`
	SuccessThreshold int     `json:"success_threshold,omitempty"`
	BatchTimeoutMS   int64   `json:"batch_timeout_ms,omitempty"`
	FailFast         bool    `json:"fail_fast,omitempty"`
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

// DispatchTaskRequest matches proto DispatchTaskRequest.
type DispatchTaskRequest struct {
	Spec            TaskSpec `json:"spec"`
	MasterSessionID string   `json:"master_session_id,omitempty"`
	AllowRedispatch bool     `json:"allow_redispatch,omitempty"`
	IdempotencyKey  string   `json:"idempotency_key,omitempty"`
}

// DispatchTaskResponse matches proto DispatchTaskResponse.
type DispatchTaskResponse struct {
	TaskID         string      `json:"task_id"`
	BatchID        string      `json:"batch_id,omitempty"`
	CallbackTopic  string      `json:"callback_topic,omitempty"`
	Status         string      `json:"status"`
	IdempotentHit  bool        `json:"idempotent_hit,omitempty"`
	ExistingResult *TaskResult `json:"existing_result,omitempty"`
	Attempt        int         `json:"attempt,omitempty"`
}

// DispatchTaskBatchRequest matches proto DispatchTaskBatchRequest.
type DispatchTaskBatchRequest struct {
	BatchID         string        `json:"batch_id"`
	Specs           []TaskSpec    `json:"specs"`
	MasterSessionID string        `json:"master_session_id,omitempty"`
	CallbackTopic   string        `json:"callback_topic,omitempty"`
	AllowRedispatch bool          `json:"allow_redispatch,omitempty"`
	Policy          *BatchPolicy  `json:"policy,omitempty"`
	IdempotencyKey  string        `json:"idempotency_key,omitempty"`
}

// DispatchTaskBatchResponse matches proto DispatchTaskBatchResponse.
type DispatchTaskBatchResponse struct {
	BatchID        string                  `json:"batch_id"`
	CallbackTopic  string                  `json:"callback_topic,omitempty"`
	Tasks          []DispatchTaskResponse  `json:"tasks,omitempty"`
	IdempotentHit  bool                    `json:"idempotent_hit,omitempty"`
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
	JoinPolicy string              `json:"join_policy,omitempty"`
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

// WorkerInfo describes a worker node.
type WorkerInfo struct {
	WorkerID        string           `json:"worker_id"`
	Status          string           `json:"status"`
	Toolsets        []string         `json:"toolsets,omitempty"`
	OS              string           `json:"os,omitempty"`
	Arch            string           `json:"arch,omitempty"`
	Region          string           `json:"region,omitempty"`
	ResumeFormats   []string         `json:"resume_formats,omitempty"`
	Resources       *WorkerResources `json:"resources,omitempty"`
	Load            *WorkerLoad      `json:"load,omitempty"`
	MaxConcurrent   int              `json:"max_concurrent,omitempty"`
	RunningTasks    int              `json:"running_tasks,omitempty"`
	LastAnnounceAt  int64            `json:"last_announce_at,omitempty"`
	LastHeartbeatAt int64            `json:"last_heartbeat_at,omitempty"`
}

// WorkerResources describes worker hardware capabilities.
type WorkerResources struct {
	CPUCores       int    `json:"cpu_cores,omitempty"`
	MemoryGB       int    `json:"memory_gb,omitempty"`
	GPUCount       int    `json:"gpu_count,omitempty"`
	GPUModel       string `json:"gpu_model,omitempty"`
	DiskGB         int    `json:"disk_gb,omitempty"`
	NetworkProfile string `json:"network_profile,omitempty"`
}

// WorkerLoad describes current worker resource utilization.
type WorkerLoad struct {
	RunningTasks  int     `json:"running_tasks,omitempty"`
	CPUPercent    float64 `json:"cpu_percent,omitempty"`
	MemoryPercent float64 `json:"memory_percent,omitempty"`
}

// AgentModel represents a pool-wide schedulable model.
type AgentModel struct {
	ModelVersionID string   `json:"model_version_id"`
	DisplayName    string   `json:"display_name,omitempty"`
	NodeCount      int64    `json:"node_count,omitempty"`
	AvailableSlots int64    `json:"available_slots,omitempty"`
	Regions        []string `json:"regions,omitempty"`
}

// CursorOutOfRange indicates the requested event cursor has expired.
type CursorOutOfRange struct {
	RequestedSinceEventID int64 `json:"requested_since_event_id"`
	OldestAvailableEventID int64 `json:"oldest_available_event_id"`
	NewestEventID         int64 `json:"newest_event_id"`
}

// SlowConsumer indicates the client cannot keep up with the event stream.
type SlowConsumer struct {
	DeliveredEventID int64 `json:"delivered_event_id"`
	NewestEventID    int64 `json:"newest_event_id"`
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
