//go:build contract

package http_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/fluentra/fluentra/internal/modules/auth/domain"
	"github.com/fluentra/fluentra/internal/modules/auth/service"
	authhttp "github.com/fluentra/fluentra/internal/modules/auth/transport/http"
	"github.com/fluentra/fluentra/internal/shared/httpx"
	"github.com/fluentra/fluentra/internal/shared/secret"
)

func loadSpec(t *testing.T) *openapi3.T {
	t.Helper()

	loader := &openapi3.Loader{Context: context.Background(), IsExternalRefsAllowed: true}
	path := filepath.Join("..", "..", "..", "..", "..", "api", "openapi", "openapi.bundle.yaml")
	spec, err := loader.LoadFromFile(path)
	if err != nil {
		t.Fatalf("load %s: %v", path, err)
	}
	if err := spec.Validate(loader.Context); err != nil {
		t.Fatalf("the spec itself is invalid: %v", err)
	}
	return spec
}

func responseSchema(t *testing.T, spec *openapi3.T, path, method string, statusCode int) *openapi3.Schema {
	t.Helper()

	item := spec.Paths.Find(path)
	if item == nil {
		t.Fatalf("the spec has no path %q", path)
	}
	operation := item.GetOperation(method)
	if operation == nil {
		t.Fatalf("the spec has no %s %s", method, path)
	}
	response := operation.Responses.Status(statusCode)
	if response == nil || response.Value == nil {
		t.Fatalf("%s %s declares no %d response", method, path, statusCode)
	}
	media := response.Value.Content.Get("application/json")
	if media == nil || media.Schema == nil {
		t.Fatalf("%s %s declares no application/json %d body", method, path, statusCode)
	}
	return media.Schema.Value
}

func assertMatchesSchema(t *testing.T, schema *openapi3.Schema, body []byte) {
	t.Helper()

	var decoded any
	if err := json.Unmarshal(body, &decoded); err != nil {
		t.Fatalf("response is not JSON: %v", err)
	}
	if err := schema.VisitJSON(decoded); err != nil {
		t.Fatalf("response does not match the published schema: %v\nbody: %s", err, body)
	}
}

type fakeContractRegistration struct{}

func (f *fakeContractRegistration) Register(_ context.Context, _ service.Registration) (service.Issued, error) {
	return service.Issued{
		Challenge: domain.Challenge{
			ID:          uuid.MustParse("018f3a5b-7c8d-7123-8123-456789abcdef"),
			Purpose:     domain.PurposeVerifyEmail,
			MaxAttempts: 5,
			ExpiresAt:   time.Now().Add(10 * time.Minute),
			LastSentAt:  time.Now(),
		},
		Code: secret.New("123456"),
	}, nil
}

func (f *fakeContractRegistration) VerifyEmail(_ context.Context, _ uuid.UUID, _ string) (service.Verification, error) {
	return service.Verification{
		Purpose:    domain.PurposeVerifyEmail,
		VerifiedAt: time.Now().UTC(),
		SignedIn: service.SignedIn{
			Session:          contractSession(),
			RefreshToken:     secret.New(testCookieValue),
			RefreshExpiresAt: time.Now().UTC().Add(service.DefaultRefreshTTL),
		},
	}, nil
}

// contractSession is a fully populated session, because the schema requires
// every member and bounds expires_in below. A zero Session serialises to
// `expires_in: 0`, which the spec rejects — and that is the contract test
// earning its place: the shape compiled fine and would have shipped a response
// no client written against the spec could accept.
func contractSession() service.Session {
	return service.Session{
		AccessToken: secret.New("eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.contract.test"),
		TokenType:   service.TokenTypeBearer,
		ExpiresIn:   900,
		UserID:      uuid.MustParse("018f3a5b-7c8d-7123-8123-456789abcdef"),
		SessionID:   uuid.MustParse("018f3a5b-7c8d-7123-8123-456789abcde0"),
		Role:        "user",
	}
}

func (f *fakeContractRegistration) Resend(_ context.Context, challengeID uuid.UUID) (service.Issued, error) {
	return service.Issued{
		Challenge: domain.Challenge{
			ID:          challengeID,
			Purpose:     domain.PurposeVerifyEmail,
			MaxAttempts: 5,
			ExpiresAt:   time.Now().Add(10 * time.Minute),
			LastSentAt:  time.Now(),
		},
		Code: secret.New("123456"),
	}, nil
}

