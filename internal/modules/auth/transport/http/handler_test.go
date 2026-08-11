package http_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/fluentra/fluentra/internal/modules/auth/domain"
	"github.com/fluentra/fluentra/internal/modules/auth/service"
	authhttp "github.com/fluentra/fluentra/internal/modules/auth/transport/http"
	"github.com/fluentra/fluentra/internal/shared/apperr"
	"github.com/fluentra/fluentra/internal/shared/httpx"
	"github.com/fluentra/fluentra/internal/shared/secret"
)

const testChallengeID = "00000000-0000-0000-0000-000000000001"

type fakeRegistrationService struct {
	registerFn    func(ctx context.Context, req service.Registration) (service.Issued, error)
	verifyEmailFn func(ctx context.Context, challengeID uuid.UUID, code string) (service.Verification, error)
	resendFn      func(ctx context.Context, challengeID uuid.UUID) (service.Issued, error)
}

func (f *fakeRegistrationService) Register(ctx context.Context, request service.Registration) (service.Issued, error) {
	if f.registerFn != nil {
		return f.registerFn(ctx, request)
	}
	code, _ := domain.NewCode(6)
	return service.Issued{
		Challenge: domain.Challenge{
			ID:          uuid.MustParse(testChallengeID),
			Purpose:     domain.PurposeVerifyEmail,
			MaxAttempts: 5,
			ExpiresAt:   time.Now().Add(10 * time.Minute),
			LastSentAt:  time.Now(),
		},
		Code: code,
	}, nil
}

func (f *fakeRegistrationService) VerifyEmail(
	ctx context.Context, challengeID uuid.UUID, code string,
) (service.Verification, error) {
	if f.verifyEmailFn != nil {
		return f.verifyEmailFn(ctx, challengeID, code)
	}
	return service.Verification{
		Purpose:    domain.PurposeVerifyEmail,
		VerifiedAt: time.Now().UTC(),
		SignedIn:   testSignedIn(),
	}, nil
}

func (f *fakeRegistrationService) Resend(ctx context.Context, challengeID uuid.UUID) (service.Issued, error) {
	if f.resendFn != nil {
		return f.resendFn(ctx, challengeID)
	}
	code, _ := domain.NewCode(6)
	return service.Issued{
		Challenge: domain.Challenge{
			ID:          challengeID,
			Purpose:     domain.PurposeVerifyEmail,
			MaxAttempts: 5,
			ExpiresAt:   time.Now().Add(10 * time.Minute),
			LastSentAt:  time.Now(),
		},
		Code: code,
	}, nil
}

type fakeAuthenticator struct {
	loginFn func(ctx context.Context, input service.LoginInput) (service.LoginResult, error)
}

func (f *fakeAuthenticator) Login(ctx context.Context, input service.LoginInput) (service.LoginResult, error) {
	if f.loginFn != nil {
		return f.loginFn(ctx, input)
	}
	return service.LoginResult{UserID: uuid.MustParse(testChallengeID), SignedIn: testSignedIn()}, nil
}

// fakeRotator stands in for the refresh service. The rotation rules themselves
// are proved against Postgres in the module integration suite; what the handler
// owes them is the cookie, which is what these tests look at.
type fakeRotator struct {
	rotateFn func(ctx context.Context, presented string) (service.SignedIn, error)
}

func (f *fakeRotator) Rotate(ctx context.Context, presented string) (service.SignedIn, error) {
	if f.rotateFn != nil {
		return f.rotateFn(ctx, presented)
	}
	return testSignedIn(), nil
}

// testSignedIn is a session plus a refresh token that is obviously not real.
func testSignedIn() service.SignedIn {
	return service.SignedIn{
		Session:          testSession(),
		RefreshToken:     secret.New(testCookieValue),
		RefreshExpiresAt: time.Now().UTC().Add(service.DefaultRefreshTTL),
	}
}

// Named around gosec: a constant whose name contains "token" trips G101's
// hardcoded-credential heuristic, and naming around a false positive is cheaper
// than a nolint a reader then has to evaluate.
const testCookieValue = "not-a-real-cookie-value" // gitleaks:allow

// refreshCookiePath is duplicated from the transport package on purpose: it is
// the value a browser keys the cookie on, so a test that read it from the
// package under test would agree with any change, including a wrong one.
const refreshCookiePath = "/api/v1/auth"

func newTestRouter(reg authhttp.Registration) chi.Router {
	return newTestRouterWithAuth(reg, &fakeAuthenticator{})
}

