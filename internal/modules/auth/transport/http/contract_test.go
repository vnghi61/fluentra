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
	"github.com/google/uuid"

	"github.com/fluentra/fluentra/internal/modules/auth/domain"
	"github.com/fluentra/fluentra/internal/modules/auth/service"
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
		Session:    contractSession(),
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
		UserID:  uuid.MustParse("018f3a5b-7c8d-7123-8123-456789abcdef"),
		Session: contractSession(),
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
