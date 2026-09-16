package cli

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/dimetron/pi-go/internal/workersidecar/backend"
	"github.com/dimetron/pi-go/internal/workersidecar/profile"
	"github.com/dimetron/pi-go/internal/workersidecar/rpc"
)

var (
	workerSocket                  string
	workerHTTP                    bool
	workerHost                    string
	workerPort                    int
	workerStateless               bool
	workerStatelessToolsets       string
	workerStateRoot               string
	workerWorkRoot                string
	workerSandbox                 string
	workerSandboxImage            string
	workerSandboxNetwork          bool
	workerSandboxCPU              float64
	workerSandboxMemoryMB         int
	workerLocalConfined           bool
	workerLocalConfinedExtraDeny  string
	workerExecutorToolsets        string
	workerExecutorAllow           string
	workerProgressMode            string
	workerCheckpointEverySteps    int
	workerProgressIntervalSeconds float64
)

func newWorkerSidecarCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "worker-sidecar",
		Short: "Start the Worker ACP sidecar JSON-RPC server",
		Long: `Start the Hermes Worker ACP sidecar with JSON-RPC 2.0 over Unix socket or HTTP.

This sidecar provides acp.run, acp.cancel, acp.status, acp.progress, and acp.toolsets 
methods with exact Hermes wire compatibility for existing Hub/task-relay Workers.

Examples:
  pi worker-sidecar --stateless                    # Unix socket, default toolsets
  pi worker-sidecar --http --port 9105             # HTTP on 127.0.0.1:9105
  pi worker-sidecar --executor-toolsets file,web   # Custom toolset whitelist`,
		RunE: runWorkerSidecar,
	}

	defaultSocket := os.Getenv("TASK_RELAY_ACP_RPC_SOCKET")
	if defaultSocket == "" {
		defaultSocket = os.ExpandEnv("$HOME/.xhermes/sub_agent/acp.sock")
	}

	cmd.Flags().StringVar(&workerSocket, "socket", defaultSocket, "Unix socket path (Env: TASK_RELAY_ACP_RPC_SOCKET)")
	cmd.Flags().BoolVar(&workerHTTP, "http", false, "Use HTTP instead of Unix socket (Env: TASK_RELAY_ACP_RPC_HTTP=1)")
	cmd.Flags().StringVar(&workerHost, "host", "127.0.0.1", "HTTP host address")
	cmd.Flags().IntVar(&workerPort, "port", 9105, "HTTP port")
	cmd.Flags().BoolVar(&workerStateless, "stateless", false, "Run in stateless mode (disposable session, no local state)")
	cmd.Flags().StringVar(&workerStatelessToolsets, "stateless-toolsets", "", "Toolsets for stateless tasks when none requested")
	cmd.Flags().StringVar(&workerStateRoot, "state-root", "", "Directory for ephemeral stateless session store")
	cmd.Flags().StringVar(&workerWorkRoot, "workdir-root", "", "Parent directory for per-task temp workdirs")
	
	cmd.Flags().StringVar(&workerSandbox, "sandbox", "", "Sandbox backend (docker)")
	cmd.Flags().StringVar(&workerSandboxImage, "sandbox-image", "", "Docker image for sandboxed tasks")
	cmd.Flags().BoolVar(&workerSandboxNetwork, "sandbox-network", false, "Allow container network access")
	cmd.Flags().Float64Var(&workerSandboxCPU, "sandbox-cpu", 0, "CPU limit for sandboxed containers (e.g. 2.0)")
	cmd.Flags().IntVar(&workerSandboxMemoryMB, "sandbox-memory-mb", 0, "Memory limit in MB for sandboxed containers")
	
	cmd.Flags().BoolVar(&workerLocalConfined, "local-confined", false, "Trusted-task lightweight mode (implies --stateless)")
	cmd.Flags().StringVar(&workerLocalConfinedExtraDeny, "local-confined-extra-deny", "", "Extra deny globs for --local-confined")
	
	cmd.Flags().StringVar(&workerExecutorToolsets, "executor-toolsets", "", "Executor toolset whitelist (Env: ACP_EXECUTOR_TOOLSETS)")
	cmd.Flags().StringVar(&workerExecutorAllow, "executor-allow-extra", "", "Additional toolsets (Env: ACP_EXECUTOR_ALLOW_EXTRA)")
	
	cmd.Flags().StringVar(&workerProgressMode, "progress-mode", "", "Progress granularity: minimal, tools, off (Env: ACP_PROGRESS_MODE)")
	cmd.Flags().IntVar(&workerCheckpointEverySteps, "checkpoint-every-steps", 0, "Checkpoint every N steps (Env: ACP_CHECKPOINT_EVERY_STEPS)")
	cmd.Flags().Float64Var(&workerProgressIntervalSeconds, "acp-progress-interval-seconds", 5.0, "Min seconds between progress frames")

	return cmd
}

