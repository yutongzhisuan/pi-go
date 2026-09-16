package confined

import "testing"

func TestEnforceStartupPolicyRefuses(t *testing.T) {
	if err := EnforceStartupPolicy(true); err == nil {
		t.Fatal("expected refuse for local-confined")
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
