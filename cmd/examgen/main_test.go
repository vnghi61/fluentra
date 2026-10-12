package main

import (
	"testing"
	"time"
)

func TestVerificationFromBody(t *testing.T) {
	body := []byte(`{
		"_provenance": {
			"model": "glm-5.3-flash",
			"verification": {"model": "qwen-3.8-27b", "verdict": "confirmed", "checked_at": "2026-09-24T10:00:00Z"}
		},
		"prompt": "x"
	}`)
	got := verificationFromBody(body)
	if !got.Confirmed {
		t.Error("a confirmed verdict must be lifted as confirmed")
	}
	if got.Model != "qwen-3.8-27b" {
		t.Errorf("model = %q, want the verifier's", got.Model)
	}
	if !got.CheckedAt.Equal(time.Date(2026, 9, 24, 10, 0, 0, 0, time.UTC)) {
		t.Errorf("checked_at = %v", got.CheckedAt)
	}

	if got := verificationFromBody([]byte(`{"prompt":"x"}`)); got.Confirmed {
		t.Error("a body without provenance is not confirmed")
	}
}
