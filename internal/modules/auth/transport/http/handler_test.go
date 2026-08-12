package http_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
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
	return newTestRouterWithSessions(reg, auth, rotator, &fakeSessions{}, cookies)
}

func newTestRouterWithSessions(
	reg authhttp.Registration, auth authhttp.Authenticator, rotator authhttp.Rotator,
	sessions authhttp.Sessions, cookies authhttp.CookieOptions,
) chi.Router {
	return newTestRouterWithPasswords(reg, auth, rotator, sessions, &fakePasswords{}, cookies)
}

func newTestRouterWithPasswords(
	reg authhttp.Registration, auth authhttp.Authenticator, rotator authhttp.Rotator,
	sessions authhttp.Sessions, passwords authhttp.Passwords, cookies authhttp.CookieOptions,
) chi.Router {
	return newTestRouterWithDevices(reg, auth, rotator, sessions, passwords, &fakeDevices{}, cookies)
}

func newTestRouterWithDevices(
	reg authhttp.Registration, auth authhttp.Authenticator, rotator authhttp.Rotator,
	sessions authhttp.Sessions, passwords authhttp.Passwords, devices authhttp.Devices,
	cookies authhttp.CookieOptions,
) chi.Router {
	r := chi.NewRouter()
	r.Route("/api/v1", func(api chi.Router) {
		handler := authhttp.NewHandler(reg, auth, rotator, sessions, passwords, devices, cookies)
		handler.Routes(api)
	})
	return r
}

// fakeDevices stands in for the trusted-device service. The windows and the
// untrust cascade are proved against Postgres; what the handler owes them is the
// actor, the shape, and the 404 passed through unchanged.
type fakeDevices struct {
	listFn    func(ctx context.Context, actor httpx.Actor) ([]service.DeviceView, error)
	untrustFn func(ctx context.Context, actor httpx.Actor, deviceID uuid.UUID) error

	sawActor  httpx.Actor
	sawDevice uuid.UUID
}

func (f *fakeDevices) List(ctx context.Context, actor httpx.Actor) ([]service.DeviceView, error) {
	f.sawActor = actor
	if f.listFn != nil {
		return f.listFn(ctx, actor)
	}
	return nil, nil
}

func (f *fakeDevices) Untrust(ctx context.Context, actor httpx.Actor, deviceID uuid.UUID) error {
	f.sawActor, f.sawDevice = actor, deviceID
	if f.untrustFn != nil {
		return f.untrustFn(ctx, actor, deviceID)
	}
	return nil
}

// fakePasswords stands in for the reset and change service. The enumeration
// safety and the revocation are proved against Postgres; what the handler owes
// them is the status, the shape, and the cookie.
type fakePasswords struct {
	forgotFn func(ctx context.Context, email string) (service.Issued, error)
	resetFn  func(ctx context.Context, input service.ResetInput) (service.PasswordChanged, error)
	changeFn func(ctx context.Context, actor httpx.Actor, input service.ChangeInput) (service.PasswordChanged, error)

	sawEmail string
	sawActor httpx.Actor
}

func (f *fakePasswords) Forgot(ctx context.Context, email string) (service.Issued, error) {
	f.sawEmail = email
	if f.forgotFn != nil {
		return f.forgotFn(ctx, email)
	}
	return service.Issued{Challenge: domain.Challenge{
		ID:          uuid.MustParse(testChallengeID),
		Purpose:     domain.PurposePasswordReset,
		MaxAttempts: 5,
		ExpiresAt:   time.Now().Add(30 * time.Minute),
		LastSentAt:  time.Now(),
	}}, nil
}

func (f *fakePasswords) Reset(ctx context.Context, input service.ResetInput) (service.PasswordChanged, error) {
	if f.resetFn != nil {
		return f.resetFn(ctx, input)
	}
	return service.PasswordChanged{ChangedAt: time.Now().UTC(), SessionsRevoked: 3}, nil
}

func (f *fakePasswords) Change(
	ctx context.Context, actor httpx.Actor, input service.ChangeInput,
) (service.PasswordChanged, error) {
	f.sawActor = actor
	if f.changeFn != nil {
		return f.changeFn(ctx, actor, input)
	}
	return service.PasswordChanged{ChangedAt: time.Now().UTC(), SessionsRevoked: 1}, nil
}

