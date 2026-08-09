package domain_test

import (
	"errors"
	"testing"

	"github.com/fluentra/fluentra/internal/shared/apperr"
)

// Codes and fixtures shared across this package's tests.
const (
	codeRequired         = "REQUIRED"
	codeInvalidCharacter = "INVALID_CHARACTER"
	codeNotIANA          = "NOT_IANA"
	nameNghi             = "Nghi"
	timezoneHoChiMinh    = "Asia/Ho_Chi_Minh"
)

// asAppErr is errors.As with the target spelled out once, so the tests below
// read as assertions rather than as error plumbing.
func asAppErr(err error, target **apperr.Error) bool { return errors.As(err, target) }

// assertFieldCode checks that err is a 422 naming exactly the field and code
// given. Every validation failure in this package carries a field, because a
// 422 that does not say which field is wrong is not actionable.
func assertFieldCode(t *testing.T, err error, field, code string) {
	t.Helper()

	var appErr *apperr.Error
	if !asAppErr(err, &appErr) {
		t.Fatalf("error = %v, want an apperr.Error", err)
	}
	if appErr.Status() != 422 {
		t.Fatalf("status = %d, want 422", appErr.Status())
	}
	if len(appErr.Fields) != 1 {
		t.Fatalf("fields = %+v, want exactly one", appErr.Fields)
	}
	if appErr.Fields[0].Field != field {
		t.Errorf("field = %q, want %q", appErr.Fields[0].Field, field)
	}
	if appErr.Fields[0].Code != code {
		t.Errorf("code = %q, want %q", appErr.Fields[0].Code, code)
	}
	if appErr.Fields[0].Message == "" {
		t.Error("message is empty; the client has nothing to show the learner")
	}
}