func runWorkerSidecar(cmd *cobra.Command, args []string) error {
	stateless := workerStateless || workerSandbox != "" || workerLocalConfined
	
	if workerSandbox != "" {
		if workerSandbox != "docker" {
			return fmt.Errorf("only --sandbox docker is supported")
		}
		return fmt.Errorf("Docker sandbox not yet implemented; fail-safe: refusing to start with --sandbox docker (sandbox would be silently ignored). Remove --sandbox to proceed without container isolation")
	}
	
	if workerLocalConfined {
		log.Printf("Warning: --local-confined approval deny rules not yet implemented")
	}
	
	allowedToolsets := resolveExecutorToolsets()
	
	if err := profile.ValidateToolsets(allowedToolsets); err != nil {
		return fmt.Errorf("invalid toolsets: %w", err)
	}
	
	prof := profile.New(allowedToolsets, stateless)
	
	workRoot := workerWorkRoot
	if workRoot == "" {
		workRoot = os.TempDir()
	}
	
	backendCfg := backend.Config{
		WorkRoot:  workRoot,
		Stateless: stateless,
	}
	
	back := backend.New(backendCfg)
	
	rpcCfg := rpc.Config{
		Backend: back,
		Profile: prof,
	}
	
	srv := rpc.NewServer(rpcCfg)
	
	useHTTP := workerHTTP || os.Getenv("TASK_RELAY_ACP_RPC_HTTP") != ""
	
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	
	errCh := make(chan error, 1)
	
	go func() {
		var err error
		if useHTTP {
			addr := fmt.Sprintf("%s:%d", workerHost, workerPort)
			log.Printf("Starting Worker sidecar on HTTP %s (stateless=%v)", addr, stateless)
			err = srv.ListenHTTP(addr)
		} else {
			log.Printf("Starting Worker sidecar on Unix socket %s (stateless=%v)", workerSocket, stateless)
			err = srv.ListenUnix(workerSocket)
		}
		errCh <- err
	}()
	
	select {
	case sig := <-sigCh:
		log.Printf("Received signal %v, shutting down", sig)
		return srv.Close()
	case err := <-errCh:
		if err != nil {
			return fmt.Errorf("server error: %w", err)
		}
		return nil
	}
}

func resolveExecutorToolsets() []string {
	allowedToolsets := profile.DefaultToolsets
	
	envToolsets := os.Getenv("ACP_EXECUTOR_TOOLSETS")
	if workerExecutorToolsets != "" {
		allowedToolsets = profile.ParseAllowedToolsets(workerExecutorToolsets)
	} else if envToolsets != "" {
		allowedToolsets = profile.ParseAllowedToolsets(envToolsets)
	}
	
	envExtra := os.Getenv("ACP_EXECUTOR_ALLOW_EXTRA")
	extraStr := workerExecutorAllow
	if extraStr == "" {
		extraStr = envExtra
	}
	
	if extraStr != "" {
		extra := profile.ParseAllowedToolsets(extraStr)
		allowedToolsets = append(allowedToolsets, extra...)
	}
	
	return allowedToolsets
}
