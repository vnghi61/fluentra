package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"unicode/utf8"
)

// The rendered slides are what core.banners accepts: JPEG, inside 1600×900,
// and the 3:1 shape the carousel frame is drawn at.
func TestSeedSlides_FixturesAreValidBannerImages(t *testing.T) {
	dir := filepath.Join("..", "..", defaultBannerFixtureDir)
	for _, slide := range seedSlides {
		img, err := readSlideImage(dir, slide.File)
		if err != nil {
			t.Fatalf("%s: %v (run node tools/banners/render.mjs)", slide.File, err)
		}
		if img.width != 3*img.height {
			t.Errorf("%s is %dx%d, want 3:1", slide.File, img.width, img.height)
		}
	}
}

// Each row has to pass the table's checks, or the seed fails in production
// rather than here.
func TestSeedSlides_RowsPassTheBannerConstraints(t *testing.T) {
	keyShape := regexp.MustCompile(`^banners/images/[0-9a-f-]{36}[.]jpg$`)
	internal := regexp.MustCompile(`^/([^/]|$)`)
	seen := map[string]bool{}
	for _, slide := range seedSlides {
		if seen[slide.ID.String()] {
			t.Errorf("duplicate id %s", slide.ID)
		}
		seen[slide.ID.String()] = true
		if !keyShape.MatchString(slide.objectKey()) {
			t.Errorf("%s: object key %q fails ck_banners_image_key", slide.File, slide.objectKey())
		}
		if n := utf8.RuneCountInString(strings.TrimSpace(slide.Title)); n < 1 || n > 120 {
			t.Errorf("%s: title is %d characters, want 1..120", slide.File, n)
		}
		if n := utf8.RuneCountInString(slide.Alt); n == 0 || n > 160 {
			t.Errorf("%s: alt text is %d characters, want 1..160", slide.File, n)
		}
		if !internal.MatchString(slide.Link) {
			t.Errorf("%s: link %q is not an internal path", slide.File, slide.Link)
		}
		if slide.Priority < 0 || slide.Priority > 1000 {
			t.Errorf("%s: priority %d outside 0..1000", slide.File, slide.Priority)
		}
	}
}

func TestReadSlideImage_RefusesWhatTheTableWouldNot(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "not.jpg"), []byte("<svg/>"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := readSlideImage(dir, "not.jpg"); err == nil {
		t.Fatal("a file that is not a JPEG was accepted")
	}
}
