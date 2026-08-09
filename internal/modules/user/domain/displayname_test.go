package domain_test

import (
	"strings"
	"testing"

	"github.com/fluentra/fluentra/internal/modules/user/domain"
	"github.com/fluentra/fluentra/internal/shared/apperr"
)

func TestValidateDisplayName_AcceptsRealNames(t *testing.T) {
	t.Parallel()

	names := []string{
		nameNghi,
		"Nguyễn Văn Nghi",
		"李雷",
		"O'Brien",
		"Jean-Luc",
		"x",
		strings.Repeat("a", 50),
	}
	for _, name := range names {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if err := domain.ValidateDisplayName(name); err != nil {
				t.Errorf("ValidateDisplayName(%q) = %v, want nil", name, err)
			}
		})
	}
}

func TestValidateDisplayName_RejectsShapeProblems(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		name string
		code string
	}{
		"empty":             {name: "", code: codeRequired},
		"only whitespace":   {name: "   ", code: codeRequired},
		"51 characters":     {name: strings.Repeat("a", 51), code: "LENGTH"},
		"control char":      {name: "Ng\x07hi", code: codeInvalidCharacter},
		"zero-width joiner": {name: "Ng\u200dhi", code: codeInvalidCharacter},
		"bidi override":     {name: "\u202eNghi", code: codeInvalidCharacter},
	}
	for label, testCase := range cases {
		t.Run(label, func(t *testing.T) {
			t.Parallel()
			err := domain.ValidateDisplayName(testCase.name)
			if err == nil {
				t.Fatalf("ValidateDisplayName(%q) = nil, want a validation error", testCase.name)
			}
			assertFieldCode(t, err, "display_name", testCase.code)
		})
	}
}

// TestValidateDisplayName_RejectsStaffImpersonation is BR-USER-02. The
// substitutions matter as much as the words: "Flu3ntra Supp0rt" is the form
// this actually arrives in.
func TestValidateDisplayName_RejectsStaffImpersonation(t *testing.T) {
	t.Parallel()

	names := []string{
		"admin",
		"Admin",
		"ADMIN",
		"Fluentra Support",
		"fluentra_support",
		"Flu3ntra",
		"$upport",
		"Fluentra Official",
		"Site Moderator",
		"security team",
		"a d m i n",
	}
	for _, name := range names {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			err := domain.ValidateDisplayName(name)
			if err == nil {
				t.Fatalf("ValidateDisplayName(%q) = nil, want it rejected as impersonation", name)
			}
			var appErr *apperr.Error
			if !asAppErr(err, &appErr) {
				t.Fatalf("error = %v, want an apperr.Error", err)
			}
			if appErr.Code != "DISPLAY_NAME_NOT_ALLOWED" {
				t.Errorf("code = %q, want DISPLAY_NAME_NOT_ALLOWED", appErr.Code)
			}
			if appErr.Status() != 422 {
				t.Errorf("status = %d, want 422", appErr.Status())
			}
		})
	}
}

// TestValidateDisplayName_KnownFalsePositives records the cost of the blunt
// fragment list. These are legitimate names the rule rejects. The test exists
// so the trade-off is visible and deliberate rather than discovered in a
// support ticket — if the list is ever refined, this test is what changes.
func TestValidateDisplayName_KnownFalsePositives(t *testing.T) {
	t.Parallel()

	for _, name := range []string{"Adminah", "Staffordshire Sam"} {
		if err := domain.ValidateDisplayName(name); err == nil {
			t.Errorf("ValidateDisplayName(%q) = nil; the fragment list is expected to reject it", name)
		}
	}
}
