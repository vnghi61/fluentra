package mailer

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const (
	testAPIKey    = "re_test_key"
	testRecipient = "test@example.com"
	testSender    = "test@fluentra.dev"
	testCode      = "Code"
	testName      = "DisplayName"
	testUserName  = "Test User"
)

func TestResendSender_Send(t *testing.T) {
	renderer, err := NewRenderer(nil, nil)
	if err != nil {
		t.Fatalf("NewRenderer: %v", err)
	}

	t.Run("successful send", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				t.Errorf("expected POST, got %s", r.Method)
			}
			if got := r.Header.Get("Authorization"); got != "Bearer "+testAPIKey {
				t.Errorf("unexpected auth header: %s", got)
			}
			if got := r.Header.Get("Content-Type"); got != "application/json" {
				t.Errorf("unexpected content type: %s", got)
			}

			var body resendRequest
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("decode request body: %v", err)
			}
			if len(body.To) != 1 || body.To[0] != testRecipient {
				t.Errorf("unexpected To: %v", body.To)
			}
			if body.Subject == "" {
				t.Error("subject is empty")
			}

			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(resendResponse{ID: "msg_123"})
		}))
		defer server.Close()

		sender := NewResendSender(
			ResendConfig{APIKey: testAPIKey, From: testSender},
			renderer, nil, nil,
		)
		sender.client = server.Client()
		sender.client.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
			req.URL.Scheme = "http"
			req.URL.Host = server.Listener.Addr().String()
			return http.DefaultTransport.RoundTrip(req)
		})

		err := sender.Send(context.Background(), Message{
			To:       testRecipient,
			Template: TemplateVerifyEmail,
			Locale:   "en",
			Data:     map[string]any{testCode: "123456", testName: testUserName},
		})
		if err != nil {
			t.Fatalf("Send returned error: %v", err)
		}
	})

	t.Run("api error returns error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
			_ = json.NewEncoder(w).Encode(resendErrorResponse{
				StatusCode: 401,
				Name:       "unauthorized",
				Message:    "API key is invalid",
			})
		}))
		defer server.Close()

		sender := NewResendSender(
			ResendConfig{APIKey: "bad_key", From: testSender},
			renderer, nil, nil,
		)
		sender.client.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
			req.URL.Scheme = "http"
			req.URL.Host = server.Listener.Addr().String()
			return http.DefaultTransport.RoundTrip(req)
		})

		err := sender.Send(context.Background(), Message{
			To:       testRecipient,
			Template: TemplateVerifyEmail,
			Locale:   "en",
			Data:     map[string]any{testCode: "123456", testName: testUserName},
		})
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("suppressed address is refused", func(t *testing.T) {
		suppressions := NewMemorySuppressionStore()
		_ = suppressions.SuppressAddress(context.Background(), "suppressed@example.com", "hard_bounce")

		sender := NewResendSender(
			ResendConfig{APIKey: testAPIKey, From: testSender},
			renderer, suppressions, nil,
		)

		err := sender.Send(context.Background(), Message{
			To:       "suppressed@example.com",
			Template: TemplateVerifyEmail,
			Locale:   "en",
			Data:     map[string]any{testCode: "123456", testName: testUserName},
		})
		if err == nil {
			t.Fatal("expected suppression error, got nil")
		}
	})

	t.Run("invalid recipient is refused", func(t *testing.T) {
		sender := NewResendSender(
			ResendConfig{APIKey: testAPIKey, From: testSender},
			renderer, nil, nil,
		)

		err := sender.Send(context.Background(), Message{
			To:       "not-an-email",
			Template: TemplateVerifyEmail,
			Locale:   "en",
			Data:     map[string]any{testCode: "123456", testName: testUserName},
		})
		if err == nil {
			t.Fatal("expected invalid address error, got nil")
		}
	})
}

func TestResendSender_InvalidFromIsRefused(t *testing.T) {
	renderer, err := NewRenderer(nil, nil)
	if err != nil {
		t.Fatalf("NewRenderer: %v", err)
	}
	sender := NewResendSender(
		ResendConfig{APIKey: testAPIKey, From: "not-an-address"},
		renderer, nil, nil,
	)

	err = sender.Send(context.Background(), Message{
		To:       testRecipient,
		Template: TemplateVerifyEmail,
		Locale:   "en",
		Data:     map[string]any{testCode: "123456", testName: testUserName},
	})
	if err == nil || !strings.Contains(err.Error(), "invalid sender address") {
		t.Fatalf("err = %v, want an invalid sender address error", err)
	}
}

func TestIsResendPermanentError(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		status    int
		resendErr resendErrorResponse
		want      bool
	}{
		{
			name:   "recipient validation error is permanent",
			status: 422,
			resendErr: resendErrorResponse{
				Name:    "validation_error",
				Message: "The `to` field must be a valid email address",
			},
			want: true,
		},
		{
			name:   "invalid_to name is permanent",
			status: 422,
			resendErr: resendErrorResponse{
				Name:    "invalid_to",
				Message: "recipient is invalid",
			},
			want: true,
		},
		{
			name:   "sender validation error is not permanent",
			status: 422,
			resendErr: resendErrorResponse{
				Name:    "validation_error",
				Message: "The `from` field must be a valid email address",
			},
			want: false,
		},
		{
			name:   "auth error is not permanent",
			status: 401,
			resendErr: resendErrorResponse{
				Name:    "unauthorized",
				Message: "API key is invalid",
			},
			want: false,
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := isResendPermanentError(tt.status, tt.resendErr); got != tt.want {
				t.Errorf("isResendPermanentError(%d, %+v) = %v, want %v", tt.status, tt.resendErr, got, tt.want)
			}
		})
	}
}

// roundTripFunc adapts a function to http.RoundTripper so tests can intercept
// outbound requests without a real network call.
type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }
