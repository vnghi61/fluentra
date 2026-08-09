package http_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/google/uuid"

	"github.com/fluentra/fluentra/internal/shared/httpx"
)

// TestGetMe_ReadsTheActorFromTheContextAndNothingElse is the P1.2 acceptance
// criterion. The claim is "impossible by construction", so the test is about
// construction: no route carries a user id, and the id the handler passes down
// is the one that came out of the token.
func TestGetMe_ReadsTheActorFromTheContextAndNothingElse(t *testing.T) {
	t.Parallel()
	accounts := &fakeAccounts{}
	server := newServer(accounts)

	recorder := authenticated(t, server, http.MethodGet, "/api/v1/me", "")
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %s)", recorder.Code, recorder.Body)
	}
	if accounts.seenActor != actorID {
		t.Errorf("service was asked for %s, want the actor %s", accounts.seenActor, actorID)
	}
}

// TestGetMe_CannotBeAimedAtAnotherUser tries every way a client could attempt
// to name somebody else. None of them route: there is no `/users/{id}`, and
// query parameters, headers and a request body are all ignored because nothing
// reads them.
func TestGetMe_CannotBeAimedAtAnotherUser(t *testing.T) {
	t.Parallel()

	t.Run("no route accepts a user id", func(t *testing.T) {
		t.Parallel()
		server := newServer(&fakeAccounts{})
		for _, path := range []string{
			"/api/v1/me/" + otherID.String(),
			"/api/v1/users/" + otherID.String(),
			"/api/v1/users",
		} {
			recorder := authenticated(t, server, http.MethodGet, path, "")
			if recorder.Code != http.StatusNotFound {
				t.Errorf("GET %s = %d, want 404: this module must expose no user-id route", path, recorder.Code)
			}
		}
	})

	t.Run("a user id in the query string is ignored", func(t *testing.T) {
		t.Parallel()
		accounts := &fakeAccounts{}
		server := newServer(accounts)

		recorder := authenticated(t, server, http.MethodGet, "/api/v1/me?user_id="+otherID.String(), "")
		if recorder.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", recorder.Code)
		}
		if accounts.seenActor != actorID {
			t.Errorf("service was asked for %s, want %s: a query parameter changed the target",
				accounts.seenActor, actorID)
		}
	})

	t.Run("a user id in the patch body is an unknown field", func(t *testing.T) {
		t.Parallel()
		accounts := &fakeAccounts{}
		server := newServer(accounts)

		body := `{"user_id":"` + otherID.String() + `","display_name":"Nghi"}`
		recorder := authenticated(t, server, http.MethodPatch, "/api/v1/me", body)
		if recorder.Code != http.StatusUnprocessableEntity {
			t.Fatalf("status = %d, want 422", recorder.Code)
		}
		if accounts.seenActor != uuid.Nil {
			t.Error("the service was called even though the body was rejected")
		}
	})
}

func TestGetMe_WithoutAnActorIsUnauthenticated(t *testing.T) {
	t.Parallel()
	accounts := &fakeAccounts{}
	server := newServer(accounts)

	recorder := do(t, server, http.MethodGet, "/api/v1/me", "", nil)
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", recorder.Code)
	}
	if accounts.seenActor != uuid.Nil {
		t.Error("the service was called for an unauthenticated request")
	}
	if got := decodeProblem(t, recorder).Code; got != "UNAUTHENTICATED" {
		t.Errorf("code = %q, want UNAUTHENTICATED", got)
	}
}

// TestGetMe_ZeroActorIsTreatedAsUnauthenticated covers the failure mode the
// (Actor, bool) signature exists to prevent: a context carrying an Actor whose
// UserID was never set would otherwise query for the nil uuid.
func TestGetMe_ZeroActorIsTreatedAsUnauthenticated(t *testing.T) {
	t.Parallel()
	accounts := &fakeAccounts{}
	server := newServer(accounts)

	recorder := do(t, server, http.MethodGet, "/api/v1/me", "", &httpx.Actor{Role: "user"})
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", recorder.Code)
	}
	if accounts.seenActor != uuid.Nil {
		t.Errorf("the service was called with %s for an actor with no user id", accounts.seenActor)
	}
}

func TestGetMe_RendersTheSchemaShape(t *testing.T) {
	t.Parallel()
	server := newServer(&fakeAccounts{})

	recorder := authenticated(t, server, http.MethodGet, "/api/v1/me", "")
	if got := recorder.Header().Get("Content-Type"); got != "application/json; charset=utf-8" {
		t.Errorf("content type = %q", got)
	}

	var body map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	for _, field := range []string{"id", "email", "status", "created_at", "updated_at", "profile"} {
		if _, present := body[field]; !present {
			t.Errorf("response is missing %q", field)
		}
	}

	profile, ok := body["profile"].(map[string]any)
	if !ok {
		t.Fatalf("profile = %T, want an object", body["profile"])
	}
	for _, field := range []string{"display_name", "avatar_url", "country", "timezone", "date_of_birth"} {
		if _, present := profile[field]; !present {
			t.Errorf("profile is missing %q", field)
		}
	}
	if profile["date_of_birth"] != "1998-03-04" {
		t.Errorf("date_of_birth = %v, want the date-only form", profile["date_of_birth"])
	}
	// The avatar is nullable and still unset; it must be present and null
	// rather than omitted, because the schema declares it required-nullable.
	if profile["avatar_url"] != nil {
		t.Errorf("avatar_url = %v, want null", profile["avatar_url"])
	}
}

func TestPatchMe_PassesThroughOnlyTheFieldsSupplied(t *testing.T) {
	t.Parallel()
	accounts := &fakeAccounts{}
	server := newServer(accounts)

	recorder := authenticated(t, server, http.MethodPatch, "/api/v1/me", `{"display_name":"Nghi Nguyen"}`)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %s)", recorder.Code, recorder.Body)
	}

	change := accounts.seenChange
	if change.DisplayName == nil || *change.DisplayName != "Nghi Nguyen" {
		t.Errorf("display name = %v, want the submitted value", change.DisplayName)
	}
	// Everything else must arrive nil, which is what the query's COALESCE
	// reads as "leave it alone".
	if change.Country != nil || change.Timezone != nil || change.DateOfBirth != nil {
		t.Errorf("change = %+v, want only display_name populated", change)
	}
}

func TestPatchMe_ParsesADateOnlyValue(t *testing.T) {
	t.Parallel()
	accounts := &fakeAccounts{}
	server := newServer(accounts)

	recorder := authenticated(t, server, http.MethodPatch, "/api/v1/me", `{"date_of_birth":"1998-03-04"}`)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %s)", recorder.Code, recorder.Body)
	}
	if accounts.seenChange.DateOfBirth == nil {
		t.Fatal("date of birth did not reach the service")
	}
	if got := accounts.seenChange.DateOfBirth.Format("2006-01-02"); got != "1998-03-04" {
		t.Errorf("date = %s, want 1998-03-04", got)
	}
}
