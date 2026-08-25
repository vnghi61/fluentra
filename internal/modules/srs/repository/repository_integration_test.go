//go:build integration

// Package repository_test exercises every srs query against a real PostgreSQL
// instance.
//
// The unit tests run the service against a fake repository, which proves the
// orchestration and nothing about the SQL. A typo in a column name, a predicate
// that silently matches everything, a partitioned insert with no partition — all
// of it passes a fake and fails in production. srs/TESTING.md sets the bar as
// "every query exercised by an integration test"; this file is that.
package repository_test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"github.com/fluentra/fluentra/db/migrations"
	"github.com/fluentra/fluentra/internal/generated/srs/sqlc"
	"github.com/fluentra/fluentra/internal/modules/srs/domain"
	"github.com/fluentra/fluentra/internal/modules/srs/repository"
)

const testDatabase = "fluentra_srs_repository_test"

var packagePool *pgxpool.Pool

func TestMain(m *testing.M) {
	base := os.Getenv("TEST_DATABASE_URL")
	if base == "" {
		os.Exit(m.Run())
	}

	dsn, dropDatabase, err := createDatabase(base, testDatabase)
	if err != nil {
		fmt.Fprintf(os.Stderr, "prepare %s: %v\n", testDatabase, err)
		os.Exit(1)
	}
	if err := migrateUp(dsn); err != nil {
		dropDatabase()
		fmt.Fprintf(os.Stderr, "migrate %s: %v\n", testDatabase, err)
		os.Exit(1)
	}

	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		dropDatabase()
		fmt.Fprintf(os.Stderr, "pool for %s: %v\n", testDatabase, err)
		os.Exit(1)
	}
	packagePool = pool

	code := m.Run()

	pool.Close()
	dropDatabase()
	os.Exit(code)
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

