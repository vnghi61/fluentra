//go:build integration

// Package exam_test verifies the `assess` exam structure against a real
// PostgreSQL instance.
package exam_test

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"github.com/fluentra/fluentra/db/migrations"
)

const schemaDatabase = "fluentra_exam_schema_test"

// Section names the format tests compare.
const (
	sectionListening = "listening"
	sectionReading   = "reading"
	sectionWriting   = "writing"
	sectionSpeaking  = "speaking"
)

var packagePool *pgxpool.Pool

func TestMain(m *testing.M) {
	base := os.Getenv("TEST_DATABASE_URL")
	if base == "" {
		os.Exit(m.Run())
	}

	dsn, dropDatabase, err := createDatabase(base, schemaDatabase)
	if err != nil {
		fmt.Fprintf(os.Stderr, "prepare %s: %v\n", schemaDatabase, err)
		os.Exit(1)
	}
	provider, err := migrateUp(dsn)
	if err != nil {
		dropDatabase()
		fmt.Fprintf(os.Stderr, "migrate %s: %v\n", schemaDatabase, err)
		os.Exit(1)
	}
	_ = provider.Close()

	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		dropDatabase()
		fmt.Fprintf(os.Stderr, "pool for %s: %v\n", schemaDatabase, err)
		os.Exit(1)
	}
	packagePool = pool

	code := m.Run()
	pool.Close()
	dropDatabase()
	os.Exit(code)
}

func migratedPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	if packagePool == nil {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	return packagePool
}

// TestExamFormat_EveryPartCarriesTheOneSpecSchema is the WO 22 Stage I spec
// gate: every part of a sat exam carries the unified constraints with the
// fields the generator, the verifier and the composer read.
func TestExamFormat_EveryPartCarriesTheOneSpecSchema(t *testing.T) {
	pool := migratedPool(t)
	ctx := context.Background()

	const query = `
		SELECT v.code, p.section, p.part_number,
		       COALESCE(p.constraints->>'schema_version', '') AS schema_version,
		       COALESCE((p.constraints->>'option_count')::int, -1) AS option_count,
		       COALESCE((p.constraints->>'questions_per_group')::int, -1) AS questions_per_group,
		       COALESCE((p.constraints->>'plays')::int, -1) AS plays,
		       p.group_size
		FROM assess.exam_parts p
		JOIN assess.exam_versions v ON v.id = p.version_id
		WHERE v.code IN ('TOEIC_LR_2026', 'VSTEP_3_5', 'IELTS_ACADEMIC_2026_R2')
		ORDER BY v.code, p.section, p.part_number`

	rows, err := pool.Query(ctx, query)
	if err != nil {
		t.Fatalf("query exam parts: %v", err)
	}
	defer rows.Close()

	seen := 0
	for rows.Next() {
		var code, section, schemaVersion string
		var partNumber, optionCount, questionsPerGroup, plays, groupSize int
		if err := rows.Scan(&code, &section, &partNumber, &schemaVersion,
			&optionCount, &questionsPerGroup, &plays, &groupSize); err != nil {
			t.Fatalf("scan exam part: %v", err)
		}
		seen++
		where := fmt.Sprintf("%s %s part %d", code, section, partNumber)
		if schemaVersion != "1" {
			t.Errorf("%s: schema_version = %q, want 1", where, schemaVersion)
		}
		if optionCount < 0 {
			t.Errorf("%s: option_count is missing", where)
		}
		if questionsPerGroup != groupSize {
			t.Errorf("%s: questions_per_group = %d, group_size = %d", where, questionsPerGroup, groupSize)
		}
		if plays < 0 {
			t.Errorf("%s: plays is missing", where)
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate exam parts: %v", err)
	}
	if seen == 0 {
		t.Fatal("no exam parts found for the three sat exams")
	}
}

// TestExamFormat_SectionCountsMatchThePublishedFormat pins the counts a test
// composed from the spec must have, per exam.
func TestExamFormat_SectionCountsMatchThePublishedFormat(t *testing.T) {
	pool := migratedPool(t)
	ctx := context.Background()

	want := map[string]map[string]int{
		"TOEIC_LR_2026": {sectionListening: 100, sectionReading: 100},
		"VSTEP_3_5": {
			sectionListening: 35, sectionReading: 40, sectionWriting: 2, sectionSpeaking: 3,
		},
		"IELTS_ACADEMIC_2026_R2": {
			sectionListening: 40, sectionReading: 40, sectionWriting: 2, sectionSpeaking: 3,
		},
	}

	for code, sections := range want {
		rows, err := pool.Query(ctx, `
			SELECT p.section, SUM(p.question_count)::int
			FROM assess.exam_parts p
			JOIN assess.exam_versions v ON v.id = p.version_id
			WHERE v.code = $1
			GROUP BY p.section`, code)
		if err != nil {
			t.Fatalf("query %s sections: %v", code, err)
		}
		got := map[string]int{}
		for rows.Next() {
			var section string
			var count int
			if err := rows.Scan(&section, &count); err != nil {
				rows.Close()
				t.Fatalf("scan %s section: %v", code, err)
			}
			got[section] = count
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			t.Fatalf("iterate %s sections: %v", code, err)
		}
		for section, count := range sections {
			if got[section] != count {
				t.Errorf("%s %s questions = %d, want %d", code, section, got[section], count)
			}
		}
	}
}

// TestExamFormat_Listing pins D22-15 and D22-16: the sat exams are listed, the
// retired versions and the mixed-skill practice rows are not.
func TestExamFormat_Listing(t *testing.T) {
	pool := migratedPool(t)
	ctx := context.Background()

	for code, want := range map[string]bool{
		"TOEIC_LR_2026":          true,
		"VSTEP_3_5":              true,
		"IELTS_ACADEMIC_2026_R2": true,
		"IELTS_ACADEMIC_2026":    false,
		"CAMBRIDGE_B2_FIRST":     false,
		"VN_THPT_2026":           false,
		"TOEFL_IBT_2026":         false,
	} {
		var listed bool
		if err := pool.QueryRow(ctx,
			`SELECT listed FROM assess.exam_versions WHERE code = $1`, code).Scan(&listed); err != nil {
			t.Fatalf("read %s listing: %v", code, err)
		}
		if listed != want {
			t.Errorf("%s listed = %v, want %v", code, listed, want)
		}
	}

	// D22-15: the three mixed-skill practice rows are unlisted, not deleted;
	// attempts point at them.
	for _, slug := range []string{"mock-toeic-a2", "mock-toeic-b1", "mock-toeic-b2"} {
		var listed bool
		if err := pool.QueryRow(ctx,
			`SELECT listed FROM assess.exams WHERE slug = $1`, slug).Scan(&listed); err != nil {
			t.Fatalf("read %s listing: %v", slug, err)
		}
		if listed {
			t.Errorf("%s listed = true, want unlisted", slug)
		}
	}

	var blueprint bool
	if err := pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM assess.blueprints b
			JOIN assess.exam_versions v ON v.id = b.version_id
			WHERE v.code = 'IELTS_ACADEMIC_2026_R2' AND b.name = 'ielts_default'
		)`).Scan(&blueprint); err != nil {
		t.Fatalf("check ielts blueprint: %v", err)
	}
	if !blueprint {
		t.Error("ielts_default blueprint is missing for IELTS_ACADEMIC_2026_R2")
	}

	// A mock test's sitting is recorded under the exam row of its version; an
	// IELTS test with none could not be started.
	var examRow bool
	if err := pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM assess.exams e
			JOIN assess.exam_versions v ON v.id = e.version_id
			WHERE v.code = 'IELTS_ACADEMIC_2026_R2'
		)`).Scan(&examRow); err != nil {
		t.Fatalf("check ielts exam row: %v", err)
	}
	if !examRow {
		t.Error("IELTS_ACADEMIC_2026_R2 has no assess.exams row to record sittings under")
	}
}

