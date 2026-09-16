package masterplanner

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	defaultBaseURL      = "https://gateway.infa.example.com"
	defaultTimeoutS     = 30
	maxWaitSeconds      = 60
	contextInlineMaxKiB = 48
)

// GatewayClient is a blocking HTTP/SSE client for the AgentRelayService.
type GatewayClient struct {
	baseURL    string
	apiKey     string
	timeout    time.Duration
	httpClient *http.Client
}

// NewGatewayClient creates a new Gateway client from environment variables.
func NewGatewayClient() (*GatewayClient, error) {
	apiKey := os.Getenv("INFA_GATEWAY_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("INFA_GATEWAY_API_KEY is required")
	}

	baseURL := os.Getenv("INFA_GATEWAY_BASE_URL")
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	baseURL = strings.TrimRight(baseURL, "/")

	timeoutS := defaultTimeoutS
	if t := os.Getenv("INFA_GATEWAY_TIMEOUT_S"); t != "" {
		if parsed, err := strconv.Atoi(t); err == nil && parsed > 0 {
			timeoutS = parsed
		}
	}

	return &GatewayClient{
		baseURL: baseURL,
		apiKey:  apiKey,
		timeout: time.Duration(timeoutS) * time.Second,
		httpClient: &http.Client{
			Timeout: time.Duration(timeoutS) * time.Second,
		},
	}, nil
}

// NewGatewayClientWithConfig creates a Gateway client with explicit configuration.
func NewGatewayClientWithConfig(baseURL, apiKey string, timeoutS int) *GatewayClient {
	return &GatewayClient{
		baseURL: strings.TrimRight(baseURL, "/"),
		apiKey:  apiKey,
		timeout: time.Duration(timeoutS) * time.Second,
		httpClient: &http.Client{
			Timeout: time.Duration(timeoutS) * time.Second,
		},
	}
}

// DispatchTask submits a single TaskSpec (idempotent by task_id).
func (c *GatewayClient) DispatchTask(ctx context.Context, spec TaskSpec, masterSessionID string) (map[string]any, error) {
	body := map[string]any{"spec": spec}
	if masterSessionID != "" {
		body["master_session_id"] = masterSessionID
	}
	return c.postJSON(ctx, "/v1/agent/tasks", body)
}

// DispatchBatch submits multiple TaskSpecs as one batch.
func (c *GatewayClient) DispatchBatch(ctx context.Context, specs []TaskSpec, batchID, masterSessionID, joinPolicy string) (map[string]any, error) {
	body := map[string]any{"specs": specs}
	if batchID != "" {
		body["batch_id"] = batchID
	}
	if masterSessionID != "" {
		body["master_session_id"] = masterSessionID
	}
	if joinPolicy != "" {
		completionMode := joinPolicyToCompletionMode(joinPolicy)
		if completionMode != "" {
			body["policy"] = map[string]any{"completion_mode": completionMode}
		}
	}
	return c.postJSON(ctx, "/v1/agent/tasks:batch", body)
}

// GetTaskResult fetches the terminal result of a task.
func (c *GatewayClient) GetTaskResult(ctx context.Context, taskID string) (map[string]any, error) {
	body := map[string]any{"include_latest_checkpoint": true}
	path := fmt.Sprintf("/v1/agent/tasks/%s:result", url.PathEscape(taskID))
	return c.postJSON(ctx, path, body)
}

// ListTasks lists tasks filtered by master session / batch / status.
func (c *GatewayClient) ListTasks(ctx context.Context, opts ListTasksOptions) (map[string]any, error) {
	params := url.Values{}
	if opts.MasterSessionID != "" {
		params.Add("master_session_id", opts.MasterSessionID)
	}
	if opts.BatchID != "" {
		params.Add("batch_id", opts.BatchID)
	}
	if opts.Status != "" {
		params.Add("statuses", taskStatusToEnum(opts.Status))
	}
	path := "/v1/agent/tasks"
	if len(params) > 0 {
		path += "?" + params.Encode()
	}
	return c.getJSON(ctx, path)
}

// ListTasksOptions configures a ListTasks call.
type ListTasksOptions struct {
	MasterSessionID string
	BatchID         string
	Status          string
}

// ListWorkers probes platform capabilities.
func (c *GatewayClient) ListWorkers(ctx context.Context, requireToolsets []string) (map[string]any, error) {
	body := map[string]any{}
	if len(requireToolsets) > 0 {
		body["require_toolsets"] = requireToolsets
	}
	return c.postJSON(ctx, "/v1/agent/workers:list", body)
}

