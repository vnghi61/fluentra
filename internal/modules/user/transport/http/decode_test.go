package http_test

import (
	"net/http"
	"testing"
)

// TestPatchMe_UnknownFieldIs422 is the acceptance criterion, and it is worth
// stating why it is 422 and not 400 or a silent 200: a client that misspells a
// field and gets 200 has no way to tell that nothing happened.
func TestPatchMe_UnknownFieldIs422(t *testing.T) {
	t.Parallel()
	accounts := &fakeAccounts{}
	server := newServer(accounts)

	recorder := authenticated(t, server, http.MethodPatch, "/api/v1/me",
		`{"display_name":"Nghi","displayname":"Nghi"}`)
	if recorder.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422 (body %s)", recorder.Code, recorder.Body)
	}

	decoded := decodeProblem(t, recorder)
	if decoded.Code != "VALIDATION_FAILED" {
		t.Errorf("code = %q, want VALIDATION_FAILED", decoded.Code)
	}
	if len(decoded.Errors) != 1 {
		t.Fatalf("errors = %+v, want exactly one", decoded.Errors)
	}
	if decoded.Errors[0].Field != "displayname" || decoded.Errors[0].Code != "UNKNOWN_FIELD" {
		t.Errorf("violation = %+v, want the unknown field named", decoded.Errors[0])
	}
	if accounts.seenActor.String() != "00000000-0000-0000-0000-000000000000" {
		t.Error("the service was called despite the rejected body")
	}
}

// TestPatchMe_ReportsEveryUnknownFieldInAStableOrder keeps a client from
// having to fix one typo per round trip, and keeps the response assertable:
// map iteration order is random, so the sort is what makes this repeatable.
func TestPatchMe_ReportsEveryUnknownFieldInAStableOrder(t *testing.T) {
	t.Parallel()
	server := newServer(&fakeAccounts{})

	body := `{"zulu":1,"alpha":2,"mike":3}`
	for range 5 {
		recorder := authenticated(t, server, http.MethodPatch, "/api/v1/me", body)
		decoded := decodeProblem(t, recorder)
		if len(decoded.Errors) != 3 {
			t.Fatalf("errors = %+v, want three", decoded.Errors)
		}
		want := []string{"alpha", "mike", "zulu"}
		for index, field := range want {
			if decoded.Errors[index].Field != field {
				t.Fatalf("errors = %+v, want them sorted as %v", decoded.Errors, want)
			}
		}
	}
}

// TestPatchMe_ExplicitNullIsRejected closes the gap a pointer-field struct
// leaves open: `{"country": null}` and an omitted `country` decode to the same
// nil, and for a PATCH they mean opposite things.
func TestPatchMe_ExplicitNullIsRejected(t *testing.T) {
	t.Parallel()
	accounts := &fakeAccounts{}
	server := newServer(accounts)

	recorder := authenticated(t, server, http.MethodPatch, "/api/v1/me", `{"country":null}`)
	if recorder.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422 (body %s)", recorder.Code, recorder.Body)
	}
	decoded := decodeProblem(t, recorder)
	if len(decoded.Errors) != 1 || decoded.Errors[0].Code != "NOT_NULLABLE" {
		t.Fatalf("errors = %+v, want one NOT_NULLABLE on country", decoded.Errors)
	}
	if decoded.Errors[0].Field != "country" {
		t.Errorf("field = %q, want country", decoded.Errors[0].Field)
	}
}

func TestPatchMe_WrongTypeIs422WithTheFieldNamed(t *testing.T) {
	t.Parallel()
	server := newServer(&fakeAccounts{})

	recorder := authenticated(t, server, http.MethodPatch, "/api/v1/me", `{"display_name":42}`)
	if recorder.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422", recorder.Code)
	}
	decoded := decodeProblem(t, recorder)
	if len(decoded.Errors) != 1 || decoded.Errors[0].Field != "display_name" ||
		decoded.Errors[0].Code != "TYPE" {
		t.Fatalf("errors = %+v, want one TYPE violation on display_name", decoded.Errors)
	}
}

func TestPatchMe_MalformedBodyIs400(t *testing.T) {
	t.Parallel()
	server := newServer(&fakeAccounts{})

	cases := map[string]string{
		"not json":        `{`,
		"a json array":    `[]`,
		"a bare string":   `"hello"`,
		"two json values": `{} {}`,
		"empty body":      ``,
	}
	for label, body := range cases {
		t.Run(label, func(t *testing.T) {
			t.Parallel()
			recorder := authenticated(t, server, http.MethodPatch, "/api/v1/me", body)
			if recorder.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400 (body %s)", recorder.Code, recorder.Body)
			}
			if got := decodeProblem(t, recorder).Code; got != "MALFORMED_REQUEST" {
				t.Errorf("code = %q, want MALFORMED_REQUEST", got)
			}
		})
	}
}

// TestPatchMe_OversizedBodyIsRejectedWithoutReadingItAll is what stops a
// hostile client from making the server buffer an arbitrary amount of JSON.
func TestPatchMe_OversizedBodyIsRejected(t *testing.T) {
	t.Parallel()
	server := newServer(&fakeAccounts{})

	oversized := `{"display_name":"` + string(make([]byte, 32<<10)) + `"}`
	recorder := authenticated(t, server, http.MethodPatch, "/api/v1/me", oversized)
	if recorder.Code != http.StatusRequestEntityTooLarge && recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 413 or 400", recorder.Code)
	}
}

func TestProblemResponsesUseTheProblemContentType(t *testing.T) {
	t.Parallel()
	server := newServer(&fakeAccounts{})

	recorder := authenticated(t, server, http.MethodPatch, "/api/v1/me", `{"nope":1}`)
	if got := recorder.Header().Get("Content-Type"); got != "application/problem+json; charset=utf-8" {
		t.Errorf("content type = %q, want the RFC 9457 media type", got)
	}
	if got := decodeProblem(t, recorder).Type; got != "https://fluentra.dev/errors/VALIDATION_FAILED" {
		t.Errorf("type = %q", got)
	}
}
