package tools

import "testing"

func TestCheckACPDenyBlocksWhenConfigured(t *testing.T) {
	t.Setenv(envACPDenyRules, `["sudo *"]`)
	if err := checkACPDeny("sudo apt update"); err == nil {
		t.Fatal("expected deny error")
	}
}

func TestCheckACPDenyNoRules(t *testing.T) {
	t.Setenv(envACPDenyRules, "")
	if err := checkACPDeny("sudo true"); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
}