// fakeSessions stands in for the session service. What the handler owes it is
// the actor, the 404 passed through unchanged, and the cookie cleared only when
// the caller signed out of the device they are holding.
type fakeSessions struct {
	listFn   func(ctx context.Context, actor httpx.Actor) ([]service.SessionView, error)
	revokeFn func(ctx context.Context, actor httpx.Actor, sessionID uuid.UUID) error
	logoutFn func(ctx context.Context, actor httpx.Actor) error

	sawActor httpx.Actor
}

func (f *fakeSessions) List(ctx context.Context, actor httpx.Actor) ([]service.SessionView, error) {
	f.sawActor = actor
	if f.listFn != nil {
		return f.listFn(ctx, actor)
	}
	return nil, nil
}

func (f *fakeSessions) Revoke(ctx context.Context, actor httpx.Actor, sessionID uuid.UUID) error {
	f.sawActor = actor
	if f.revokeFn != nil {
		return f.revokeFn(ctx, actor, sessionID)
	}
	return nil
}

func (f *fakeSessions) Logout(ctx context.Context, actor httpx.Actor) error {
	f.sawActor = actor
	if f.logoutFn != nil {
		return f.logoutFn(ctx, actor)
	}
	return nil
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

// signedIn puts an actor in the request context the way the middleware would.
func signedIn(req *http.Request, actor httpx.Actor) *http.Request {
	return req.WithContext(httpx.WithActor(req.Context(), actor))
}

// testJTI is the `jti` claim a real actor carries. It is declared apart from
// the literal below because gosec's G101 flags a composite literal that assigns
// a string to a field whose name contains "token" — and this is an identifier
// the denylist is keyed on, not a credential that opens anything. Naming around
// the false positive is cheaper than a nolint a reader has to evaluate.
const testJTI = "01J8XQ7Z9K3M4N5P6Q7R8S9T0V"

func testActor(sessionID uuid.UUID) httpx.Actor {
	return httpx.Actor{
		UserID:    uuid.MustParse(testChallengeID),
		SessionID: sessionID,
		Role:      "user",
		TokenID:   testJTI,
	}
}

// TestHandler_TheAuthenticatedSessionOperationsRefuseAnAnonymousCaller is the
// guard each of them starts with. The middleware lets a request with no
// Authorization header through unauthenticated so the public operations still
// work, which means a handler that forgot to ask for an actor would serve the
// zero value — and uuid.Nil is an account id that matches nothing, so the bug
// would look like an empty list rather than like a failure.
func TestHandler_TheAuthenticatedSessionOperationsRefuseAnAnonymousCaller(t *testing.T) {
	sessions := &fakeSessions{
		listFn: func(context.Context, httpx.Actor) ([]service.SessionView, error) {
			t.Error("the service was reached by a caller with no actor")
			return nil, nil
		},
	}
	router := newTestRouterWithSessions(
		&fakeRegistrationService{}, &fakeAuthenticator{}, &fakeRotator{}, sessions,
		authhttp.CookieOptions{Secure: true})

	anonymous := map[string]*http.Request{
		"logout": httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil),
		"list":   httptest.NewRequest(http.MethodGet, "/api/v1/auth/sessions", nil),
		"revoke": httptest.NewRequest(http.MethodDelete, "/api/v1/auth/sessions/"+testChallengeID, nil),
	}
	for name, req := range anonymous {
		t.Run(name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)
			if rec.Code != http.StatusUnauthorized {
				t.Errorf("status = %d, want 401; body: %s", rec.Code, rec.Body.String())
			}
		})
	}
}

// TestHandler_LogoutClearsTheCookieAndAnswers204 covers the response a client
// acts on: no body, and a cookie it will stop replaying.
func TestHandler_LogoutClearsTheCookieAndAnswers204(t *testing.T) {
	sessions := &fakeSessions{}
	router := newTestRouterWithSessions(
		&fakeRegistrationService{}, &fakeAuthenticator{}, &fakeRotator{}, sessions,
		authhttp.CookieOptions{Secure: true})

	actor := testActor(uuid.New())
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, signedIn(httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil), actor))

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204; body: %s", rec.Code, rec.Body.String())
	}
	if rec.Body.Len() != 0 {
		t.Errorf("204 carried a body: %s", rec.Body.String())
	}
	if sessions.sawActor.SessionID != actor.SessionID {
		t.Errorf("the service was given session %s, want %s", sessions.sawActor.SessionID, actor.SessionID)
	}

	cookie := findCookie(rec.Result(), authhttp.RefreshCookieName)
	if cookie == nil || cookie.Value != "" || cookie.MaxAge >= 0 {
		t.Errorf("the refresh cookie was not cleared: %+v", cookie)
	}
}

