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
	workerSocket           string
	workerHTTP             bool
	workerHost             string
	workerPort             int
	workerStateless        bool
	workerExecutorToolsets string
	workerExecutorAllow    string
	workerWorkRoot         string
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

	cmd.Flags().StringVar(&workerSocket, "socket", defaultSocket, "Unix socket path")
	cmd.Flags().BoolVar(&workerHTTP, "http", false, "Use HTTP instead of Unix socket")
	cmd.Flags().StringVar(&workerHost, "host", "127.0.0.1", "HTTP host address")
	cmd.Flags().IntVar(&workerPort, "port", 9105, "HTTP port")
	cmd.Flags().BoolVar(&workerStateless, "stateless", false, "Run in stateless mode")
	cmd.Flags().StringVar(&workerExecutorToolsets, "executor-toolsets", "", "Executor toolset whitelist (comma-separated)")
	cmd.Flags().StringVar(&workerExecutorAllow, "executor-allow-extra", "", "Additional toolsets to allow (comma-separated)")
	cmd.Flags().StringVar(&workerWorkRoot, "workdir-root", "", "Root directory for run workdirs")

	return cmd
}

func runWorkerSidecar(cmd *cobra.Command, args []string) error {
	allowedToolsets := profile.DefaultToolsets
	if workerExecutorToolsets != "" {
		allowedToolsets = profile.ParseAllowedToolsets(workerExecutorToolsets)
	}

	if workerExecutorAllow != "" {
		extra := profile.ParseAllowedToolsets(workerExecutorAllow)
		allowedToolsets = append(allowedToolsets, extra...)
	}

	if err := profile.ValidateToolsets(allowedToolsets); err != nil {
		return fmt.Errorf("invalid toolsets: %w", err)
	}

	prof := profile.New(allowedToolsets, workerStateless)

	workRoot := workerWorkRoot
	if workRoot == "" {
		workRoot = os.TempDir()
	}

	backendCfg := backend.Config{
		WorkRoot:  workRoot,
		Stateless: workerStateless,
	}

	back := backend.New(backendCfg)

	rpcCfg := rpc.Config{
		Backend: back,
		Profile: prof,
	}

	srv := rpc.NewServer(rpcCfg)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	errCh := make(chan error, 1)

	go func() {
		var err error
		if workerHTTP {
			addr := fmt.Sprintf("%s:%d", workerHost, workerPort)
			log.Printf("Starting Worker sidecar on HTTP %s", addr)
			err = srv.ListenHTTP(addr)
		} else {
			log.Printf("Starting Worker sidecar on Unix socket %s", workerSocket)
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
