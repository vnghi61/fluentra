package apperr

import (
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
)

func TestError_ToProblem_DoesNotExposeCauseOrInternal(t *testing.T) {
	t.Parallel()
	sentinel := errors.New("pq: secret query detail")
	err := New(Conflict, "DECK_LIMIT_REACHED", "Deck limit reached.").
		WithCause(sentinel).
		WithInternal("private diagnostic")
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
	}{
		{Validation, 422}, {BadRequest, 400}, {Unauthenticated, 401}, {Forbidden, 403},
		{NotFound, 404}, {Conflict, 409}, {PreconditionFailed, 412}, {TooLarge, 413},
		{RateLimited, 429}, {Unavailable, 503}, {Timeout, 504}, {Internal, 500},
	}
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

// TestWith_DoesNotMutateSharedError is the regression test for package-level
// sentinels: decorating one must not be visible to the next caller.
func TestWith_DoesNotMutateSharedError(t *testing.T) {
	t.Parallel()
	sentinel := New(NotFound, "USER_NOT_FOUND", "No such user.")

	decorated := sentinel.WithMeta("user_id", "u-1").WithFields(FieldViolation{Field: "id"}).WithRetryAfter(3)

	if len(sentinel.Meta) != 0 || len(sentinel.Fields) != 0 || sentinel.Retryable {
		t.Fatalf("sentinel was mutated: %#v", sentinel)
	}
	if decorated.Meta["user_id"] != "u-1" || len(decorated.Fields) != 1 || decorated.RetryAfter != 3 {
		t.Fatalf("copy did not carry the decoration: %#v", decorated)
	}
	if decorated == sentinel {
		t.Fatal("With* returned the same pointer")
	}
}

// TestWith_CopiesNestedContainers proves the copy does not share the sentinel's
// map or slice backing arrays.
func TestWith_CopiesNestedContainers(t *testing.T) {
	t.Parallel()
	base := New(Validation, "INVALID", "Invalid.").WithMeta("a", 1).WithFields(FieldViolation{Field: "x"})

	first := base.WithMeta("b", 2).WithFields(FieldViolation{Field: "y"})
	second := base.WithMeta("c", 3).WithFields(FieldViolation{Field: "z"})

	if len(base.Meta) != 1 || len(base.Fields) != 1 {
		t.Fatalf("base mutated: %#v", base)
	}
	if _, leaked := first.Meta["c"]; leaked {
		t.Fatalf("copies share a map: %#v", first.Meta)
	}
	if first.Fields[1].Field != "y" || second.Fields[1].Field != "z" {
		t.Fatalf("copies share a slice: %#v / %#v", first.Fields, second.Fields)
	}
}

func TestWith_ConcurrentDecorationIsRaceFree(t *testing.T) {
	sentinel := New(Conflict, "CONFLICT", "Conflict.")
	var wait sync.WaitGroup
	for index := range 50 {
		wait.Add(1)
		go func(n int) {
			defer wait.Done()
			_ = Wrap(sentinel, Internal, "X", "x").WithMeta("n", n).WithRetryAfter(n)
		}(index)
	}
	wait.Wait()
	if len(sentinel.Meta) != 0 || sentinel.Retryable {
		t.Fatalf("sentinel mutated under concurrency: %#v", sentinel)
	}
}
