package examfixture

import (
	"os"
	"path/filepath"
	"testing"
)

func writeFixture(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "photos.json")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	return path
}

func TestLoadPart1Photos_AcceptsCreditedOpenLicence(t *testing.T) {
	path := writeFixture(t, `{
		"format": "fluentra.exam.part1photos.fixture.v1",
		"photos": [
			{
				"id": "p1",
				"url": "https://example.org/p1.jpg",
				"credit_page": "https://example.org/p1",
				"licence": "CC0-1.0",
				"description": "A man in a white shirt hands a document to a woman at a desk."
			}
		]
	}`)

	file, err := LoadPart1Photos(path)
	if err != nil {
		t.Fatalf("a credited CC0 photo must load: %v", err)
	}
	if len(file.Photos) != 1 || file.Photos[0].ID != "p1" {
		t.Fatalf("photos = %+v, want one photo p1", file.Photos)
	}
}

func TestLoadPart1Photos_RefusesWhatMayNotBeShown(t *testing.T) {
	tests := map[string]string{ //nolint:gosec // fixture strings, not credentials
		"a non-open licence": `{"id":"p1","url":"u","credit_page":"c","licence":"CC-BY-NC-4.0","description":"d"}`,
		"no credit page":     `{"id":"p1","url":"u","credit_page":"","licence":"CC0-1.0","description":"d"}`,
		"no description":     `{"id":"p1","url":"u","credit_page":"c","licence":"CC0-1.0","description":""}`,
	}
	for name, photo := range tests {
		t.Run(name, func(t *testing.T) {
			path := writeFixture(t, `{"format":"fluentra.exam.part1photos.fixture.v1","photos":[`+photo+`]}`)
			if _, err := LoadPart1Photos(path); err == nil {
				t.Fatal("an unusable photo must be refused")
			}
		})
	}
}

func TestLoadPart1Photos_RefusesAnotherFormat(t *testing.T) {
	path := writeFixture(t, `{"format":"something-else","photos":[]}`)
	if _, err := LoadPart1Photos(path); err == nil {
		t.Fatal("a fixture with another format must be refused")
	}
}

func TestIsOpenLicence(t *testing.T) {
	open := []string{"CC0-1.0", "cc0", "CC-BY-4.0", "CC BY 2.0"}
	for _, licence := range open {
		if !IsOpenLicence(licence) {
			t.Errorf("IsOpenLicence(%q) = false, want true", licence)
		}
	}
	closed := []string{"CC-BY-NC-4.0", "CC-BY-ND-4.0", "All rights reserved", ""}
	for _, licence := range closed {
		if IsOpenLicence(licence) {
			t.Errorf("IsOpenLicence(%q) = true, want false", licence)
		}
	}
}
