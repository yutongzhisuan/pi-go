package masterplanner

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	defaultBaseURL     = "http://localhost:18082"
	defaultTimeout     = 30 * time.Second
	defaultSSETimeout  = 5 * time.Minute
)

type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

type ClientOption func(*Client)

func WithBaseURL(baseURL string) ClientOption {
	return func(c *Client) {
		c.baseURL = baseURL
	}
}

func WithAPIKey(apiKey string) ClientOption {
	return func(c *Client) {
		c.apiKey = apiKey
	}
}

func WithHTTPClient(client *http.Client) ClientOption {
	return func(c *Client) {
		c.httpClient = client
	}
}

func NewClient(opts ...ClientOption) *Client {
	c := &Client{
		baseURL: defaultBaseURL,
		httpClient: &http.Client{
			Timeout: defaultTimeout,
		},
	}
	for _, opt := range opts {
		opt(c)
	}
	if c.baseURL == "" {
		c.baseURL = defaultBaseURL
	}
	return c
}

func (c *Client) doRequest(ctx context.Context, method, path string, body, result interface{}) error {
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshal request: %w", err)
		}
		bodyReader = bytes.NewReader(data)
	}

	reqURL := c.baseURL + path
	req, err := http.NewRequestWithContext(ctx, method, reqURL, bodyReader)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("request failed: status=%d, body=%s", resp.StatusCode, string(bodyBytes))
	}

	if result != nil {
		if err := json.NewDecoder(resp.Body).Decode(result); err != nil {
			return fmt.Errorf("decode response: %w", err)
		}
	}

	return nil
}

func (c *Client) CreateTask(ctx context.Context, req *CreateTaskRequest) (*CreateTaskResponse, error) {
	var resp CreateTaskResponse
	if err := c.doRequest(ctx, http.MethodPost, "/v1/agent/tasks", req, &resp); err != nil {
		return nil, fmt.Errorf("create task: %w", err)
	}
	return &resp, nil
}

func (c *Client) BatchCreateTasks(ctx context.Context, req *BatchCreateTasksRequest) (*BatchCreateTasksResponse, error) {
	var resp BatchCreateTasksResponse
	if err := c.doRequest(ctx, http.MethodPost, "/v1/agent/tasks:batch", req, &resp); err != nil {
		return nil, fmt.Errorf("batch create tasks: %w", err)
	}
	return &resp, nil
}

func (c *Client) ListTasks(ctx context.Context, req *ListTasksRequest) (*ListTasksResponse, error) {
	query := url.Values{}
	if req.WorkerID != "" {
		query.Set("worker_id", req.WorkerID)
	}
	if req.Status != "" {
		query.Set("status", string(req.Status))
	}
	if req.PageSize > 0 {
		query.Set("page_size", fmt.Sprintf("%d", req.PageSize))
	}
	if req.PageToken != "" {
		query.Set("page_token", req.PageToken)
	}

	path := "/v1/agent/tasks"
	if len(query) > 0 {
		path += "?" + query.Encode()
	}

	var resp ListTasksResponse
	if err := c.doRequest(ctx, http.MethodGet, path, nil, &resp); err != nil {
		return nil, fmt.Errorf("list tasks: %w", err)
	}
	return &resp, nil
}

type TaskEventStream struct {
	Events <-chan *TaskEvent
	Errors <-chan error
	cancel context.CancelFunc
}

func (s *TaskEventStream) Close() {
	if s.cancel != nil {
		s.cancel()
	}
}

