package tools

import (
	"os"
	"testing"
)

func TestCheckACPDenyBlocksWhenConfigured(t *testing.T) {
	t.Setenv(envACPLocalConfined, "1")
	t.Setenv(envACPDenyRules, `["sudo *"]`)
	if err := checkACPDeny("sudo apt update"); err == nil {
		t.Fatal("expected deny error")
	}
}

func TestCheckACPDenyHardlineBeforeGlobs(t *testing.T) {
	t.Setenv(envACPLocalConfined, "1")
	t.Setenv(envACPDenyRules, `[]`)
	if err := checkACPDeny("rm -rf /"); err == nil {
		t.Fatal("expected hardline block")
	}
}

func TestCheckACPDenyNoRulesWhenInactive(t *testing.T) {
	os.Unsetenv(envACPLocalConfined)
	os.Unsetenv(envACPDenyRules)
	if err := checkACPDeny("sudo true"); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
}