// TestHandler_RevokingAnotherDeviceLeavesThisOneSignedIn is the distinction the
// cookie makes. Clearing it unconditionally would sign the learner out of the
// device in their hand for tidying up a different one.
func TestHandler_RevokingAnotherDeviceLeavesThisOneSignedIn(t *testing.T) {
	router := newTestRouterWithSessions(
		&fakeRegistrationService{}, &fakeAuthenticator{}, &fakeRotator{}, &fakeSessions{},
		authhttp.CookieOptions{Secure: true})

	current, other := uuid.New(), uuid.New()

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, signedIn(
		httptest.NewRequest(http.MethodDelete, "/api/v1/auth/sessions/"+other.String(), nil),
		testActor(current)))

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204; body: %s", rec.Code, rec.Body.String())
	}
	if cookie := findCookie(rec.Result(), authhttp.RefreshCookieName); cookie != nil {
		t.Errorf("revoking another device cleared this one's refresh cookie: %+v", cookie)
	}

	// Revoking the session the caller is holding does clear it.
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, signedIn(
		httptest.NewRequest(http.MethodDelete, "/api/v1/auth/sessions/"+current.String(), nil),
		testActor(current)))

	cookie := findCookie(rec.Result(), authhttp.RefreshCookieName)
	if cookie == nil || cookie.MaxAge >= 0 {
		t.Errorf("revoking the current session did not clear its cookie: %+v", cookie)
	}
}

// TestHandler_AMalformedSessionIDIsTheSame404AsAnUnknownOne keeps the shape of
// the id from being something to probe for. "That is not a uuid" and "you have
// no such session" must be indistinguishable, for the same reason the service
// refuses to distinguish a stranger's session from one that never existed.
func TestHandler_AMalformedSessionIDIsTheSame404AsAnUnknownOne(t *testing.T) {
	sessions := &fakeSessions{
		revokeFn: func(context.Context, httpx.Actor, uuid.UUID) error {
			return apperr.New(apperr.NotFound, "RESOURCE_NOT_FOUND", "That session was not found.")
		},
	}
	router := newTestRouterWithSessions(
		&fakeRegistrationService{}, &fakeAuthenticator{}, &fakeRotator{}, sessions,
		authhttp.CookieOptions{Secure: true})

	bodies := map[string]map[string]any{}
	for name, path := range map[string]string{
		"malformed": "/api/v1/auth/sessions/not-a-uuid",
		"unknown":   "/api/v1/auth/sessions/" + uuid.New().String(),
	} {
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, signedIn(httptest.NewRequest(http.MethodDelete, path, nil), testActor(uuid.New())))

		if rec.Code != http.StatusNotFound {
			t.Fatalf("%s: status = %d, want 404; body: %s", name, rec.Code, rec.Body.String())
		}

		var problem map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &problem); err != nil {
			t.Fatalf("%s: decode problem: %v", name, err)
		}
		// `instance` is the request path, which differs only because the caller
		// sent two different ones. It tells them nothing they did not already
		// know; every other member of the document must match.
		delete(problem, "instance")
		bodies[name] = problem
	}

	if !reflect.DeepEqual(bodies["malformed"], bodies["unknown"]) {
		t.Errorf("the two 404s differ:\n malformed: %v\n unknown:   %v", bodies["malformed"], bodies["unknown"])
	}
}

