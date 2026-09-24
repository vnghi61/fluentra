package examfixture

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestExamFixture_RoundTrips(t *testing.T) {
	dir := t.TempDir()
	file := &ExamFile{
		Exam:        "toeic",
		ExamVersion: "TOEIC_LR_2026",
		Items: []ExamItem{
			{
				Slug:        "toeic-part1-1",
				Kind:        "photo_description",
				CEFRLevel:   "B1",
				Skill:       "listening",
				ExamVersion: "TOEIC_LR_2026",
				ExamPartID:  "20000000-0000-0000-0000-000000000001",
				Body:        json.RawMessage(`{"prompt":"A man is reading.","statements":[]}`),
				Verification: Verification{
					Confirmed: true,
					Model:     "glm-5.3-flash",
				},
			},
		},
	}

	path, err := WriteExamFile(dir, file)
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	loaded, err := LoadExamFile(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if loaded.Format != ExamFormat {
		t.Errorf("format = %q, want %q", loaded.Format, ExamFormat)
	}
	if len(loaded.Items) != 1 || loaded.Items[0].Slug != "toeic-part1-1" {
		t.Fatalf("items = %+v, want one item", loaded.Items)
	}
	if !loaded.Items[0].Verification.Confirmed {
		t.Error("the approval the item had must survive the round trip")
	}
}

func TestReadAll_SkipsThePhotosFixtureInTheSameDirectory(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "toeic-part1-photos.json"),
		[]byte(`{"format":"`+PhotosFormat+`","photos":[]}`), 0o600); err != nil {
		t.Fatalf("write photos fixture: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "toeic.json"),
		[]byte(`{"format":"`+ExamFormat+`","exam":"toeic","items":[]}`), 0o600); err != nil {
		t.Fatalf("write exam fixture: %v", err)
	}

	files, err := ReadAll(dir)
	if err != nil {
		t.Fatalf("read all: %v", err)
	}
	if len(files) != 1 || files[0].Exam != "toeic" {
		t.Fatalf("files = %+v, want the one exam fixture", files)
	}
}

func TestLoadExamFile_RefusesAnUnusableItem(t *testing.T) {
	tests := map[string]string{ //nolint:gosec // fixture strings, not credentials
		"no part": `{"slug":"s","kind":"k","body":{}}`,
		"no body": `{"slug":"s","kind":"k","exam_part_id":"p"}`,
		"no kind": `{"slug":"s","exam_part_id":"p","body":{}}`,
	}
	for name, item := range tests {
		t.Run(name, func(t *testing.T) {
			path := writeFixture(t, `{"format":"`+ExamFormat+`","exam":"toeic","items":[`+item+`]}`)
			if _, err := LoadExamFile(path); err == nil {
				t.Fatal("an unusable item must be refused")
			}
		})
	}
}

func TestLoadExamFile_RefusesDuplicateSlugsAndAnotherFormat(t *testing.T) {
	dup := `{"slug":"s","kind":"k","exam_part_id":"p","body":{}}`
	path := writeFixture(t, `{"format":"`+ExamFormat+`","exam":"toeic","items":[`+dup+`,`+dup+`]}`)
	if _, err := LoadExamFile(path); err == nil {
		t.Fatal("a duplicate slug must be refused")
	}

	path = writeFixture(t, `{"format":"other","exam":"toeic","items":[]}`)
	if _, err := LoadExamFile(path); err == nil {
		t.Fatal("another format must be refused")
	}
}
