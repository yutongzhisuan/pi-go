package sandbox

import (
	"os"
	"testing"
)

func TestValidateModeEmpty(t *testing.T) {
	cfg, err := ValidateMode("")
	if err != nil || cfg != nil {
		t.Fatalf("expected nil config, got cfg=%v err=%v", cfg, err)
	}
}

func TestValidateModeUnsupported(t *testing.T) {
	if _, err := ValidateMode("podman"); err == nil {
		t.Fatal("expected error for unsupported sandbox")
	}
}

func TestApplyEnvSetsTerminalVars(t *testing.T) {
	t.Setenv("TERMINAL_DOCKER_IMAGE", "")
	cfg := Config{Image: "img:test", Network: false, CPUs: 2, MemoryMB: 512}
	cfg.ApplyEnv()
	if got := os.Getenv("TERMINAL_ENV"); got != "docker" {
		t.Fatalf("TERMINAL_ENV=%q", got)
	}
	if got := os.Getenv("TERMINAL_DOCKER_IMAGE"); got != "img:test" {
		t.Fatalf("TERMINAL_DOCKER_IMAGE=%q", got)
	}
	if got := os.Getenv("TERMINAL_DOCKER_NETWORK"); got != "false" {
		t.Fatalf("TERMINAL_DOCKER_NETWORK=%q", got)
	}
	if got := os.Getenv("TERMINAL_CONTAINER_PERSISTENT"); got != "false" {
		t.Fatalf("TERMINAL_CONTAINER_PERSISTENT=%q", got)
	}
}

func TestCheckDockerAvailableOrSkip(t *testing.T) {
	if os.Getenv("PI_SKIP_DOCKER_TESTS") == "1" {
		t.Skip("docker tests disabled")
	}
	err := CheckDockerAvailable()
	if err != nil {
		t.Logf("docker unavailable (expected in some CI): %v", err)
		if _, vErr := ValidateMode("docker"); vErr == nil {
			t.Fatal("ValidateMode should fail when docker unavailable")
		}
		return
	}
	if _, err := ValidateMode("docker"); err != nil {
		t.Fatalf("ValidateMode(docker): %v", err)
	}
}
