package main

import (
	"io/fs"
	"strings"
	"testing"

	"github.com/fluentra/fluentra/db/migrations"
)

func TestMigrationFS_FlattensEmbeddedPerModuleMigrations(t *testing.T) {
	t.Parallel()

	entries, err := fs.ReadDir(migrationFS(), ".")
	if err != nil {
		t.Fatalf("read embedded migrations: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("migration count = %d, want 1", len(entries))
	}
	if !strings.HasPrefix(entries[0].Name(), "1700000000_") {
		t.Fatalf("migration name = %q, want globally timestamped prefix", entries[0].Name())
	}
}

func TestBootstrapMigration_CreatesAllSchemasAndRestrictsApplicationRole(t *testing.T) {
	t.Parallel()

	contents, err := migrations.Files.ReadFile("_bootstrap/1700000000_bootstrap_database.sql")
	if err != nil {
		t.Fatalf("read bootstrap migration: %v", err)
	}
	sql := string(contents)
	for _, schema := range []string{
		"core", "audit", "content", "learn", "skill", "assess", "comm", "billing", "ai", "ops", "analytics",
	} {
		if !strings.Contains(sql, "CREATE SCHEMA IF NOT EXISTS "+schema) {
			t.Errorf("bootstrap migration does not create schema %q", schema)
		}
	}
	if !strings.Contains(sql, "GRANT USAGE ON SCHEMA") || strings.Contains(sql, "GRANT CREATE ON SCHEMA") {
		t.Fatal("application role must receive schema usage but no DDL privileges")
	}
}

func TestRun_RejectsInvalidCommandBeforeDatabaseAccess(t *testing.T) {
	t.Parallel()

	err := run(t.Context(), []string{"invalid"}, nil)
	if err == nil || !strings.Contains(err.Error(), "unknown migration command") {
		t.Fatalf("run error = %v, want invalid command error", err)
	}
}
