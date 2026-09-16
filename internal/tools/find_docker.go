package tools

import (
	"bufio"
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

func findViaWorkerDocker(sb *Sandbox, input FindInput) (FindOutput, bool, error) {
	sess, err := workerDockerSession()
	if err != nil {
		return FindOutput{}, true, err
	}
	if sess == nil {
		return FindOutput{}, false, nil
	}
	searchPath := input.Path
	if searchPath == "" {
		searchPath = "."
	}
	cp, err := sb.containerPath(searchPath)
	if err != nil {
		return FindOutput{}, true, err
	}
	pattern := input.Pattern
	args := []string{"find", cp, "-type", "f", "-name", pattern}
	ctx := context.Background()
	cmd := exec.CommandContext(ctx, sess.bin, append([]string{
		"exec", "-i", "-w", sess.workdir, sess.container,
	}, args...)...)
	out, err := cmd.Output()
	if err != nil {
		return FindOutput{}, true, fmt.Errorf("docker find: %w", err)
	}
	var files []string
	total := 0
	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		total++
		if len(files) >= maxFindResults {
			continue
		}
		if rel, err := dockerResultToSandboxPath(sb, sess.workdir, line); err == nil {
			files = append(files, rel)
		} else {
			files = append(files, line)
		}
	}
	return FindOutput{
		Files:      files,
		TotalFiles: total,
		Truncated:  total > len(files),
	}, true, nil
}

func dockerResultToSandboxPath(sb *Sandbox, workdir, abs string) (string, error) {
	abs = filepath.ToSlash(abs)
	wd := filepath.ToSlash(workdir)
	if !strings.HasPrefix(abs, wd) {
		return abs, fmt.Errorf("path outside container workdir")
	}
	rel := strings.TrimPrefix(abs, wd)
	rel = strings.TrimPrefix(rel, "/")
	if rel == "" {
		return ".", nil
	}
	return rel, nil
}
