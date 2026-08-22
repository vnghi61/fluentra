package domain_test

import (
	"errors"
	"testing"

	"github.com/fluentra/fluentra/internal/modules/content/domain"
	"github.com/fluentra/fluentra/internal/shared/apperr"
)

func TestValidateSlug(t *testing.T) {
	t.Parallel()
	cases := []struct {
		slug    string
		wantErr bool
		code    string
	}{
		{"hello-world", false, ""},
		{"a", false, ""},
		{"hello-world-123", false, ""},
		{"", true, "INVALID_SLUG"},
		{"Hello-World", true, "INVALID_SLUG"},
		{"hello_world", true, "INVALID_SLUG"},
		{"hello--world", true, "INVALID_SLUG"},
		{"-hello", true, "INVALID_SLUG"},
		{"hello-", true, "INVALID_SLUG"},
	}
	for _, tc := range cases {
		t.Run(tc.slug, func(t *testing.T) {
			err := domain.ValidateSlug(tc.slug)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("ValidateSlug(%q) = nil, want error", tc.slug)
				}
				var ae *apperr.Error
				if !errors.As(err, &ae) || ae.Code != tc.code {
					t.Errorf("code = %v, want %q", ae, tc.code)
				}
			} else if err != nil {
				t.Errorf("ValidateSlug(%q) = %v, want nil", tc.slug, err)
			}
		})
	}
}

func TestValidateKind(t *testing.T) {
	t.Parallel()
	if err := domain.ValidateKind("vocab_word"); err != nil {
		t.Errorf("valid kind: %v", err)
	}
	if err := domain.ValidateKind(""); err == nil {
		t.Error("empty kind should fail")
	}
	long := make([]byte, 51)
	for i := range long {
		long[i] = 'a'
	}
	if err := domain.ValidateKind(string(long)); err == nil {
		t.Error("51-char kind should fail")
	}
}

func TestValidateCEFRLevel(t *testing.T) {
	t.Parallel()
	valid := []string{"A1", "A2", "B1", "B2", "C1", "C2"}
	for _, lvl := range valid {
		if err := domain.ValidateCEFRLevel(lvl); err != nil {
			t.Errorf("ValidateCEFRLevel(%q) = %v, want nil", lvl, err)
		}
	}
	invalid := []string{"", "B3", "b1", "A0", "C3"}
	for _, lvl := range invalid {
		err := domain.ValidateCEFRLevel(lvl)
		if err == nil {
			t.Errorf("ValidateCEFRLevel(%q) = nil, want error", lvl)
			continue
		}
		var ae *apperr.Error
		if !errors.As(err, &ae) || ae.Code != "INVALID_CEFR_LEVEL" {
			t.Errorf("code = %v, want INVALID_CEFR_LEVEL", err)
		}
	}
}

func TestParseAuthoringStatus(t *testing.T) {
	t.Parallel()
	for _, s := range domain.AllAuthoringStatuses {
		got, err := domain.ParseAuthoringStatus(string(s))
		if err != nil || got != s {
			t.Errorf("ParseAuthoringStatus(%q) = %v, %v; want %q, nil", s, got, err, s)
		}
	}
	if _, err := domain.ParseAuthoringStatus("not-a-status"); err == nil {
		t.Error("invalid status should error")
	}
}

func TestParseReviewDecision(t *testing.T) {
	t.Parallel()
	for _, d := range []domain.ReviewDecision{domain.ReviewDecisionApproved, domain.ReviewDecisionChangesRequested} {
		got, err := domain.ParseReviewDecision(string(d))
		if err != nil || got != d {
			t.Errorf("ParseReviewDecision(%q) = %v, %v", d, got, err)
		}
	}
	if _, err := domain.ParseReviewDecision("bogus"); err == nil {
		t.Error("invalid decision should error")
	}
}

func TestAuthoringStatusString(t *testing.T) {
	t.Parallel()
	if domain.StatusDraft.String() != "draft" {
		t.Errorf("String() = %q, want draft", domain.StatusDraft.String())
	}
}