func newTestRouterWithAuth(reg authhttp.Registration, auth authhttp.Authenticator) chi.Router {
	return newTestRouterWith(reg, auth, &fakeRotator{}, authhttp.CookieOptions{Secure: true})
}

func newTestRouterWith(
	reg authhttp.Registration, auth authhttp.Authenticator,
	rotator authhttp.Rotator, cookies authhttp.CookieOptions,
) chi.Router {
	r := chi.NewRouter()
	r.Route("/api/v1", func(api chi.Router) {
		handler := authhttp.NewHandler(reg, auth, rotator, cookies)
		handler.Routes(api)
	})
	return r
}

func TestHandler_Register_Success(t *testing.T) {
	serviceFake := &fakeRegistrationService{}
	router := newTestRouter(serviceFake)

	body := `{"email":"learner@example.com","password":"password12345","display_name":"Learner"}` // gitleaks:allow
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	if _, ok := resp["code"]; ok {
		t.Fatal("code field MUST NOT be in registration HTTP response body")
	}

	if resp["challenge_id"] != testChallengeID {
		t.Fatalf("challenge_id = %v, want %s", resp["challenge_id"], testChallengeID)
	}
}

func TestHandler_Register_MissingFields(t *testing.T) {
	serviceFake := &fakeRegistrationService{}
	router := newTestRouter(serviceFake)

	body := `{"email":"learner@example.com"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422 for missing required fields; body: %s", rec.Code, rec.Body.String())
	}
}

func TestHandler_Register_PasswordPolicyViolation(t *testing.T) {
	serviceFake := &fakeRegistrationService{
		registerFn: func(_ context.Context, _ service.Registration) (service.Issued, error) {
			return service.Issued{}, apperr.New(apperr.Validation, "PASSWORD_TOO_WEAK", "Password does not meet requirements.")
		},
	}
	router := newTestRouter(serviceFake)

	body := `{"email":"learner@example.com","password":"short","display_name":"Learner"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422; body: %s", rec.Code, rec.Body.String())
	}
}

func TestHandler_Verify_Success(t *testing.T) {
	serviceFake := &fakeRegistrationService{}
	router := newTestRouter(serviceFake)

	challengeID := testChallengeID
	body := `{"code":"123456"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/challenges/"+challengeID+"/verify", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	if resp["purpose"] != "verify_email" {
		t.Fatalf("purpose = %v, want verify_email", resp["purpose"])
	}
}

func TestHandler_Verify_InvalidUUID(t *testing.T) {
	serviceFake := &fakeRegistrationService{}
	router := newTestRouter(serviceFake)

	body := `{"code":"123456"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/challenges/invalid-uuid/verify", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404 for malformed challenge UUID; body: %s", rec.Code, rec.Body.String())
	}
}

func TestHandler_Resend_Success(t *testing.T) {
	serviceFake := &fakeRegistrationService{}
	router := newTestRouter(serviceFake)

	challengeID := testChallengeID
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/challenges/"+challengeID+"/resend", bytes.NewBuffer(nil))
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	if _, ok := resp["code"]; ok {
		t.Fatal("code field MUST NOT be in resend HTTP response body")
	}
}

func TestHandler_CodeNeverInResponseJSON(t *testing.T) {
	serviceFake := &fakeRegistrationService{
		registerFn: func(_ context.Context, _ service.Registration) (service.Issued, error) {
			return service.Issued{
				Challenge: domain.Challenge{
					ID:          uuid.New(),
					Purpose:     domain.PurposeVerifyEmail,
					MaxAttempts: 5,
					ExpiresAt:   time.Now().Add(10 * time.Minute),
				},
				Code: secret.New("654321"),
			}, nil
		},
	}
	router := newTestRouter(serviceFake)

	body := `{"email":"learner@example.com","password":"password12345","display_name":"Learner"}` // gitleaks:allow
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if strings.Contains(rec.Body.String(), "654321") {
		t.Fatalf("plaintext code 654321 was leaked in HTTP response body: %s", rec.Body.String())
	}
}

func TestHandler_Login_Success(t *testing.T) {
	serviceFake := &fakeRegistrationService{}
	router := newTestRouter(serviceFake)

	body := `{"email":"learner@example.com","password":"password12345"}` // gitleaks:allow
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
}

