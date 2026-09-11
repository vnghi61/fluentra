//go:build integration

package vocabulary_test

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"github.com/fluentra/fluentra/db/migrations"
	"github.com/fluentra/fluentra/internal/modules/content"
	contentcontract "github.com/fluentra/fluentra/internal/modules/content/contract"
	"github.com/fluentra/fluentra/internal/modules/lesson"
	"github.com/fluentra/fluentra/internal/modules/vocabulary/repository"
	"github.com/fluentra/fluentra/internal/modules/vocabulary/service"
	"github.com/fluentra/fluentra/internal/shared/clock"
)

const generatorTestDatabase = "fluentra_vocabulary_generator_test"

var generatorPool *pgxpool.Pool

func TestMain(m *testing.M) {
	base := os.Getenv("TEST_DATABASE_URL")
	if base == "" {
		os.Exit(m.Run())
	}

	dsn, dropDatabase, err := createDatabase(base, generatorTestDatabase)
	if err != nil {
		fmt.Fprintf(os.Stderr, "prepare %s: %v\n", generatorTestDatabase, err)
		os.Exit(1)
	}
	if err := migrateUp(dsn); err != nil {
		dropDatabase()
		fmt.Fprintf(os.Stderr, "migrate %s: %v\n", generatorTestDatabase, err)
		os.Exit(1)
	}

	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		dropDatabase()
		fmt.Fprintf(os.Stderr, "pool for %s: %v\n", generatorTestDatabase, err)
		os.Exit(1)
	}
	generatorPool = pool

	code := m.Run()
	pool.Close()
	dropDatabase()
	os.Exit(code)
}

func createDatabase(baseURL, name string) (string, func(), error) {
	parsed, err := url.Parse(baseURL)
	if err != nil {
		return "", nil, fmt.Errorf("parse TEST_DATABASE_URL: %w", err)
	}
	target := *parsed
	target.Path = "/" + name

	maintenance := *parsed
	maintenance.Path = "/postgres"

	admin, err := sql.Open("pgx", maintenance.String())
	if err != nil {
		return "", nil, fmt.Errorf("open maintenance db: %w", err)
	}
	defer func() { _ = admin.Close() }()

	_, _ = admin.Exec(fmt.Sprintf("DROP DATABASE IF EXISTS %s WITH (FORCE)", name))
	if _, err := admin.Exec(fmt.Sprintf("CREATE DATABASE %s", name)); err != nil {
		return "", nil, fmt.Errorf("create database %s: %w", name, err)
	}

	drop := func() {
		cleanup, err := sql.Open("pgx", maintenance.String())
		if err != nil {
			return
		}
		defer func() { _ = cleanup.Close() }()
		_, _ = cleanup.Exec(fmt.Sprintf("DROP DATABASE IF EXISTS %s WITH (FORCE)", name))
	}

	return target.String(), drop, nil
}

func migrateUp(dsn string) error {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()

	sources, err := migrations.Flattened()
	if err != nil {
		return err
	}
	provider, err := goose.NewProvider(goose.DialectPostgres, db, sources)
	if err != nil {
		return err
	}
	_, err = provider.Up(context.Background())
	return err
}

type workerGuard struct{}

func (workerGuard) Require(_ context.Context, _ string) error { return nil }

