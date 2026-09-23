// Package foundationfixture is the frozen Foundation content format shared by
// the two commands that produce and consume it (WO 22 Stage G).
//
// Generate once, verify once, freeze into `db/fixtures/foundation/`, and let
// `make seed` load it offline. Generating on every machine would make the seed
// depend on a model that is slow, paid and sometimes down, and two machines
// would seed different content.
package foundationfixture

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Format is the fixture schema version. A file with another format is refused
// rather than guessed at.
const Format = "fluentra.foundation.fixture.v1"

// File is one spine node's published content.
type File struct {
	Format      string    `json:"format"`
	GeneratedAt time.Time `json:"generated_at"`
	Node        Node      `json:"node"`
	Items       []Item    `json:"items"`
}

// Node identifies the spine node the file belongs to.
type Node struct {
	Namespace string `json:"namespace"`
	Code      string `json:"code"`
	Label     string `json:"label"`
	CEFRLevel string `json:"cefr_level"`
}

// Item is one published version, with the approval it already had so the seed
// does not claim a person reviewed it.
type Item struct {
	Slug         string          `json:"slug"`
	Kind         string          `json:"kind"`
	CEFRLevel    string          `json:"cefr_level"`
	Body         json.RawMessage `json:"body"`
	Verification Verify          `json:"verification"`
}

// Verify is the independent verifier's marking, lifted from the body's
// provenance.
type Verify struct {
	Confirmed bool      `json:"confirmed"`
	Model     string    `json:"model,omitempty"`
	CheckedAt time.Time `json:"checked_at,omitempty"`
	Reason    string    `json:"reason,omitempty"`
}

// Write encodes one node's fixture to dir.
func Write(dir string, file File) error {
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return fmt.Errorf("create fixture directory: %w", err)
	}
	encoded, err := json.MarshalIndent(file, "", "  ")
	if err != nil {
		return fmt.Errorf("encode fixture for %s: %w", file.Node.Code, err)
	}
	path := filepath.Join(dir, strings.ToLower(file.Node.Code)+".json")
	if err := os.WriteFile(path, append(encoded, '\n'), 0o600); err != nil {
		return fmt.Errorf("write fixture %s: %w", path, err)
	}
	return nil
}

// ReadAll reads and validates every fixture in dir, ordered by node code so a
// load is deterministic.
func ReadAll(dir string) ([]File, error) {
	paths, err := filepath.Glob(filepath.Join(dir, "*.json"))
	if err != nil {
		return nil, fmt.Errorf("list fixtures: %w", err)
	}
	sort.Strings(paths)

	files := make([]File, 0, len(paths))
	for _, path := range paths {
		raw, err := os.ReadFile(path) //nolint:gosec // the operator's own fixtures directory
		if err != nil {
			return nil, fmt.Errorf("read fixture %s: %w", path, err)
		}
		var file File
		if err := json.Unmarshal(raw, &file); err != nil {
			return nil, fmt.Errorf("decode fixture %s: %w", path, err)
		}
		if err := Validate(file); err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
		files = append(files, file)
	}
	return files, nil
}

// Validate refuses a file the loader could not apply.
func Validate(file File) error {
	if file.Format != Format {
		return fmt.Errorf("unknown fixture format %q, want %q", file.Format, Format)
	}
	if strings.TrimSpace(file.Node.Code) == "" || strings.TrimSpace(file.Node.Namespace) == "" {
		return errors.New("fixture node needs a namespace and a code")
	}
	for _, item := range file.Items {
		if strings.TrimSpace(item.Slug) == "" || strings.TrimSpace(item.Kind) == "" {
			return fmt.Errorf("fixture item in %s needs a slug and a kind", file.Node.Code)
		}
		if len(item.Body) == 0 || !json.Valid(item.Body) {
			return fmt.Errorf("fixture item %s has no valid body", item.Slug)
		}
	}
	return nil
}
