package sandbox

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

// CheckDockerAvailable verifies the container CLI can reach a daemon.
func CheckDockerAvailable() error {
	if runtime.GOOS != "linux" {
		return fmt.Errorf("docker sandbox is only supported on linux (got %s)", runtime.GOOS)
	}
	docker, err := FindDocker()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, docker, "info", "--format", "{{.ServerVersion}}")
	out, runErr := cmd.CombinedOutput()
	if runErr != nil {
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			msg = runErr.Error()
		}
		return fmt.Errorf("docker daemon not reachable: %s", msg)
	}
	if strings.TrimSpace(string(out)) == "" {
		return fmt.Errorf("docker daemon returned empty server version")
	}
	return nil
}

// FindDocker locates docker or podman on PATH (Hermes find_docker subset).
func FindDocker() (string, error) {
	if override := strings.TrimSpace(os.Getenv("PI_DOCKER_BINARY")); override != "" {
		return override, nil
	}
	for _, name := range []string{"docker", "podman"} {
		if path, err := exec.LookPath(name); err == nil {
			return path, nil
		}
	}
	return "", fmt.Errorf("docker or podman not found in PATH")
}

// ValidateDockerStartup is kept for tests; prefer ValidateMode with full CLI wiring.
func ValidateDockerStartup(cfg Config) error {
	if cfg.Image != "" || cfg.Network || cfg.CPUs > 0 || cfg.MemoryMB > 0 {
		return CheckDockerAvailable()
	}
	_, err := ValidateMode("docker")
	return err
}
