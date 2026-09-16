package rpc

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/dimetron/pi-go/internal/workersidecar"
	"github.com/dimetron/pi-go/internal/workersidecar/backend"
	"github.com/dimetron/pi-go/internal/workersidecar/profile"
)

func TestDispatch_MethodNotFound(t *testing.T) {
	srv := NewServer(Config{
		Backend: backend.New(backend.Config{}),
		Profile: profile.Default(true),
	})

	req := JSONRPCRequest{
		JSONRPC: "2.0",
		Method:  "unknown.method",
		ID:      1,
	}

	resp := srv.dispatch(context.Background(), req)
	if resp.Error == nil {
		t.Error("Expected error for unknown method")
	}
	if resp.Error.Code != MethodNotFound {
		t.Errorf("Expected code %d, got %d", MethodNotFound, resp.Error.Code)
	}
}

func TestHandleToolsets(t *testing.T) {
	srv := NewServer(Config{
		Backend: backend.New(backend.Config{}),
		Profile: profile.Default(true),
	})

	req := JSONRPCRequest{
		JSONRPC: "2.0",
		Method:  "acp.toolsets",
		ID:      1,
	}

	resp := srv.dispatch(context.Background(), req)
	if resp.Error != nil {
		t.Fatalf("Unexpected error: %v", resp.Error)
	}

	result, ok := resp.Result.(workersidecar.ToolsetsResult)
	if !ok {
		t.Fatal("Result is not ToolsetsResult")
	}

	if len(result.Toolsets) == 0 {
		t.Error("Expected toolsets to be non-empty")
	}
}

func TestHandleStatus_NotFound(t *testing.T) {
	srv := NewServer(Config{
		Backend: backend.New(backend.Config{}),
		Profile: profile.Default(true),
	})

	params, _ := json.Marshal(workersidecar.StatusParams{RunID: "missing"})
	req := JSONRPCRequest{
		JSONRPC: "2.0",
		Method:  "acp.status",
		Params:  params,
		ID:      1,
	}

	resp := srv.dispatch(context.Background(), req)
	if resp.Error != nil {
		t.Fatalf("Unexpected error: %v", resp.Error)
	}
}

func TestHandleProgress_Empty(t *testing.T) {
	srv := NewServer(Config{
		Backend: backend.New(backend.Config{}),
		Profile: profile.Default(true),
	})

	params, _ := json.Marshal(workersidecar.ProgressParams{RunID: "missing"})
	req := JSONRPCRequest{
		JSONRPC: "2.0",
		Method:  "acp.progress",
		Params:  params,
		ID:      1,
	}

	resp := srv.dispatch(context.Background(), req)
	if resp.Error != nil {
		t.Fatalf("Unexpected error: %v", resp.Error)
	}
}

func TestHandleRPC_POST(t *testing.T) {
	srv := NewServer(Config{
		Backend: backend.New(backend.Config{}),
		Profile: profile.Default(true),
	})

	reqBody := JSONRPCRequest{
		JSONRPC: "2.0",
		Method:  "acp.toolsets",
		ID:      1,
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/rpc", bytes.NewReader(body))
	w := httptest.NewRecorder()

	srv.handleRPC(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var resp JSONRPCResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if resp.Error != nil {
		t.Errorf("Unexpected error in response: %v", resp.Error)
	}
}

func TestHandleRPC_InvalidMethod(t *testing.T) {
	srv := NewServer(Config{
		Backend: backend.New(backend.Config{}),
		Profile: profile.Default(true),
	})

	req := httptest.NewRequest(http.MethodGet, "/rpc", nil)
	w := httptest.NewRecorder()

	srv.handleRPC(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}
