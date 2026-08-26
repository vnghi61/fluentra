//go:build integration

package main

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"net/url"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"github.com/fluentra/fluentra/db/migrations"
)

const seedDatabase = "fluentra_seed_test"

var seedPool *pgxpool.Pool

func TestMain(m *testing.M) {
	base := os.Getenv("TEST_DATABASE_URL")
	if base == "" {
		os.Exit(m.Run())
	}

	dsn, dropDatabase, err := createSeedDatabase(base, seedDatabase)
	if err != nil {
		fmt.Fprintf(os.Stderr, "prepare %s: %v\n", seedDatabase, err)
		os.Exit(1)
	}
	if err := migrateSeedUp(dsn); err != nil {
		dropDatabase()
		fmt.Fprintf(os.Stderr, "migrate %s: %v\n", seedDatabase, err)
		os.Exit(1)
	}

	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		dropDatabase()
		fmt.Fprintf(os.Stderr, "pool for %s: %v\n", seedDatabase, err)
		os.Exit(1)
	}
	seedPool = pool

	code := m.Run()

	pool.Close()
	dropDatabase()
	os.Exit(code)
}

func createSeedDatabase(base, name string) (string, func(), error) {
	maintenance, err := replaceSeedDatabase(base, "postgres")
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

	dsn, err := replaceSeedDatabase(base, name)
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

func migrateSeedUp(dsn string) error {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	sources, err := migrations.Flattened()
	if err != nil {
		_ = db.Close()
		return fmt.Errorf("flatten migrations: %w", err)
	}
	provider, err := goose.NewProvider(goose.DialectPostgres, db, sources)
	if err != nil {
		_ = db.Close()
		return fmt.Errorf("create goose provider: %w", err)
	}
	defer func() { _ = provider.Close() }()

	if _, err := provider.Up(context.Background()); err != nil {
		return fmt.Errorf("apply migrations: %w", err)
	}
	return nil
}

func replaceSeedDatabase(dsn, database string) (string, error) {
	parsed, err := url.Parse(dsn)
	if err != nil {
		return "", fmt.Errorf("parse TEST_DATABASE_URL: %w", err)
	}
	parsed.Path = "/" + database
	return parsed.String(), nil
}

// runSeed seeds an admin and the whole curriculum into the test database.
func runSeed(ctx context.Context, t *testing.T) uuid.UUID {
	t.Helper()
	if seedPool == nil {
		t.Skip("TEST_DATABASE_URL is not set")
	}

	var adminID uuid.UUID
	const insertAdmin = `
		INSERT INTO core.users (email, status)
		VALUES ('seed-admin@example.com', 'active')
		ON CONFLICT (email) DO UPDATE SET status = 'active'
		RETURNING id`
	if err := seedPool.QueryRow(ctx, insertAdmin).Scan(&adminID); err != nil {
		t.Fatalf("seed admin: %v", err)
	}

	if err := seedContentAndCurriculum(ctx, seedPool, adminID, io.Discard); err != nil {
		t.Fatalf("seed content: %v", err)
	}
	return adminID
}

// TestSeededContentReachesAPIProducibleState is the check P11 §3 asks for.
//
// The seed writes SQL rather than driving the authoring API, which is allowed
// only if the rows it produces are a state the API could also have produced —
// "and something should check that claim rather than assert it". This is that
// something.
//
// The failure it guards against is not a broken seed. It is a seed that looks
// fine until the first real author hits a transition the code has never seen,
// and the bug appears to be in the workflow.
func TestSeededContentReachesAPIProducibleState(t *testing.T) {
	ctx := context.Background()
	runSeed(ctx, t)

	t.Run("every published version was approved by someone", func(t *testing.T) {
		const unreviewed = `
			SELECT count(*)
			FROM content.content_versions v
			LEFT JOIN content.content_reviews r
			       ON r.version_id = v.id AND r.decision = 'approved'
			WHERE v.status = 'published' AND r.id IS NULL`
		var count int
		if err := seedPool.QueryRow(ctx, unreviewed).Scan(&count); err != nil {
			t.Fatalf("count unreviewed published versions: %v", err)
		}
		if count != 0 {
			t.Errorf("%d published versions have no approval; the API cannot publish without one", count)
		}
	})

	t.Run("every published version has a publish timestamp", func(t *testing.T) {
		// ck_content_versions_published_at enforces it, so this is really a check
		// that the constraint was not worked around.
		var count int
		const missing = `
			SELECT count(*) FROM content.content_versions
			WHERE status = 'published' AND published_at IS NULL`
		if err := seedPool.QueryRow(ctx, missing).Scan(&count); err != nil {
			t.Fatalf("count versions without published_at: %v", err)
		}
		if count != 0 {
			t.Errorf("%d published versions have no published_at", count)
		}
	})

	t.Run("every published item points at a published version", func(t *testing.T) {
		const dangling = `
			SELECT count(*)
			FROM content.content_items i
			LEFT JOIN content.content_versions v ON v.id = i.current_version_id
			WHERE i.status = 'published'
			  AND (v.id IS NULL OR v.status <> 'published')`
		var count int
		if err := seedPool.QueryRow(ctx, dangling).Scan(&count); err != nil {
			t.Fatalf("count dangling items: %v", err)
		}
		if count != 0 {
			t.Errorf("%d published items do not point at a published version", count)
		}
	})

	t.Run("no lesson is published ahead of its content", func(t *testing.T) {
		// The publish gate P7.3 added: a lesson may not go live while an activity
		// it contains still points at unpublished content.
		const early = `
			SELECT count(*)
			FROM learn.lessons l
			JOIN learn.activities a ON a.lesson_id = l.id
			JOIN content.content_versions v ON v.id = a.content_version_id
			WHERE l.status = 'published' AND v.status <> 'published'`
		var count int
		if err := seedPool.QueryRow(ctx, early).Scan(&count); err != nil {
			t.Fatalf("count early-published lessons: %v", err)
		}
		if count != 0 {
			t.Errorf("%d published lessons contain unpublished content", count)
		}
	})

	t.Run("every activity points at content that exists", func(t *testing.T) {
		// activities.content_version_id carries no foreign key by DB4, so nothing
		// in the schema catches a seed that writes a version id it never created.
		const orphaned = `
			SELECT count(*)
			FROM learn.activities a
			LEFT JOIN content.content_versions v ON v.id = a.content_version_id
			WHERE v.id IS NULL`
		var count int
		if err := seedPool.QueryRow(ctx, orphaned).Scan(&count); err != nil {
			t.Fatalf("count orphaned activities: %v", err)
		}
		if count != 0 {
			t.Errorf("%d activities point at a content version that does not exist", count)
		}
	})
}

// TestSeedIsIdempotent is the P11.1 acceptance: re-running changes nothing.
func TestSeedIsIdempotent(t *testing.T) {
	ctx := context.Background()
	adminID := runSeed(ctx, t)

	before := countCurriculum(ctx, t)

	if err := seedContentAndCurriculum(ctx, seedPool, adminID, io.Discard); err != nil {
		t.Fatalf("second seed run: %v", err)
	}

	after := countCurriculum(ctx, t)
	for table, count := range before {
		if after[table] != count {
			t.Errorf("%s went from %d to %d rows on a second seed run", table, count, after[table])
		}
	}
}

func countCurriculum(ctx context.Context, t *testing.T) map[string]int {
	t.Helper()

	tables := []string{
		"learn.courses", "learn.course_units", "learn.lessons", "learn.activities",
		"content.content_items", "content.content_versions", "content.content_reviews",
		"skill.words", "skill.word_senses", "skill.decks", "skill.deck_items",
	}
	counts := make(map[string]int, len(tables))
	for _, table := range tables {
		var count int
		if err := seedPool.QueryRow(ctx, "SELECT count(*) FROM "+table).Scan(&count); err != nil {
			t.Fatalf("count %s: %v", table, err)
		}
		counts[table] = count
	}
	return counts
}

// TestSeededCuratedDeckIsVisibleToLearners: a deck owned by the admin is curated
// in name only. ListDecksByUser shows a learner their own decks plus the ones
// with no owner, so an owned "curated" deck is visible to exactly one person.
func TestSeededCuratedDeckIsVisibleToLearners(t *testing.T) {
	ctx := context.Background()
	runSeed(ctx, t)

	var learnerID uuid.UUID
	const insertLearner = `
		INSERT INTO core.users (email, status) VALUES ('seed-learner@example.com', 'active')
		ON CONFLICT (email) DO UPDATE SET status = 'active'
		RETURNING id`
	if err := seedPool.QueryRow(ctx, insertLearner).Scan(&learnerID); err != nil {
		t.Fatalf("seed learner: %v", err)
	}

	// The predicate ListDecksByUser uses.
	const visible = `
		SELECT count(*) FROM skill.decks d
		WHERE (d.owner_id = $1 OR (d.owner_id IS NULL AND d.is_public = true))
		  AND d.slug = 'a2-b1-essentials'`
	var count int
	if err := seedPool.QueryRow(ctx, visible, learnerID).Scan(&count); err != nil {
		t.Fatalf("count visible decks: %v", err)
	}
	if count != 1 {
		t.Errorf("a learner sees %d curated decks, want 1", count)
	}
}

// TestSeededWordSensesCarryWhatTheFlashcardRenders: P10.4 renders the sense, and
// P11.1's acceptance is that a learner can complete the course end to end.
func TestSeededWordSensesCarryWhatTheFlashcardRenders(t *testing.T) {
	ctx := context.Background()
	runSeed(ctx, t)

	var senses, withIPA, withExamples, withContent int
	const query = `
		SELECT count(*),
		       count(*) FILTER (WHERE w.ipa IS NOT NULL AND w.ipa <> ''),
		       count(*) FILTER (WHERE jsonb_array_length(s.examples) > 0),
		       count(*) FILTER (WHERE s.content_version_id IS NOT NULL)
		FROM skill.word_senses s
		JOIN skill.words w ON w.id = s.word_id`
	if err := seedPool.QueryRow(ctx, query).Scan(&senses, &withIPA, &withExamples, &withContent); err != nil {
		t.Fatalf("inspect seeded senses: %v", err)
	}

	if senses < 200 {
		t.Errorf("seeded %d word senses, want the 200 P11.1 asks for", senses)
	}
	if withIPA != senses {
		t.Errorf("%d of %d senses have no IPA", senses-withIPA, senses)
	}
	if withExamples != senses {
		t.Errorf("%d of %d senses have no examples", senses-withExamples, senses)
	}
	if withContent != senses {
		t.Errorf("%d of %d senses resolve no content version", senses-withContent, senses)
	}
}