// ListModels lists schedulable models.
func (c *GatewayClient) ListModels(ctx context.Context, region string) (map[string]any, error) {
	body := map[string]any{}
	if region != "" {
		body["region"] = region
	}
	return c.postJSON(ctx, "/v1/agent/models:list", body)
}

// CancelTask cancels a task or batch.
func (c *GatewayClient) CancelTask(ctx context.Context, taskID, batchID, reason string) (map[string]any, error) {
	if taskID == "" {
		return nil, fmt.Errorf("task_id is required for cancel")
	}
	body := map[string]any{}
	if batchID != "" {
		body["batch_id"] = batchID
	}
	if reason != "" {
		body["reason"] = reason
	}
	path := fmt.Sprintf("/v1/agent/tasks/%s:cancel", url.PathEscape(taskID))
	return c.postJSON(ctx, path, body)
}

// WatchResult holds the outcome of a Watch call.
type WatchResult struct {
	Events      []map[string]any
	Cursor      string
	Error       map[string]any
	Interrupted bool
	Reason      string // terminal, timeout, interrupted, error, stream_closed
}

// Watch blocks up to waitSeconds for the next batch of task events (SSE stream).
func (c *GatewayClient) Watch(ctx context.Context, opts WatchOptions) (WatchResult, error) {
	waitSec := opts.WaitSeconds
	if waitSec <= 0 || waitSec > maxWaitSeconds {
		waitSec = maxWaitSeconds
	}

	body := map[string]any{}
	if opts.TaskID != "" {
		body["task_id"] = opts.TaskID
	} else if opts.BatchID != "" {
		body["batch_id"] = opts.BatchID
	}
	if opts.SinceEventID != "" {
		body["since_event_id"] = opts.SinceEventID
	}

	req, err := c.newRequest(ctx, "POST", "/v1/agent/tasks:watch", body)
	if err != nil {
		return WatchResult{}, err
	}
	req.Header.Set("Accept", "text/event-stream, application/protojson")
	req.Header.Set("Cache-Control", "no-cache")

	// Set a generous timeout for the watch stream
	watchCtx, cancel := context.WithTimeout(ctx, c.timeout+time.Duration(waitSec)*time.Second+15*time.Second)
	defer cancel()
	req = req.WithContext(watchCtx)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return WatchResult{}, fmt.Errorf("watch request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return WatchResult{}, c.parseHTTPError(resp)
	}

	return c.consumeSSEStream(watchCtx, resp.Body, waitSec)
}

// WatchOptions configures a Watch call.
type WatchOptions struct {
	TaskID       string
	BatchID      string
	SinceEventID string
	WaitSeconds  float64
}

