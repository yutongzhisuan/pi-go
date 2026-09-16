package sandbox

import "testing"

func TestValidateDockerStartupRefuses(t *testing.T) {
	if err := ValidateDockerStartup("docker"); err == nil {
		t.Fatal("expected error for docker sandbox")
	}
	if err := ValidateDockerStartup(""); err != nil {
		t.Fatalf("unexpected error for empty sandbox: %v", err)
	}
}
