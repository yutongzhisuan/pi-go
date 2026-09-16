package responses

import "testing"

func TestParseEnvelopeAbsent(t *testing.T) {
	p := ParseEnvelope(nil, "goal", "m")
	if p.Present {
		t.Fatal("expected absent envelope")
	}
}

func TestParseEnvelopeValidStringInput(t *testing.T) {
	params := map[string]interface{}{
		EnvelopeKey: `{"request":{"input":"do work","instructions":"sys"}}`,
	}
	p := ParseEnvelope(params, "fallback", "m")
	if !p.Present || p.ErrorCode != "" {
		t.Fatalf("unexpected parse: %+v", p)
	}
	if p.UserMessage != "sys\n\ndo work" {
		t.Fatalf("user message: %q", p.UserMessage)
	}
}

func TestParseEnvelopeInvalid(t *testing.T) {
	params := map[string]interface{}{EnvelopeKey: `{bad`}
	p := ParseEnvelope(params, "g", "m")
	if p.ErrorCode != ErrorInvalidEnvelope {
		t.Fatalf("error code: %q", p.ErrorCode)
	}
}

func TestParseEnvelopeStructuredReplayUserMessage(t *testing.T) {
	params := map[string]interface{}{
		EnvelopeKey: `{"request":{"input":[
			{"role":"user","content":"first"},
			{"role":"assistant","content":"mid"},
			{"role":"user","content":"last"}
		],"instructions":"sys"}}`,
	}
	p := ParseEnvelope(params, "fallback", "m")
	if !p.Replay.Structured {
		t.Fatal("expected structured replay")
	}
	if p.UserMessage != "sys\n\nlast" {
		t.Fatalf("user message = %q", p.UserMessage)
	}
	if len(p.Replay.History) != 2 {
		t.Fatalf("history len = %d", len(p.Replay.History))
	}
}
