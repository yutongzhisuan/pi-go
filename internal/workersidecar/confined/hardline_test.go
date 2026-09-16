package confined

import "testing"

func TestMatchHardlineBlocksRmRoot(t *testing.T) {
	if desc, ok := MatchHardline("rm -rf /"); !ok || desc == "" {
		t.Fatalf("expected hardline block, got %q ok=%v", desc, ok)
	}
}

func TestMatchHardlineIgnoresEchoData(t *testing.T) {
	if _, ok := MatchHardline(`echo "rm -rf /"`); ok {
		t.Fatal("expected no hardline match inside echo string")
	}
}

func TestMatchHardlineShutdownAtCommandPosition(t *testing.T) {
	if _, ok := MatchHardline("sudo reboot"); !ok {
		t.Fatal("expected reboot block")
	}
}
