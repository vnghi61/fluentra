package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSeedExamFixtures_ReportsAnEmptyDirectory checks the loader reports a
// database with no exam content rather than failing on it. It returns before
// touching the pool, so a nil pool is safe here.
func TestSeedExamFixtures_ReportsAnEmptyDirectory(t *testing.T) {
	var out bytes.Buffer
	err := seedExamFixtures(context.Background(), nil, uuid.Nil, t.TempDir(), &out)
	require.NoError(t, err)
	assert.Contains(t, out.String(), "No exam fixtures")
}

// TestSeedExamFixtures_RefusesABadFixture checks a malformed fixture is refused
// before anything is written.
func TestSeedExamFixtures_RefusesABadFixture(t *testing.T) {
	dir := t.TempDir()
	body := `{"format":"fluentra.exam.fixture.v1","exam":"toeic","items":[{"slug":"s"}]}`
	if err := os.WriteFile(filepath.Join(dir, "toeic.json"), []byte(body), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	err := seedExamFixtures(context.Background(), nil, uuid.Nil, dir, &bytes.Buffer{})
	require.Error(t, err, "an item with no kind/part/body must be refused")
}
