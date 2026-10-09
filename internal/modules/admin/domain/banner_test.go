package domain_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/fluentra/fluentra/internal/modules/admin/domain"
	"github.com/fluentra/fluentra/internal/shared/apperr"
)

func strPtr(s string) *string { return &s }

// Field names and violation codes the rejection table asserts on.
const (
	fTitle    = "title"
	fCTAURL   = "cta_url"
	fPriority = "priority"
	cTooLong  = "too_long"
	cInvalid  = "invalid"
	cUnsafe   = "unsafe_url"
)

func TestIsSafeBannerURL(t *testing.T) {
	cases := []struct {
		url  string
		safe bool
	}{
		{"/", true},
		{"/exams", true},
		{"/review?deck=1#top", true},
		{"https://fluentra.dev/ielts", true},
		{"https://example.com", true},

		{"", false},
		{"//evil.example", false},
		{"/\\evil.example", false},
		{"\\\\evil.example", false},
		{"javascript:alert(1)", false},
		{"JavaScript:alert(1)", false},
		{" javascript:alert(1)", false},
		{"java\nscript:alert(1)", false},
		{"\x01javascript:alert(1)", false},
		{"data:text/html,<script>alert(1)</script>", false},
		{"vbscript:msgbox", false},
		{"http://example.com", false},
		{"HTTPS://example.com", false},
		{"https://", false},
		{"https:///path", false},
		{"https://user:pw@example.com", false},
		{"exams", false},
		{"/exams page", false},
		{"/" + strings.Repeat("a", domain.BannerCTAURLMaxLen), false},
	}
	for _, tc := range cases {
		if got := domain.IsSafeBannerURL(tc.url); got != tc.safe {
			t.Errorf("IsSafeBannerURL(%q) = %v, want %v", tc.url, got, tc.safe)
		}
	}
}

func fieldsOf(t *testing.T, err error) map[string]string {
	t.Helper()
	var appErr *apperr.Error
	if !errors.As(err, &appErr) {
		t.Fatalf("want *apperr.Error, got %T (%v)", err, err)
	}
	if appErr.Kind != apperr.Validation {
		t.Fatalf("kind = %s, want validation", appErr.Kind)
	}
	out := map[string]string{}
	for _, f := range appErr.Fields {
		out[f.Field] = f.Code
	}
	return out
}