func TestContract_RegisterMatchesTheSpec(t *testing.T) {
	spec := loadSpec(t)
	router := newTestRouter(&fakeContractRegistration{})

	body := `{"email":"learner@example.com","password":"password12345","display_name":"Learner"}` // gitleaks:allow
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body: %s", rec.Code, rec.Body.String())
	}

	assertMatchesSchema(t, responseSchema(t, spec, "/auth/register", http.MethodPost, http.StatusCreated), rec.Body.Bytes())
}

func TestContract_VerifyMatchesTheSpec(t *testing.T) {
	spec := loadSpec(t)
	router := newTestRouter(&fakeContractRegistration{})

	challengeID := "018f3a5b-7c8d-7123-8123-456789abcdef"
	body := `{"code":"123456"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/challenges/"+challengeID+"/verify", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}

	assertMatchesSchema(t, responseSchema(t, spec, "/auth/challenges/{id}/verify", http.MethodPost, http.StatusOK), rec.Body.Bytes())
}

func TestContract_ResendMatchesTheSpec(t *testing.T) {
	spec := loadSpec(t)
	router := newTestRouter(&fakeContractRegistration{})

	challengeID := "018f3a5b-7c8d-7123-8123-456789abcdef"
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/challenges/"+challengeID+"/resend", bytes.NewBuffer(nil))
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}

	assertMatchesSchema(t, responseSchema(t, spec, "/auth/challenges/{id}/resend", http.MethodPost, http.StatusOK), rec.Body.Bytes())
}

// fakeContractAuthenticator returns a session shaped like a real one, for the
// same reason contractSession exists: the schema requires every member and
// bounds expires_in, so a zero value would fail validation rather than the
// rendering.
type fakeContractAuthenticator struct{}

func (fakeContractAuthenticator) Login(context.Context, service.LoginInput) (service.LoginResult, error) {
	return service.LoginResult{
		UserID: uuid.MustParse("018f3a5b-7c8d-7123-8123-456789abcdef"),
		SignedIn: service.SignedIn{
			Session:          contractSession(),
			RefreshToken:     secret.New(testCookieValue),
			RefreshExpiresAt: time.Now().UTC().Add(service.DefaultRefreshTTL),
		},
	}, nil
}

// TestContract_LoginMatchesTheSpec is the one endpoint in this module whose
// response shape changed twice — P2.3 invented it inline, P2.4 replaced it with
// AuthSession. Both times the Go compiled and the spec was the only thing that
// would have noticed a mismatch, so this is the check that keeps them together.
func TestContract_LoginMatchesTheSpec(t *testing.T) {
	spec := loadSpec(t)
	router := newTestRouterWithAuth(&fakeContractRegistration{}, fakeContractAuthenticator{})

	body := `{"email":"learner@example.com","password":"password12345"}` // gitleaks:allow
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}

	assertMatchesSchema(t, responseSchema(t, spec, "/auth/login", http.MethodPost, http.StatusOK), rec.Body.Bytes())
}

// TestContract_RefreshMatchesTheSpec covers the operation this card added. It
// returns the same AuthSession every other sign-in returns, which is the whole
// reason the schema was reused rather than a RefreshResponse invented beside it
// -- and this is what would notice if the two drifted apart.
func TestContract_RefreshMatchesTheSpec(t *testing.T) {
	spec := loadSpec(t)
	router := newTestRouterWithAuth(&fakeContractRegistration{}, fakeContractAuthenticator{})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", bytes.NewBuffer(nil))
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}

	assertMatchesSchema(t, responseSchema(t, spec, "/auth/refresh", http.MethodPost, http.StatusOK), rec.Body.Bytes())
}

// fakeContractSessions returns a fully populated row, because the schema marks
// four of the five members required and forbids anything it does not name.
type fakeContractSessions struct{}

func (fakeContractSessions) List(context.Context, httpx.Actor) ([]service.SessionView, error) {
	label := "Chrome on macOS"
	return []service.SessionView{
		{
			ID:          uuid.MustParse("018f3a5b-7c8d-7123-8123-456789abcdef"),
			Current:     true,
			DeviceLabel: &label,
			CreatedAt:   time.Now().UTC().Add(-48 * time.Hour),
			LastSeenAt:  time.Now().UTC(),
		},
		{
			// The nullable label, because `null` and "absent" are different to a
			// schema with additionalProperties false and a nullable type.
			ID:         uuid.MustParse("018f3a5b-7c8d-7123-8123-456789abcdee"),
			Current:    false,
			CreatedAt:  time.Now().UTC().Add(-72 * time.Hour),
			LastSeenAt: time.Now().UTC().Add(-time.Hour),
		},
	}, nil
}

func (fakeContractSessions) Revoke(context.Context, httpx.Actor, uuid.UUID) error { return nil }
func (fakeContractSessions) Logout(context.Context, httpx.Actor) error            { return nil }

// TestContract_SessionListMatchesTheSpec is the operation this card added with a
// body of its own. The other two answer 204 and have nothing to validate.
func TestContract_SessionListMatchesTheSpec(t *testing.T) {
	spec := loadSpec(t)
	router := newTestRouterWithSessions(
		&fakeContractRegistration{}, fakeContractAuthenticator{}, &fakeRotator{},
		fakeContractSessions{}, authhttp.CookieOptions{Secure: true})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/sessions", nil)
	req = req.WithContext(httpx.WithActor(req.Context(), httpx.Actor{
		UserID:    uuid.MustParse("018f3a5b-7c8d-7123-8123-456789abcdef"),
		SessionID: uuid.MustParse("018f3a5b-7c8d-7123-8123-456789abcdef"),
		Role:      "user",
		TokenID:   testJTI,
	}))
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}

	assertMatchesSchema(t, responseSchema(t, spec, "/auth/sessions", http.MethodGet, http.StatusOK), rec.Body.Bytes())
}

type fakeContractPasswords struct{}

func (fakeContractPasswords) Forgot(context.Context, string) (service.Issued, error) {
	return service.Issued{Challenge: domain.Challenge{
		ID:          uuid.MustParse("018f3a5b-7c8d-7123-8123-456789abcdef"),
		Purpose:     domain.PurposePasswordReset,
		MaxAttempts: 5,
		ExpiresAt:   time.Now().Add(30 * time.Minute),
		LastSentAt:  time.Now(),
	}}, nil
}

func (fakeContractPasswords) Reset(context.Context, service.ResetInput) (service.PasswordChanged, error) {
	return service.PasswordChanged{ChangedAt: time.Now().UTC(), SessionsRevoked: 3}, nil
}

func (fakeContractPasswords) Change(
	context.Context, httpx.Actor, service.ChangeInput,
) (service.PasswordChanged, error) {
	return service.PasswordChanged{ChangedAt: time.Now().UTC(), SessionsRevoked: 1}, nil
}

// TestContract_PasswordOperationsMatchTheSpec covers the three this card added.
// forgot-password reuses the Challenge schema and the other two share
// PasswordChanged, so a drift in either shape shows up here rather than in a
// client.
func TestContract_PasswordOperationsMatchTheSpec(t *testing.T) {
	spec := loadSpec(t)
	router := newTestRouterWithPasswords(
		&fakeContractRegistration{}, fakeContractAuthenticator{}, &fakeRotator{},
		fakeContractSessions{}, fakeContractPasswords{}, authhttp.CookieOptions{Secure: true})

	actor := httpx.Actor{
		UserID:    uuid.MustParse("018f3a5b-7c8d-7123-8123-456789abcdef"),
		SessionID: uuid.MustParse("018f3a5b-7c8d-7123-8123-456789abcdef"),
		Role:      "user",
		TokenID:   testJTI,
	}

	cases := map[string]struct {
		path     string
		body     string
		status   int
		signedIn bool
		specPath string
		specCode int
	}{
		"forgot": {
			path:     "/api/v1/auth/forgot-password",
			body:     `{"email":"learner@example.com"}`,
			status:   http.StatusAccepted,
			specPath: "/auth/forgot-password",
			specCode: http.StatusAccepted,
		},
		"reset": {
			path: "/api/v1/auth/reset-password",
			body: `{"challenge_id":"018f3a5b-7c8d-7123-8123-456789abcdef","code":"482913",` +
				`"password":"a perfectly fine passphrase"}`,
			status:   http.StatusOK,
			specPath: "/auth/reset-password",
			specCode: http.StatusOK,
		},
		"change": {
			path:     "/api/v1/auth/change-password",
			body:     `{"current_password":"a-secret-password","new_password":"a perfectly fine passphrase"}`,
			status:   http.StatusOK,
			signedIn: true,
			specPath: "/auth/change-password",
			specCode: http.StatusOK,
		},
	}

	for name, testCase := range cases {
		t.Run(name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, testCase.path, strings.NewReader(testCase.body))
			req.Header.Set("Content-Type", "application/json")
			if testCase.signedIn {
				req = req.WithContext(httpx.WithActor(req.Context(), actor))
			}
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)

			if rec.Code != testCase.status {
				t.Fatalf("status = %d, want %d; body: %s", rec.Code, testCase.status, rec.Body.String())
			}
			assertMatchesSchema(t,
				responseSchema(t, spec, testCase.specPath, http.MethodPost, testCase.specCode),
				rec.Body.Bytes())
		})
	}
}

type fakeContractDevices struct{}

func (fakeContractDevices) List(context.Context, httpx.Actor) ([]service.DeviceView, error) {
	label := "Chrome on macOS"
	return []service.DeviceView{
		{
			ID:                uuid.MustParse("018f3a5b-7c8d-7123-8123-456789abcdef"),
			Label:             &label,
			TrustedAt:         time.Now().UTC().Add(-48 * time.Hour),
			LastSeenAt:        time.Now().UTC(),
			IdleExpiresAt:     time.Now().UTC().Add(90 * 24 * time.Hour),
			AbsoluteExpiresAt: time.Now().UTC().Add(180 * 24 * time.Hour),
		},
		{
			// The nullable label, because `null` and "absent" are different to a
			// schema with additionalProperties false and a nullable type.
			ID:                uuid.MustParse("018f3a5b-7c8d-7123-8123-456789abcdee"),
			TrustedAt:         time.Now().UTC().Add(-72 * time.Hour),
			LastSeenAt:        time.Now().UTC().Add(-time.Hour),
			IdleExpiresAt:     time.Now().UTC().Add(30 * 24 * time.Hour),
			AbsoluteExpiresAt: time.Now().UTC().Add(180 * 24 * time.Hour),
		},
	}, nil
}

func (fakeContractDevices) Untrust(context.Context, httpx.Actor, uuid.UUID) error { return nil }

// TestContract_DeviceListMatchesTheSpec covers the operation this card added
// with a body of its own. The untrust operation answers 204 and has nothing to
// validate.
func TestContract_DeviceListMatchesTheSpec(t *testing.T) {
	spec := loadSpec(t)
	router := newTestRouterWithDevices(
		&fakeContractRegistration{}, fakeContractAuthenticator{}, &fakeRotator{},
		fakeContractSessions{}, fakeContractPasswords{}, fakeContractDevices{},
		authhttp.CookieOptions{Secure: true})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/devices", nil)
	req = req.WithContext(httpx.WithActor(req.Context(), httpx.Actor{
		UserID:    uuid.MustParse("018f3a5b-7c8d-7123-8123-456789abcdef"),
		SessionID: uuid.MustParse("018f3a5b-7c8d-7123-8123-456789abcdef"),
		Role:      "user",
		TokenID:   testJTI,
	}))
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
	assertMatchesSchema(t, responseSchema(t, spec, "/auth/devices", http.MethodGet, http.StatusOK), rec.Body.Bytes())
}

// fakeContractOAuth answers with fully populated values, because the schemas
// mark every member required and `additionalProperties: false` — the two ways a
// response can compile and still be unacceptable to a client written against the
// published spec.
type fakeContractOAuth struct{}

func (fakeContractOAuth) Start(context.Context, string) (service.Started, error) {
	return service.Started{
		AuthorizationURL: "https://accounts.google.com/o/oauth2/v2/auth?client_id=example&state=example",
	}, nil
}

func (fakeContractOAuth) Callback(context.Context, service.CallbackInput) (service.SignedIn, error) {
	return service.SignedIn{
		Session:          contractSession(),
		RefreshToken:     secret.New(testCookieValue),
		RefreshExpiresAt: time.Now().UTC().Add(service.DefaultRefreshTTL),
	}, nil
}

func (fakeContractOAuth) Link(context.Context, httpx.Actor, service.CallbackInput) (
	service.LinkedIdentity, error,
) {
	return service.LinkedIdentity{
		Provider: "google", Email: "learner@example.com", LinkedAt: time.Now().UTC(),
	}, nil
}

func (fakeContractOAuth) Unlink(context.Context, httpx.Actor) error { return nil }

func newContractOAuthRouter() chi.Router {
	return newTestRouterWithOAuth(
		&fakeContractRegistration{}, fakeContractAuthenticator{}, &fakeRotator{},
		fakeContractSessions{}, fakeContractPasswords{}, fakeContractDevices{},
		fakeContractOAuth{}, authhttp.CookieOptions{Secure: true})
}

// TestContract_GoogleSignInMatchesTheSpec covers the three operations that
// return a body. The unlink answers 204 and has nothing to validate.
//
// The `start` case is the one worth having twice over: its schema is
// `additionalProperties: false` around a single member, so a handler that leaked
// the state or the nonce into the response would fail here as a schema
// violation, not merely as a security review finding.
func TestContract_GoogleSignInMatchesTheSpec(t *testing.T) {
	spec := loadSpec(t)
	router := newContractOAuthRouter()

	actor := httpx.Actor{
		UserID:    uuid.MustParse("018f3a5b-7c8d-7123-8123-456789abcdef"),
		SessionID: uuid.MustParse("018f3a5b-7c8d-7123-8123-456789abcdef"),
		Role:      "user",
		TokenID:   testJTI,
	}
	callbackBody := `{"code":"4/0AeanS0by-example","state":"9f2c1a7e4b3d4c119d217f0a5c8e2b44"}`

	for name, testCase := range map[string]struct {
		method       string
		path         string
		specPath     string
		body         string
		authenticate bool
	}{
		"start": {
			method: http.MethodGet, path: "/api/v1/auth/oauth/google/start",
			specPath: "/auth/oauth/google/start",
		},
		"callback": {
			method: http.MethodPost, path: "/api/v1/auth/oauth/google/callback",
			specPath: "/auth/oauth/google/callback", body: callbackBody,
		},
		"link": {
			method: http.MethodPost, path: "/api/v1/auth/oauth/google/link",
			specPath: "/auth/oauth/google/link", body: callbackBody, authenticate: true,
		},
	} {
		t.Run(name, func(t *testing.T) {
			var req *http.Request
			if testCase.body == "" {
				req = httptest.NewRequest(testCase.method, testCase.path, nil)
			} else {
				req = httptest.NewRequest(testCase.method, testCase.path, strings.NewReader(testCase.body))
				req.Header.Set("Content-Type", "application/json")
			}
			if testCase.authenticate {
				req = req.WithContext(httpx.WithActor(req.Context(), actor))
			}

			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
			}
			assertMatchesSchema(t,
				responseSchema(t, spec, testCase.specPath, testCase.method, http.StatusOK), rec.Body.Bytes())
		})
	}
}

// TestContract_UnlinkAnswers204WithNoBody. The spec declares no content for it,
// and a handler that wrote one would be sending something no client will read
// and no schema describes.
func TestContract_UnlinkAnswers204WithNoBody(t *testing.T) {
	spec := loadSpec(t)
	router := newContractOAuthRouter()

	item := spec.Paths.Find("/auth/oauth/google")
	if item == nil || item.Delete == nil {
		t.Fatal("the spec has no DELETE /auth/oauth/google")
	}
	if response := item.Delete.Responses.Status(http.StatusNoContent); response == nil {
		t.Fatal("DELETE /auth/oauth/google declares no 204")
	}

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/auth/oauth/google", nil)
	req = req.WithContext(httpx.WithActor(req.Context(), httpx.Actor{
		UserID:    uuid.MustParse("018f3a5b-7c8d-7123-8123-456789abcdef"),
		SessionID: uuid.MustParse("018f3a5b-7c8d-7123-8123-456789abcdef"),
		Role:      "user",
		TokenID:   testJTI,
	}))
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204; body: %s", rec.Code, rec.Body.String())
	}
	if rec.Body.Len() != 0 {
		t.Errorf("204 carried a body: %s", rec.Body.String())
	}
}

func (fakeContractOAuth) LinkStatus(context.Context, httpx.Actor) (service.LinkState, error) {
	return service.LinkState{Linked: true, CanUnlink: true}, nil
}
