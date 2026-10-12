package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestFixturePhotos_HandsEachPhotographOutOnce: a photograph an item already
// carries, or one handed out earlier in the run, is never handed out again.
func TestFixturePhotos_HandsEachPhotographOutOnce(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, photosFile), []byte(`{
		"format": "fluentra.exam.part1photos.fixture.v1",
		"photos": [
			{"id": "a", "url": "https://example.org/a.jpg", "credit_page": "https://example.org/a",
			 "licence": "CC0-1.0", "description": "A man carries a box."},
			{"id": "b", "url": "https://example.org/b.jpg", "credit_page": "https://example.org/b",
			 "licence": "CC-BY-4.0", "description": "Two women sit at a table."},
			{"id": "c", "url": "https://example.org/c.jpg", "credit_page": "https://example.org/c",
			 "licence": "CC-BY-4.0", "description": "A bus waits at a stop."}
		]
	}`), 0o600))
	source, err := loadFixturePhotos(nil, dir)
	require.NoError(t, err)
	source.used = func(context.Context) (map[string]bool, error) {
		return map[string]bool{"https://example.org/a.jpg": true}, nil
	}

	first, err := source.NextPhotos(context.Background(), 1)
	require.NoError(t, err)
	require.Len(t, first, 1)
	assert.Equal(t, "https://example.org/b.jpg", first[0].URL, "a is already on an item")
	assert.Equal(t, "Two women sit at a table.", first[0].Description)

	rest, err := source.NextPhotos(context.Background(), 5)
	require.NoError(t, err)
	require.Len(t, rest, 1, "only c is left")
	assert.Equal(t, "https://example.org/c.jpg", rest[0].URL)

	none, err := source.NextPhotos(context.Background(), 5)
	require.NoError(t, err)
	assert.Empty(t, none)
}

// TestFixturePhotos_NoFixtureIsNoPhotographs: a fixtures directory without the
// file leaves Part 1 waiting rather than failing the run.
func TestFixturePhotos_NoFixtureIsNoPhotographs(t *testing.T) {
	source, err := loadFixturePhotos(nil, t.TempDir())
	require.NoError(t, err)
	photos, err := source.NextPhotos(context.Background(), 3)
	require.NoError(t, err)
	assert.Empty(t, photos)
}

// TestMockPhotos_EachIsNew: a -mock run's placeholders never collide.
func TestMockPhotos_EachIsNew(t *testing.T) {
	photos, err := mockPhotos{}.NextPhotos(context.Background(), 2)
	require.NoError(t, err)
	require.Len(t, photos, 2)
	assert.NotEqual(t, photos[0].URL, photos[1].URL)
}
