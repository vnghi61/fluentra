package apperr

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestError_ToProblem_DoesNotExposeCauseOrInternal(t *testing.T) {
	t.Parallel()
	sentinel := errors.New("pq: secret query detail")
	err := New(Conflict, "DECK_LIMIT_REACHED", "Deck limit reached.").WithCause(sentinel).WithInternal("private diagnostic")
	body, marshalErr := json.Marshal(err.ToProblem("/decks", "request-1"))
	if marshalErr != nil {
		t.Fatalf("marshal problem: %v", marshalErr)
	}
	if strings.Contains(string(body), "secret") || strings.Contains(string(body), "private") {
		t.Fatalf("response leaked internal detail: %s", body)
	}
	if !errors.Is(err, sentinel) || !Is(err, Conflict) {
		t.Fatal("error classification or unwrapping failed")
	}
}

func TestError_Status_AllKinds(t *testing.T) {
	t.Parallel()
	tests := []struct {
		kind Kind
		want int
	}{{Validation, 422}, {BadRequest, 400}, {Unauthenticated, 401}, {Forbidden, 403}, {NotFound, 404}, {Conflict, 409}, {PreconditionFailed, 412}, {TooLarge, 413}, {RateLimited, 429}, {Unavailable, 503}, {Timeout, 504}, {Internal, 500}}
	for _, test := range tests {
		test := test
		t.Run(string(test.kind), func(t *testing.T) {
			t.Parallel()
			if got := New(test.kind, "CODE", "message").Status(); got != test.want {
				t.Fatalf("status = %d, want %d", got, test.want)
			}
			if problem := New(test.kind, "CODE", "message").ToProblem("", ""); problem.Title == "" {
				t.Fatal("problem title is empty")
			}
		})
	}
}

func TestError_BuildersAndProblem(t *testing.T) {
	t.Parallel()
	err := New(Validation, "VALIDATION_FAILED", "Invalid input.").
		WithFields(FieldViolation{Field: "email", Code: "email", Message: "invalid"}).
		WithMeta("limit", 5).
		WithRetryAfter(30)
	if err.Error() != "VALIDATION_FAILED: Invalid input." || err.Status() != 422 {
		t.Fatalf("unexpected error: %#v", err)
	}
	if err.RetryAfter != 30 || !err.Retryable || err.Meta["limit"] != 5 || len(err.Fields) != 1 {
		t.Fatalf("builder fields not set: %#v", err)
	}
	problem := err.ToProblem("/input", "request-1")
	if problem.Title != "Validation failed" || problem.Status != 422 || problem.Code != "VALIDATION_FAILED" {
		t.Fatalf("unexpected problem: %#v", problem)
	}
}

func TestError_Wrap(t *testing.T) {
	t.Parallel()
	if Wrap(nil, Internal, "INTERNAL_ERROR", "message") != nil {
		t.Fatal("Wrap(nil) must return nil")
	}
	original := New(NotFound, "RESOURCE_NOT_FOUND", "Not found.")
	if got := Wrap(original, Internal, "INTERNAL_ERROR", "message"); got != original {
		t.Fatal("Wrap must preserve an existing application error")
	}
	wrapped := Wrap(errors.New("database unavailable"), Unavailable, "DEPENDENCY_UNAVAILABLE", "Dependency unavailable.")
	if !Is(wrapped, Unavailable) || wrapped.Cause() == nil || wrapped.InternalDetail() != "" {
		t.Fatalf("unexpected wrapped error: %#v", wrapped)
	}
}

func TestError_NilProblemAndUnknownKind(t *testing.T) {
	t.Parallel()
	var nilError *Error
	if got := nilError.Error(); got != "" {
		t.Fatalf("nil error string = %q", got)
	}
	if got := nilError.Status(); got != 500 {
		t.Fatalf("nil status = %d", got)
	}
	problem := nilError.ToProblem("", "")
	if problem.Code != "INTERNAL_ERROR" || problem.Title != "Internal server error" {
		t.Fatalf("unexpected nil problem: %#v", problem)
	}
	if got := New(Kind("unknown"), "UNKNOWN", "message").Status(); got != 500 {
		t.Fatalf("unknown status = %d", got)
	}
	err := New(Internal, "INTERNAL_ERROR", "message").WithInternal("diagnostic")
	if err.InternalDetail() != "diagnostic" || err.Cause() != nil {
		t.Fatalf("unexpected details: %#v", err)
	}
	if err.Unwrap() != nil {
		t.Fatal("unwrap without cause must return nil")
	}
}
