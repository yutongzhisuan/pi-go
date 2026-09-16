package sandbox

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// Session is a reusable per-run Docker container bound to a host workdir.
type Session struct {
	dockerBin   string
	containerID string
	name        string
}

// StartSession creates a detached container with the task workdir mounted at /workspace.
func StartSession(ctx context.Context, cfg Config, runID, hostWorkDir string) (*Session, error) {
	dockerBin, err := FindDocker()
	if err != nil {
		return nil, err
	}
	name := sanitizeContainerName("pi-go-acp-" + runID)
	args := []string{
		"run", "-d",
		"--name", name,
		"-v", hostWorkDir + ":" + ContainerWorkdir + ":rw",
		"-w", ContainerWorkdir,
		"--label", "pi-go-acp=1",
		"--label", "pi-go-run-id=" + sanitizeLabel(runID),
		"--cap-drop", "ALL",
		"--security-opt", "no-new-privileges",
	}
	if !cfg.Network {
		args = append(args, "--network", "none")
	}
	if cfg.MemoryMB > 0 {
		args = append(args, "--memory", fmt.Sprintf("%dm", cfg.MemoryMB))
	}
	if cfg.CPUs > 0 {
		args = append(args, "--cpus", fmt.Sprintf("%g", cfg.CPUs))
	}
	image := cfg.ResolvedImage()
	args = append(args, image, "sleep", "infinity")

	cmd := exec.CommandContext(ctx, dockerBin, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("docker run: %w: %s", err, strings.TrimSpace(string(out)))
	}
	id := strings.TrimSpace(string(out))
	if id == "" {
		return nil, fmt.Errorf("docker run returned empty container id")
	}
	return &Session{dockerBin: dockerBin, containerID: id, name: name}, nil
}

// ContainerID returns the Docker container id for this session.
func (s *Session) ContainerID() string {
	if s == nil {
		return ""
	}
	return s.containerID
}

// Destroy stops and removes the container.
func (s *Session) Destroy(ctx context.Context) {
	if s == nil || s.containerID == "" {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, s.dockerBin, "rm", "-f", s.containerID)
	_ = cmd.Run()
}

func sanitizeContainerName(raw string) string {
	var b strings.Builder
	for _, r := range raw {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_':
			b.WriteRune(r)
		default:
			b.WriteRune('-')
		}
	}
	name := b.String()
	if name == "" {
		return "pi-go-acp-run"
	}
	if len(name) > 63 {
		name = name[:63]
	}
	return name
}

func sanitizeLabel(raw string) string {
	var b strings.Builder
	for _, r := range raw {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_', r == '.':
			b.WriteRune(r)
		default:
			b.WriteRune('-')
		}
	}
	out := b.String()
	if out == "" {
		return "unknown"
	}
	if len(out) > 63 {
		out = out[:63]
	}
	return out
}
