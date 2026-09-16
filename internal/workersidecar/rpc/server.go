package rpc

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/dimetron/pi-go/internal/subagent"
	"github.com/dimetron/pi-go/internal/workersidecar"
	"github.com/dimetron/pi-go/internal/workersidecar/profile"
)

// BackendRunner defines the interface for running agent sessions.
type BackendRunner interface {
	RunSession(ctx context.Context, params workersidecar.RunParams) workersidecar.RunResult
}

// Server implements the JSON-RPC 2.0 server for Worker ACP sidecar.
type Server struct {
	backend  BackendRunner
	profile  *profile.Profile
	runs     map[string]*runState
	progress map[string]*progressBucket
	mu       sync.RWMutex
	httpSrv  *http.Server
	listener net.Listener
}

// runState tracks an active or recently completed run.
type runState struct {
	running bool
	process *subagent.Process
	result  *workersidecar.RunResult
}

// progressBucket stores progress summaries for a run.
type progressBucket struct {
	summaries []string
	mu        sync.Mutex
}

// Config holds configuration for the RPC server.
type Config struct {
	Backend BackendRunner
	Profile *profile.Profile
}

// NewServer creates a new JSON-RPC server.
func NewServer(cfg Config) *Server {
	return &Server{
		backend:  cfg.Backend,
		profile:  cfg.Profile,
		runs:     make(map[string]*runState),
		progress: make(map[string]*progressBucket),
	}
}

// ListenUnix starts listening on a Unix domain socket.
func (s *Server) ListenUnix(socketPath string) error {
	if err := os.MkdirAll(filepath.Dir(socketPath), 0o700); err != nil {
		return fmt.Errorf("create socket dir: %w", err)
	}

	if err := os.Remove(socketPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove stale socket: %w", err)
	}

	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		return fmt.Errorf("listen on socket: %w", err)
	}

	if err := os.Chmod(socketPath, 0o600); err != nil {
		listener.Close()
		return fmt.Errorf("chmod socket: %w", err)
	}

	s.listener = listener
	return s.serve()
}

// ListenHTTP starts listening on an HTTP address.
func (s *Server) ListenHTTP(addr string) error {
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", addr, err)
	}

	s.listener = listener
	return s.serve()
}

// serve starts the HTTP server on the configured listener.
func (s *Server) serve() error {
	mux := http.NewServeMux()
	mux.HandleFunc("/rpc", s.handleRPC)
	mux.HandleFunc("/", s.handleRPC)

	s.httpSrv = &http.Server{
		Handler:      mux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	return s.httpSrv.Serve(s.listener)
}

// Close shuts down the server gracefully.
func (s *Server) Close() error {
	if s.httpSrv != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return s.httpSrv.Shutdown(ctx)
	}
	if s.listener != nil {
		return s.listener.Close()
	}
	return nil
}

// handleRPC processes JSON-RPC requests.
func (s *Server) handleRPC(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req JSONRPCRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeResponse(w, NewParseError(nil))
		return
	}

	resp := s.dispatch(r.Context(), req)
	s.writeResponse(w, resp)
}

// writeResponse writes a JSON-RPC response.
func (s *Server) writeResponse(w http.ResponseWriter, resp JSONRPCResponse) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		log.Printf("Error encoding response: %v", err)
	}
}

// dispatch routes requests to the appropriate handler.
func (s *Server) dispatch(ctx context.Context, req JSONRPCRequest) JSONRPCResponse {
	switch req.Method {
	case "acp.run":
		return s.handleRun(ctx, req)
	case "acp.cancel":
		return s.handleCancel(ctx, req)
	case "acp.status":
		return s.handleStatus(ctx, req)
	case "acp.progress":
		return s.handleProgress(ctx, req)
	case "acp.toolsets":
		return s.handleToolsets(ctx, req)
	default:
		return NewMethodNotFoundError(req.ID, req.Method)
	}
}

