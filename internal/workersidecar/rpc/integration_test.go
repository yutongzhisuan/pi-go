package rpc

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dimetron/pi-go/internal/workersidecar"
	"github.com/dimetron/pi-go/internal/workersidecar/profile"
)

// testBackend implements a minimal backend for integration testing.
type testBackend struct {
	runResult     workersidecar.RunResult
	progressQueue []string
	cancelCalled  bool
}

func (f *testBackend) RunSession(ctx context.Context, params workersidecar.RunParams) workersidecar.RunResult {
	f.progressQueue = append(f.progressQueue, "Starting task", "Processing", "Completed")
	return f.runResult
}

// TestIntegrationSmokeHTTP tests the full RPC flow over HTTP.
func TestIntegrationSmokeHTTP(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	testBE := &testBackend{
		runResult: workersidecar.RunResult{
			Status:     "completed",
			Summary:    "Integration test completed",
			ResultText: "Test result",
			Usage: &workersidecar.UsageInfo{
				InputTokens:  100,
				OutputTokens: 50,
				TotalTokens:  150,
			},
		},
	}

	prof := profile.Default(true)
	
	srv := &Server{
		backend:  &testServerBackend{testBE},
		profile:  prof,
		runs:     make(map[string]*runState),
		progress: make(map[string]*progressBucket),
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to create listener: %v", err)
	}
	defer listener.Close()

	srv.listener = listener
	addr := listener.Addr().String()

	go func() {
		_ = srv.serve()
	}()

	time.Sleep(100 * time.Millisecond)

	baseURL := fmt.Sprintf("http://%s/rpc", addr)
	
	t.Run("toolsets", func(t *testing.T) {
		req := JSONRPCRequest{
			JSONRPC: "2.0",
			Method:  "acp.toolsets",
			ID:      1,
		}
		
		resp := sendRPCRequest(t, baseURL, req)
		assertSuccess(t, resp, 1)
		
		result, ok := resp.Result.(map[string]interface{})
		if !ok {
			t.Fatal("Result is not a map")
		}
		
		toolsets, ok := result["toolsets"].([]interface{})
		if !ok || len(toolsets) == 0 {
			t.Error("Expected non-empty toolsets array")
		}
	})

	runID := fmt.Sprintf("test-run-%d", time.Now().Unix())
	
	t.Run("run", func(t *testing.T) {
		params := workersidecar.RunParams{
			RunID:          runID,
			Goal:           "Test goal",
			Toolsets:       []string{"file", "web"},
			TimeoutSeconds: 10,
		}
		
		paramsJSON, _ := json.Marshal(params)
		req := JSONRPCRequest{
			JSONRPC: "2.0",
			Method:  "acp.run",
			Params:  paramsJSON,
			ID:      2,
		}
		
		resp := sendRPCRequest(t, baseURL, req)
		assertSuccess(t, resp, 2)
		
		result, ok := resp.Result.(map[string]interface{})
		if !ok {
			t.Fatal("Result is not a map")
		}
		
		if result["status"] != "completed" {
			t.Errorf("Expected status=completed, got %v", result["status"])
		}
		
		if result["summary"] == nil || result["summary"] == "" {
			t.Error("Expected non-empty summary")
		}
	})

	t.Run("progress", func(t *testing.T) {
		params := workersidecar.ProgressParams{RunID: runID}
		paramsJSON, _ := json.Marshal(params)
		
		req := JSONRPCRequest{
			JSONRPC: "2.0",
			Method:  "acp.progress",
			Params:  paramsJSON,
			ID:      3,
		}
		
		resp := sendRPCRequest(t, baseURL, req)
		assertSuccess(t, resp, 3)
	})

	t.Run("status", func(t *testing.T) {
		params := workersidecar.StatusParams{RunID: runID}
		paramsJSON, _ := json.Marshal(params)
		
		req := JSONRPCRequest{
			JSONRPC: "2.0",
			Method:  "acp.status",
			Params:  paramsJSON,
			ID:      4,
		}
		
		resp := sendRPCRequest(t, baseURL, req)
		assertSuccess(t, resp, 4)
		
		result, ok := resp.Result.(map[string]interface{})
		if !ok {
			t.Fatal("Result is not a map")
		}
		
		if _, ok := result["running"]; !ok {
			t.Error("Expected 'running' field in status response")
		}
	})

	t.Run("cancel", func(t *testing.T) {
		params := workersidecar.CancelParams{
			RunID:  runID,
			Reason: "test cancellation",
		}
		paramsJSON, _ := json.Marshal(params)
		
		req := JSONRPCRequest{
			JSONRPC: "2.0",
			Method:  "acp.cancel",
			Params:  paramsJSON,
			ID:      5,
		}
		
		resp := sendRPCRequest(t, baseURL, req)
		assertSuccess(t, resp, 5)
		
		result, ok := resp.Result.(map[string]interface{})
		if !ok {
			t.Fatal("Result is not a map")
		}
		
		if _, ok := result["cancelled"]; !ok {
			t.Error("Expected 'cancelled' field in cancel response")
		}
	})

	t.Run("duplicate_run_id", func(t *testing.T) {
		srv.mu.Lock()
		srv.runs[runID] = &runState{running: true}
		srv.mu.Unlock()
		
		params := workersidecar.RunParams{
			RunID: runID,
			Goal:  "Should fail",
		}
		paramsJSON, _ := json.Marshal(params)
		
		req := JSONRPCRequest{
			JSONRPC: "2.0",
			Method:  "acp.run",
			Params:  paramsJSON,
			ID:      6,
		}
		
		resp := sendRPCRequest(t, baseURL, req)
		
		if resp.Error == nil {
			t.Error("Expected error for duplicate run_id")
		}
		
		if resp.Error.Code != ServerError {
			t.Errorf("Expected error code %d, got %d", ServerError, resp.Error.Code)
		}
	})
}