func TestHandler_Login_UsesOnlyTrustedResolvedClientIP(t *testing.T) {
	var got service.LoginInput
	auth := &fakeAuthenticator{
		loginFn: func(_ context.Context, input service.LoginInput) (service.LoginResult, error) {
			got = input
			return service.LoginResult{UserID: uuid.New(), SignedIn: testSignedIn()}, nil
		},
	}
	resolver, err := httpx.NewClientIPResolver(nil)
	if err != nil {
		t.Fatalf("NewClientIPResolver: %v", err)
	}
	router := resolver.Middleware(newTestRouterWithAuth(&fakeRegistrationService{}, auth))

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login",
		strings.NewReader(`{"email":"learner@example.com","password":"password12345"}`)) // gitleaks:allow
	req.RemoteAddr = "198.51.100.42:4242"
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Forwarded-For", "203.0.113.9")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
	if got.ClientIP != "198.51.100.42" {
		t.Fatalf("ClientIP = %q, want unspoofable peer address", got.ClientIP)
	}
}

// testSession is the signed-in session the fakes hand back. The token is a
// placeholder string rather than a real JWT: these tests exercise the handler's
// rendering, and what a valid token looks like is the token service's own test.
func testSession() service.Session {
	return service.Session{
		AccessToken: secret.New("test.access.token"),
		TokenType:   service.TokenTypeBearer,
		ExpiresIn:   900,
		UserID:      uuid.MustParse(testChallengeID),
		SessionID:   uuid.New(),
		Role:        "user",
	}
}

// findCookie returns the Set-Cookie the response carries for name, if any.
func findCookie(response *http.Response, name string) *http.Cookie {
	for _, cookie := range response.Cookies() {
		if cookie.Name == name {
			return cookie
		}
	}
	return nil
}

// TestHandler_SignInSetsTheRefreshCookieWithItsProtections is the transport
// half of the refresh design. The attributes are the protection -- a refresh
// token in a script-readable cookie is a standing account takeover -- and they
// live in a struct literal that compiles just as happily with any of them
// missing, so this is what notices.
func TestHandler_SignInSetsTheRefreshCookieWithItsProtections(t *testing.T) {
	cases := map[string]func() *http.Request{
		"login": func() *http.Request {
			body := `{"email":"learner@example.com","password":"password12345"}` // gitleaks:allow
			req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			return req
		},
		"verify": func() *http.Request {
			req := httptest.NewRequest(http.MethodPost,
				"/api/v1/auth/challenges/"+testChallengeID+"/verify", strings.NewReader(`{"code":"123456"}`))
			req.Header.Set("Content-Type", "application/json")
			return req
		},
		"refresh": func() *http.Request {
			return httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", nil)
		},
	}

	for name, build := range cases {
		t.Run(name, func(t *testing.T) {
			router := newTestRouter(&fakeRegistrationService{})
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, build())

			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
			}

			cookie := findCookie(rec.Result(), authhttp.RefreshCookieName)
			if cookie == nil {
				t.Fatal("no refresh cookie was set, so the learner is signed out in fifteen minutes")
			}
			if cookie.Value != testCookieValue {
				t.Errorf("cookie value = %q, want the token the service issued", cookie.Value)
			}
			if !cookie.HttpOnly {
				t.Error("the refresh cookie is readable by script")
			}
			if !cookie.Secure {
				t.Error("the refresh cookie is not marked Secure")
			}
			if cookie.SameSite != http.SameSiteLaxMode {
				t.Errorf("SameSite = %v, want Lax", cookie.SameSite)
			}
			if cookie.Path != refreshCookiePath {
				t.Errorf("path = %q, want the cookie scoped to the auth operations", cookie.Path)
			}
			if cookie.MaxAge <= 0 {
				t.Errorf("max-age = %d, want the token's remaining life", cookie.MaxAge)
			}

			// The refresh token must not also appear in the body, where a
			// script could read what HttpOnly exists to hide.
			if strings.Contains(rec.Body.String(), testCookieValue) {
				t.Errorf("the refresh token is in the response body: %s", rec.Body.String())
			}
		})
	}
}

