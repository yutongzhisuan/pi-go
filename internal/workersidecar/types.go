package workersidecar

import (
	"encoding/json"
	"time"
)

// RunParams defines the parameters for acp.run method.
type RunParams struct {
	RunID                 string                 `json:"run_id"`
	TaskID                string                 `json:"task_id,omitempty"`
	Attempt               int                    `json:"attempt,omitempty"`
	Goal                  string                 `json:"goal,omitempty"`
	Params                map[string]interface{} `json:"params,omitempty"`
	Context               map[string]interface{} `json:"context,omitempty"`
	Toolsets              []string               `json:"toolsets,omitempty"`
	TimeoutSeconds        int                    `json:"timeout_seconds,omitempty"`
	FirstProgressSeconds  int                    `json:"first_progress_seconds,omitempty"`
	TraceContext          map[string]interface{} `json:"trace_context,omitempty"`
	ResumeFromCheckpoint  string                 `json:"resume_from_checkpoint,omitempty"`
	ResumeBlob            string                 `json:"resume_blob,omitempty"`
	Model                 string                 `json:"model,omitempty"`
	MasterSessionID       string                 `json:"master_session_id,omitempty"`
	// ResolvedToolsets is filled by the RPC server after executor profile intersection (not on wire).
	ResolvedToolsets []string `json:"-"`
}

// RunResult defines the result returned by acp.run method.
type RunResult struct {
	Status     string                 `json:"status"`
	Summary    string                 `json:"summary,omitempty"`
	ResultText string                 `json:"result_text,omitempty"`
	Fields     map[string]interface{} `json:"fields,omitempty"`
	Usage      *UsageInfo             `json:"usage,omitempty"`
	Error      string                 `json:"error,omitempty"`
	ErrorCode  string                 `json:"error_code,omitempty"`
	Checkpoint *CheckpointInfo        `json:"checkpoint,omitempty"`
}

// UsageInfo tracks token/resource usage for a run.
type UsageInfo struct {
	InputTokens  int `json:"input_tokens,omitempty"`
	OutputTokens int `json:"output_tokens,omitempty"`
	TotalTokens  int `json:"total_tokens,omitempty"`
}

// CheckpointInfo holds checkpoint data for resume capability.
type CheckpointInfo struct {
	CheckpointID string                 `json:"checkpoint_id"`
	Summary      string                 `json:"summary,omitempty"`
	Fields       map[string]interface{} `json:"fields,omitempty"`
	ResumeBlob   string                 `json:"resume_blob,omitempty"`
}

// CancelParams defines parameters for acp.cancel method.
type CancelParams struct {
	RunID  string `json:"run_id"`
	Reason string `json:"reason,omitempty"`
}

// CancelResult defines the result returned by acp.cancel method.
type CancelResult struct {
	Cancelled bool   `json:"cancelled"`
	Reason    string `json:"reason,omitempty"`
}

// StatusParams defines parameters for acp.status method.
type StatusParams struct {
	RunID string `json:"run_id"`
}

// StatusResult defines the result returned by acp.status method.
type StatusResult struct {
	Running bool `json:"running"`
}

// ProgressParams defines parameters for acp.progress method.
type ProgressParams struct {
	RunID string `json:"run_id"`
}

// ProgressResult defines the result returned by acp.progress method.
type ProgressResult struct {
	Summaries      []string          `json:"summaries"`
	ResponseEvents []json.RawMessage `json:"response_events,omitempty"`
}

// ToolsetsResult defines the result returned by acp.toolsets method.
type ToolsetsResult struct {
	Toolsets []string `json:"toolsets,omitempty"`
	Detail   string   `json:"detail,omitempty"`
}

// ProgressSummary represents a progress update enqueued for draining.
type ProgressSummary struct {
	Timestamp time.Time
	Text      string
}
