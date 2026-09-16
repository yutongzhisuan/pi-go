package confined

import (
	"encoding/json"
	"testing"
)

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
