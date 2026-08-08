package mailer

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"mime"
	"net/mail"
	"net/smtp"
	"strings"
	"time"
)

// ErrHeaderInjection is returned when a value destined for a MIME header
// carries a line break.
var ErrHeaderInjection = errors.New("mailer: header value contains a line break")

// SMTPConfig holds configuration for the SMTP mail transport.
type SMTPConfig struct {
	Host     string
	Port     int
	Username string
	Password string
	From     string
	DevMode  bool
}

// DeliveryRecorder persists one delivery attempt.
type DeliveryRecorder interface {
	Record(ctx context.Context, entry LogEntry) error
}

// LogEntry records one delivery attempt. The recipient appears only as a hash.
type LogEntry struct {
	ToHash            string
	Template          string
	Locale            string
	Status            string
	ProviderMessageID string
	Error             string
	CreatedAt         time.Time
}

// SMTPSender delivers emails via SMTP (Mailpit in development).
type SMTPSender struct {
	config       SMTPConfig
	renderer     *Renderer
	suppressions SuppressionStore
	recorder     DeliveryRecorder
	sendMail     func(addr string, a smtp.Auth, from string, to []string, msg []byte) error
}

// NewSMTPSender initializes an SMTP mail sender.
func NewSMTPSender(
	config SMTPConfig, renderer *Renderer, suppressions SuppressionStore, recorder DeliveryRecorder,
) *SMTPSender {
	if suppressions == nil {
		suppressions = NewMemorySuppressionStore()
	}
	return &SMTPSender{
		config:       config,
		renderer:     renderer,
		suppressions: suppressions,
		recorder:     recorder,
		sendMail:     smtp.SendMail,
	}
}

// Send renders and delivers a transactional email message.
func (s *SMTPSender) Send(ctx context.Context, msg Message) error {
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
	body, err := buildMIME(from, address.Address, rendered)
	if err != nil {
		s.record(ctx, toHash, msg, "failed", "", err)
		return err
	}

	host := s.config.Host
	if host == "" {
		host = "localhost"
	}
	var auth smtp.Auth
	if s.config.Username != "" && !s.config.DevMode {
		auth = smtp.PlainAuth("", s.config.Username, s.config.Password, host)
	}

	addr := fmt.Sprintf("%s:%d", host, s.config.Port)
	if err := s.sendMail(addr, auth, from, []string{address.Address}, body); err != nil {
		permanent := isPermanentSMTPError(err)
		status := "transient_failure"
		if permanent {
			status = "hard_bounce"
			if suppressErr := s.suppressions.SuppressAddress(ctx, address.Address, "hard_bounce"); suppressErr != nil {
				slog.ErrorContext(ctx, "failed to suppress bounced address", "to_hash", toHash, "error", suppressErr)
			}
		}
		s.record(ctx, toHash, msg, status, "", err)
		slog.ErrorContext(ctx, "failed to send email", "to_hash", toHash, "template", msg.Template, "error", err)
		return fmt.Errorf("send email via smtp: %w", err)
	}

	s.record(ctx, toHash, msg, "sent", "", nil)
	return nil
}

// buildMIME assembles the message.
//
// Every header value is checked for CR or LF first. A recipient or subject
// carrying "\r\nBcc:" would otherwise append headers of the attacker's
// choosing to a message the service signs and sends — classic SMTP header
// injection. The subject is also RFC 2047 encoded so Vietnamese subjects
// survive intact.
func buildMIME(from, to string, rendered *RenderedEmail) ([]byte, error) {
	for _, value := range []string{from, to, rendered.Subject} {
		if strings.ContainsAny(value, "\r\n") {
			return nil, ErrHeaderInjection
		}
	}

	boundary, err := randomBoundary()
	if err != nil {
		return nil, err
	}

	var builder strings.Builder
	fmt.Fprintf(&builder, "From: %s\r\n", from)
	fmt.Fprintf(&builder, "To: %s\r\n", to)
	fmt.Fprintf(&builder, "Subject: %s\r\n", mime.QEncoding.Encode("utf-8", rendered.Subject))
	builder.WriteString("MIME-Version: 1.0\r\n")
	fmt.Fprintf(&builder, "Content-Type: multipart/alternative; boundary=%s\r\n\r\n", boundary)
	fmt.Fprintf(&builder, "--%s\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n%s\r\n", boundary, rendered.TextBody)
	fmt.Fprintf(&builder, "--%s\r\nContent-Type: text/html; charset=UTF-8\r\n\r\n%s\r\n", boundary, rendered.HTMLBody)
	fmt.Fprintf(&builder, "--%s--\r\n", boundary)
	return []byte(builder.String()), nil
}

// randomBoundary keeps a body that happens to contain the delimiter from
// truncating the message.
func randomBoundary() (string, error) {
	buffer := make([]byte, 16)
	if _, err := rand.Read(buffer); err != nil {
		return "", fmt.Errorf("mailer: generate MIME boundary: %w", err)
	}
	return "fluentra-" + hex.EncodeToString(buffer), nil
}

func (s *SMTPSender) record(
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

func isPermanentSMTPError(err error) bool {
	if err == nil {
		return false
	}
	text := strings.ToLower(err.Error())
	for _, marker := range []string{"550", "551", "553", "554", "user unknown", "mailbox unavailable", "no such user"} {
		if strings.Contains(text, marker) {
			return true
		}
	}
	return false
}

var _ Sender = (*SMTPSender)(nil)

// PostgresRecorder writes delivery attempts to comm.email_log.
type PostgresRecorder struct {
	db DB
}

// NewPostgresRecorder creates a durable delivery recorder.
func NewPostgresRecorder(db DB) *PostgresRecorder { return &PostgresRecorder{db: db} }

// Record appends one delivery attempt to comm.email_log. Only the recipient hash
// is stored, so the log cannot be turned back into a mailing list.
func (r *PostgresRecorder) Record(ctx context.Context, entry LogEntry) error {
	const query = `
		INSERT INTO comm.email_log (to_hash, template, locale, status, provider_message_id, error)
		VALUES ($1, $2, $3, $4, NULLIF($5, ''), NULLIF($6, ''))
	`
	_, err := r.db.Exec(ctx, query,
		entry.ToHash, entry.Template, entry.Locale, entry.Status, entry.ProviderMessageID, entry.Error)
	if err != nil {
		return fmt.Errorf("record email delivery: %w", err)
	}
	return nil
}

var _ DeliveryRecorder = (*PostgresRecorder)(nil)