// TestIntegrationSmokeUnixSocket tests the full RPC flow over Unix socket.
func TestIntegrationSmokeUnixSocket(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	tmpDir := t.TempDir()
	socketPath := filepath.Join(tmpDir, "test.sock")

	testBE := &testBackend{
		runResult: workersidecar.RunResult{
			Status:     "completed",
			Summary:    "Unix socket test completed",
			ResultText: "Test result",
		},
	}

	prof := profile.Default(true)
	
	srv := &Server{
		backend:  &testServerBackend{testBE},
		profile:  prof,
		runs:     make(map[string]*runState),
		progress: make(map[string]*progressBucket),
	}

	go func() {
		_ = srv.ListenUnix(socketPath)
	}()

	time.Sleep(200 * time.Millisecond)
	defer srv.Close()

	client := &http.Client{
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
				return net.Dial("unix", socketPath)
			},
		},
	}

	req := JSONRPCRequest{
		JSONRPC: "2.0",
		Method:  "acp.toolsets",
		ID:      1,
	}

	reqBody, _ := json.Marshal(req)
	httpReq, _ := http.NewRequest("POST", "http://unix/rpc", bytes.NewReader(reqBody))
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(httpReq)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	var rpcResp JSONRPCResponse
	if err := json.NewDecoder(resp.Body).Decode(&rpcResp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	assertSuccess(t, rpcResp, 1)
}

// TestGoldenFixtureCompliance validates responses match golden fixture structure.
func TestGoldenFixtureCompliance(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	testdataDir := filepath.Join("..", "testdata")
	
	goldenResp, err := os.ReadFile(filepath.Join(testdataDir, "acp_run_response_completed.json"))
	if err != nil {
		t.Fatalf("Failed to read golden fixture: %v", err)
	}

	var golden JSONRPCResponse
	if err := json.Unmarshal(goldenResp, &golden); err != nil {
		t.Fatalf("Failed to parse golden fixture: %v", err)
	}

	result, ok := golden.Result.(map[string]interface{})
	if !ok {
		t.Fatal("Golden result is not a map")
	}

	requiredFields := []string{"status", "summary", "result_text", "fields", "usage"}
	for _, field := range requiredFields {
		if _, ok := result[field]; !ok {
			t.Errorf("Golden fixture missing required field: %s", field)
		}
	}

	if result["status"] != "completed" {
		t.Errorf("Expected status=completed, got %v", result["status"])
	}
}

// testServerBackend wraps testBackend to satisfy the Server's backend field type.
// The Server expects *backend.Backend but we can't import that in tests without
// circular dependencies, so we create a compatible interface wrapper.
type testServerBackend struct {
	be *testBackend
}

func (t *testServerBackend) RunSession(ctx context.Context, params workersidecar.RunParams) workersidecar.RunResult {
	return t.be.RunSession(ctx, params)
}

// sendRPCRequest sends a JSON-RPC request and returns the response.
func sendRPCRequest(t *testing.T, url string, req JSONRPCRequest) JSONRPCResponse {
	t.Helper()
	
	reqBody, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("Failed to marshal request: %v", err)
	}

	httpReq, err := http.NewRequest("POST", url, bytes.NewReader(reqBody))
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 5 * time.Second}
	httpResp, err := client.Do(httpReq)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", httpResp.StatusCode)
	}

	var rpcResp JSONRPCResponse
	if err := json.NewDecoder(httpResp.Body).Decode(&rpcResp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	return rpcResp
}

// assertSuccess checks that a JSON-RPC response is successful.
func assertSuccess(t *testing.T, resp JSONRPCResponse, expectedID interface{}) {
	t.Helper()
	
	if resp.JSONRPC != "2.0" {
		t.Errorf("Expected jsonrpc=2.0, got %q", resp.JSONRPC)
	}

	respIDFloat, ok := resp.ID.(float64)
	if ok {
		expectedIDInt, isInt := expectedID.(int)
		if isInt && int(respIDFloat) != expectedIDInt {
			t.Errorf("Expected id=%v, got %v", expectedID, resp.ID)
		}
	} else if resp.ID != expectedID {
		t.Errorf("Expected id=%v, got %v (types: %T vs %T)", expectedID, resp.ID, expectedID, resp.ID)
	}

	if resp.Error != nil {
		t.Errorf("Expected no error, got: %+v", resp.Error)
	}

	if resp.Result == nil {
		t.Error("Expected result to be non-nil")
	}
}
