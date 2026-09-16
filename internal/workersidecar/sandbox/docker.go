package sandbox

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

// CheckDockerAvailable verifies the Docker CLI can reach a daemon.
func CheckDockerAvailable() error {
	if runtime.GOOS != "linux" {
		return fmt.Errorf("docker sandbox is only supported on linux (got %s)", runtime.GOOS)
	}
	docker, err := exec.LookPath("docker")
	if err != nil {
		return fmt.Errorf("docker CLI not found in PATH")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, docker, "info", "--format", "{{.ServerVersion}}")
	out, err := cmd.CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			msg = err.Error()
		}
		return fmt.Errorf("docker daemon not reachable: %s", msg)
	}
	if strings.TrimSpace(string(out)) == "" {
		return fmt.Errorf("docker daemon returned empty server version")
	}
	return nil
}

// ValidateDockerStartup is kept for tests and CLI; prefer ValidateMode with full config.
func ValidateDockerStartup(mode string) error {
	_, err := ValidateMode(mode)
	return err
}
