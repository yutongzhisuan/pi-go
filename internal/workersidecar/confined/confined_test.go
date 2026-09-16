package confined

import (
	"encoding/json"
	"testing"
)

func TestEnforceStartupPolicyAllowsLocalConfined(t *testing.T) {
	if err := EnforceStartupPolicy(true); err != nil {
		t.Fatalf("local-confined should start with deny enforcement: %v", err)
	}
	if err := EnforceStartupPolicy(false); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestMergeRulesIncludesDefaults(t *testing.T) {
	rules := MergeRules([]string{"custom *"})
	if len(rules) <= len(DefaultDenyRules) {
		t.Fatalf("expected merged rules, got %d", len(rules))
	}
}

func TestDenyRulesJSONRoundTrip(t *testing.T) {
	raw, err := DenyRulesJSON([]string{"custom *"})
	if err != nil {
		t.Fatal(err)
	}
	var decoded []string
	if err := json.Unmarshal([]byte(raw), &decoded); err != nil {
		t.Fatal(err)
	}
	if len(decoded) <= len(DefaultDenyRules) {
		t.Fatalf("expected defaults in JSON, got %d rules", len(decoded))
	}
}
