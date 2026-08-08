// Package migrations exposes the immutable migration source embedded in release binaries.
package migrations

import "embed"

// Files contains every per-module Goose migration.
//
//go:embed all:*
var Files embed.FS
