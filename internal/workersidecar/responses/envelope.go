package responses

import (
	"encoding/json"
	"fmt"
	"strings"
)

const (
	EnvelopeKey           = "responses.v1"
	ErrorInvalidEnvelope  = "invalid_responses_payload"
)

// ParsedEnvelope is the result of parsing params["responses.v1"] (Hermes subset).
type ParsedEnvelope struct {
	Present     bool
	UserMessage string
	ErrorCode   string
}

// ParseEnvelope parses the OpenAI Responses envelope when present in task params.
func ParseEnvelope(params map[string]interface{}, goal, boundModel string) ParsedEnvelope {
	if params == nil {
		return ParsedEnvelope{}
	}
	raw, ok := params[EnvelopeKey]
	if !ok {
		return ParsedEnvelope{}
	}
	rawStr, ok := raw.(string)
	if !ok {
		return ParsedEnvelope{Present: true, ErrorCode: ErrorInvalidEnvelope}
	}
	var env map[string]interface{}
	if err := json.Unmarshal([]byte(rawStr), &env); err != nil {
		return ParsedEnvelope{Present: true, ErrorCode: ErrorInvalidEnvelope}
	}
	req, ok := env["request"].(map[string]interface{})
	if !ok {
		return ParsedEnvelope{Present: true, ErrorCode: ErrorInvalidEnvelope}
	}
	instructions := stringField(req["instructions"])
	userMessage := normalizeUserMessage(req["input"], instructions, goal)
	_ = boundModel // echo-only in Hermes; execution model comes from run.model
	return ParsedEnvelope{
		Present:     true,
		UserMessage: userMessage,
	}
}

func normalizeUserMessage(rawInput interface{}, instructions, goal string) string {
	text := extractInputText(rawInput)
	if strings.TrimSpace(text) == "" {
		text = goal
	}
	if instructions != "" {
		return instructions + "\n\n" + text
	}
	return text
}

func extractInputText(rawInput interface{}) string {
	switch v := rawInput.(type) {
	case nil:
		return ""
	case string:
		return v
	case []interface{}:
		var lines []string
		for _, item := range v {
			m, ok := item.(map[string]interface{})
			if !ok {
				continue
			}
			role := strings.ToLower(strings.TrimSpace(stringField(m["role"])))
			if role == "" {
				role = "user"
			}
			body := contentText(m["content"])
			if strings.TrimSpace(body) == "" {
				continue
			}
			lines = append(lines, fmt.Sprintf("%s: %s", role, body))
		}
		return strings.Join(lines, "\n")
	default:
		return ""
	}
}

func contentText(raw interface{}) string {
	switch v := raw.(type) {
	case string:
		return v
	case []interface{}:
		var parts []string
		for _, item := range v {
			m, ok := item.(map[string]interface{})
			if !ok {
				continue
			}
			if t := stringField(m["text"]); t != "" {
				parts = append(parts, t)
			}
		}
		return strings.Join(parts, "\n")
	default:
		return ""
	}
}

func stringField(v interface{}) string {
	if v == nil {
		return ""
	}
	s, _ := v.(string)
	return strings.TrimSpace(s)
}
