package responses

import (
	"encoding/json"
	"testing"

	"github.com/dimetron/pi-go/internal/workersidecar"
)

func TestWrapRunResultDoesNotUseDurationSummaryAsOutput(t *testing.T) {
	parsed := ParsedEnvelope{Present: true, ResponseID: "r", MaxResultBytes: 4096}
	result := workersidecar.RunResult{
		Status:     "completed",
		ResultText: "",
		Summary:    "Completed in 0s",
	}
	out := WrapRunResult(result, parsed, "task-1", "bound")
	var obj map[string]interface{}
	if err := json.Unmarshal([]byte(out.ResultText), &obj); err != nil {
		t.Fatalf("parse: %v", err)
	}
	text := firstOutputTextFromResponse(obj)
	if text == "Completed in 0s" {
		t.Fatalf("output_text must not be duration summary, got %q", text)
	}
}

func firstOutputTextFromResponse(obj map[string]interface{}) string {
	output, _ := obj["output"].([]interface{})
	if len(output) == 0 {
		return ""
	}
	m, _ := output[0].(map[string]interface{})
	content, _ := m["content"].([]interface{})
	if len(content) == 0 {
		return ""
	}
	part, _ := content[0].(map[string]interface{})
	t, _ := part["text"].(string)
	return t
}

func TestWrapRunResultBuildsResponseJSON(t *testing.T) {
	parsed := ParsedEnvelope{
		Present:        true,
		ResponseID:     "resp-1",
		Model:          "m",
		Instructions:   "sys",
		MaxResultBytes: 4096,
		RequestEcho:    map[string]interface{}{"tools": []interface{}{}},
	}
	result := workersidecar.RunResult{
		Status:     "completed",
		ResultText: "done",
		Summary:    "",
	}
	out := WrapRunResult(result, parsed, "task-1", "bound")
	var obj map[string]interface{}
	if err := json.Unmarshal([]byte(out.ResultText), &obj); err != nil {
		t.Fatalf("parse: %v text=%q", err, out.ResultText)
	}
	if obj["object"] != "response" {
		t.Fatalf("object: %v", obj["object"])
	}
}

func TestSerializeResponseTrimsWhenOverBudget(t *testing.T) {
	obj := BuildResponseObject("r", "m", "", "completed", stringsRepeat("x", 12000), "t", "", nil, nil, nil)
	const budget = 4096
	s := SerializeResponse(obj, budget)
	if len([]byte(s)) > budget {
		t.Fatalf("expected trim under budget, got %d bytes", len([]byte(s)))
	}
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(s), &parsed); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	meta, _ := parsed["metadata"].(map[string]interface{})
	if meta["truncated"] != true {
		t.Fatalf("expected truncated metadata, got %v", meta)
	}
}

func stringsRepeat(s string, n int) string {
	out := make([]byte, 0, n*len(s))
	for i := 0; i < n; i++ {
		out = append(out, s...)
	}
	return string(out)
}
