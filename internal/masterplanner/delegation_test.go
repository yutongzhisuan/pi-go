package masterplanner

import (
	"strings"
	"testing"
)

func TestDelegationRefusal(t *testing.T) {
	t.Cleanup(func() { SetDelegatedChild(false) })
	if DelegationRefusalJSON() != "" {
		t.Fatal("expected no refusal by default")
	}
	SetDelegatedChild(true)
	raw := DelegationRefusalJSON()
	if raw == "" {
		t.Fatal("expected refusal payload")
	}
	if !strings.Contains(raw, "delegated_child_refused") {
		t.Fatalf("unexpected payload: %s", raw)
	}
}

func TestGatewayGuardBlocksDispatch(t *testing.T) {
	t.Cleanup(func() { SetDelegatedChild(false) })
	SetDelegatedChild(true)
	msg, stop := gatewayGuard()
	if !stop || msg == "" {
		t.Fatalf("gatewayGuard() = %q, %v", msg, stop)
	}
}
