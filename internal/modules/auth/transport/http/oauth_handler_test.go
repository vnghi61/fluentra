package http_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/fluentra/fluentra/internal/modules/auth/domain"
	"github.com/fluentra/fluentra/internal/modules/auth/service"
	authhttp "github.com/fluentra/fluentra/internal/modules/auth/transport/http"
	"github.com/fluentra/fluentra/internal/shared/httpx"
)

// newOAuthRouter wires a router whose only interesting collaborator is Google.
func newOAuthRouter(oauth authhttp.OAuth) chi.Router {
	return newTestRouterWithOAuth(
		&fakeRegistrationService{}, &fakeAuthenticator{}, &fakeRotator{}, &fakeSessions{},
		&fakePasswords{}, &fakeDevices{}, oauth, authhttp.CookieOptions{Secure: true})
}

// callbackRequest builds a well-formed callback POST.
func callbackRequest(path string) *http.Request {
	request := httptest.NewRequest(http.MethodPost, path,
		strings.NewReader(`{"code":"an-auth-code","state":"a-state"}`))
	request.Header.Set("Content-Type", "application/json")
	return request
}

// TestHandler_OAuthStartRevealsOnlyTheURL is the response shape as a security
// property rather than as a schema detail.
//
// The `state`, the `nonce` and the PKCE verifier are all generated for this flow
// and none of them may appear in the body: a value the page can read is a value
// an attacker who can read the same page can replay, and the verifier in
// particular would defeat PKCE outright.
func TestHandler_OAuthStartRevealsOnlyTheURL(t *testing.T) {
	oauth := &fakeOAuth{}
	router := newOAuthRouter(oauth)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(
		http.MethodGet, "/api/v1/auth/oauth/google/start?redirect_to=/lessons/3", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}

	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if len(body) != 1 {
		t.Errorf("the response carries %d members, want only authorization_url: %v", len(body), body)
	}
	if _, ok := body["authorization_url"]; !ok {
		t.Error("no authorization_url in the response")
	}
	for _, leaked := range []string{"state", "nonce", "code_verifier", "verifier", "pkce"} {
		if _, present := body[leaked]; present {
			t.Errorf("the response carries %q, which only this server is supposed to know", leaked)
		}
	}
	if oauth.sawRedirect != "/lessons/3" {
		t.Errorf("redirect_to reached the service as %q, want the query value", oauth.sawRedirect)
	}
}

// TestHandler_OAuthCallbackSetsTheRefreshCookie: what the callback produced is a
// session, and it has to arrive the same way login's does — otherwise the
// browser holds an access token it can never renew.
func TestHandler_OAuthCallbackSetsTheRefreshCookie(t *testing.T) {
	router := newOAuthRouter(&fakeOAuth{})

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, callbackRequest("/api/v1/auth/oauth/google/callback"))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}

	cookies := rec.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("no refresh cookie was set, so the session cannot be renewed")
	}
	cookie := cookies[0]
	if !cookie.HttpOnly || !cookie.Secure || cookie.SameSite != http.SameSiteLaxMode ||
		cookie.Path != refreshCookiePath {
		t.Errorf("the google sign-in cookie is weaker than the login one: %+v", cookie)
	}
}

// TestHandler_OAuthRefusalsKeepTheirStatus checks that the refusals the spec
// promises reach a client unchanged.
//
// They are not interchangeable, and the client acts on the difference: 409
// OAUTH_ACCOUNT_CONFLICT means "verify that address first", 403 means "this
// Google account cannot be used here", 401 means "start again", 503 means "try
// again shortly". A handler that flattened them into one refusal would make the
// flow unusable without ever saying anything untrue.
func TestHandler_OAuthRefusalsKeepTheirStatus(t *testing.T) {
	for name, testCase := range map[string]struct {
		failure error
		status  int
		code    string
	}{
		"an unverified local match": {
			domain.ErrOAuthAccountConflict, http.StatusConflict, "OAUTH_ACCOUNT_CONFLICT",
		},
		"an unverified provider email": {
			domain.ErrOAuthEmailUnverified, http.StatusForbidden, "OAUTH_EMAIL_UNVERIFIED",
		},
		"a bad id token": {
			domain.ErrOAuthTokenInvalid, http.StatusUnauthorized, "TOKEN_INVALID",
		},
		"google unreachable": {
			domain.ErrOAuthProviderUnavailable, http.StatusServiceUnavailable, "DEPENDENCY_UNAVAILABLE",
		},
	} {
		t.Run(name, func(t *testing.T) {
			router := newOAuthRouter(&fakeOAuth{
				callbackFn: func(context.Context, service.CallbackInput) (service.SignedIn, error) {
					return service.SignedIn{}, testCase.failure
				},
			})

			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, callbackRequest("/api/v1/auth/oauth/google/callback"))

			if rec.Code != testCase.status {
				t.Errorf("status = %d, want %d; body: %s", rec.Code, testCase.status, rec.Body.String())
			}
			if !strings.Contains(rec.Body.String(), testCase.code) {
				t.Errorf("body does not carry %s: %s", testCase.code, rec.Body.String())
			}
			if len(rec.Result().Cookies()) != 0 {
				t.Error("a refused sign-in set a cookie")
			}
		})
	}
}

