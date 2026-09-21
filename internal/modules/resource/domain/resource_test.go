package domain_test

import (
	"testing"

	"github.com/fluentra/fluentra/internal/modules/resource/domain"
)

func TestCanTransition(t *testing.T) {
	tests := []struct {
		from     string
		to       string
		expected bool
	}{
		{domain.StatusPending, domain.StatusUploaded, true},
		{domain.StatusPending, domain.StatusFailed, true},
		{domain.StatusPending, domain.StatusValidated, false},
		{domain.StatusPending, domain.StatusRejected, false},

		{domain.StatusUploaded, domain.StatusValidated, true},
		{domain.StatusUploaded, domain.StatusRejected, true},
		{domain.StatusUploaded, domain.StatusFailed, true},
		{domain.StatusUploaded, domain.StatusPending, false},

		{domain.StatusValidated, domain.StatusUploaded, false},
		{domain.StatusValidated, domain.StatusFailed, false},

		{domain.StatusRejected, domain.StatusUploaded, false},
		{domain.StatusFailed, domain.StatusUploaded, false},
	}

	for _, tt := range tests {
		got := domain.CanTransition(tt.from, tt.to)
		if got != tt.expected {
			t.Errorf("CanTransition(%s, %s) = %v; want %v", tt.from, tt.to, got, tt.expected)
		}
	}
}

func TestIsValidKind(t *testing.T) {
	if !domain.IsValidKind("file") {
		t.Errorf("expected 'file' to be valid")
	}
	if !domain.IsValidKind("url") {
		t.Errorf("expected 'url' to be valid")
	}
	if domain.IsValidKind("archive") {
		t.Errorf("expected 'archive' to be invalid")
	}
}
