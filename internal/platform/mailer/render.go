package mailer

import (
	"bytes"
	"embed"
	"fmt"
	htmltemplate "html/template"
	"io/fs"
	"regexp"
	"sort"
	"strings"
	texttemplate "text/template"
)

// Templates are compiled into the binary. Reading them from disk at send time
// made every email depend on the working directory and re-parsed the same file
// on every request.
//
//go:embed all:templates
var templateFS embed.FS

// DefaultLocales is the supported set. Every template must exist in each of
// them or startup fails.
var DefaultLocales = []string{"en", "vi"}

// Template name constants for standard transactional emails.
const (
	TemplateVerifyEmail         = "verify_email"
	TemplateReset               = "reset"
	TemplateNewDevice           = "new_device"
	TemplateRegistrationAttempt = "registration_attempt"
	TemplateDataExport          = "data_export"
)

// DefaultTemplates is the set validated at startup.
var DefaultTemplates = []string{
	TemplateVerifyEmail,
	TemplateReset,
	TemplateNewDevice,
	TemplateRegistrationAttempt,
	TemplateDataExport,
}

// safeName is what may appear in a template or locale name. Both reach a file
// path, and a locale arrives from a user preference — `../../etc` must never
// resolve to a read outside the embedded template set.
var safeName = regexp.MustCompile(`^[a-z][a-z0-9_-]*$`)

type compiled struct {
	subject string
	html    *htmltemplate.Template
	text    *texttemplate.Template
}

// Renderer holds the parsed, localized email templates.
type Renderer struct {
	templates map[string]compiled
	locales   []string
	names     []string
}

// NewRenderer parses every template in every locale up front, so a missing or
// broken template fails startup rather than one learner's request.
func NewRenderer(templates, locales []string) (*Renderer, error) {
	if len(locales) == 0 {
		locales = DefaultLocales
	}
	if len(templates) == 0 {
		templates = DefaultTemplates
	}

	renderer := &Renderer{
		templates: make(map[string]compiled, len(templates)*len(locales)),
		locales:   append([]string(nil), locales...),
		names:     append([]string(nil), templates...),
	}
	sort.Strings(renderer.locales)
	sort.Strings(renderer.names)

	for _, locale := range renderer.locales {
		if !safeName.MatchString(locale) {
			return nil, fmt.Errorf("mailer: unsupported locale name %q", locale)
		}
		for _, name := range renderer.names {
			if !safeName.MatchString(name) {
				return nil, fmt.Errorf("mailer: unsupported template name %q", name)
			}
			entry, err := compile(locale, name)
			if err != nil {
				return nil, err
			}
			renderer.templates[locale+"/"+name] = entry
		}
	}
	return renderer, nil
}

func compile(locale, name string) (compiled, error) {
	base := "templates/" + locale + "/" + name

	subject, err := fs.ReadFile(templateFS, base+".subject")
	if err != nil {
		return compiled{}, fmt.Errorf("mailer: missing subject for template %s in locale %s: %w", name, locale, err)
	}
	htmlBody, err := fs.ReadFile(templateFS, base+".html")
	if err != nil {
		return compiled{}, fmt.Errorf("mailer: missing HTML body for template %s in locale %s: %w", name, locale, err)
	}
	textBody, err := fs.ReadFile(templateFS, base+".txt")
	if err != nil {
		return compiled{}, fmt.Errorf("mailer: missing text body for template %s in locale %s: %w", name, locale, err)
	}

	htmlTemplate, err := htmltemplate.New(name).Parse(string(htmlBody))
	if err != nil {
		return compiled{}, fmt.Errorf("mailer: parse HTML template %s/%s: %w", locale, name, err)
	}
	textTemplate, err := texttemplate.New(name).Parse(string(textBody))
	if err != nil {
		return compiled{}, fmt.Errorf("mailer: parse text template %s/%s: %w", locale, name, err)
	}

	return compiled{
		subject: strings.TrimSpace(string(subject)),
		html:    htmlTemplate,
		text:    textTemplate,
	}, nil
}

// Locales returns the locales this renderer was built for.
func (r *Renderer) Locales() []string { return append([]string(nil), r.locales...) }

// Render produces the localized subject, HTML and plain-text bodies.
func (r *Renderer) Render(templateName, locale string, data map[string]any) (*RenderedEmail, error) {
	if locale == "" {
		locale = r.locales[0]
	}
	entry, ok := r.templates[locale+"/"+templateName]
	if !ok {
		// Fall back to the default locale before failing, so an unknown
		// preference still gets the email.
		entry, ok = r.templates[DefaultLocales[0]+"/"+templateName]
		if !ok {
			return nil, fmt.Errorf("mailer: no template %q for locale %q", templateName, locale)
		}
	}

	var htmlBuffer bytes.Buffer
	if err := entry.html.Execute(&htmlBuffer, data); err != nil {
		return nil, fmt.Errorf("mailer: execute HTML template %s/%s: %w", locale, templateName, err)
	}
	var textBuffer bytes.Buffer
	if err := entry.text.Execute(&textBuffer, data); err != nil {
		return nil, fmt.Errorf("mailer: execute text template %s/%s: %w", locale, templateName, err)
	}

	return &RenderedEmail{
		Subject:  entry.subject,
		HTMLBody: htmlBuffer.String(),
		TextBody: textBuffer.String(),
	}, nil
}
