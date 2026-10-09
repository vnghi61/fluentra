package domain

import (
	"net/url"
	"regexp"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/fluentra/fluentra/internal/shared/apperr"
)

// Banner limits. The migration repeats them as CHECK constraints.
const (
	BannerTitleMaxLen    = 120
	BannerBodyMaxLen     = 500
	BannerCTALabelMaxLen = 40
	BannerCTAURLMaxLen   = 2048
	BannerPriorityMax    = 1000
	BannerImageAltMaxLen = 160

	// ActiveAnnouncementLimit and ActiveSlideLimit are how many of each kind
	// the dashboard is handed at once.
	ActiveAnnouncementLimit = 3
	ActiveSlideLimit        = 6
)

// BannerKind is how a banner is shown on the dashboard.
type BannerKind string

// The complete set of banner kinds.
const (
	// BannerKindAnnouncement is a message shown as a dialog, which the learner
	// closes (for good when it is dismissible).
	BannerKindAnnouncement BannerKind = "announcement"
	// BannerKindSlide is an image-only promotion in the dashboard carousel:
	// always shown while live, never dismissed, optionally a link.
	BannerKindSlide BannerKind = "slide"
)

// Valid reports whether k is one of the known kinds.
func (k BannerKind) Valid() bool {
	return k == BannerKindAnnouncement || k == BannerKindSlide
}

// BannerTone is how a banner is coloured on the dashboard.
type BannerTone string

// The complete set of banner tones.
const (
	BannerToneInfo    BannerTone = "info"
	BannerToneSuccess BannerTone = "success"
	BannerToneWarning BannerTone = "warning"
	BannerTonePromo   BannerTone = "promo"
)

// Valid reports whether t is one of the known tones.
func (t BannerTone) Valid() bool {
	switch t {
	case BannerToneInfo, BannerToneSuccess, BannerToneWarning, BannerTonePromo:
		return true
	default:
		return false
	}
}

