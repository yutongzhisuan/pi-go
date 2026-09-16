package tools

import (
	"os"
	"strings"
	"testing"
)

func TestDockerShellCommandArgs(t *testing.T) {
	t.Setenv(envTerminalEnv, "docker")
	t.Setenv(envTerminalDockerImage, "alpine:3")
	t.Setenv(envTerminalDockerNetwork, "false")
	t.Setenv(envTerminalContainerCPU, "1.5")
	t.Setenv(envTerminalContainerMem, "256")

	args, err := dockerRunArgs(t.TempDir(), "echo hi")
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(args, " ")
	for _, want := range []string{
		"run", "--rm", "--network=none", "alpine:3", "bash", "-c", "echo hi",
		"/workspace", "--cpus", "1.5", "--memory", "256m",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("missing %q in %q", want, joined)
		}
	}
}

func TestDockerTerminalDisabledByDefault(t *testing.T) {
	os.Unsetenv(envTerminalEnv)
	if dockerTerminalEnabled() {
		t.Fatal("expected docker terminal off")
	}
}