// TestHandler_AnEmptySessionListSerialisesAsAnArray is the difference between
// `[]` and `null`. The schema says array; a client written against it should not
// have to handle a second spelling of "none".
func TestHandler_AnEmptySessionListSerialisesAsAnArray(t *testing.T) {
	router := newTestRouterWithSessions(
		&fakeRegistrationService{}, &fakeAuthenticator{}, &fakeRotator{}, &fakeSessions{},
		authhttp.CookieOptions{Secure: true})

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, signedIn(
		httptest.NewRequest(http.MethodGet, "/api/v1/auth/sessions", nil), testActor(uuid.New())))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"sessions":[]`) {
		t.Errorf("an empty list did not serialise as an array: %s", rec.Body.String())
	}
}

// TestHandler_ForgotPasswordAnswers202WithAHandleAndNoCode is the transport half
// of BR-AUTH-26. The status is unconditional and the body carries the handle,
// never the code — a code in the response would verify nothing, because whoever
// asked for it would already have it.
func TestHandler_ForgotPasswordAnswers202WithAHandleAndNoCode(t *testing.T) {
	passwords := &fakePasswords{}
	router := newTestRouterWithPasswords(
		&fakeRegistrationService{}, &fakeAuthenticator{}, &fakeRotator{}, &fakeSessions{},
		passwords, authhttp.CookieOptions{Secure: true})

	body := `{"email":"  Learner@Example.com  "}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/forgot-password", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202; body: %s", rec.Code, rec.Body.String())
	}
	if passwords.sawEmail != "Learner@Example.com" {
		t.Errorf("the service was given %q, want the address trimmed", passwords.sawEmail)
	}

	var response map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if _, leaked := response["code"]; leaked {
		t.Fatal("the response body carries the code")
	}
	if response["challenge_id"] != testChallengeID {
		t.Errorf("challenge_id = %v, want the handle", response["challenge_id"])
	}
	if response["purpose"] != "password_reset" {
		t.Errorf("purpose = %v, want password_reset", response["purpose"])
	}
}

// TestHandler_AMalformedChallengeHandleIsTheSameRefusalAsAWrongCode keeps the
// shape of the handle from being something to probe for.
func TestHandler_AMalformedChallengeHandleIsTheSameRefusalAsAWrongCode(t *testing.T) {
	passwords := &fakePasswords{
		resetFn: func(context.Context, service.ResetInput) (service.PasswordChanged, error) {
			return service.PasswordChanged{}, domain.ErrChallengeInvalidCode
		},
	}
	router := newTestRouterWithPasswords(
		&fakeRegistrationService{}, &fakeAuthenticator{}, &fakeRotator{}, &fakeSessions{},
		passwords, authhttp.CookieOptions{Secure: true})

	bodies := map[string]map[string]any{}
	for name, payload := range map[string]string{
		"malformed handle": `{"challenge_id":"not-a-uuid","code":"482913","password":"a perfectly fine passphrase"}`,
		"wrong code": `{"challenge_id":"` + testChallengeID +
			`","code":"000000","password":"a perfectly fine passphrase"}`,
	} {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/reset-password", strings.NewReader(payload))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("%s: status = %d, want 401; body: %s", name, rec.Code, rec.Body.String())
		}
		var problem map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &problem); err != nil {
			t.Fatalf("%s: decode problem: %v", name, err)
		}
		delete(problem, "instance")
		bodies[name] = problem
	}

	if !reflect.DeepEqual(bodies["malformed handle"], bodies["wrong code"]) {
		t.Errorf("the two refusals differ:\n malformed: %v\n wrong:     %v",
			bodies["malformed handle"], bodies["wrong code"])
	}
}