// Banner is an announcement shown at the top of the learner dashboard.
// Title, Body and CTALabel are plain text; nothing here is ever HTML.
type Banner struct {
	ID          uuid.UUID
	Kind        BannerKind
	Title       string
	Body        string
	CTALabel    *string
	CTAURL      *string
	Tone        BannerTone
	IsActive    bool
	Priority    int
	Dismissible bool
	StartsAt    *time.Time
	EndsAt      *time.Time
	Image       *BannerImage
	CreatedBy   uuid.UUID
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// BannerImage is a processed image in the media bucket. Key is the stored
// object; Alt is plain text, empty for a decorative image. URL is filled per
// read with a short-lived signature and is never persisted.
type BannerImage struct {
	Key    string
	Alt    string
	Width  int
	Height int
	URL    string
}

// bannerImageKeyPattern is the only shape of key a banner may point at: an
// image the admin module itself processed. The migration repeats it.
var bannerImageKeyPattern = regexp.MustCompile(`^banners/images/[0-9a-f-]{36}[.]jpg$`)

// BannerImageUpload is a presigned browser upload for a banner image. The
// store enforces the content type and size; nothing is published until the
// upload is processed.
type BannerImageUpload struct {
	URL         string
	Method      string
	FormData    map[string]string
	FileField   string
	ObjectKey   string
	ExpiresAt   time.Time
	MaxBytes    int64
	ContentType string
}

// ErrBannerImageInUse is returned when a second banner names an image that
// another banner already shows.
var ErrBannerImageInUse = apperr.New(
	apperr.Validation, "BANNER_INVALID", "The banner has invalid fields.",
).WithFields(apperr.FieldViolation{
	Field: "image_key", Code: "in_use", Message: "This image belongs to another banner; upload it again.",
})

// IsBannerImageKey reports whether key names a processed banner image.
func IsBannerImageKey(key string) bool {
	return bannerImageKeyPattern.MatchString(key)
}

// LiveAt reports whether the banner shows at instant now: switched on, and
// now inside [StartsAt, EndsAt) with either bound optional.
func (b Banner) LiveAt(now time.Time) bool {
	if !b.IsActive {
		return false
	}
	if b.StartsAt != nil && now.Before(*b.StartsAt) {
		return false
	}
	if b.EndsAt != nil && !now.Before(*b.EndsAt) {
		return false
	}
	return true
}

// BannerInput is the editable part of a banner, as an administrator submits it.
type BannerInput struct {
	Kind        BannerKind
	Title       string
	Body        string
	CTALabel    *string
	CTAURL      *string
	Tone        BannerTone
	IsActive    bool
	Priority    int
	Dismissible bool
	StartsAt    *time.Time
	EndsAt      *time.Time
	// ImageKey names a processed image (from ProcessBannerImage), or nil for
	// none. ImageAlt is its alternative text.
	ImageKey *string
	ImageAlt string
}

// ErrBannerNotFound is returned when a banner id names no row.
var ErrBannerNotFound = apperr.New(apperr.NotFound, "BANNER_NOT_FOUND", "The banner was not found.")

// errBannerInvalid is the base of every banner validation failure; the
// per-field reasons ride in its Fields so a form can point at each one.
var errBannerInvalid = apperr.New(apperr.Validation, "BANNER_INVALID", "The banner has invalid fields.")

// Field violation codes, stable for clients.
const (
	violationRequired = "required"
	violationTooLong  = "too_long"
	violationInvalid  = "invalid"
	violationUnsafe   = "unsafe_url"
	violationRange    = "out_of_range"
	violationPair     = "cta_incomplete"
	violationWindow   = "ends_before_start"
)

// NormalizeBannerInput trims the text fields, turns blank optional fields into
// nil, defaults the tone, and validates the result. It returns the cleaned
// input or a Validation apperr listing every offending field.
func NormalizeBannerInput(in BannerInput) (BannerInput, error) {
	out := in
	out.Title = strings.TrimSpace(in.Title)
	out.Body = strings.TrimSpace(in.Body)
	out.CTALabel = trimmedOrNil(in.CTALabel)
	out.CTAURL = trimmedOrNil(in.CTAURL)
	out.ImageKey = trimmedOrNil(in.ImageKey)
	out.ImageAlt = strings.TrimSpace(in.ImageAlt)
	if out.ImageKey == nil {
		out.ImageAlt = ""
	}
	if out.Tone == "" {
		out.Tone = BannerToneInfo
	}
	if out.Kind == "" {
		out.Kind = BannerKindAnnouncement
	}
	if out.Kind == BannerKindSlide {
		// A slide is its picture: no text, no button, nothing to dismiss. The
		// fields are cleared rather than refused so a form that switched kind
		// does not have to empty them first. Its title names the picture for
		// screen readers when no description was given.
		out.Body = ""
		out.CTALabel = nil
		out.Dismissible = false
		if out.ImageKey != nil && out.ImageAlt == "" {
			out.ImageAlt = out.Title
		}
	}

	var v violations
	v.checkText(out)
	v.checkCTA(out)
	v.checkSettings(out)
	v.checkImage(out)

	if len(v.fields) > 0 {
		return BannerInput{}, errBannerInvalid.WithFields(v.fields...)
	}
	return out, nil
}

// violations collects every offending field so the form can mark each one.
type violations struct {
	fields []apperr.FieldViolation
}

func (v *violations) add(field, code, message string) {
	v.fields = append(v.fields, apperr.FieldViolation{Field: field, Code: code, Message: message})
}

func (v *violations) checkText(in BannerInput) {
	switch {
	case in.Title == "":
		v.add("title", violationRequired, "Title is required.")
	case utf8.RuneCountInString(in.Title) > BannerTitleMaxLen:
		v.add("title", violationTooLong, "Title must be at most 120 characters.")
	case hasControl(in.Title, false):
		v.add("title", violationInvalid, "Title must be plain text on one line.")
	}

	switch {
	case utf8.RuneCountInString(in.Body) > BannerBodyMaxLen:
		v.add("body", violationTooLong, "Body must be at most 500 characters.")
	case hasControl(in.Body, true):
		v.add("body", violationInvalid, "Body must be plain text.")
	}
}

func (v *violations) checkCTA(in BannerInput) {
	if in.Kind == BannerKindSlide {
		// The whole picture is the link; there is no button text to pair.
		if in.CTAURL != nil && !IsSafeBannerURL(*in.CTAURL) {
			v.add("cta_url", violationUnsafe,
				"Link must be an internal path starting with / or an https:// URL.")
		}
		return
	}
	if in.CTALabel != nil {
		switch {
		case utf8.RuneCountInString(*in.CTALabel) > BannerCTALabelMaxLen:
			v.add("cta_label", violationTooLong, "Button text must be at most 40 characters.")
		case hasControl(*in.CTALabel, false):
			v.add("cta_label", violationInvalid, "Button text must be plain text on one line.")
		}
	}
	if in.CTAURL != nil && !IsSafeBannerURL(*in.CTAURL) {
		v.add("cta_url", violationUnsafe,
			"Link must be an internal path starting with / or an https:// URL.")
	}
	if (in.CTALabel == nil) != (in.CTAURL == nil) {
		field := "cta_label"
		if in.CTAURL == nil {
			field = "cta_url"
		}
		v.add(field, violationPair, "Button text and link must be set together.")
	}
}

func (v *violations) checkSettings(in BannerInput) {
	if !in.Kind.Valid() {
		v.add("kind", violationInvalid, "Kind must be announcement or slide.")
	}
	if !in.Tone.Valid() {
		v.add("tone", violationInvalid, "Tone must be one of info, success, warning, promo.")
	}
	if in.Priority < 0 || in.Priority > BannerPriorityMax {
		v.add("priority", violationRange, "Priority must be between 0 and 1000.")
	}
	if in.StartsAt != nil && in.EndsAt != nil && !in.EndsAt.After(*in.StartsAt) {
		v.add("ends_at", violationWindow, "End must be later than start.")
	}
}

func (v *violations) checkImage(in BannerInput) {
	if in.Kind == BannerKindSlide && in.ImageKey == nil {
		v.add("image_key", violationRequired, "A slide needs an image.")
	}
	if in.ImageKey != nil && !IsBannerImageKey(*in.ImageKey) {
		v.add("image_key", violationInvalid, "Image must be one processed by the image upload.")
	}
	switch {
	case utf8.RuneCountInString(in.ImageAlt) > BannerImageAltMaxLen:
		v.add("image_alt", violationTooLong, "Image description must be at most 160 characters.")
	case hasControl(in.ImageAlt, false):
		v.add("image_alt", violationInvalid, "Image description must be plain text on one line.")
	}
}

// IsSafeBannerURL reports whether raw is a link the dashboard may follow: an
// internal path ("/exams", never "//host" or "/\host", which browsers resolve
// as protocol-relative) or an absolute https:// URL with a host. Whitespace,
// control characters and backslashes are refused outright, which also closes
// the "java\nscript:" and "\x01javascript:" spellings.
func IsSafeBannerURL(raw string) bool {
	if raw == "" || len(raw) > BannerCTAURLMaxLen {
		return false
	}
	for _, r := range raw {
		if r == '\\' || unicode.IsSpace(r) || unicode.IsControl(r) {
			return false
		}
	}
	if strings.HasPrefix(raw, "/") {
		return !strings.HasPrefix(raw, "//")
	}
	if !strings.HasPrefix(raw, "https://") {
		return false
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return false
	}
	return parsed.Scheme == "https" && parsed.Host != "" && parsed.User == nil
}

func trimmedOrNil(value *string) *string {
	if value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

// hasControl reports a control character; newlines are allowed when
// multiline is set (the body), and nowhere else.
func hasControl(value string, multiline bool) bool {
	for _, r := range value {
		if multiline && r == '\n' {
			continue
		}
		if unicode.IsControl(r) {
			return true
		}
	}
	return false
}
