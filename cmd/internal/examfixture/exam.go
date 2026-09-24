package examfixture

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ExamFormat is the frozen exam-questions fixture schema version. A file with
// another format is refused rather than guessed at.
const ExamFormat = "fluentra.exam.fixture.v1"

// ExamFile is one exam's published questions, frozen so `make seed` loads them
// offline with the approval each already had (WO 22 Stage N).
type ExamFile struct {
	Format      string     `json:"format"`
	Exam        string     `json:"exam"`
	ExamVersion string     `json:"exam_version"`
	GeneratedAt time.Time  `json:"generated_at"`
	Items       []ExamItem `json:"items"`
}

// ExamItem is one published question and the bank activity that carries it.
type ExamItem struct {
	Slug         string          `json:"slug"`
	Kind         string          `json:"kind"`
	CEFRLevel    string          `json:"cefr_level"`
	Skill        string          `json:"skill"`
	ExamVersion  string          `json:"exam_version"`
	ExamPartID   string          `json:"exam_part_id"`
	Group        string          `json:"group,omitempty"`
	Body         json.RawMessage `json:"body"`
	Verification Verification    `json:"verification"`
}

// Verification is the independent verifier's marking, lifted from the body's
// provenance so the seed records the approval the item already had.
type Verification struct {
	Confirmed bool      `json:"confirmed"`
	Model     string    `json:"model,omitempty"`
	CheckedAt time.Time `json:"checked_at,omitempty"`
	Reason    string    `json:"reason,omitempty"`
}

// ReadAll reads and validates every exam fixture in dir, in file-name order.
// A directory with no fixtures is not an error: a database with no exam content
// is a valid state the seed reports rather than fails on.
func ReadAll(dir string) ([]*ExamFile, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read exam fixtures directory: %w", err)
	}
	var files []*ExamFile
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		// The directory also holds the Part 1 photos fixture, which is a
		// different format. Skip it rather than refuse it as a malformed exam.
		if format, err := peekFormat(path); err == nil && format == PhotosFormat {
			continue
		}
		file, err := LoadExamFile(path)
		if err != nil {
			return nil, err
		}
		files = append(files, file)
	}
	return files, nil
}

// peekFormat reads just the format field of a fixture, to tell the kinds of
// fixture a shared directory holds apart.
func peekFormat(path string) (string, error) {
	raw, err := os.ReadFile(path) //nolint:gosec // the operator's own fixtures directory
	if err != nil {
		return "", err
	}
	var parsed struct {
		Format string `json:"format"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return "", err
	}
	return parsed.Format, nil
}

// LoadExamFile reads and validates one exam fixture. An item missing its slug,
// kind, part or body is refused: the seed would otherwise write a question with
// no home.
func LoadExamFile(path string) (*ExamFile, error) {
	raw, err := os.ReadFile(path) //nolint:gosec // the operator's own fixtures directory
	if err != nil {
		return nil, fmt.Errorf("read exam fixture: %w", err)
	}
	var file ExamFile
	if err := json.Unmarshal(raw, &file); err != nil {
		return nil, fmt.Errorf("parse exam fixture: %w", err)
	}
	if file.Format != ExamFormat {
		return nil, fmt.Errorf("exam fixture format is %q, want %q", file.Format, ExamFormat)
	}
	if strings.TrimSpace(file.Exam) == "" {
		return nil, errors.New("exam fixture needs an exam name")
	}
	seen := make(map[string]bool, len(file.Items))
	for i, item := range file.Items {
		if err := item.validate(); err != nil {
			return nil, fmt.Errorf("item %d: %w", i+1, err)
		}
		if seen[item.Slug] {
			return nil, fmt.Errorf("item %d: duplicate slug %q", i+1, item.Slug)
		}
		seen[item.Slug] = true
	}
	return &file, nil
}

func (it ExamItem) validate() error {
	if strings.TrimSpace(it.Slug) == "" {
		return errors.New("needs a slug")
	}
	if strings.TrimSpace(it.Kind) == "" {
		return errors.New("needs a kind")
	}
	if strings.TrimSpace(it.ExamPartID) == "" {
		return errors.New("needs an exam_part_id")
	}
	if len(it.Body) == 0 {
		return errors.New("needs a body")
	}
	return nil
}

// WriteExamFile writes one exam fixture to dir, returning the path written. The
// file name is the exam name, so two exams never overwrite each other.
func WriteExamFile(dir string, file *ExamFile) (string, error) {
	if file.Format == "" {
		file.Format = ExamFormat
	}
	if file.GeneratedAt.IsZero() {
		file.GeneratedAt = time.Now().UTC()
	}
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return "", fmt.Errorf("create exam fixtures directory: %w", err)
	}
	raw, err := json.MarshalIndent(file, "", "  ")
	if err != nil {
		return "", fmt.Errorf("encode exam fixture: %w", err)
	}
	path := filepath.Join(dir, strings.ToLower(file.Exam)+".json")
	if err := os.WriteFile(path, append(raw, '\n'), 0o600); err != nil {
		return "", fmt.Errorf("write exam fixture: %w", err)
	}
	return path, nil
}