// TestHandler_ARefusedRefreshClearsTheCookie stops a browser replaying a dead
// token on every launch. After a reuse detection that matters twice over: the
// incident has already been filed, and re-reporting it on each app start would
// bury the next one.
func TestHandler_ARefusedRefreshClearsTheCookie(t *testing.T) {
	rotator := &fakeRotator{
		rotateFn: func(context.Context, string) (service.SignedIn, error) {
			return service.SignedIn{}, domain.ErrSessionRevoked
		},
	}
	router := newTestRouterWith(
		&fakeRegistrationService{}, &fakeAuthenticator{}, rotator, authhttp.CookieOptions{Secure: true})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", nil)
	// A request cookie carries a name and a value; the attributes gosec G124
	// looks for exist only on the response side.
	req.AddCookie(&http.Cookie{ //nolint:gosec // attributes are meaningless on a request cookie
		Name: authhttp.RefreshCookieName, Value: "a-spent-cookie-value",
	})
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401; body: %s", rec.Code, rec.Body.String())
	}

	cookie := findCookie(rec.Result(), authhttp.RefreshCookieName)
	if cookie == nil {
		t.Fatal("the refused refresh did not clear the cookie")
	}
	if cookie.Value != "" || cookie.MaxAge >= 0 {
		t.Errorf("cookie = %q with max-age %d, want it expired", cookie.Value, cookie.MaxAge)
	}
	// The attributes must match the ones it was set with, or the browser keeps
	// the original alongside the deletion and nothing is cleared at all.
	if cookie.Path != refreshCookiePath || !cookie.HttpOnly || !cookie.Secure {
		t.Errorf("the clearing cookie does not match the attributes it was set with: %+v", cookie)
	}
}

// TestHandler_RefreshPassesTheCookieValueAndNothingElse pins where the
// credential comes from. A body or a header would be a second way in, and the
// one the browser cannot be told to withhold cross-site is the cookie.
func TestHandler_RefreshPassesTheCookieValueAndNothingElse(t *testing.T) {
	var presented string
	rotator := &fakeRotator{
		rotateFn: func(_ context.Context, value string) (service.SignedIn, error) {
			presented = value
			return testSignedIn(), nil
		},
	}
	router := newTestRouterWith(
		&fakeRegistrationService{}, &fakeAuthenticator{}, rotator, authhttp.CookieOptions{Secure: true})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", strings.NewReader(`{"token":"from-the-body"}`))
	req.Header.Set("Content-Type", "application/json")
	// A request cookie: the browser sends a name and a value, and the
	// attributes gosec G124 looks for exist only on the response side.
	req.AddCookie(&http.Cookie{ //nolint:gosec // attributes are meaningless on a request cookie
		Name: authhttp.RefreshCookieName, Value: "from-the-cookie",
	})
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
	if presented != "from-the-cookie" {
		t.Errorf("the service was given %q, want the cookie value", presented)
	}
}

// TestHandler_ARefreshWithNoCookieIsRefusedWithoutSayingWhy keeps "you sent no
// cookie" and "your token is dead" indistinguishable. The first would tell a
// caller probing the endpoint what it is looking for.
func TestHandler_ARefreshWithNoCookieIsRefusedWithoutSayingWhy(t *testing.T) {
	var presented = "unset"
	rotator := &fakeRotator{
		rotateFn: func(_ context.Context, value string) (service.SignedIn, error) {
			presented = value
			return service.SignedIn{}, domain.ErrTokenInvalid
		},
	}
	router := newTestRouterWith(
		&fakeRegistrationService{}, &fakeAuthenticator{}, rotator, authhttp.CookieOptions{Secure: true})

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", nil))

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401; body: %s", rec.Code, rec.Body.String())
	}
	if presented != "" {
		t.Errorf("the service was given %q, want the empty string for a missing cookie", presented)
	}
}

// TestHandler_LocalDevelopmentDropsSecureAndNothingElse is the one attribute
// that varies by environment, and it varies because a browser discards a Secure
// cookie over plain HTTP entirely -- a local learner could not sign in at all.
// The protections that do not depend on the transport stay on.
func TestHandler_LocalDevelopmentDropsSecureAndNothingElse(t *testing.T) {
	router := newTestRouterWith(
		&fakeRegistrationService{}, &fakeAuthenticator{}, &fakeRotator{}, authhttp.CookieOptions{Secure: false})

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", nil))

	cookie := findCookie(rec.Result(), authhttp.RefreshCookieName)
	if cookie == nil {
		t.Fatal("no refresh cookie was set")
	}
	if cookie.Secure {
		t.Error("Secure was set for local development, where the browser would discard the cookie")
	}
	if !cookie.HttpOnly || cookie.SameSite != http.SameSiteLaxMode || cookie.Path != refreshCookiePath {
		t.Errorf("dropping Secure also dropped a protection that does not depend on TLS: %+v", cookie)
	}
}
