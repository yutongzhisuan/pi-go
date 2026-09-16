package tools

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

const (
	envTerminalEnv            = "TERMINAL_ENV"
	envTerminalDockerImage    = "TERMINAL_DOCKER_IMAGE"
	envTerminalDockerNetwork  = "TERMINAL_DOCKER_NETWORK"
	envTerminalContainerCPU   = "TERMINAL_CONTAINER_CPU"
	envTerminalContainerMem   = "TERMINAL_CONTAINER_MEMORY"
	defaultSandboxDockerImage = "nikolaik/python-nodejs:python3.11-nodejs20"
)

func dockerTerminalEnabled() bool {
	v := strings.TrimSpace(strings.ToLower(os.Getenv(envTerminalEnv)))
	return v == "docker"
}

func dockerNetworkEnabled() bool {
	v := strings.TrimSpace(strings.ToLower(os.Getenv(envTerminalDockerNetwork)))
	switch v {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func dockerRunArgs(hostWorkDir, script string) ([]string, error) {
	image := strings.TrimSpace(os.Getenv(envTerminalDockerImage))
	if image == "" {
		image = defaultSandboxDockerImage
	}
	workDir := strings.TrimSpace(hostWorkDir)
	if workDir == "" {
		return nil, fmt.Errorf("docker sandbox requires a host workdir")
	}

	args := []string{
		"run", "--rm",
		"--cap-drop", "ALL",
		"--cap-add", "DAC_OVERRIDE",
		"--cap-add", "CHOWN",
		"--cap-add", "FOWNER",
		"--security-opt", "no-new-privileges",
		"--tmpfs", "/tmp:rw,nosuid,size=512m",
		"--tmpfs", "/var/tmp:rw,noexec,nosuid,size=256m",
		"-v", workDir + ":/workspace:rw",
		"-w", "/workspace",
	}
	if !dockerNetworkEnabled() {
		args = append(args, "--network=none")
	}
	if cpu := strings.TrimSpace(os.Getenv(envTerminalContainerCPU)); cpu != "" {
		if f, err := strconv.ParseFloat(cpu, 64); err == nil && f > 0 {
			args = append(args, "--cpus", cpu)
		}
	}
	if mem := strings.TrimSpace(os.Getenv(envTerminalContainerMem)); mem != "" {
		if n, err := strconv.Atoi(mem); err == nil && n > 0 {
			args = append(args, "--memory", fmt.Sprintf("%dm", n))
		}
	}
	args = append(args, image, "bash", "-c", script)
	return args, nil
}

func dockerShellCommand(ctx context.Context, hostWorkDir, script string) (*exec.Cmd, error) {
	docker, err := exec.LookPath("docker")
	if err != nil {
		return nil, fmt.Errorf("docker CLI not found: %w", err)
	}
	args, err := dockerRunArgs(hostWorkDir, script)
	if err != nil {
		return nil, err
	}
	return exec.CommandContext(ctx, docker, args...), nil
}