func (c *GatewayClient) consumeSSEStream(ctx context.Context, r io.Reader, waitSec float64) (WatchResult, error) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)

	var events []map[string]any
	var cursor string
	var errorFrame map[string]any
	reason := "timeout"
	interrupted := false

	deadline := time.Now().Add(time.Duration(waitSec) * time.Second)

	var eventType, eventID string
	var dataLines []string

	flushFrame := func() {
		if len(dataLines) == 0 && eventType == "" {
			return
		}
		raw := strings.Join(dataLines, "\n")
		var data any
		if raw != "" {
			if err := json.Unmarshal([]byte(raw), &data); err != nil {
				data = raw
			}
		}

		if eventType == "error" {
			if m, ok := data.(map[string]any); ok {
				errorFrame = normalizeSSEError(m)
			} else {
				errorFrame = map[string]any{"code": "unknown", "raw": data}
			}
			reason = "error"
			return
		}

		if m, ok := data.(map[string]any); ok {
			data = normalizeResponse(m)
			if eid, ok := m["event_id"].(string); ok && eid != "" && eid != "0" {
				cursor = eid
			}
			kind := m["kind"]
			if kindStr, ok := kind.(string); ok {
				eventType = taskEventKindType(kindStr)
			} else if kindInt, ok := kind.(float64); ok {
				eventType = taskEventKindType(fmt.Sprintf("%d", int(kindInt)))
			}
		}

		events = append(events, map[string]any{
			"type": eventType,
			"id":   eventID,
			"data": data,
		})

		if eventType == "terminal" {
			reason = "terminal"
		}

		eventType, eventID = "", ""
		dataLines = nil
	}

	for {
		select {
		case <-ctx.Done():
			interrupted = true
			reason = "interrupted"
			flushFrame()
			return WatchResult{Events: events, Cursor: cursor, Error: errorFrame, Interrupted: interrupted, Reason: reason}, nil
		default:
		}

		if time.Now().After(deadline) {
			flushFrame()
			return WatchResult{Events: events, Cursor: cursor, Error: errorFrame, Interrupted: false, Reason: reason}, nil
		}

		if !scanner.Scan() {
			flushFrame()
			if scanner.Err() != nil {
				return WatchResult{}, fmt.Errorf("sse scan: %w", scanner.Err())
			}
			if len(events) > 0 && reason == "timeout" {
				if events[len(events)-1]["type"] == "terminal" {
					reason = "terminal"
				} else {
					reason = "stream_closed"
				}
			} else if reason == "timeout" {
				reason = "stream_closed"
			}
			return WatchResult{Events: events, Cursor: cursor, Error: errorFrame, Interrupted: false, Reason: reason}, nil
		}

		line := strings.TrimRight(scanner.Text(), "\r\n")
		if line == "" {
			flushFrame()
			if reason == "error" {
				return WatchResult{Events: events, Cursor: cursor, Error: errorFrame, Interrupted: false, Reason: reason}, nil
			}
			continue
		}
		if strings.HasPrefix(line, ":") {
			continue
		}

		field, value, found := strings.Cut(line, ":")
		if !found {
			continue
		}
		value = strings.TrimPrefix(value, " ")

		switch field {
		case "event":
			eventType = value
		case "id":
			eventID = value
		case "data":
			dataLines = append(dataLines, value)
		}
	}
}

func (c *GatewayClient) postJSON(ctx context.Context, path string, body map[string]any) (map[string]any, error) {
	req, err := c.newRequest(ctx, "POST", path, body)
	if err != nil {
		return nil, err
	}
	return c.doJSON(req)
}

func (c *GatewayClient) getJSON(ctx context.Context, path string) (map[string]any, error) {
	req, err := c.newRequest(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}
	return c.doJSON(req)
}

func (c *GatewayClient) newRequest(ctx context.Context, method, path string, body map[string]any) (*http.Request, error) {
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("new request: %w", err)
	}

	if bodyReader != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "application/protojson")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("User-Agent", "pi-go-master-planner/0.1")

	return req, nil
}

func (c *GatewayClient) doJSON(req *http.Request) (map[string]any, error) {
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, c.parseHTTPError(resp)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if len(bytes.TrimSpace(data)) == 0 {
		return map[string]any{}, nil
	}

	var result map[string]any
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	normalized := normalizeResponse(result)
	if m, ok := normalized.(map[string]any); ok {
		return m, nil
	}
	return result, nil
}

func (c *GatewayClient) parseHTTPError(resp *http.Response) error {
	data, _ := io.ReadAll(resp.Body)
	msg := fmt.Sprintf("gateway HTTP %d", resp.StatusCode)
	code := ""

	if len(data) > 0 {
		var parsed map[string]any
		if json.Unmarshal(data, &parsed) == nil {
			if m, ok := parsed["message"].(string); ok && m != "" {
				msg = m
			}
			if c, ok := parsed["reason"].(string); ok && c != "" {
				code = strings.ToLower(c)
			} else if c, ok := parsed["code"].(string); ok && c != "" {
				code = c
			}
		}
	}

	return &GatewayError{Message: msg, Status: resp.StatusCode, Code: code}
}

// GatewayError is a structured error from the gateway.
type GatewayError struct {
	Message string
	Status  int
	Code    string
}

func (e *GatewayError) Error() string {
	if e.Code != "" {
		return fmt.Sprintf("%s (code=%s, status=%d)", e.Message, e.Code, e.Status)
	}
	return fmt.Sprintf("%s (status=%d)", e.Message, e.Status)
}