// TestHandler_TheOAuthLinkOperationsRefuseAnAnonymousCaller: both are `self`,
// and the middleware lets a request with no Authorization header through, so a
// handler that forgot to ask for an actor would act on uuid.Nil.
func TestHandler_TheOAuthLinkOperationsRefuseAnAnonymousCaller(t *testing.T) {
	oauth := &fakeOAuth{
		linkFn: func(context.Context, httpx.Actor, service.CallbackInput) (service.LinkedIdentity, error) {
			t.Error("link reached the service with no actor")
			return service.LinkedIdentity{}, nil
		},
		unlinkFn: func(context.Context, httpx.Actor) error {
			t.Error("unlink reached the service with no actor")
			return nil
		},
	}
	router := newOAuthRouter(oauth)

	for name, request := range map[string]*http.Request{
		"link":   callbackRequest("/api/v1/auth/oauth/google/link"),
		"unlink": httptest.NewRequest(http.MethodDelete, "/api/v1/auth/oauth/google", nil),
	} {
		t.Run(name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, request)
			if rec.Code != http.StatusUnauthorized {
				t.Errorf("status = %d, want 401; body: %s", rec.Code, rec.Body.String())
			}
		})
	}
}

// TestHandler_LinkPassesTheActorThroughAndSetsNoCookie.
//
// The caller already holds a session, and rotating it for an operation that
// changed nothing about their sign-in would sign them out of every other tab for
// adding a second way in.
func TestHandler_LinkPassesTheActorThroughAndSetsNoCookie(t *testing.T) {
	oauth := &fakeOAuth{}
	router := newOAuthRouter(oauth)
	actor := testActor(uuid.New())

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, signedIn(callbackRequest("/api/v1/auth/oauth/google/link"), actor))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
	if oauth.sawActor.UserID != actor.UserID {
		t.Errorf("the service saw %s, want the signed-in caller %s", oauth.sawActor.UserID, actor.UserID)
	}
	if len(rec.Result().Cookies()) != 0 {
		t.Error("linking rotated the caller's session cookie, signing out their other tabs")
	}

	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	for _, member := range []string{"provider", "email", "linked_at"} {
		if _, ok := body[member]; !ok {
			t.Errorf("the response has no %q member", member)
		}
	}
}

// TestHandler_UnlinkAnswers204AndTheLastMethodRefusalIs409.
func TestHandler_UnlinkAnswers204AndTheLastMethodRefusalIs409(t *testing.T) {
	actor := testActor(uuid.New())

	t.Run("removed", func(t *testing.T) {
		oauth := &fakeOAuth{}
		rec := httptest.NewRecorder()
		newOAuthRouter(oauth).ServeHTTP(rec, signedIn(
			httptest.NewRequest(http.MethodDelete, "/api/v1/auth/oauth/google", nil), actor))

		if rec.Code != http.StatusNoContent {
			t.Errorf("status = %d, want 204; body: %s", rec.Code, rec.Body.String())
		}
		if oauth.sawActor.UserID != actor.UserID {
			t.Errorf("the service saw %s, want %s", oauth.sawActor.UserID, actor.UserID)
		}
	})

	t.Run("the only way in", func(t *testing.T) {
		router := newOAuthRouter(&fakeOAuth{
			unlinkFn: func(context.Context, httpx.Actor) error { return domain.ErrLastSignInMethod },
		})
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, signedIn(
			httptest.NewRequest(http.MethodDelete, "/api/v1/auth/oauth/google", nil), actor))

		if rec.Code != http.StatusConflict {
			t.Errorf("status = %d, want 409; body: %s", rec.Code, rec.Body.String())
		}
		if !strings.Contains(rec.Body.String(), "LAST_SIGN_IN_METHOD") {
			t.Errorf("body does not carry LAST_SIGN_IN_METHOD: %s", rec.Body.String())
		}
	})
}

// TestHandler_TheCallbackRequiresBothMembersAndNothingElse.
//
// The code is worthless without the state and the state is worthless unless this
// server issued it, so a body missing either is refused rather than
// half-processed. An unknown member is refused too — that is what
// `additionalProperties: false` means on this side, and it is what stops a
// client believing it can hand us an `id_token` we would trust.
func TestHandler_TheCallbackRequiresBothMembersAndNothingElse(t *testing.T) {
	router := newOAuthRouter(&fakeOAuth{
		callbackFn: func(context.Context, service.CallbackInput) (service.SignedIn, error) {
			t.Error("an incomplete callback body reached the service")
			return service.SignedIn{}, nil
		},
	})

	for name, body := range map[string]string{
		"no state":          `{"code":"an-auth-code"}`,
		"no code":           `{"state":"a-state"}`,
		"an unknown member": `{"code":"c","state":"s","id_token":"forged"}`,
	} {
		t.Run(name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/oauth/google/callback",
				strings.NewReader(body))
			request.Header.Set("Content-Type", "application/json")
			router.ServeHTTP(rec, request)

			if rec.Code != http.StatusUnprocessableEntity && rec.Code != http.StatusBadRequest {
				t.Errorf("status = %d, want a refusal; body: %s", rec.Code, rec.Body.String())
			}
		})
	}
}
