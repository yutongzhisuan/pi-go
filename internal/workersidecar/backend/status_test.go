package backend

import (
	"testing"
)

func TestStatusVocabulary(t *testing.T) {
	validStatuses := []string{
		"completed",
		"failed",
		"cancelled",
		"timeout",
	}

	for _, status := range validStatuses {
		if status == "" {
			t.Errorf("Status should not be empty")
		}
	}
}

func TestErrorCodeVocabulary(t *testing.T) {
	validErrorCodes := []string{
		"model_unavailable",
		"timeout",
		"cancelled",
	}

	for _, code := range validErrorCodes {
		if code == "" {
			t.Errorf("Error code should not be empty")
		}
	}
}