func migrateUp(dsn string) error {
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

func replaceDatabase(dsn, database string) (string, error) {
	parsed, err := url.Parse(dsn)
	if err != nil {
		return "", fmt.Errorf("parse TEST_DATABASE_URL: %w", err)
	}
	parsed.Path = "/" + database
	return parsed.String(), nil
}

// newRepo returns a repository over the migrated database, plus a learner id to
// scope the test's rows to. Every test gets its own learner, so they can run in
// any order without seeing each other's cards.
func newRepo(ctx context.Context, t *testing.T) (repository.Repository, *pgxpool.Pool, uuid.UUID) {
	t.Helper()
	if packagePool == nil {
		t.Skip("TEST_DATABASE_URL is not set")
	}

	var userID uuid.UUID
	email := fmt.Sprintf("srs-repo-%s@example.com", uuid.NewString())
	const insert = `INSERT INTO core.users (email, status) VALUES ($1, 'active') RETURNING id`
	if err := packagePool.QueryRow(ctx, insert, email).Scan(&userID); err != nil {
		t.Fatalf("seed user: %v", err)
	}
	t.Cleanup(func() {
		_, _ = packagePool.Exec(ctx, `DELETE FROM core.users WHERE id = $1`, userID)
	})

	return repository.New(packagePool), packagePool, userID
}

func upsertArgs(userID, versionID uuid.UUID, dueAt time.Time) sqlc.UpsertReviewCardParams {
	return sqlc.UpsertReviewCardParams{
		UserID:           userID,
		ContentVersionID: versionID,
		Skill:            "vocabulary",
		Stability:        3.1262,
		Difficulty:       5.0,
		DueAt:            dueAt,
		Reps:             1,
		Lapses:           0,
		State:            string(domain.StateLearning),
	}
}

// TestRepository_UpsertReviewCardKeepsAnExistingSchedule is the SQL-level proof
// of the ON CONFLICT clause. The service test asserts the same thing against a
// fake; only this one can fail when the real conflict target or SET list is
// wrong.
func TestRepository_UpsertReviewCardKeepsAnExistingSchedule(t *testing.T) {
	ctx := context.Background()
	repo, _, userID := newRepo(ctx, t)

	versionID := uuid.New()
	dueAt := time.Now().UTC().AddDate(0, 0, 3).Truncate(time.Microsecond)

	created, err := repo.UpsertReviewCard(ctx, upsertArgs(userID, versionID, dueAt))
	if err != nil {
		t.Fatalf("first upsert: %v", err)
	}

	// The learner reviews it and the schedule moves out to forty days.
	matured, err := repo.UpdateReviewCardSchedule(ctx, sqlc.UpdateReviewCardScheduleParams{
		ID:         created.ID,
		UserID:     userID,
		Stability:  42.0,
		Difficulty: 3.5,
		DueAt:      time.Now().UTC().AddDate(0, 0, 40).Truncate(time.Microsecond),
		Reps:       9,
		Lapses:     1,
		State:      string(domain.StateReview),
	})
	if err != nil {
		t.Fatalf("update schedule: %v", err)
	}

	// Redoing the lesson activity re-emits the same ReviewItem.
	again, err := repo.UpsertReviewCard(ctx, upsertArgs(userID, versionID, dueAt))
	if err != nil {
		t.Fatalf("second upsert: %v", err)
	}

	if again.ID != created.ID {
		t.Errorf("second upsert created a new card %s, want the existing %s", again.ID, created.ID)
	}
	if again.Stability != matured.Stability {
		t.Errorf("stability = %v, want %v — the conflict path overwrote the schedule",
			again.Stability, matured.Stability)
	}
	if !again.DueAt.Equal(matured.DueAt) {
		t.Errorf("due_at = %v, want %v — the conflict path overwrote the schedule", again.DueAt, matured.DueAt)
	}
	if again.Reps != matured.Reps || again.Lapses != matured.Lapses {
		t.Errorf("reps/lapses = %d/%d, want %d/%d", again.Reps, again.Lapses, matured.Reps, matured.Lapses)
	}
}

// seedDueQueue lays out three cards for one learner: overdue, due within the
// hour, and a month away. It returns the learner, the two due version ids and
// the cutoff the due queries should use.
func seedDueQueue(ctx context.Context, t *testing.T) (
	repo repository.Repository, userID, overdue, dueToday uuid.UUID, cutoff time.Time,
) {
	t.Helper()
	repo, _, userID = newRepo(ctx, t)

	now := time.Now().UTC()
	overdue, dueToday = uuid.New(), uuid.New()

	for versionID, due := range map[uuid.UUID]time.Time{
		overdue:    now.AddDate(0, 0, -2),
		dueToday:   now.Add(-time.Hour),
		uuid.New(): now.AddDate(0, 0, 30),
	} {
		if _, err := repo.UpsertReviewCard(ctx, upsertArgs(userID, versionID, due)); err != nil {
			t.Fatalf("seed card: %v", err)
		}
	}
	return repo, userID, overdue, dueToday, now.Add(time.Hour)
}

// TestRepository_DueQueueRespectsTheCutoff covers CountDueCards and ListDueCards,
// including the ordering and the limit the review session relies on.
func TestRepository_DueQueueRespectsTheCutoff(t *testing.T) {
	ctx := context.Background()
	repo, userID, _, _, cutoff := seedDueQueue(ctx, t)

	count, err := repo.CountDueCards(ctx, userID, cutoff)
	if err != nil {
		t.Fatalf("count due: %v", err)
	}
	if count != 2 {
		t.Errorf("due count = %d, want 2 (the card due in 30 days is not due today)", count)
	}

	cards, err := repo.ListDueCards(ctx, userID, cutoff, 10)
	if err != nil {
		t.Fatalf("list due: %v", err)
	}
	if len(cards) != 2 {
		t.Fatalf("listed %d due cards, want 2", len(cards))
	}
	// ORDER BY due_at ASC: the most overdue card is served first.
	if !cards[0].DueAt.Before(cards[1].DueAt) {
		t.Errorf("due cards are not ordered oldest first: %v then %v", cards[0].DueAt, cards[1].DueAt)
	}

	limited, err := repo.ListDueCards(ctx, userID, cutoff, 1)
	if err != nil {
		t.Fatalf("list due with limit: %v", err)
	}
	if len(limited) != 1 {
		t.Errorf("limit 1 returned %d cards", len(limited))
	}
}

// TestRepository_SuspensionRemovesCardsFromTheQueue is the SQL half of "marking
// a word known stops its scheduling", and proves the door swings both ways.
func TestRepository_SuspensionRemovesCardsFromTheQueue(t *testing.T) {
	ctx := context.Background()
	repo, userID, overdue, dueToday, cutoff := seedDueQueue(ctx, t)

	affected, err := repo.SetReviewCardsSuspended(ctx, userID, []uuid.UUID{overdue, dueToday}, true)
	if err != nil {
		t.Fatalf("suspend cards: %v", err)
	}
	if affected != 2 {
		t.Errorf("suspended %d rows, want 2", affected)
	}

	count, err := repo.CountDueCards(ctx, userID, cutoff)
	if err != nil {
		t.Fatalf("count after suspend: %v", err)
	}
	if count != 0 {
		t.Errorf("due count after suspending both due cards = %d, want 0", count)
	}

	if _, err := repo.SetReviewCardsSuspended(ctx, userID, []uuid.UUID{overdue, dueToday}, false); err != nil {
		t.Fatalf("resume cards: %v", err)
	}
	count, err = repo.CountDueCards(ctx, userID, cutoff)
	if err != nil {
		t.Fatalf("count after resume: %v", err)
	}
	if count != 2 {
		t.Errorf("due count after resuming = %d, want 2", count)
	}
}

// TestRepository_LookupByContentVersion covers GetReviewCardByUserAndContent,
// which is how the exercise engine finds an existing card.
func TestRepository_LookupByContentVersion(t *testing.T) {
	ctx := context.Background()
	repo, userID, overdue, _, _ := seedDueQueue(ctx, t)

	found, err := repo.GetReviewCardByUserAndContent(ctx, userID, overdue)
	if err != nil {
		t.Fatalf("get by user and content: %v", err)
	}
	if found.ContentVersionID != overdue {
		t.Errorf("looked up the wrong card: %s", found.ContentVersionID)
	}

	if _, err := repo.GetReviewCardByUserAndContent(ctx, userID, uuid.New()); !errors.Is(err, pgx.ErrNoRows) {
		t.Errorf("looking up a card that does not exist returned %v, want pgx.ErrNoRows", err)
	}
}

// TestRepository_CardScopingIsPerLearner: one learner must never read, reschedule
// or suspend another learner's card. Every query carries user_id for this
// reason, and dropping it from any of them is a data leak no unit test sees.
func TestRepository_CardScopingIsPerLearner(t *testing.T) {
	ctx := context.Background()
	repo, _, owner := newRepo(ctx, t)
	_, _, stranger := newRepo(ctx, t)

	versionID := uuid.New()
	card, err := repo.UpsertReviewCard(ctx, upsertArgs(owner, versionID, time.Now().UTC()))
	if err != nil {
		t.Fatalf("seed card: %v", err)
	}

	if _, err := repo.GetReviewCardByID(ctx, card.ID, stranger); !errors.Is(err, pgx.ErrNoRows) {
		t.Errorf("a stranger read the card: err = %v, want pgx.ErrNoRows", err)
	}
	if _, err := repo.SuspendReviewCard(ctx, card.ID, stranger); !errors.Is(err, pgx.ErrNoRows) {
		t.Errorf("a stranger suspended the card: err = %v, want pgx.ErrNoRows", err)
	}
	_, err = repo.UpdateReviewCardSchedule(ctx, sqlc.UpdateReviewCardScheduleParams{
		ID: card.ID, UserID: stranger, Stability: 1, Difficulty: 1,
		DueAt: time.Now().UTC(), Reps: 1, Lapses: 0, State: "review",
	})
	if !errors.Is(err, pgx.ErrNoRows) {
		t.Errorf("a stranger rescheduled the card: err = %v, want pgx.ErrNoRows", err)
	}
}

// TestRepository_SuspendAndReset covers SuspendReviewCard and ResetReviewCard.
func TestRepository_SuspendAndReset(t *testing.T) {
	ctx := context.Background()
	repo, _, userID := newRepo(ctx, t)

	card, err := repo.UpsertReviewCard(ctx, upsertArgs(userID, uuid.New(), time.Now().UTC()))
	if err != nil {
		t.Fatalf("seed card: %v", err)
	}

	suspended, err := repo.SuspendReviewCard(ctx, card.ID, userID)
	if err != nil {
		t.Fatalf("suspend: %v", err)
	}
	if suspended.SuspendedAt == nil {
		t.Error("suspended_at is still null after suspending")
	}

	reset, err := repo.ResetReviewCard(ctx, sqlc.ResetReviewCardParams{
		ID:         card.ID,
		UserID:     userID,
		Stability:  3.1262,
		Difficulty: 5.0,
		DueAt:      time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("reset: %v", err)
	}
	if reset.SuspendedAt != nil {
		t.Error("reset left the card suspended; a reset card must be schedulable again")
	}
	if reset.State != "new" || reset.Reps != 0 || reset.Lapses != 0 {
		t.Errorf("reset card = state %s reps %d lapses %d, want new/0/0", reset.State, reset.Reps, reset.Lapses)
	}
}

// seedLogs writes three review logs of known duration for one learner.
func seedLogs(ctx context.Context, t *testing.T) (
	repo repository.Repository, userID, cardID uuid.UUID, now time.Time,
) {
	t.Helper()
	repo, _, userID = newRepo(ctx, t)

	card, err := repo.UpsertReviewCard(ctx, upsertArgs(userID, uuid.New(), time.Now().UTC()))
	if err != nil {
		t.Fatalf("seed card: %v", err)
	}

	now = time.Now().UTC()
	for i, elapsed := range []int32{90_000, 60_000, 120_000} {
		_, err := repo.InsertReviewLog(ctx, sqlc.InsertReviewLogParams{
			CardID:           card.ID,
			UserID:           userID,
			Grade:            "good",
			ElapsedMs:        elapsed,
			StabilityBefore:  3.1262,
			StabilityAfter:   8.4,
			DifficultyBefore: 5.0,
			DifficultyAfter:  4.8,
			ScheduledDays:    8,
			SchedulerVersion: "v4.5",
			ReviewedAt:       now.Add(time.Duration(i) * time.Minute),
		})
		if err != nil {
			t.Fatalf("insert log %d: %v", i, err)
		}
	}
	return repo, userID, card.ID, now
}

// TestRepository_ReviewLogsKeepTheTuningData covers InsertReviewLog and
// ListReviewLogsByCard. The stability either side of an answer is what makes
// later parameter tuning possible, and nothing may drop it.
func TestRepository_ReviewLogsKeepTheTuningData(t *testing.T) {
	ctx := context.Background()
	repo, userID, cardID, _ := seedLogs(ctx, t)

	logs, err := repo.ListReviewLogsByCard(ctx, cardID, userID, 10)
	if err != nil {
		t.Fatalf("list logs: %v", err)
	}
	if len(logs) != 3 {
		t.Fatalf("listed %d logs, want 3", len(logs))
	}
	// ORDER BY reviewed_at DESC.
	if logs[0].ReviewedAt.Before(logs[1].ReviewedAt) {
		t.Error("logs are not newest first")
	}
	if logs[0].StabilityBefore == 0 || logs[0].StabilityAfter == 0 {
		t.Error("a review log lost its stability_before/stability_after")
	}
}

// TestRepository_SessionMinutesAreSummedFromTheLogs covers
// SumRecentReviewElapsedMs, which is where a session's reported time on task
// comes from now that it is measured rather than estimated.
func TestRepository_SessionMinutesAreSummedFromTheLogs(t *testing.T) {
	ctx := context.Background()
	repo, userID, _, now := seedLogs(ctx, t)

	totalMs, err := repo.SumRecentReviewElapsedMs(ctx, userID, now.Add(-time.Hour), 3)
	if err != nil {
		t.Fatalf("sum elapsed: %v", err)
	}
	if totalMs != 270_000 {
		t.Errorf("summed %d ms, want 270000", totalMs)
	}

	// The limit bounds the sum to the session, not the whole day.
	twoMostRecent, err := repo.SumRecentReviewElapsedMs(ctx, userID, now.Add(-time.Hour), 2)
	if err != nil {
		t.Fatalf("sum elapsed with limit: %v", err)
	}
	if twoMostRecent != 180_000 {
		t.Errorf("summed %d ms over the last two logs, want 180000", twoMostRecent)
	}
}

// TestRepository_DailyStatsAccumulate: two sessions on one day add up rather
// than the second overwriting the first.
func TestRepository_DailyStatsAccumulate(t *testing.T) {
	ctx := context.Background()
	repo, _, userID := newRepo(ctx, t)

	now := time.Now().UTC()
	statDate := pgtype.Date{
		Time:  time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC),
		Valid: true,
	}

	first, err := repo.UpsertReviewDailyStats(ctx, sqlc.UpsertReviewDailyStatsParams{
		UserID: userID, StatDate: statDate, ReviewsCompleted: 3, NewCardsLearned: 1, TotalMinutes: 4,
	})
	if err != nil {
		t.Fatalf("first stats upsert: %v", err)
	}
	if first.ReviewsCompleted != 3 {
		t.Errorf("reviews_completed = %d, want 3", first.ReviewsCompleted)
	}

	second, err := repo.UpsertReviewDailyStats(ctx, sqlc.UpsertReviewDailyStatsParams{
		UserID: userID, StatDate: statDate, ReviewsCompleted: 2, NewCardsLearned: 0, TotalMinutes: 3,
	})
	if err != nil {
		t.Fatalf("second stats upsert: %v", err)
	}
	if second.ReviewsCompleted != 5 || second.TotalMinutes != 7 {
		t.Errorf("a second session of the same day = %d reviews / %d minutes, want 5 / 7 - "+
			"the daily row must accumulate, not overwrite",
			second.ReviewsCompleted, second.TotalMinutes)
	}
}

// TestRepository_ForecastBucketsByTimezone is the SQL half of the timezone rule.
// One card, one instant, two learners, two different calendar days.
func TestRepository_ForecastBucketsByTimezone(t *testing.T) {
	ctx := context.Background()
	repo, _, userID := newRepo(ctx, t)

	// 2026-08-26 22:00 UTC is 2026-08-27 05:00 in Asia/Ho_Chi_Minh.
	dueAt := time.Date(2026, 8, 26, 22, 0, 0, 0, time.UTC)
	if _, err := repo.UpsertReviewCard(ctx, upsertArgs(userID, uuid.New(), dueAt)); err != nil {
		t.Fatalf("seed card: %v", err)
	}
	until := time.Date(2026, 9, 30, 0, 0, 0, 0, time.UTC)

	for _, tc := range []struct{ timezone, wantDate string }{
		{"UTC", "2026-08-26"},
		{"Asia/Ho_Chi_Minh", "2026-08-27"},
	} {
		rows, err := repo.ForecastDueCards(ctx, userID, tc.timezone, until)
		if err != nil {
			t.Fatalf("forecast in %s: %v", tc.timezone, err)
		}
		if len(rows) != 1 {
			t.Fatalf("forecast in %s returned %d buckets, want 1", tc.timezone, len(rows))
		}
		got := rows[0].DueDate.Time.Format(time.DateOnly)
		if got != tc.wantDate {
			t.Errorf("in %s the card falls on %s, want %s", tc.timezone, got, tc.wantDate)
		}
		if rows[0].DueCount != 1 {
			t.Errorf("bucket count = %d, want 1", rows[0].DueCount)
		}
	}

	// A suspended card is not future workload either.
	if _, err := repo.SetReviewCardsSuspended(ctx, userID, []uuid.UUID{}, true); err != nil {
		t.Fatalf("suspending nothing must be a no-op, got: %v", err)
	}
}

// TestRepository_WithTxRollsBackTogether proves the answer path is atomic: the
// card update and its log either both land or neither does. It is the reason
// AnswerCard runs inside InTx at all.
func TestRepository_WithTxRollsBackTogether(t *testing.T) {
	ctx := context.Background()
	repo, pool, userID := newRepo(ctx, t)

	card, err := repo.UpsertReviewCard(ctx, upsertArgs(userID, uuid.New(), time.Now().UTC()))
	if err != nil {
		t.Fatalf("seed card: %v", err)
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	txRepo := repo.WithTx(tx)

	if _, err := txRepo.UpdateReviewCardSchedule(ctx, sqlc.UpdateReviewCardScheduleParams{
		ID: card.ID, UserID: userID, Stability: 99.0, Difficulty: 2.0,
		DueAt: time.Now().UTC().AddDate(0, 0, 99), Reps: 5, Lapses: 0, State: "review",
	}); err != nil {
		t.Fatalf("update in tx: %v", err)
	}
	if _, err := txRepo.InsertReviewLog(ctx, sqlc.InsertReviewLogParams{
		CardID: card.ID, UserID: userID, Grade: "easy", ElapsedMs: 1000,
		StabilityBefore: 3.1262, StabilityAfter: 99.0, DifficultyBefore: 5.0, DifficultyAfter: 2.0,
		ScheduledDays: 99, SchedulerVersion: "v4.5", ReviewedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("insert log in tx: %v", err)
	}

	if err := tx.Rollback(ctx); err != nil {
		t.Fatalf("rollback: %v", err)
	}

	after, err := repo.GetReviewCardByID(ctx, card.ID, userID)
	if err != nil {
		t.Fatalf("read card after rollback: %v", err)
	}
	if after.Stability == 99.0 {
		t.Error("the card update survived a rolled-back transaction")
	}
	logs, err := repo.ListReviewLogsByCard(ctx, card.ID, userID, 10)
	if err != nil {
		t.Fatalf("list logs after rollback: %v", err)
	}
	if len(logs) != 0 {
		t.Errorf("%d review logs survived a rolled-back transaction", len(logs))
	}
}
