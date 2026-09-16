package rpc

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/dimetron/pi-go/internal/workersidecar"
)

func TestGoldenFixtures(t *testing.T) {
	testdataDir := filepath.Join("..", "testdata")

	tests := []struct {
		name         string
		requestFile  string
		responseFile string
	}{
		{
			name:         "acp.run_completed",
			requestFile:  "acp_run_request.json",
			responseFile: "acp_run_response_completed.json",
		},
		{
			name:         "acp.run_model_unavailable",
			requestFile:  "acp_run_request.json",
			responseFile: "acp_run_response_model_unavailable.json",
		},
		{
			name:         "acp.cancel",
			requestFile:  "acp_cancel_request.json",
			responseFile: "acp_cancel_response.json",
		},
		{
			name:         "acp.status",
			requestFile:  "acp_status_request.json",
			responseFile: "acp_status_response_running.json",
		},
		{
			name:         "acp.progress",
			requestFile:  "acp_progress_request.json",
			responseFile: "acp_progress_response.json",
		},
		{
			name:         "acp.toolsets",
			requestFile:  "acp_toolsets_request.json",
			responseFile: "acp_toolsets_response.json",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reqPath := filepath.Join(testdataDir, tt.requestFile)
			respPath := filepath.Join(testdataDir, tt.responseFile)

			reqData, err := os.ReadFile(reqPath)
			if err != nil {
				t.Fatalf("Failed to read request fixture: %v", err)
			}

			var req JSONRPCRequest
			if err := json.Unmarshal(reqData, &req); err != nil {
				t.Fatalf("Failed to parse request: %v", err)
			}

			if req.JSONRPC != "2.0" {
				t.Errorf("Expected jsonrpc=2.0, got %q", req.JSONRPC)
			}

			if req.Method == "" {
				t.Error("Method should not be empty")
			}

			respData, err := os.ReadFile(respPath)
			if err != nil {
				t.Fatalf("Failed to read response fixture: %v", err)
			}

			var resp JSONRPCResponse
			if err := json.Unmarshal(respData, &resp); err != nil {
				t.Fatalf("Failed to parse response: %v", err)
			}

			if resp.JSONRPC != "2.0" {
				t.Errorf("Expected jsonrpc=2.0, got %q", resp.JSONRPC)
			}
		})
	}
}

func TestGoldenFixture_ModelUnavailable(t *testing.T) {
	testdataDir := filepath.Join("..", "testdata")
	respPath := filepath.Join(testdataDir, "acp_run_response_model_unavailable.json")

	respData, err := os.ReadFile(respPath)
	if err != nil {
		t.Fatalf("Failed to read fixture: %v", err)
	}

	var resp JSONRPCResponse
	if err := json.Unmarshal(respData, &resp); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	result, ok := resp.Result.(map[string]interface{})
	if !ok {
		t.Fatal("Result is not a map")
	}

	if result["status"] != "failed" {
		t.Errorf("Expected status=failed, got %v", result["status"])
	}

	if result["error_code"] != "model_unavailable" {
		t.Errorf("Expected error_code=model_unavailable, got %v", result["error_code"])
	}
}

func TestGoldenFixture_DuplicateRunID(t *testing.T) {
	testdataDir := filepath.Join("..", "testdata")
	errPath := filepath.Join(testdataDir, "duplicate_run_id_error.json")

	errData, err := os.ReadFile(errPath)
	if err != nil {
		t.Fatalf("Failed to read fixture: %v", err)
	}

	var resp JSONRPCResponse
	if err := json.Unmarshal(errData, &resp); err != nil {
		t.Fatalf("Failed to parse error response: %v", err)
	}

	if resp.Error == nil {
		t.Fatal("Expected error to be non-nil")
	}

	if resp.Error.Code != ServerError {
		t.Errorf("Expected code %d, got %d", ServerError, resp.Error.Code)
	}
}

func TestRunResultMarshaling(t *testing.T) {
	result := workersidecar.RunResult{
		Status:     "completed",
		Summary:    "Test summary",
		ResultText: "Test result",
		Fields: map[string]interface{}{
			"count": 5,
		},
		Usage: &workersidecar.UsageInfo{
			InputTokens:  100,
			OutputTokens: 50,
			TotalTokens:  150,
		},
		Error:     "",
		ErrorCode: "",
	}

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("Failed to marshal: %v", err)
	}

	var decoded workersidecar.RunResult
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if decoded.Status != result.Status {
		t.Errorf("Status mismatch: got %q, want %q", decoded.Status, result.Status)
	}

	if decoded.Usage.TotalTokens != result.Usage.TotalTokens {
		t.Errorf("Usage.TotalTokens mismatch: got %d, want %d", decoded.Usage.TotalTokens, result.Usage.TotalTokens)
	}
}
