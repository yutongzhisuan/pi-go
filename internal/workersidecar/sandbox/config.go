package sandbox

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// DefaultImage mirrors Hermes DEFAULT_SANDBOX_IMAGE (extend/sub_agent/stateless.py).
const DefaultImage = "nikolaik/python-nodejs:python3.11-nodejs20"

// ContainerWorkdir is the fixed cwd inside disposable sandbox containers.
const ContainerWorkdir = "/workspace"

// Config holds operator-selected Docker sandbox settings for the sidecar process.
type Config struct {
	Image    string
	Network  bool
	CPUs     float64
	MemoryMB int
}

// ApplyEnv configures process-global terminal/docker env vars (Hermes apply_sandbox_env).
// Must run before the first agent/bash command in this process tree.
func (c Config) ApplyEnv() {
	os.Setenv("TERMINAL_ENV", "docker")
	image := strings.TrimSpace(c.Image)
	if image == "" {
		image = strings.TrimSpace(os.Getenv("TERMINAL_DOCKER_IMAGE"))
	}
	if image == "" {
		image = DefaultImage
	}
	os.Setenv("TERMINAL_DOCKER_IMAGE", image)
	if c.Network {
		os.Setenv("TERMINAL_DOCKER_NETWORK", "true")
	} else {
		os.Setenv("TERMINAL_DOCKER_NETWORK", "false")
	}
	os.Setenv("TERMINAL_CONTAINER_PERSISTENT", "false")
	os.Setenv("TERMINAL_DOCKER_PERSIST_ACROSS_PROCESSES", "false")
	if c.CPUs > 0 {
		os.Setenv("TERMINAL_CONTAINER_CPU", strconv.FormatFloat(c.CPUs, 'f', -1, 64))
	} else {
		os.Unsetenv("TERMINAL_CONTAINER_CPU")
	}
	if c.MemoryMB > 0 {
		os.Setenv("TERMINAL_CONTAINER_MEMORY", strconv.Itoa(c.MemoryMB))
	} else {
		os.Unsetenv("TERMINAL_CONTAINER_MEMORY")
	}
}

// EnvPairs returns KEY=value entries for child pi executor processes.
func (c Config) EnvPairs() []string {
	c.ApplyEnv()
	out := []string{
		"TERMINAL_ENV=docker",
		"TERMINAL_DOCKER_IMAGE=" + os.Getenv("TERMINAL_DOCKER_IMAGE"),
		"TERMINAL_DOCKER_NETWORK=" + os.Getenv("TERMINAL_DOCKER_NETWORK"),
		"TERMINAL_CONTAINER_PERSISTENT=false",
		"TERMINAL_DOCKER_PERSIST_ACROSS_PROCESSES=false",
	}
	if v := os.Getenv("TERMINAL_CONTAINER_CPU"); v != "" {
		out = append(out, "TERMINAL_CONTAINER_CPU="+v)
	}
	if v := os.Getenv("TERMINAL_CONTAINER_MEMORY"); v != "" {
		out = append(out, "TERMINAL_CONTAINER_MEMORY="+v)
	}
	return out
}

// FromFlags builds Config from worker-sidecar CLI flag values.
func FromFlags(image string, network bool, cpu float64, memoryMB int) Config {
	return Config{
		Image:    strings.TrimSpace(image),
		Network:  network,
		CPUs:     cpu,
		MemoryMB: memoryMB,
	}
}

// ValidateMode checks sandbox mode and Docker availability (fail-closed).
func ValidateMode(mode string) (*Config, error) {
	if mode == "" {
		return nil, nil
	}
	if mode != "docker" {
		return nil, fmt.Errorf("only --sandbox docker is supported")
	}
	if err := CheckDockerAvailable(); err != nil {
		return nil, fmt.Errorf("docker sandbox unavailable: %w", err)
	}
	return &Config{}, nil
}
