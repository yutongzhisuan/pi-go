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

func TestMatchHardlineBlocksRedirectToBlockDevice(t *testing.T) {
	if _, ok := MatchHardline("echo x > /dev/sda"); !ok {
		t.Fatal("expected redirect to block device block")
	}
}

func TestMatchHardlineBlocksKillAll(t *testing.T) {
	if _, ok := MatchHardline("kill -1"); !ok {
		t.Fatal("expected kill -1 block")
	}
}

func TestMatchHardlineBlocksRmEtc(t *testing.T) {
	if _, ok := MatchHardline("rm -rf /etc"); !ok {
		t.Fatal("expected rm /etc block")
	}
}

func TestMatchHardlineSystemctlAfterSeparator(t *testing.T) {
	if _, ok := MatchHardline("true; systemctl poweroff"); !ok {
		t.Fatal("expected systemctl poweroff after ;")
	}
}