func (c *Client) WatchTasks(ctx context.Context, req *WatchTasksRequest) (*TaskEventStream, error) {
	query := url.Values{}
	if req.WorkerID != "" {
		query.Set("worker_id", req.WorkerID)
	}
	if req.Status != "" {
		query.Set("status", string(req.Status))
	}

	path := "/v1/agent/tasks:watch"
	if len(query) > 0 {
		path += "?" + query.Encode()
	}

	reqURL := c.baseURL + path
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create watch request: %w", err)
	}

	httpReq.Header.Set("Accept", "text/event-stream")
	httpReq.Header.Set("Cache-Control", "no-cache")
	httpReq.Header.Set("Connection", "keep-alive")
	if c.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	sseClient := &http.Client{
		Timeout: defaultSSETimeout,
	}

	streamCtx, cancel := context.WithCancel(ctx)
	events := make(chan *TaskEvent, 10)
	errors := make(chan error, 1)

	go func() {
		defer close(events)
		defer close(errors)
		defer cancel()

		resp, err := sseClient.Do(httpReq)
		if err != nil {
			errors <- fmt.Errorf("watch request: %w", err)
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			bodyBytes, _ := io.ReadAll(resp.Body)
			errors <- fmt.Errorf("watch failed: status=%d, body=%s", resp.StatusCode, string(bodyBytes))
			return
		}

		scanner := bufio.NewScanner(resp.Body)
		var eventData strings.Builder

		for scanner.Scan() {
			select {
			case <-streamCtx.Done():
				return
			default:
			}

			line := scanner.Text()

			if strings.HasPrefix(line, "event:") {
				eventType := strings.TrimSpace(strings.TrimPrefix(line, "event:"))
				if eventType == "error" {
					continue
				}
			} else if strings.HasPrefix(line, "data:") {
				data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
				eventData.WriteString(data)
			} else if line == "" && eventData.Len() > 0 {
				var event TaskEvent
				if err := json.Unmarshal([]byte(eventData.String()), &event); err != nil {
					var sseErr SSEError
					if jsonErr := json.Unmarshal([]byte(eventData.String()), &sseErr); jsonErr == nil {
						errors <- fmt.Errorf("SSE error: code=%s, message=%s", sseErr.Code, sseErr.Message)
						return
					}
					errors <- fmt.Errorf("decode event: %w", err)
					return
				}

				select {
				case events <- &event:
				case <-streamCtx.Done():
					return
				}

				eventData.Reset()
			}
		}

		if err := scanner.Err(); err != nil {
			errors <- fmt.Errorf("scan SSE stream: %w", err)
		}
	}()

	return &TaskEventStream{
		Events: events,
		Errors: errors,
		cancel: cancel,
	}, nil
}

func (c *Client) SubmitTaskResult(ctx context.Context, taskID string, req *SubmitTaskResultRequest) (*SubmitTaskResultResponse, error) {
	var resp SubmitTaskResultResponse
	path := fmt.Sprintf("/v1/agent/tasks/%s:result", taskID)
	if err := c.doRequest(ctx, http.MethodPost, path, req, &resp); err != nil {
		return nil, fmt.Errorf("submit task result: %w", err)
	}
	return &resp, nil
}

func (c *Client) CancelTask(ctx context.Context, taskID string, req *CancelTaskRequest) (*CancelTaskResponse, error) {
	var resp CancelTaskResponse
	path := fmt.Sprintf("/v1/agent/tasks/%s:cancel", taskID)
	if err := c.doRequest(ctx, http.MethodPost, path, req, &resp); err != nil {
		return nil, fmt.Errorf("cancel task: %w", err)
	}
	return &resp, nil
}

func (c *Client) ListWorkers(ctx context.Context, req *ListWorkersRequest) (*ListWorkersResponse, error) {
	query := url.Values{}
	if req.Status != "" {
		query.Set("status", req.Status)
	}
	if req.PageSize > 0 {
		query.Set("page_size", fmt.Sprintf("%d", req.PageSize))
	}
	if req.PageToken != "" {
		query.Set("page_token", req.PageToken)
	}

	path := "/v1/agent/workers:list"
	if len(query) > 0 {
		path += "?" + query.Encode()
	}

	var resp ListWorkersResponse
	if err := c.doRequest(ctx, http.MethodPost, path, nil, &resp); err != nil {
		return nil, fmt.Errorf("list workers: %w", err)
	}
	return &resp, nil
}

func (c *Client) ListModels(ctx context.Context, req *ListModelsRequest) (*ListModelsResponse, error) {
	query := url.Values{}
	if req.Provider != "" {
		query.Set("provider", req.Provider)
	}
	if req.PageSize > 0 {
		query.Set("page_size", fmt.Sprintf("%d", req.PageSize))
	}
	if req.PageToken != "" {
		query.Set("page_token", req.PageToken)
	}

	path := "/v1/agent/models:list"
	if len(query) > 0 {
		path += "?" + query.Encode()
	}

	var resp ListModelsResponse
	if err := c.doRequest(ctx, http.MethodPost, path, nil, &resp); err != nil {
		return nil, fmt.Errorf("list models: %w", err)
	}
	return &resp, nil
}
