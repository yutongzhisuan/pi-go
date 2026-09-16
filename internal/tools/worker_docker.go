package tools

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"time"
)

const (
	envWorkerSandboxDocker = "PI_WORKER_SANDBOX_DOCKER"
)

type dockerWorkerSession struct {
	bin       string
	container string
	workdir   string
}

func workerDockerRequired() bool {
	v := strings.TrimSpace(strings.ToLower(os.Getenv(envWorkerSandboxDocker)))
	switch v {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func workerDockerSession() (*dockerWorkerSession, error) {
	container := strings.TrimSpace(os.Getenv(envWorkerDockerContainer))
	if container == "" {
		if workerDockerRequired() {
			return nil, fmt.Errorf("docker sandbox session missing (PI_WORKER_DOCKER_CONTAINER unset)")
		}
		return nil, nil
	}
	bin := strings.TrimSpace(os.Getenv(envWorkerDockerBin))
	if bin == "" {
		bin = "docker"
	}
	workdir := strings.TrimSpace(os.Getenv(envWorkerDockerWorkdir))
	if workdir == "" {
		workdir = "/workspace"
	}
	return &dockerWorkerSession{bin: bin, container: container, workdir: workdir}, nil
}

func (s *Sandbox) containerPath(name string) (string, error) {
	rel, err := s.Resolve(name)
	if err != nil {
		return "", err
	}
	rel = filepath.ToSlash(rel)
	if rel == "." {
		return s.workerDockerWorkdir()
	}
	wd, err := s.workerDockerWorkdir()
	if err != nil {
		return "", err
	}
	return path.Join(wd, rel), nil
}

func (s *Sandbox) workerDockerWorkdir() (string, error) {
	sess, err := workerDockerSession()
	if err != nil || sess == nil {
		return "", err
	}
	return sess.workdir, nil
}

func (s *Sandbox) dockerReadFile(name string) ([]byte, error) {
	sess, err := workerDockerSession()
	if err != nil {
		return nil, err
	}
	if sess == nil {
		return nil, nil
	}
	cp, err := s.containerPath(name)
	if err != nil {
		return nil, err
	}
	ctx := context.Background()
	cmd := exec.CommandContext(ctx, sess.bin, "exec", "-i", "-w", sess.workdir, sess.container,
		"cat", cp)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("docker read %s: %w: %s", name, err, strings.TrimSpace(string(out)))
	}
	return out, nil
}

func (s *Sandbox) dockerWriteFile(name string, data []byte, perm os.FileMode) error {
	sess, err := workerDockerSession()
	if err != nil {
		return err
	}
	if sess == nil {
		return nil
	}
	cp, err := s.containerPath(name)
	if err != nil {
		return err
	}
	ctx := context.Background()
	dir := path.Dir(cp)
	if dir != "" && dir != "." {
		mkdir := exec.CommandContext(ctx, sess.bin, "exec", "-i", "-w", sess.workdir, sess.container,
			"mkdir", "-p", dir)
		if out, err := mkdir.CombinedOutput(); err != nil {
			return fmt.Errorf("docker mkdir: %w: %s", err, strings.TrimSpace(string(out)))
		}
	}
	cmd := exec.CommandContext(ctx, sess.bin, "exec", "-i", "-w", sess.workdir, sess.container,
		"tee", cp)
	cmd.Stdin = bytes.NewReader(data)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("docker write %s: %w: %s", name, err, strings.TrimSpace(string(out)))
	}
	if perm != 0 {
		chmod := exec.CommandContext(ctx, sess.bin, "exec", "-i", sess.container,
			"chmod", fmt.Sprintf("%o", perm&0o777), cp)
		_ = chmod.Run()
	}
	return nil
}

func (s *Sandbox) dockerStat(name string) (os.FileInfo, error) {
	sess, err := workerDockerSession()
	if err != nil {
		return nil, err
	}
	if sess == nil {
		return nil, nil
	}
	cp, err := s.containerPath(name)
	if err != nil {
		return nil, err
	}
	ctx := context.Background()
	cmd := exec.CommandContext(ctx, sess.bin, "exec", "-i", sess.container,
		"stat", "-c", "%F %s", cp)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("docker stat %s: %w", name, err)
	}
	fields := strings.SplitN(strings.TrimSpace(string(out)), " ", 2)
	if len(fields) < 1 {
		return nil, fmt.Errorf("docker stat %s: unexpected output", name)
	}
	isDir := strings.Contains(strings.ToLower(fields[0]), "directory")
	return &dockerFileInfo{name: name, isDir: isDir}, nil
}

type dockerFileInfo struct {
	name  string
	isDir bool
}

func (d *dockerFileInfo) Name() string       { return filepath.Base(d.name) }
func (d *dockerFileInfo) Size() int64        { return 0 }
func (d *dockerFileInfo) Mode() os.FileMode  { return 0o644 }
func (d *dockerFileInfo) ModTime() time.Time { return time.Time{} }
func (d *dockerFileInfo) IsDir() bool        { return d.isDir }
func (d *dockerFileInfo) Sys() any           { return nil }

// openReadStream returns a reader for sandbox file content (docker exec cat when session active).
func (s *Sandbox) openReadStream(name string) (io.ReadCloser, error) {
	sess, err := workerDockerSession()
	if err != nil {
		return nil, err
	}
	if sess == nil {
		f, err := s.openHost(name)
		if err != nil {
			return nil, err
		}
		return f, nil
	}
	cp, err := s.containerPath(name)
	if err != nil {
		return nil, err
	}
	ctx := context.Background()
	cmd := exec.CommandContext(ctx, sess.bin, "exec", "-i", "-w", sess.workdir, sess.container,
		"cat", cp)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("docker cat %s: %w", name, err)
	}
	return &dockerCatReader{cmd: cmd, r: stdout}, nil
}

type dockerCatReader struct {
	cmd *exec.Cmd
	r   io.ReadCloser
}

func (d *dockerCatReader) Read(p []byte) (int, error) { return d.r.Read(p) }

func (d *dockerCatReader) Close() error {
	err := d.r.Close()
	waitErr := d.cmd.Wait()
	if err != nil {
		return err
	}
	return waitErr
}

func (s *Sandbox) openHost(name string) (*os.File, error) {
	root, rel, err := s.resolveToRoot(name)
	if err != nil {
		return nil, err
	}
	return root.Open(rel)
}