func createDatabase(base, name string) (string, func(), error) {
	maintenance, err := replaceDatabase(base, "postgres")
	if err != nil {
		return "", nil, err
	}
	admin, err := sql.Open("pgx", maintenance)
	if err != nil {
		return "", nil, fmt.Errorf("open maintenance database: %w", err)
	}
	defer func() { _ = admin.Close() }()

	ctx := context.Background()
	drop := fmt.Sprintf("DROP DATABASE IF EXISTS %q WITH (FORCE)", name)
	if _, err := admin.ExecContext(ctx, drop); err != nil {
		return "", nil, fmt.Errorf("drop stale %s: %w", name, err)
	}
	if _, err := admin.ExecContext(ctx, fmt.Sprintf("CREATE DATABASE %q", name)); err != nil {
		return "", nil, fmt.Errorf("create %s: %w", name, err)
	}

	dsn, err := replaceDatabase(base, name)
	if err != nil {
		return "", nil, err
	}
	return dsn, func() {
		cleanup, err := sql.Open("pgx", maintenance)
		if err != nil {
			return
		}
		defer func() { _ = cleanup.Close() }()
		_, _ = cleanup.ExecContext(context.Background(), drop)
	}, nil
}

func migrateUp(dsn string) (*goose.Provider, error) {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	sources, err := migrations.Flattened()
	if err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("flatten migrations: %w", err)
	}
	provider, err := goose.NewProvider(goose.DialectPostgres, db, sources)
	if err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("create goose provider: %w", err)
	}
	if _, err := provider.Up(context.Background()); err != nil {
		_ = provider.Close()
		return nil, fmt.Errorf("apply migrations: %w", err)
	}
	return provider, nil
}

func replaceDatabase(dsn, database string) (string, error) {
	parsed, err := url.Parse(dsn)
	if err != nil {
		return "", fmt.Errorf("parse TEST_DATABASE_URL: %w", err)
	}
	parsed.Path = "/" + database
	return parsed.String(), nil
}
