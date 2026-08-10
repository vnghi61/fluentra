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
	return service.LoginResult{UserID: uuid.MustParse(testChallengeID), Verified: true}, nil
}

func newTestRouter(reg authhttp.Registration) chi.Router {
	return newTestRouterWithAuth(reg, &fakeAuthenticator{})
}

func newTestRouterWithAuth(reg authhttp.Registration, auth authhttp.Authenticator) chi.Router {
	r := chi.NewRouter()
	r.Route("/api/v1", func(api chi.Router) {
		handler := authhttp.NewHandler(reg, auth)
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
			return service.LoginResult{UserID: uuid.New(), Verified: true}, nil
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