func TestNormalizeBannerInput_TrimsAndDefaults(t *testing.T) {
	got, err := domain.NormalizeBannerInput(domain.BannerInput{
		Title:    "  Hello  ",
		Body:     "  line one\nline two ",
		CTALabel: strPtr("   "),
		CTAURL:   strPtr(""),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Title != "Hello" || got.Body != "line one\nline two" {
		t.Fatalf("not trimmed: %+v", got)
	}
	if got.CTALabel != nil || got.CTAURL != nil {
		t.Fatalf("blank CTA should become nil: %+v", got)
	}
	if got.Tone != domain.BannerToneInfo {
		t.Fatalf("tone = %q, want info", got.Tone)
	}
}

func TestNormalizeBannerInput_Rejections(t *testing.T) {
	start := time.Date(2026, 10, 8, 0, 0, 0, 0, time.UTC)
	before := start.Add(-time.Hour)

	cases := []struct {
		name  string
		in    domain.BannerInput
		field string
		code  string
	}{
		{"empty title", domain.BannerInput{Title: "   "}, fTitle, "required"},
		{"long title", domain.BannerInput{Title: strings.Repeat("é", 121)}, fTitle, cTooLong},
		{"multiline title", domain.BannerInput{Title: "a\nb"}, fTitle, cInvalid},
		{"long body", domain.BannerInput{Title: "t", Body: strings.Repeat("x", 501)}, "body", cTooLong},
		{"control in body", domain.BannerInput{Title: "t", Body: "a\x00b"}, "body", cInvalid},
		{
			"long label",
			domain.BannerInput{Title: "t", CTALabel: strPtr(strings.Repeat("x", 41)), CTAURL: strPtr("/x")},
			"cta_label", cTooLong,
		},
		{
			"javascript url",
			domain.BannerInput{Title: "t", CTALabel: strPtr("Go"), CTAURL: strPtr("javascript:alert(1)")},
			fCTAURL, cUnsafe,
		},
		{
			"data url",
			domain.BannerInput{Title: "t", CTALabel: strPtr("Go"), CTAURL: strPtr("data:text/html,x")},
			fCTAURL, cUnsafe,
		},
		{
			"protocol-relative url",
			domain.BannerInput{Title: "t", CTALabel: strPtr("Go"), CTAURL: strPtr("//evil.example")},
			fCTAURL, cUnsafe,
		},
		{"url without label", domain.BannerInput{Title: "t", CTAURL: strPtr("/x")}, "cta_label", "cta_incomplete"},
		{"label without url", domain.BannerInput{Title: "t", CTALabel: strPtr("Go")}, fCTAURL, "cta_incomplete"},
		{"unknown tone", domain.BannerInput{Title: "t", Tone: "danger"}, "tone", cInvalid},
		{"negative priority", domain.BannerInput{Title: "t", Priority: -1}, fPriority, "out_of_range"},
		{"huge priority", domain.BannerInput{Title: "t", Priority: 1001}, fPriority, "out_of_range"},
		{
			"ends before start",
			domain.BannerInput{Title: "t", StartsAt: &start, EndsAt: &before},
			"ends_at", "ends_before_start",
		},
		{
			"ends at start",
			domain.BannerInput{Title: "t", StartsAt: &start, EndsAt: &start},
			"ends_at", "ends_before_start",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := domain.NormalizeBannerInput(tc.in)
			fields := fieldsOf(t, err)
			if fields[tc.field] != tc.code {
				t.Fatalf("fields = %v, want %s=%s", fields, tc.field, tc.code)
			}
		})
	}
}

func TestNormalizeBannerInput_ReportsEveryField(t *testing.T) {
	_, err := domain.NormalizeBannerInput(domain.BannerInput{
		Title: "", Tone: "nope", Priority: 5000,
		CTALabel: strPtr("Go"), CTAURL: strPtr("javascript:x"),
	})
	fields := fieldsOf(t, err)
	for _, want := range []string{fTitle, "tone", fPriority, fCTAURL} {
		if _, ok := fields[want]; !ok {
			t.Fatalf("missing %s in %v", want, fields)
		}
	}
}

func TestNormalizeBannerInput_AcceptsValidCTA(t *testing.T) {
	for _, link := range []string{"/exams", "https://fluentra.dev/ielts"} {
		got, err := domain.NormalizeBannerInput(domain.BannerInput{
			Title: "t", CTALabel: strPtr("Go"), CTAURL: strPtr(link), Tone: domain.BannerTonePromo, Priority: 1000,
		})
		if err != nil {
			t.Fatalf("%s: unexpected error %v", link, err)
		}
		if got.CTAURL == nil || *got.CTAURL != link {
			t.Fatalf("%s: url lost: %+v", link, got)
		}
	}
}

func TestBannerLiveAt(t *testing.T) {
	now := time.Date(2026, 10, 8, 12, 0, 0, 0, time.UTC)
	past, future := now.Add(-time.Hour), now.Add(time.Hour)

	cases := []struct {
		name string
		b    domain.Banner
		live bool
	}{
		{"off", domain.Banner{IsActive: false}, false},
		{"on, no window", domain.Banner{IsActive: true}, true},
		{"started", domain.Banner{IsActive: true, StartsAt: &past}, true},
		{"not started", domain.Banner{IsActive: true, StartsAt: &future}, false},
		{"starts exactly now", domain.Banner{IsActive: true, StartsAt: &now}, true},
		{"ended", domain.Banner{IsActive: true, EndsAt: &past}, false},
		{"ends exactly now", domain.Banner{IsActive: true, EndsAt: &now}, false},
		{"inside window", domain.Banner{IsActive: true, StartsAt: &past, EndsAt: &future}, true},
		{"off inside window", domain.Banner{IsActive: false, StartsAt: &past, EndsAt: &future}, false},
	}
	for _, tc := range cases {
		if got := tc.b.LiveAt(now); got != tc.live {
			t.Errorf("%s: LiveAt = %v, want %v", tc.name, got, tc.live)
		}
	}
}

func TestNormalizeBannerInput_Slide(t *testing.T) {
	key := "banners/images/0199c3e4-5f60-7a82-9cde-f01234567899.jpg"
	got, err := domain.NormalizeBannerInput(domain.BannerInput{
		Kind:        domain.BannerKindSlide,
		Title:       "Summer sale",
		Body:        "left over from an announcement",
		CTALabel:    strPtr("Buy"),
		CTAURL:      strPtr("/pricing"),
		Dismissible: true,
		ImageKey:    &key,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// A slide is its picture: text, button and dismissal are cleared, the link
	// stays without button text, and the title describes the picture.
	if got.Body != "" || got.CTALabel != nil || got.Dismissible {
		t.Fatalf("slide kept announcement fields: %+v", got)
	}
	if got.CTAURL == nil || *got.CTAURL != "/pricing" || got.ImageAlt != "Summer sale" {
		t.Fatalf("slide lost its link or alt text: %+v", got)
	}
}

func TestNormalizeBannerInput_SlideRules(t *testing.T) {
	cases := map[string]struct {
		in    domain.BannerInput
		field string
		code  string
	}{
		"slide without an image": {
			in:    domain.BannerInput{Kind: domain.BannerKindSlide, Title: "x"},
			field: "image_key", code: "required",
		},
		"slide with an unsafe link": {
			in: domain.BannerInput{
				Kind: domain.BannerKindSlide, Title: "x", CTAURL: strPtr("javascript:alert(1)"),
				ImageKey: strPtr("banners/images/0199c3e4-5f60-7a82-9cde-f01234567899.jpg"),
			},
			field: fCTAURL, code: cUnsafe,
		},
		"unknown kind": {
			in:    domain.BannerInput{Kind: "popup", Title: "x"},
			field: "kind", code: cInvalid,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := domain.NormalizeBannerInput(tc.in)
			var appErr *apperr.Error
			if !errors.As(err, &appErr) {
				t.Fatalf("err = %v, want a validation error", err)
			}
			for _, f := range appErr.Fields {
				if f.Field == tc.field && f.Code == tc.code {
					return
				}
			}
			t.Fatalf("fields = %+v, want %s/%s", appErr.Fields, tc.field, tc.code)
		})
	}
}

func TestNormalizeBannerInput_DefaultsToAnnouncement(t *testing.T) {
	got, err := domain.NormalizeBannerInput(domain.BannerInput{Title: "x"})
	if err != nil || got.Kind != domain.BannerKindAnnouncement {
		t.Fatalf("got %+v, %v; want an announcement", got, err)
	}
	// An announcement still pairs its button text and link.
	_, err = domain.NormalizeBannerInput(domain.BannerInput{Title: "x", CTAURL: strPtr("/exams")})
	if err == nil {
		t.Fatalf("an announcement link without button text was accepted")
	}
}