// handleRun implements acp.run method.
func (s *Server) handleRun(ctx context.Context, req JSONRPCRequest) JSONRPCResponse {
	var params workersidecar.RunParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return NewServerError(req.ID, fmt.Sprintf("invalid params: %v", err))
	}

	if params.RunID == "" {
		return NewServerError(req.ID, "missing run_id")
	}

	s.mu.Lock()
	if _, exists := s.runs[params.RunID]; exists {
		s.mu.Unlock()
		return NewServerError(req.ID, fmt.Sprintf("duplicate run_id: %s", params.RunID))
	}
	s.runs[params.RunID] = &runState{running: true}
	s.progress[params.RunID] = &progressBucket{}
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		if state, ok := s.runs[params.RunID]; ok {
			state.running = false
		}
		s.mu.Unlock()
	}()

	result := s.backend.RunSession(ctx, params)

	s.mu.Lock()
	if state, ok := s.runs[params.RunID]; ok {
		state.result = &result
	}
	s.mu.Unlock()

	return NewSuccessResponse(req.ID, result)
}

// handleCancel implements acp.cancel method.
func (s *Server) handleCancel(ctx context.Context, req JSONRPCRequest) JSONRPCResponse {
	var params workersidecar.CancelParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return NewServerError(req.ID, fmt.Sprintf("invalid params: %v", err))
	}

	s.mu.RLock()
	state, exists := s.runs[params.RunID]
	s.mu.RUnlock()

	if !exists || !state.running {
		return NewSuccessResponse(req.ID, workersidecar.CancelResult{
			Cancelled: false,
			Reason:    "run not found",
		})
	}

	if state.process != nil {
		state.process.Cancel()
	}

	return NewSuccessResponse(req.ID, workersidecar.CancelResult{
		Cancelled: true,
	})
}

// handleStatus implements acp.status method.
func (s *Server) handleStatus(ctx context.Context, req JSONRPCRequest) JSONRPCResponse {
	var params workersidecar.StatusParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return NewServerError(req.ID, fmt.Sprintf("invalid params: %v", err))
	}

	s.mu.RLock()
	state, exists := s.runs[params.RunID]
	s.mu.RUnlock()

	if !exists {
		return NewSuccessResponse(req.ID, workersidecar.StatusResult{Running: false})
	}

	return NewSuccessResponse(req.ID, workersidecar.StatusResult{Running: state.running})
}

// handleProgress implements acp.progress method.
func (s *Server) handleProgress(ctx context.Context, req JSONRPCRequest) JSONRPCResponse {
	var params workersidecar.ProgressParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return NewServerError(req.ID, fmt.Sprintf("invalid params: %v", err))
	}

	s.mu.RLock()
	bucket, exists := s.progress[params.RunID]
	s.mu.RUnlock()

	if !exists {
		return NewSuccessResponse(req.ID, workersidecar.ProgressResult{Summaries: []string{}})
	}

	bucket.mu.Lock()
	summaries := bucket.summaries
	bucket.summaries = nil
	bucket.mu.Unlock()

	s.mu.RLock()
	state, runExists := s.runs[params.RunID]
	s.mu.RUnlock()

	if !runExists || (!state.running && len(summaries) == 0) {
		s.mu.Lock()
		delete(s.progress, params.RunID)
		s.mu.Unlock()
	}

	return NewSuccessResponse(req.ID, workersidecar.ProgressResult{Summaries: summaries})
}

// handleToolsets implements acp.toolsets method.
func (s *Server) handleToolsets(ctx context.Context, req JSONRPCRequest) JSONRPCResponse {
	if s.profile == nil {
		return NewSuccessResponse(req.ID, workersidecar.ToolsetsResult{
			Toolsets: nil,
			Detail:   "no profile configured",
		})
	}

	toolsets := s.profile.Announce()
	return NewSuccessResponse(req.ID, workersidecar.ToolsetsResult{
		Toolsets: toolsets,
	})
}

// enqueueProgress adds a progress summary to the run's bucket.
func (s *Server) enqueueProgress(runID, summary string) {
	s.mu.RLock()
	bucket, exists := s.progress[runID]
	s.mu.RUnlock()

	if !exists {
		return
	}

	bucket.mu.Lock()
	bucket.summaries = append(bucket.summaries, summary)
	bucket.mu.Unlock()
}
