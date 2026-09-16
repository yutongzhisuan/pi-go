package tools

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWorkerDockerRequiredWithoutContainer(t *testing.T) {
	t.Setenv(envWorkerSandboxDocker, "1")
	os.Unsetenv(envWorkerDockerContainer)
	sb, err := NewSandbox(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := sb.ReadFile("x.txt"); err == nil {
		t.Fatal("expected fail-closed read without container session")
	}
}

func TestContainerPathRelative(t *testing.T) {
	dir := t.TempDir()
	sb, err := NewSandbox(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv(envWorkerDockerContainer, "test-container")
	t.Setenv(envWorkerDockerWorkdir, "/workspace")
	rel, err := sb.containerPath("src/a.go")
	if err != nil {
		t.Fatal(err)
	}
	if rel != "/workspace/src/a.go" {
		t.Fatalf("container path: %q", rel)
	}
}

func TestDockerResultToSandboxPath(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "w"), 0o755); err != nil {
		t.Fatal(err)
	}
	sb, err := NewSandbox(filepath.Join(dir, "w"))
	if err != nil {
		t.Fatal(err)
	}
	got, err := dockerResultToSandboxPath(sb, "/workspace", "/workspace/pkg/main.go")
	if err != nil {
		t.Fatal(err)
	}
	if got != "pkg/main.go" {
		t.Fatalf("rel: %q", got)
	}
}
