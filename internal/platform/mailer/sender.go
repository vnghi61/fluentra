package mailer

import (
	"context"
)

// Message represents an email delivery payload.
type Message struct {
	To       string
	Template string
	Locale   string
	Data     map[string]any
	Category string
}

// RenderedEmail represents the outcome of rendering an email template.
type RenderedEmail struct {
	Subject  string
	HTMLBody string
	TextBody string
}

// Sender defines the contract for sending transactional emails.
type Sender interface {
	Send(ctx context.Context, msg Message) error
}
