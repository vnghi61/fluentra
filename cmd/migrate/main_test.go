package main

import (
	"io/fs"
	"regexp"
	"slices"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/fluentra/fluentra/db/migrations"
)

// timestampedName is the naming contract from DATABASE_GUIDELINE.md: a globally
// unique Unix timestamp, then the description, then the module the flattening
// appended.
var timestampedName = regexp.MustCompile(`^\d{10}_[a-z0-9_]+\.sql$`)

// embeddedSQLCount counts the .sql files the embed actually carries, so this
// test does not restate a number that changes every time a module adds a
// migration.
func embeddedSQLCount(t *testing.T) int {
	t.Helper()
	count := 0
	err := fs.WalkDir(migrations.Files, ".", func(name string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() && strings.HasSuffix(name, ".sql") {
			count++
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk embedded migrations: %v", err)
	}
	return count
}

func TestMigrationFS_FlattensEveryEmbeddedMigrationToTheRoot(t *testing.T) {
	t.Parallel()

	sources, err := migrationFS()
	if err != nil {
		t.Fatalf("build migration FS: %v", err)
	}
	entries, err := fs.ReadDir(sources, ".")
	if err != nil {
		t.Fatalf("read embedded migrations: %v", err)
	}

	if want := embeddedSQLCount(t); len(entries) != want {
		t.Fatalf("flattened %d migrations, want %d — one per embedded .sql file", len(entries), want)
	}
	if len(entries) < 2 {
		t.Fatal("expected migrations from more than one module; flattening is untested otherwise")
	}
	for _, entry := range entries {
		if entry.IsDir() {
			t.Errorf("%q is a directory; migrations must be flattened to the root", entry.Name())
		}
		if !timestampedName.MatchString(entry.Name()) {
			t.Errorf("%q does not start with a globally unique unix timestamp", entry.Name())
		}
	}
}

// TestMigrationFS_LexicalOrderIsGlobalApplyOrder is the property that matters:
// goose applies in lexical order, so sorting names must reproduce timestamp
// order even when consecutive migrations come from different modules.
func TestMigrationFS_LexicalOrderIsGlobalApplyOrder(t *testing.T) {
	t.Parallel()

	source := fstest.MapFS{
		"_bootstrap/1700000000_bootstrap_database.sql": {Data: []byte("-- a")},
		"mailer/1700000002_create_mailer_tables.sql":   {Data: []byte("-- c")},
		"job/1700000001_job_outbox_tables.sql":         {Data: []byte("-- b")},
		"auth/1700000003_create_auth_challenges.sql":   {Data: []byte("-- d")},
		"job/1700000004_add_job_dead_letter.sql":       {Data: []byte("-- e")},
		"db/queries/user/not_a_migration.txt":          {Data: []byte("ignored")},
	}

	sources, err := migrations.Flatten(source)
	if err != nil {
		t.Fatalf("flatten: %v", err)
	}
	entries, err := fs.ReadDir(sources, ".")
	if err != nil {
		t.Fatalf("read dir: %v", err)
	}

	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	if len(names) != 5 {
		t.Fatalf("names = %v, want the five .sql files only", names)
	}
	if !slices.IsSorted(names) {
		t.Fatalf("ReadDir did not return names in lexical order: %v", names)
	}

	timestamps := make([]string, 0, len(names))
	for _, name := range names {
		prefix, _, _ := strings.Cut(name, "_")
		timestamps = append(timestamps, prefix)
	}
	if !slices.IsSorted(timestamps) {
		t.Fatalf("lexical name order does not follow timestamp order: %v", names)
	}
	// The interleave is the point: bootstrap, job, mailer, auth, job.
	if names[1] != "1700000001_job_outbox_tables_job.sql" || names[2] != "1700000002_create_mailer_tables_mailer.sql" {
		t.Fatalf("modules did not interleave by timestamp: %v", names)
	}
}

// TestFlattenMigrations_RejectsDuplicateFlattenedNames exercises the collision
// guard. Appending the module with an underscore is ambiguous — a file named
// `<x>_<y>.sql` in module `z` collides with `<x>.sql` in a module named `y_z` —
// so the guard is what keeps one migration from silently replacing another.
func TestFlattenMigrations_RejectsDuplicateFlattenedNames(t *testing.T) {
	t.Parallel()

	source := fstest.MapFS{
		"job/1700000001_outbox.sql":      {Data: []byte("-- a")},
		"outbox_job/1700000001.sql":      {Data: []byte("-- b")},
		"unrelated/1700000002_other.sql": {Data: []byte("-- c")},
	}
	if _, err := migrations.Flatten(source); err == nil {
		t.Fatal("expected an error for two migrations flattening to the same name")
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
