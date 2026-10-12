// Package examfixture is the frozen exam-media format: the Part 1 photographs
// (WO 22 Stage M) that TOEIC Listening Part 1 needs and nothing in the bank can
// generate.
//
// A photograph is openly licensed and human-described. The generator writes the
// statements from the description and never sees the image (D22-21); the credit
// shows wherever the photo does.
package examfixture

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
)

// PhotosFormat is the Part 1 photos fixture schema version. A file with another
// format is refused rather than guessed at.
const PhotosFormat = "fluentra.exam.part1photos.fixture.v1"

// Part1PhotosFile is the whole fixture: its provenance and the photographs.
type Part1PhotosFile struct {
	Format      string       `json:"format"`
	Source      string       `json:"source"`
	CheckedAt   string       `json:"checked_at"`
	LicenceNote string       `json:"licence_note"`
	Photos      []Part1Photo `json:"photos"`
}

// Part1Photo is one openly licensed photograph and its human-written
// description.
type Part1Photo struct {
	ID          string `json:"id"`
	URL         string `json:"url"`
	CreditPage  string `json:"credit_page"`
	Licence     string `json:"licence"`
	Description string `json:"description"`
}

// LoadPart1Photos reads and validates the fixture. A photograph without a URL,
// a credit page, a description or an open licence is refused: an uncredited
// image, or one whose licence is not CC0 or CC BY, must not reach a learner.
func LoadPart1Photos(path string) (*Part1PhotosFile, error) {
	raw, err := os.ReadFile(path) //nolint:gosec // the operator's own fixtures directory
	if err != nil {
		return nil, fmt.Errorf("read part 1 photos fixture: %w", err)
	}
	var file Part1PhotosFile
	if err := json.Unmarshal(raw, &file); err != nil {
		return nil, fmt.Errorf("parse part 1 photos fixture: %w", err)
	}
	if file.Format != PhotosFormat {
		return nil, fmt.Errorf("part 1 photos fixture format is %q, want %q", file.Format, PhotosFormat)
	}
	seen := make(map[string]bool, len(file.Photos))
	for i, photo := range file.Photos {
		if err := photo.validate(); err != nil {
			return nil, fmt.Errorf("photo %d: %w", i+1, err)
		}
		if seen[photo.ID] {
			return nil, fmt.Errorf("photo %d: duplicate id %q", i+1, photo.ID)
		}
		seen[photo.ID] = true
	}
	return &file, nil
}

func (p Part1Photo) validate() error {
	if strings.TrimSpace(p.ID) == "" {
		return errors.New("needs an id")
	}
	if strings.TrimSpace(p.URL) == "" {
		return errors.New("needs a url")
	}
	if strings.TrimSpace(p.CreditPage) == "" {
		return errors.New("needs a credit_page")
	}
	if strings.TrimSpace(p.Description) == "" {
		return errors.New("needs a human description")
	}
	if !IsOpenLicence(p.Licence) {
		return fmt.Errorf("licence %q is not CC0 or CC BY", p.Licence)
	}
	return nil
}

// IsOpenLicence reports whether a licence is one we may use: CC0 or CC BY. CC BY
// variants (any version, any port) count; NC, ND and SA do not, so a photograph
// can be reused in any material without further conditions than the credit.
func IsOpenLicence(licence string) bool {
	compact := strings.NewReplacer(" ", "", "-", "", "_", "").
		Replace(strings.ToUpper(strings.TrimSpace(licence)))
	if strings.HasPrefix(compact, "CC0") {
		return true
	}
	// CC BY, but not the share-alike, non-commercial or no-derivatives variants:
	// those would constrain the material a photograph appears in.
	if !strings.HasPrefix(compact, "CCBY") {
		return false
	}
	return !strings.Contains(compact, "NC") &&
		!strings.Contains(compact, "ND") &&
		!strings.Contains(compact, "SA")
}
