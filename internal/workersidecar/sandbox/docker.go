package sandbox

import (
	"fmt"
	"runtime"
)

// ValidateDockerStartup enforces fail-closed policy for --sandbox docker.
// Full container-per-task execution is not implemented yet; starting would be a false security signal.
func ValidateDockerStartup(mode string) error {
	if mode == "" {
		return nil
	}
	if mode != "docker" {
		return fmt.Errorf("only --sandbox docker is supported")
	}
	if runtime.GOOS != "linux" {
		return fmt.Errorf("docker sandbox is only supported on linux (got %s)", runtime.GOOS)
	}
	return fmt.Errorf("docker sandbox not yet implemented; refusing to start with --sandbox docker (would be silently ignored). Remove --sandbox or use --stateless with executor toolsets")
}
