package domain_test

import (
	"testing"

	"github.com/fluentra/fluentra/internal/modules/learning/domain"
)

func TestAttempt_StatusHelpers(t *testing.T) {
	inProgress := &domain.Attempt{Status: domain.StatusInProgress}
	if !inProgress.CanSubmit() {
		t.Errorf("expected CanSubmit() true for in_progress attempt")
	}
	if inProgress.IsGraded() {
		t.Errorf("expected IsGraded() false for in_progress attempt")
	}

	graded := &domain.Attempt{Status: domain.StatusGraded}
	if graded.CanSubmit() {
		t.Errorf("expected CanSubmit() false for graded attempt")
	}
	if !graded.IsGraded() {
		t.Errorf("expected IsGraded() true for graded attempt")
	}

	grading := &domain.Attempt{Status: domain.StatusGrading}
	if !grading.IsGrading() {
		t.Errorf("expected IsGrading() true for grading attempt")
	}
}

func TestValidStatus(t *testing.T) {
	cases := []struct {
		status string
		valid  bool
	}{
		{domain.StatusInProgress, true},
		{domain.StatusGrading, true},
		{domain.StatusGraded, true},
		{domain.StatusFailed, true},
		{"unknown_status", false},
		{"", false},
		// The three the schema rejects. ck_attempts_status accepts four values
		// and these are not among them.
		{"expired", false},
		{"abandoned", false},
		{"completed", false},
	}

	for _, tc := range cases {
		if got := domain.ValidStatus(tc.status); got != tc.valid {
			t.Errorf("ValidStatus(%q) = %v, want %v", tc.status, got, tc.valid)
		}
	}
}