// TestHandler_ResetClearsTheCookieAndChangeDoesNot is the difference between the
// two. A reset revoked every session, so a cookie left in place would be
// replayed once and refused; a change kept this one, so taking its cookie away
// would sign the learner out of the device they just used.
func TestHandler_ResetClearsTheCookieAndChangeDoesNot(t *testing.T) {
	router := newTestRouterWithPasswords(
		&fakeRegistrationService{}, &fakeAuthenticator{}, &fakeRotator{}, &fakeSessions{},
		&fakePasswords{}, authhttp.CookieOptions{Secure: true})

	reset := httptest.NewRequest(http.MethodPost, "/api/v1/auth/reset-password", strings.NewReader(
		`{"challenge_id":"`+testChallengeID+`","code":"482913","password":"a perfectly fine passphrase"}`))
	reset.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, reset)

	if rec.Code != http.StatusOK {
		t.Fatalf("reset status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
	cookie := findCookie(rec.Result(), authhttp.RefreshCookieName)
	if cookie == nil || cookie.MaxAge >= 0 {
		t.Errorf("the reset did not clear the refresh cookie: %+v", cookie)
	}

	change := httptest.NewRequest(http.MethodPost, "/api/v1/auth/change-password", strings.NewReader(
		`{"current_password":"a-secret-password","new_password":"a perfectly fine passphrase"}`))
	change.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, signedIn(change, testActor(uuid.New())))

	if rec.Code != http.StatusOK {
		t.Fatalf("change status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
	if cookie := findCookie(rec.Result(), authhttp.RefreshCookieName); cookie != nil {
		t.Errorf("the change touched the refresh cookie: %+v", cookie)
	}
}

// TestHandler_ChangePasswordRefusesAnAnonymousCaller is the guard. Without it a
// handler would serve the zero actor, and uuid.Nil is an account id that matches
// nothing — the bug would look like a wrong password rather than a missing one.
func TestHandler_ChangePasswordRefusesAnAnonymousCaller(t *testing.T) {
	passwords := &fakePasswords{
		changeFn: func(context.Context, httpx.Actor, service.ChangeInput) (service.PasswordChanged, error) {
			t.Error("the service was reached by a caller with no actor")
			return service.PasswordChanged{}, nil
		},
	}
	router := newTestRouterWithPasswords(
		&fakeRegistrationService{}, &fakeAuthenticator{}, &fakeRotator{}, &fakeSessions{},
		passwords, authhttp.CookieOptions{Secure: true})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/change-password", strings.NewReader(
		`{"current_password":"a-secret-password","new_password":"a perfectly fine passphrase"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401; body: %s", rec.Code, rec.Body.String())
	}
}

// TestHandler_TheDeviceOperationsRefuseAnAnonymousCaller is the same guard the
// session operations start with, for the same reason: the middleware lets a
// request with no Authorization header through, so a handler that forgot to ask
// would serve the zero actor and uuid.Nil matches no account — the bug would
// look like an empty list rather than a failure.
func TestHandler_TheDeviceOperationsRefuseAnAnonymousCaller(t *testing.T) {
	devices := &fakeDevices{
		listFn: func(context.Context, httpx.Actor) ([]service.DeviceView, error) {
			t.Error("the service was reached by a caller with no actor")
			return nil, nil
		},
	}
	router := newTestRouterWithDevices(
		&fakeRegistrationService{}, &fakeAuthenticator{}, &fakeRotator{}, &fakeSessions{},
		&fakePasswords{}, devices, authhttp.CookieOptions{Secure: true})

	for name, req := range map[string]*http.Request{
		"list":    httptest.NewRequest(http.MethodGet, "/api/v1/auth/devices", nil),
		"untrust": httptest.NewRequest(http.MethodDelete, "/api/v1/auth/devices/"+testChallengeID, nil),
	} {
		t.Run(name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)
			if rec.Code != http.StatusUnauthorized {
				t.Errorf("status = %d, want 401; body: %s", rec.Code, rec.Body.String())
			}
		})
	}
}

// TestHandler_TheDeviceListCarriesBothExpiries is what makes "stay signed in"
// defensible to the person it applies to. A list showing only the idle date
// implies the trust renews forever, which is the thing it deliberately does not
// do.
func TestHandler_TheDeviceListCarriesBothExpiries(t *testing.T) {
	label := "Chrome on macOS"
	devices := &fakeDevices{
		listFn: func(context.Context, httpx.Actor) ([]service.DeviceView, error) {
			return []service.DeviceView{{
				ID:                uuid.MustParse(testChallengeID),
				Label:             &label,
				TrustedAt:         time.Now().UTC().Add(-48 * time.Hour),
				LastSeenAt:        time.Now().UTC(),
				IdleExpiresAt:     time.Now().UTC().Add(90 * 24 * time.Hour),
				AbsoluteExpiresAt: time.Now().UTC().Add(180 * 24 * time.Hour),
			}}, nil
		},
	}
	router := newTestRouterWithDevices(
		&fakeRegistrationService{}, &fakeAuthenticator{}, &fakeRotator{}, &fakeSessions{},
		&fakePasswords{}, devices, authhttp.CookieOptions{Secure: true})

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, signedIn(
		httptest.NewRequest(http.MethodGet, "/api/v1/auth/devices", nil), testActor(uuid.New())))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}

	var response struct {
		Devices []map[string]any `json:"devices"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(response.Devices) != 1 {
		t.Fatalf("%d devices, want 1", len(response.Devices))
	}
	for _, field := range []string{"idle_expires_at", "absolute_expires_at", "trusted_at", "last_seen_at"} {
		if value, ok := response.Devices[0][field]; !ok || value == "" {
			t.Errorf("the device row is missing %s", field)
		}
	}
}

// TestHandler_AnEmptyDeviceListSerialisesAsAnArray is the `[]` versus `null`
// distinction the session list draws, for the same reason.
func TestHandler_AnEmptyDeviceListSerialisesAsAnArray(t *testing.T) {
	router := newTestRouterWithDevices(
		&fakeRegistrationService{}, &fakeAuthenticator{}, &fakeRotator{}, &fakeSessions{},
		&fakePasswords{}, &fakeDevices{}, authhttp.CookieOptions{Secure: true})

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, signedIn(
		httptest.NewRequest(http.MethodGet, "/api/v1/auth/devices", nil), testActor(uuid.New())))

	if !strings.Contains(rec.Body.String(), `"devices":[]`) {
		t.Errorf("an empty list did not serialise as an array: %s", rec.Body.String())
	}
}

// TestHandler_AMalformedDeviceIDIsTheSame404AsAnUnknownOne keeps the shape of
// the id from being something to probe for.
func TestHandler_AMalformedDeviceIDIsTheSame404AsAnUnknownOne(t *testing.T) {
	devices := &fakeDevices{
		untrustFn: func(context.Context, httpx.Actor, uuid.UUID) error {
			return apperr.New(apperr.NotFound, "RESOURCE_NOT_FOUND", "That device was not found.")
		},
	}
	router := newTestRouterWithDevices(
		&fakeRegistrationService{}, &fakeAuthenticator{}, &fakeRotator{}, &fakeSessions{},
		&fakePasswords{}, devices, authhttp.CookieOptions{Secure: true})

	bodies := map[string]map[string]any{}
	for name, path := range map[string]string{
		"malformed": "/api/v1/auth/devices/not-a-uuid",
		"unknown":   "/api/v1/auth/devices/" + uuid.New().String(),
	} {
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, signedIn(httptest.NewRequest(http.MethodDelete, path, nil), testActor(uuid.New())))

		if rec.Code != http.StatusNotFound {
			t.Fatalf("%s: status = %d, want 404; body: %s", name, rec.Code, rec.Body.String())
		}
		var problem map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &problem); err != nil {
			t.Fatalf("%s: decode problem: %v", name, err)
		}
		delete(problem, "instance")
		bodies[name] = problem
	}

	if !reflect.DeepEqual(bodies["malformed"], bodies["unknown"]) {
		t.Errorf("the two 404s differ:\n malformed: %v\n unknown:   %v", bodies["malformed"], bodies["unknown"])
	}
}

// TestHandler_UntrustingAnsWers204AndPassesTheActorThrough covers the ordinary
// path: no body, and the account taken from the token rather than the request.
func TestHandler_UntrustingAnsWers204AndPassesTheActorThrough(t *testing.T) {
	devices := &fakeDevices{}
	router := newTestRouterWithDevices(
		&fakeRegistrationService{}, &fakeAuthenticator{}, &fakeRotator{}, &fakeSessions{},
		&fakePasswords{}, devices, authhttp.CookieOptions{Secure: true})

	deviceID := uuid.New()
	actor := testActor(uuid.New())
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, signedIn(
		httptest.NewRequest(http.MethodDelete, "/api/v1/auth/devices/"+deviceID.String(), nil), actor))

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204; body: %s", rec.Code, rec.Body.String())
	}
	if rec.Body.Len() != 0 {
		t.Errorf("204 carried a body: %s", rec.Body.String())
	}
	if devices.sawDevice != deviceID {
		t.Errorf("the service was given device %s, want %s", devices.sawDevice, deviceID)
	}
	if devices.sawActor.UserID != actor.UserID {
		t.Errorf("the service was given account %s, want %s", devices.sawActor.UserID, actor.UserID)
	}
}
