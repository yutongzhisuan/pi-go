package rpc

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/dimetron/pi-go/internal/workersidecar"
	"github.com/dimetron/pi-go/internal/workersidecar/profile"
	"github.com/dimetron/pi-go/internal/workersidecar/runtime"
)

type slowBackend struct {
	started chan struct{}
}

func (s *slowBackend) RunSession(ctx context.Context, params workersidecar.RunParams) workersidecar.RunResult {
	close(s.started)
	<-ctx.Done()
	return workersidecar.RunResult{
		Status:  "cancelled",
		Summary: "cancelled",
		Error:   ctx.Err().Error(),
	}
}

func TestCancelDuringRun(t *testing.T) {
	started := make(chan struct{})
	back := &slowBackend{started: started}
	srv := NewServer(Config{
		Backend: back,
		Profile: profile.Default(true),
	})

	runID := "cancel-run"
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		params, _ := json.Marshal(workersidecar.RunParams{RunID: runID, Goal: "wait"})
		req := JSONRPCRequest{JSONRPC: "2.0", Method: "acp.run", Params: params, ID: 1}
		_ = srv.dispatch(context.Background(), req)
	}()

	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("run did not start")
	}

	cancelParams, _ := json.Marshal(workersidecar.CancelParams{RunID: runID})
	cancelReq := JSONRPCRequest{JSONRPC: "2.0", Method: "acp.cancel", Params: cancelParams, ID: 2}
	resp := srv.dispatch(context.Background(), cancelReq)
	if resp.Error != nil {
		t.Fatalf("cancel error: %+v", resp.Error)
	}
	result, ok := resp.Result.(workersidecar.CancelResult)
	if !ok || !result.Cancelled {
		t.Fatalf("expected cancelled=true, got %+v", resp.Result)
	}

	wg.Wait()
}

func TestEnqueueProgressDrain(t *testing.T) {
	srv := NewServer(Config{Backend: &slowBackend{started: make(chan struct{})}, Profile: profile.Default(true)})
	runID := "prog-run"
	srv.mu.Lock()
	srv.progress[runID] = &progressBucket{}
	srv.mu.Unlock()

	srv.EnqueueProgress(runID, "hello")
	srv.EnqueueProgress(runID, "world")

	params, _ := json.Marshal(workersidecar.ProgressParams{RunID: runID})
	req := JSONRPCRequest{JSONRPC: "2.0", Method: "acp.progress", Params: params, ID: 3}
	resp := srv.dispatch(context.Background(), req)
	out, ok := resp.Result.(workersidecar.ProgressResult)
	if !ok || len(out.Summaries) != 2 {
		t.Fatalf("expected 2 summaries, got %+v", resp.Result)
	}
}

func TestModelUnavailable(t *testing.T) {
	srv := NewServer(Config{
		Backend: &slowBackend{started: make(chan struct{})},
		Profile: profile.Default(true),
		Runtime: runtime.Config{AllowedModels: []string{"allowed-model"}},
	})
	params, _ := json.Marshal(workersidecar.RunParams{
		RunID: "m1",
		Model: "missing-model",
		Goal:  "x",
	})
	req := JSONRPCRequest{JSONRPC: "2.0", Method: "acp.run", Params: params, ID: 4}
	resp := srv.dispatch(context.Background(), req)
	result, ok := resp.Result.(workersidecar.RunResult)
	if !ok {
		t.Fatalf("result type: %T", resp.Result)
	}
	if result.ErrorCode != "model_unavailable" {
		t.Fatalf("error_code = %q", result.ErrorCode)
	}
}