// BuildTaskSpec builds a TaskSpec from tool input, encoding context if needed.
func BuildTaskSpec(input DispatchTaskInput, taskID string) TaskSpec {
	spec := TaskSpec{
		TaskID:               taskID,
		Goal:                 input.Goal,
		Model:                input.Model,
		Toolsets:             input.Toolsets,
		Params:               input.Params,
		TimeoutSeconds:       input.TimeoutSeconds,
		Priority:             input.Priority,
		DependsOn:            input.DependsOn,
		ResumeFromCheckpoint: input.ResumeFromCheckpoint,
	}

	if input.ResumeSummary != "" {
		if spec.Params == nil {
			spec.Params = make(map[string]any)
		}
		spec.Params["resume_summary"] = input.ResumeSummary
	}

	if input.Context != nil {
		spec.Context = encodeContext(input.Context)
	}

	return spec
}

func encodeContext(ctx any) *TaskContext {
	if ctx == nil {
		return nil
	}

	var raw []byte
	if s, ok := ctx.(string); ok {
		raw = []byte(s)
	} else {
		data, err := json.Marshal(ctx)
		if err != nil {
			return &TaskContext{Inline: fmt.Sprintf("%v", ctx)}
		}
		raw = data
	}

	if len(raw) > contextInlineMaxKiB*1024 {
		var buf bytes.Buffer
		w := gzip.NewWriter(&buf)
		w.Write(raw)
		w.Close()
		return &TaskContext{InlineGzip: base64.StdEncoding.EncodeToString(buf.Bytes())}
	}

	return &TaskContext{Inline: string(raw)}
}

// MasterSessionID generates a stable session ID from the given session key.
func MasterSessionID(sessionKey string) string {
	if sessionKey == "" {
		return "local"
	}
	h := sha256.Sum256([]byte(sessionKey))
	return fmt.Sprintf("%x", h[:8])
}

// normalizeResponse converts camelCase keys to snake_case recursively.
func normalizeResponse(v any) any {
	switch val := v.(type) {
	case map[string]any:
		result := make(map[string]any, len(val))
		for k, v := range val {
			result[normalizeKey(k)] = normalizeResponse(v)
		}
		return result
	case []any:
		result := make([]any, len(val))
		for i, v := range val {
			result[i] = normalizeResponse(v)
		}
		return result
	default:
		return v
	}
}

func normalizeKey(key string) string {
	if !strings.ContainsAny(key, "ABCDEFGHIJKLMNOPQRSTUVWXYZ") {
		return key
	}
	if strings.ToUpper(key) == key {
		return key
	}
	var result []rune
	for i, r := range key {
		if r >= 'A' && r <= 'Z' {
			if i > 0 {
				result = append(result, '_')
			}
			result = append(result, r+32)
		} else {
			result = append(result, r)
		}
	}
	return string(result)
}

func normalizeSSEError(data map[string]any) map[string]any {
	data = normalizeResponse(data).(map[string]any)
	reason := ""
	if r, ok := data["reason"].(string); ok {
		reason = strings.ToLower(r)
	}
	httpCode := 0
	if c, ok := data["code"].(float64); ok {
		httpCode = int(c)
	}

	result := map[string]any{
		"code":      reason,
		"reason":    reason,
		"http_code": httpCode,
		"message":   data["message"],
	}

	if meta, ok := data["metadata"].(map[string]any); ok {
		for k, v := range meta {
			result[k] = v
		}
	}

	return result
}

func taskStatusToEnum(status string) string {
	status = strings.ToLower(strings.TrimSpace(status))
	if strings.HasPrefix(status, "task_status_") {
		return strings.ToUpper(status)
	}
	if status == "" || status == "unspecified" {
		return ""
	}
	return "TASK_STATUS_" + strings.ToUpper(status)
}

func joinPolicyToCompletionMode(policy string) string {
	switch strings.ToLower(strings.TrimSpace(policy)) {
	case "all":
		return "COMPLETION_MODE_ALL"
	case "any":
		return "COMPLETION_MODE_ANY"
	case "majority":
		return "COMPLETION_MODE_MAJORITY"
	default:
		return ""
	}
}

func taskEventKindType(kind string) string {
	kind = strings.TrimSpace(kind)
	if kind == "" {
		return "event"
	}

	kindMap := map[string]string{
		"1": "status",
		"2": "progress",
		"3": "terminal",
		"4": "checkpoint",
		"5": "aggregate",
	}

	if t, ok := kindMap[kind]; ok {
		return t
	}

	kind = strings.ToLower(kind)
	if strings.HasPrefix(kind, "task_event_kind_") {
		kind = kind[len("task_event_kind_"):]
	}
	if kind == "unspecified" {
		return "event"
	}
	return kind
}
