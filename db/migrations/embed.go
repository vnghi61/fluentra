// Package migrations exposes the immutable migration source embedded in release binaries.
package migrations

import (
	"embed"
	"fmt"
	"io/fs"
	"path"
	"sort"
	"strings"
	"testing/fstest"
)

// Files contains every per-module Goose migration.
//
//go:embed all:*
var Files embed.FS

// Flattened returns every embedded migration in one virtual directory, named
// `<timestamp>_<description>_<module>.sql`. Keeping the globally unique
// timestamp leading is what makes lexical order the apply order across
// modules, which is the ordering goose relies on.
func Flattened() (fs.FS, error) { return Flatten(Files) }

// Flatten is Flattened over an arbitrary source, so tests can supply their own.
func Flatten(source fs.FS) (fs.FS, error) {
	files := make(fstest.MapFS)
	err := fs.WalkDir(source, ".", func(name string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || path.Ext(name) != ".sql" {
			return nil
		}
		content, err := fs.ReadFile(source, name)
		if err != nil {
			return err
		}
		base := path.Base(name)
		key := strings.TrimSuffix(base, path.Ext(base)) + "_" + strings.ReplaceAll(path.Dir(name), "/", "_") + path.Ext(base)
		if _, exists := files[key]; exists {
			return fmt.Errorf("duplicate migration filename %q", key)
		}
		files[key] = &fstest.MapFile{Data: content, Mode: 0o444}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("load embedded migrations: %w", err)
	}
	return sortedMapFS{MapFS: files}, nil
}

// sortedMapFS makes ordering deterministic even for callers that inspect it directly.
type sortedMapFS struct{ fstest.MapFS }

func (s sortedMapFS) ReadDir(name string) ([]fs.DirEntry, error) {
	entries, err := s.MapFS.ReadDir(name)
	if err != nil {
		return nil, err
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	return entries, nil
}