func TestGenerator_AttemptsSurviveGeneratorRuns_Integration(t *testing.T) {
	if generatorPool == nil {
		t.Skip("TEST_DATABASE_URL is not set")
	}

	ctx := context.Background()

	// 1. Seed author user
	authorID := uuid.New()
	if _, err := generatorPool.Exec(ctx,
		`INSERT INTO core.users (id, email, status) VALUES ($1, $2, 'active')`,
		authorID, fmt.Sprintf("author-%s@example.com", authorID),
	); err != nil {
		t.Fatalf("seed author: %v", err)
	}

	// 2. Wire content and lesson modules
	contentMod := content.NewAuthoring(content.Deps{
		Pool:  generatorPool,
		Clock: clock.Real{},
	})
	lessonMod := lesson.New(lesson.Deps{
		Pool:  generatorPool,
		Clock: clock.Real{},
		Guard: workerGuard{},
	})

	seedDictionarySenses(ctx, t, authorID, contentMod)

	repo := repository.New(generatorPool)
	gen := service.NewGenerator(repo, service.GeneratorDeps{
		Content:  contentMod.Author(),
		Lessons:  lessonMod.Author(),
		AuthorID: authorID,
	})

	// 3. First generator run
	if err := gen.GenerateExercises(ctx); err != nil {
		t.Fatalf("first generator run: %v", err)
	}

	// Query created activities
	var initialActivityID uuid.UUID
	var lessonID uuid.UUID
	if err := generatorPool.QueryRow(ctx,
		`SELECT id, lesson_id FROM learn.activities WHERE position = 1 LIMIT 1`,
	).Scan(&initialActivityID, &lessonID); err != nil {
		t.Fatalf("find generated activity: %v", err)
	}

	// 4. Learner attempts the generated activity
	attemptID := seedLearnerAttempt(ctx, t, initialActivityID)

	// Verify attempt exists
	var countBefore int
	if err := generatorPool.QueryRow(ctx,
		`SELECT count(*) FROM learn.attempts WHERE id = $1`, attemptID,
	).Scan(&countBefore); err != nil || countBefore != 1 {
		t.Fatalf("attempt not created properly: %v", err)
	}

	// 6. Second generator run (simulating 12 hours later)
	if err := gen.GenerateExercises(ctx); err != nil {
		t.Fatalf("second generator run: %v", err)
	}

	// 7. Check if attempt survived
	var countAfter int
	if err := generatorPool.QueryRow(ctx,
		`SELECT count(*) FROM learn.attempts WHERE id = $1`, attemptID,
	).Scan(&countAfter); err != nil {
		t.Fatalf("check attempt survival: %v", err)
	}

	if countAfter != 1 {
		t.Fatalf("learner attempt did not survive generator run! count=%d", countAfter)
	}

	// 8. Third generator run (simulating another 12 hours later)
	if err := gen.GenerateExercises(ctx); err != nil {
		t.Fatalf("third generator run: %v", err)
	}

	var countAfterThird int
	if err := generatorPool.QueryRow(ctx,
		`SELECT count(*) FROM learn.attempts WHERE id = $1`, attemptID,
	).Scan(&countAfterThird); err != nil || countAfterThird != 1 {
		t.Fatalf("learner attempt did not survive two generator runs! count=%d", countAfterThird)
	}

	// Check that activity ID was updated in place and preserved across both runs
	var newActivityID uuid.UUID
	if err := generatorPool.QueryRow(ctx,
		`SELECT id FROM learn.activities WHERE lesson_id = $1 AND position = 1`, lessonID,
	).Scan(&newActivityID); err != nil {
		t.Fatalf("read activity at pos 1: %v", err)
	}

	if newActivityID != initialActivityID {
		t.Fatalf("activity ID changed from %s to %s, want in-place update", initialActivityID, newActivityID)
	}
}

func seedDictionarySenses(ctx context.Context, t *testing.T, authorID uuid.UUID, contentMod *content.Module) {
	t.Helper()
	for i := 1; i <= 4; i++ {
		wordID := uuid.New()
		lemma := fmt.Sprintf("word%d", i)
		if _, err := generatorPool.Exec(ctx,
			`INSERT INTO skill.words (id, lemma, pos, cefr_level, ipa, frequency_rank)
			 VALUES ($1, $2, 'noun', 'A1', '/w/', $3)`,
			wordID, lemma, i,
		); err != nil {
			t.Fatalf("seed word: %v", err)
		}

		contentVerID, err := contentMod.Author().EnsurePublished(ctx, contentcontract.AuthorSpec{
			Slug:      fmt.Sprintf("vocab-def-%s", lemma),
			Kind:      "vocab_definition",
			CEFRLevel: "A1",
			Body:      []byte(fmt.Sprintf(`{"word":"%s"}`, lemma)),
			AuthorID:  authorID,
		})
		if err != nil {
			t.Fatalf("publish sense content: %v", err)
		}

		senseID := uuid.New()
		if _, err := generatorPool.Exec(ctx,
			`INSERT INTO skill.word_senses (id, word_id, definition, definition_vi, examples, content_version_id)
			 VALUES ($1, $2, $3, $4, '[]'::jsonb, $5)`,
			senseID, wordID, fmt.Sprintf("Definition of %s", lemma), fmt.Sprintf("Nghia cua %s", lemma), contentVerID,
		); err != nil {
			t.Fatalf("seed sense: %v", err)
		}
	}
}

func seedLearnerAttempt(ctx context.Context, t *testing.T, activityID uuid.UUID) uuid.UUID {
	t.Helper()
	learnerID := uuid.New()
	if _, err := generatorPool.Exec(ctx,
		`INSERT INTO core.users (id, email, status) VALUES ($1, $2, 'active')`,
		learnerID, fmt.Sprintf("learner-%s@example.com", learnerID),
	); err != nil {
		t.Fatalf("seed learner: %v", err)
	}

	attemptID := uuid.New()
	if _, err := generatorPool.Exec(ctx,
		`INSERT INTO learn.attempts (id, created_at, user_id, activity_id, status, score, max_score)
		 VALUES ($1, now(), $2, $3, 'graded', 100, 100)`,
		attemptID, learnerID, activityID,
	); err != nil {
		t.Fatalf("create attempt: %v", err)
	}
	return attemptID
}
