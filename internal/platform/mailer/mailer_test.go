package mailer_test

import (
	"context"
	"strings"
	"testing"

	"github.com/fluentra/fluentra/internal/platform/mailer"
)

// templateVerifyEmail is the one template these tests render. It lives here
// rather than in integration_test.go so it is visible without the build tag.
const templateVerifyEmail = "verify_email"

// The template variables the verify_email body substitutes.
const (
	fieldCode        = "Code"
	fieldDisplayName = "DisplayName"
)

func newRenderer(t *testing.T) *mailer.Renderer {
	t.Helper()
	renderer, err := mailer.NewRenderer(nil, nil)
	if err != nil {
		t.Fatalf("new renderer: %v", err)
	}
	return renderer
}

// TestNewRenderer_FailsStartupOnAMissingLocale is the P0.11 acceptance: a
// missing template must break the process, not one learner's request.
func TestNewRenderer_FailsStartupOnAMissingLocale(t *testing.T) {
	t.Parallel()
	if _, err := mailer.NewRenderer(nil, []string{"en", "fr"}); err == nil {
		t.Fatal("expected startup to fail for a locale with no templates")
	}
	if _, err := mailer.NewRenderer([]string{"no_such_template"}, nil); err == nil {
		t.Fatal("expected startup to fail for an unknown template")
	}
}

// TestNewRenderer_RejectsTraversalInNames closes the path-traversal hole: the
// locale comes from a user preference and used to be concatenated into a file
// path.
func TestNewRenderer_RejectsTraversalInNames(t *testing.T) {
	t.Parallel()
	for _, locale := range []string{"../../etc", "..", "en/../..", `..\..\windows`} {
		if _, err := mailer.NewRenderer(nil, []string{locale}); err == nil {
			t.Errorf("locale %q was accepted", locale)
		}
	}
	for _, name := range []string{"../secret", "..", "verify_email/../../go.mod"} {
		if _, err := mailer.NewRenderer([]string{name}, nil); err == nil {
			t.Errorf("template %q was accepted", name)
		}
	}
}

// TestRender_SubjectIsLocalised is the fix for a hardcoded English subject
// built from the template name.
func TestRender_SubjectIsLocalised(t *testing.T) {
	t.Parallel()
	renderer := newRenderer(t)

	english, err := renderer.Render(templateVerifyEmail, "en",
		map[string]any{fieldCode: "123456", fieldDisplayName: "Mai"})
	if err != nil {
		t.Fatalf("render en: %v", err)
	}
	vietnamese, err := renderer.Render(templateVerifyEmail, "vi",
		map[string]any{fieldCode: "123456", fieldDisplayName: "Mai"})
	if err != nil {
		t.Fatalf("render vi: %v", err)
	}

	if english.Subject == vietnamese.Subject {
		t.Fatalf("subject is not localised: both are %q", english.Subject)
	}
	if strings.Contains(english.Subject, templateVerifyEmail) {
		t.Errorf("subject leaks the template name: %q", english.Subject)
	}
}

// TestRender_EscapesHTMLInUserSuppliedData keeps a display name from becoming
// markup in the message body.
func TestRender_EscapesHTMLInUserSuppliedData(t *testing.T) {
	t.Parallel()
	rendered, err := newRenderer(t).Render(templateVerifyEmail, "en", map[string]any{
		fieldDisplayName: "<script>alert(1)</script>",
		fieldCode:        "123456",
	})
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if strings.Contains(rendered.HTMLBody, "<script>") {
		t.Fatalf("display name was not escaped: %s", rendered.HTMLBody)
	}
}

func TestRender_FallsBackToDefaultLocale(t *testing.T) {
	t.Parallel()
	rendered, err := newRenderer(t).Render(templateVerifyEmail, "de",
		map[string]any{fieldCode: "1", fieldDisplayName: "x"})
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if rendered.Subject == "" {
		t.Error("expected the default locale to be used for an unknown locale")
	}
}

// TestSend_RejectsHeaderInjectionInRecipient is the security regression test:
// a recipient carrying CRLF used to be written straight into the MIME headers.
func TestSend_RejectsHeaderInjectionInRecipient(t *testing.T) {
	t.Parallel()
	sender := mailer.NewSMTPSender(
		mailer.SMTPConfig{Host: "localhost", Port: 1025, DevMode: true}, newRenderer(t), nil, nil)

	err := sender.Send(context.Background(), mailer.Message{
		To:       "learner@example.com\r\nBcc: attacker@evil.example",
		Template: templateVerifyEmail,
		Locale:   "en",
		Data:     map[string]any{fieldCode: "1", fieldDisplayName: "x"},
	})
	if err == nil {
		t.Fatal("a recipient containing CRLF was accepted")
	}
}

func TestSend_SuppressedAddressIsNotDelivered(t *testing.T) {
	t.Parallel()
	suppressions := mailer.NewMemorySuppressionStore()
	if err := suppressions.SuppressAddress(context.Background(), "bounced@example.com", "hard_bounce"); err != nil {
		t.Fatal(err)
	}
	sender := mailer.NewSMTPSender(
		mailer.SMTPConfig{Host: "localhost", Port: 1025, DevMode: true}, newRenderer(t), suppressions, nil)

	err := sender.Send(context.Background(), mailer.Message{
		To: "bounced@example.com", Template: templateVerifyEmail, Locale: "en",
		Data: map[string]any{fieldCode: "1", fieldDisplayName: "x"},
	})
	if err == nil || !strings.Contains(err.Error(), "suppressed") {
		t.Fatalf("error = %v, want a suppression error", err)
	}
}

func TestHashEmail_NormalisesCaseAndSpace(t *testing.T) {
	t.Parallel()
	if mailer.HashEmail("  Learner@Example.COM ") != mailer.HashEmail("learner@example.com") {
		t.Error("hash should normalise case and surrounding space")
	}
	if mailer.HashEmail("a@b.com") == mailer.HashEmail("c@d.com") {
		t.Error("different addresses must not collide")
	}
}

func TestMemorySuppressionStore_RejectsEmptyAddress(t *testing.T) {
	t.Parallel()
	if err := mailer.NewMemorySuppressionStore().SuppressAddress(context.Background(), "   ", "x"); err == nil {
		t.Error("expected an empty address to be rejected")
	}
}
