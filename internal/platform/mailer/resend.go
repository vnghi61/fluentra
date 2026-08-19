package mailer

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/mail"
	"strings"
	"time"
)

// ResendConfig holds configuration for the Resend HTTP API transport.
type ResendConfig struct {
	APIKey string
	From   string
}

// ResendSender delivers emails via the Resend HTTP API (https://api.resend.com).
//
// Render Free blocks outbound SMTP ports (25, 465, 587). The Resend API uses
// HTTPS (port 443), which is always open. This sender implements the same
// Sender interface as SMTPSender so the two are interchangeable at the
// composition root.
type ResendSender struct {
	config       ResendConfig
	renderer     *Renderer
	suppressions SuppressionStore
	recorder     DeliveryRecorder
	client       *http.Client
}

// resendRequest is the JSON body Resend's /emails endpoint expects.
type resendRequest struct {
	From    string   `json:"from"`
	To      []string `json:"to"`
	Subject string   `json:"subject"`
	HTML    string   `json:"html"`
	Text    string   `json:"text,omitempty"`
}

// resendResponse is the successful JSON response from Resend.
type resendResponse struct {
	ID string `json:"id"`
}

// resendErrorResponse is the error JSON response from Resend.
type resendErrorResponse struct {
	StatusCode int    `json:"statusCode"`
	Name       string `json:"name"`
	Message    string `json:"message"`
}

const resendAPIURL = "https://api.resend.com/emails"

// NewResendSender initializes a Resend HTTP API mail sender.
func NewResendSender(
	config ResendConfig, renderer *Renderer, suppressions SuppressionStore, recorder DeliveryRecorder,
) *ResendSender {
	if suppressions == nil {
		suppressions = NewMemorySuppressionStore()
	}
	return &ResendSender{
		config:       config,
		renderer:     renderer,
		suppressions: suppressions,
		recorder:     recorder,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Send renders and delivers a transactional email message via the Resend API.
func (s *ResendSender) Send(ctx context.Context, msg Message) error {
	toHash := HashEmail(msg.To)

	address, err := mail.ParseAddress(msg.To)
	if err != nil {
		s.record(ctx, toHash, msg, "invalid_address", "", err)
		return fmt.Errorf("mailer: invalid recipient: %w", err)
	}

	suppressed, reason, err := s.suppressions.IsSuppressed(ctx, address.Address)
	if err != nil {
		return fmt.Errorf("check suppression list: %w", err)
	}
	if suppressed {
		s.record(ctx, toHash, msg, "suppressed", "", fmt.Errorf("address suppressed: %s", reason))
		return fmt.Errorf("mailer: address is suppressed (%s)", reason)
	}

	rendered, err := s.renderer.Render(msg.Template, msg.Locale, msg.Data)
	if err != nil {
		s.record(ctx, toHash, msg, "failed", "", err)
		return fmt.Errorf("render template: %w", err)
	}

	from := s.config.From
	if from == "" {
		from = "no-reply@fluentra.local"
	}
	// Validate the sender up front: SMTPSender does the same, and leaving it to
	// Resend means a 422 validation_error that could be misread as a recipient
	// bounce and suppress a healthy address.
	if _, parseErr := mail.ParseAddress(from); parseErr != nil {
		s.record(ctx, toHash, msg, "invalid_from_address", "", parseErr)
		return fmt.Errorf("mailer: invalid sender address %q: %w", from, parseErr)
	}

	reqBody := resendRequest{
		From:    from,
		To:      []string{address.Address},
		Subject: rendered.Subject,
		HTML:    rendered.HTMLBody,
		Text:    rendered.TextBody,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		s.record(ctx, toHash, msg, "failed", "", err)
		return fmt.Errorf("mailer: marshal resend request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, resendAPIURL, bytes.NewReader(body))
	if err != nil {
		s.record(ctx, toHash, msg, "failed", "", err)
		return fmt.Errorf("mailer: create resend request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+s.config.APIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		s.record(ctx, toHash, msg, "transient_failure", "", err)
		slog.ErrorContext(ctx, "failed to send email via resend", "to_hash", toHash, "template", msg.Template, "error", err)
		return fmt.Errorf("send email via resend api: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		var result resendResponse
		_ = json.Unmarshal(respBody, &result)
		s.record(ctx, toHash, msg, "sent", result.ID, nil)
		return nil
	}

	// Classify the failure.
	var resendErr resendErrorResponse
	_ = json.Unmarshal(respBody, &resendErr)

	permanent := isResendPermanentError(resp.StatusCode, resendErr)
	status := "transient_failure"
	if permanent {
		status = "hard_bounce"
		if suppressErr := s.suppressions.SuppressAddress(ctx, address.Address, "hard_bounce"); suppressErr != nil {
			slog.ErrorContext(ctx, "failed to suppress bounced address", "to_hash", toHash, "error", suppressErr)
		}
	}

	apiErr := fmt.Errorf("resend: status=%d name=%s message=%s", resp.StatusCode, resendErr.Name, resendErr.Message)
	s.record(ctx, toHash, msg, status, "", apiErr)
	slog.ErrorContext(ctx, "failed to send email via resend", "to_hash", toHash, "template", msg.Template, "status", resp.StatusCode, "error", apiErr)
	return fmt.Errorf("send email via resend api: %w", apiErr)
}

func (s *ResendSender) record(
	ctx context.Context, toHash string, msg Message, status, providerMessageID string, cause error,
) {
	if s.recorder == nil {
		return
	}
	entry := LogEntry{
		ToHash:            toHash,
		Template:          msg.Template,
		Locale:            msg.Locale,
		Status:            status,
		ProviderMessageID: providerMessageID,
		CreatedAt:         time.Now().UTC(),
	}
	if cause != nil {
		entry.Error = cause.Error()
	}
	if err := s.recorder.Record(ctx, entry); err != nil {
		slog.ErrorContext(ctx, "failed to record email delivery", "to_hash", toHash, "error", err)
	}
}

// isResendPermanentError reports whether a Resend API error means the address
// itself is undeliverable and should be suppressed. Only recipient-specific
// failures qualify: Resend returns 422 validation_error for many reasons, and
// suppressing the recipient on, say, an invalid `from` would silently blacklist
// a healthy address.
func isResendPermanentError(statusCode int, resendErr resendErrorResponse) bool {
	if statusCode != 422 {
		return false
	}
	name := strings.ToLower(resendErr.Name)
	if strings.Contains(name, "invalid_to") {
		return true
	}
	message := strings.ToLower(resendErr.Message)
	for _, marker := range []string{"`to`", "recipient"} {
		if strings.Contains(message, marker) {
			return true
		}
	}
	return false
}

var _ Sender = (*ResendSender)(nil)
