package main

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/fluentra/fluentra/cmd/internal/examfixture"
	questionbankcontract "github.com/fluentra/fluentra/internal/modules/questionbank/contract"
)

// photosFile is the Part 1 photographs fixture inside the fixtures directory.
const photosFile = "toeic-part1-photos.json"

// fixturePhotos hands out the fixture's photographs (WO 22 D22-21), each once:
// a photograph an item already carries, in any state, is not handed out again,
// nor is one handed out earlier in this run.
type fixturePhotos struct {
	photos []examfixture.Part1Photo
	used   func(ctx context.Context) (map[string]bool, error)
	handed map[string]bool
}

// loadFixturePhotos reads the fixture from the fixtures directory. A directory
// without one has no photographs, so Part 1 waits.
func loadFixturePhotos(pool *pgxpool.Pool, dir string) (*fixturePhotos, error) {
	file, err := examfixture.LoadPart1Photos(filepath.Join(dir, photosFile))
	if errors.Is(err, fs.ErrNotExist) {
		return &fixturePhotos{used: usedPhotoURLs(pool), handed: map[string]bool{}}, nil
	}
	if err != nil {
		return nil, err
	}
	return &fixturePhotos{photos: file.Photos, used: usedPhotoURLs(pool), handed: map[string]bool{}}, nil
}

// NextPhotos returns up to n photographs no item carries yet.
func (f *fixturePhotos) NextPhotos(ctx context.Context, n int) ([]questionbankcontract.Photo, error) {
	if n <= 0 || len(f.photos) == 0 {
		return nil, nil
	}
	used, err := f.used(ctx)
	if err != nil {
		return nil, err
	}
	var next []questionbankcontract.Photo
	for _, photo := range f.photos {
		if len(next) == n {
			break
		}
		if used[photo.URL] || f.handed[photo.URL] {
			continue
		}
		f.handed[photo.URL] = true
		next = append(next, questionbankcontract.Photo{
			URL: photo.URL, CreditPage: photo.CreditPage, Licence: photo.Licence, Description: photo.Description,
		})
	}
	return next, nil
}

// usedPhotoURLs reads the photographs items already carry.
func usedPhotoURLs(pool *pgxpool.Pool) func(ctx context.Context) (map[string]bool, error) {
	return func(ctx context.Context) (map[string]bool, error) {
		rows, err := pool.Query(ctx, `
			SELECT DISTINCT body->>'image_url'
			FROM content.content_versions
			WHERE body ? 'image_url'`)
		if err != nil {
			return nil, fmt.Errorf("read the photographs items carry: %w", err)
		}
		defer rows.Close()
		used := map[string]bool{}
		for rows.Next() {
			var url string
			if err := rows.Scan(&url); err != nil {
				return nil, fmt.Errorf("scan a photograph url: %w", err)
			}
			used[url] = true
		}
		return used, rows.Err()
	}
}

// mockPhotos stands in for the fixture in a -mock run, so a throwaway database
// composes TOEIC tests offline. Its photographs point nowhere and must never
// reach a real bank, which -mock already guarantees for everything it writes.
type mockPhotos struct{}

// NextPhotos returns n placeholder photographs, each with a fresh URL.
func (mockPhotos) NextPhotos(_ context.Context, n int) ([]questionbankcontract.Photo, error) {
	photos := make([]questionbankcontract.Photo, 0, max(n, 0))
	for range max(n, 0) {
		id := uuid.NewString()
		photos = append(photos, questionbankcontract.Photo{
			URL:         "https://example.invalid/mock-photo-" + id + ".jpg",
			CreditPage:  "https://example.invalid/mock-photo-" + id,
			Licence:     "CC0-1.0",
			Description: "A mock photograph: a person at a desk reads a document beside a window.",
		})
	}
	return photos, nil
}
