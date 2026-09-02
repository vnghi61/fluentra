//go:build integration

// Package service_test runs the srs service against a real PostgreSQL instance.
//
// The unit tests in this package drive the service through a fake repository,
// which cannot reach the transaction. AnswerCard writes the card, the review log
// and the outbox event inside one `dbx.InTx`, and everything interesting about
// that — the partitioned insert finding its partition, the three writes
// committing together, the event landing in the outbox — is invisible to a fake.
//
// This file is also the P9.5 acceptance criterion: an attempt on a vocabulary
// activity is graded, a card appears, and its due date is what the pure FSRS
// function returns for that grade and that `now`.
package service_test

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fluentra/fluentra/db/migrations"
	learningcontract "github.com/fluentra/fluentra/internal/modules/learning/contract"
	"github.com/fluentra/fluentra/internal/modules/srs/domain"
	"github.com/fluentra/fluentra/internal/modules/srs/repository"
	"github.com/fluentra/fluentra/internal/modules/srs/service"
	usercontract "github.com/fluentra/fluentra/internal/modules/user/contract"
	"github.com/fluentra/fluentra/internal/shared/clock"
	"github.com/fluentra/fluentra/internal/shared/outbox"
)

const integrationDatabase = "fluentra_srs_service_test"

var integrationPool *pgxpool.Pool

func TestMain(m *testing.M) {
	base := os.Getenv("TEST_DATABASE_URL")
	if base == "" {
		os.Exit(m.Run())
	}

	dsn, dropDatabase, err := createIntegrationDatabase(base, integrationDatabase)
	if err != nil {
		fmt.Fprintf(os.Stderr, "prepare %s: %v\n", integrationDatabase, err)
		os.Exit(1)
	}
	if err := migrateIntegrationUp(dsn); err != nil {
		dropDatabase()
		fmt.Fprintf(os.Stderr, "migrate %s: %v\n", integrationDatabase, err)
		os.Exit(1)
	}

	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		dropDatabase()
		fmt.Fprintf(os.Stderr, "pool for %s: %v\n", integrationDatabase, err)
		os.Exit(1)
	}
	integrationPool = pool

	code := m.Run()

	pool.Close()
	dropDatabase()
	os.Exit(code)
}

func createIntegrationDatabase(base, name string) (string, func(), error) {
	maintenance, err := replaceIntegrationDatabase(base, "postgres")
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

	dsn, err := replaceIntegrationDatabase(base, name)
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

func migrateIntegrationUp(dsn string) error {
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

func replaceIntegrationDatabase(dsn, database string) (string, error) {
	parsed, err := url.Parse(dsn)
	if err != nil {
		return "", fmt.Errorf("parse TEST_DATABASE_URL: %w", err)
	}
	parsed.Path = "/" + database
	return parsed.String(), nil
}

// integrationOutbox is the same adapter module.go wires in production: it
// bridges the service's narrow OutboxTx to the shared writer's DBTx. Using the
// real writer is the point — the event has to survive the outbox table's own
// constraints, which a stub would not exercise.
type integrationOutbox struct{ writer *outbox.Writer }

func (o integrationOutbox) Write(
	ctx context.Context, tx service.OutboxTx, aggregate, event string, payload any,
) (uuid.UUID, error) {
	return o.writer.Write(ctx, outboxTxAdapter{tx}, aggregate, event, payload)
}

type outboxTxAdapter struct{ inner service.OutboxTx }

func (a outboxTxAdapter) Exec(
	ctx context.Context, sql string, arguments ...any,
) (pgconn.CommandTag, error) {
	return a.inner.Exec(ctx, sql, arguments...)
}

// integrationUsers answers the timezone lookup the due queue makes.
type integrationUsers struct{ timezone string }

func (u integrationUsers) GetByID(_ context.Context, id uuid.UUID) (usercontract.Summary, error) {
	return usercontract.Summary{ID: id, Timezone: u.timezone}, nil
}

func (u integrationUsers) GetManyByIDs(
	_ context.Context, ids []uuid.UUID,
) (map[uuid.UUID]usercontract.Summary, error) {
	out := make(map[uuid.UUID]usercontract.Summary, len(ids))
	for _, id := range ids {
		out[id] = usercontract.Summary{ID: id, Timezone: u.timezone}
	}
	return out, nil
}

func (integrationUsers) Exists(_ context.Context, _ uuid.UUID) (bool, error) { return true, nil }

func newIntegrationService(t *testing.T, now time.Time, timezone string) (*service.Service, uuid.UUID) {
	t.Helper()
	if integrationPool == nil {
		t.Skip("TEST_DATABASE_URL is not set")
	}

	ctx := context.Background()
	var userID uuid.UUID
	email := fmt.Sprintf("srs-service-%s@example.com", uuid.NewString())
	const insert = `INSERT INTO core.users (email, status) VALUES ($1, 'active') RETURNING id`
	require.NoError(t, integrationPool.QueryRow(ctx, insert, email).Scan(&userID))
	t.Cleanup(func() {
		_, _ = integrationPool.Exec(ctx, `DELETE FROM core.users WHERE id = $1`, userID)
	})

	svc := service.New(service.Deps{
		Pool:   integrationPool,
		Repo:   repository.New(integrationPool),
		Users:  integrationUsers{timezone: timezone},
		Events: integrationOutbox{writer: outbox.NewWriter()},
		Clock:  clock.NewFake(now),
		Env:    "test",
	})
	return svc, userID
}

// TestIntegration_AttemptToReviewLoop is the P9.5 acceptance criterion end to
// end, against real tables: a graded vocabulary attempt produces a card whose
// due date equals what the pure function returns, the card appears in the due
// queue when it comes due, answering it writes a real review log into a real
// partition, and the outbox carries the event.
func TestIntegration_AttemptToReviewLoop(t *testing.T) {
	// A late-evening `now` on purpose: it is the hour at which day-truncated
	// scheduling used to put `hard` before `again`. The hour is the fixture;
	// the date must not be.
	//
	// It was `time.Date(2026, 8, 25, 23, 55, ...)`, and that passed every day
	// until the first of September and then failed with
	//
	//	no partition of relation "review_logs" found for row
	//
	// with nobody touching the code. review_logs is partitioned by month and
	// `learn.ensure_srs_partitions` only ever creates partitions *forward* from
	// the current month, which is right for production -- rows are written at
	// now() -- and fatal for a fixture pinned to a month the window has moved
	// past. Anchoring to the wall clock keeps the row inside a partition that
	// exists, today and every day after it.
	//
	// Step 4 moves the clock to the card's due date, a few days out. That can
	// cross into next month, which is covered: the migration and the worker's
	// rotate job both keep three months ahead.
	today := time.Now().UTC()
	now := time.Date(today.Year(), today.Month(), today.Day(), 23, 55, 0, 0, time.UTC)
	svc, userID := newIntegrationService(t, now, tzUTC)
	ctx := context.Background()

	contentVersionID := uuid.New()

	// 1. The grader's ReviewItems reach srs through the contract.
	require.NoError(t, svc.UpsertCards(ctx, userID, []learningcontract.ReviewItem{
		{ContentVersionID: contentVersionID, Skill: "vocabulary", InitialGrade: "good"},
	}))

	// 2. The stored due date is exactly the pure function's answer.
	want := domain.Schedule(
		domain.CardState{State: domain.StateNew}, domain.RatingGood, now, domain.DefaultParameters(),
	)

	var storedDueAt time.Time
	var storedStability float64
	var storedState string
	const readCard = `
		SELECT due_at, stability, state FROM learn.review_cards
		WHERE user_id = $1 AND content_version_id = $2`
	require.NoError(t, integrationPool.QueryRow(ctx, readCard, userID, contentVersionID).
		Scan(&storedDueAt, &storedStability, &storedState))

	assert.WithinDuration(t, want.DueAt, storedDueAt, time.Millisecond,
		"the stored due date must be the FSRS output for that grade and that now")
	assert.InDelta(t, want.Stability, storedStability, 1e-9)
	assert.Equal(t, string(want.State), storedState)

	// 3. Nothing is due yet; the card is scheduled days out.
	dueNow, err := svc.DueCount(ctx, userID)
	require.NoError(t, err)
	assert.Equal(t, 0, dueNow, "a card scheduled for next week is not due tonight")

	// 4. Move to the day it comes due and take the session. The clock is a
	//    dependency, so "later" is a new service over the same tables rather
	//    than a sleep.
	dueDay := want.DueAt
	svc = service.New(service.Deps{
		Pool:   integrationPool,
		Repo:   repository.New(integrationPool),
		Users:  integrationUsers{timezone: tzUTC},
		Events: integrationOutbox{writer: outbox.NewWriter()},
		Clock:  clock.NewFake(dueDay),
		Env:    "test",
	})

	count, err := svc.DueCount(ctx, userID)
	require.NoError(t, err)
	assert.Equal(t, 1, count, "the card is due on the day the scheduler chose")

	cards, err := svc.DueCards(ctx, userID, 20)
	require.NoError(t, err)
	require.Len(t, cards, 1)
	card := cards[0]

	// 5. Answering runs the card update, the log insert and the outbox write in
	//    one transaction.
	result, err := svc.AnswerCard(ctx, userID, card.ID, "good", 2100)
	require.NoError(t, err)
	assert.Equal(t, string(domain.StateReview), result.Card.State)
	assert.Positive(t, result.IntervalDays)
	assert.True(t, result.NextDueAt.After(dueDay))

	// 6. The log is real, carries the stability either side, and landed in the
	//    partition for its month.
	var grade, partition string
	var stabilityBefore, stabilityAfter float64
	const readLog = `
		SELECT grade, stability_before, stability_after, tableoid::regclass::text
		FROM learn.review_logs WHERE user_id = $1`
	require.NoError(t, integrationPool.QueryRow(ctx, readLog, userID).
		Scan(&grade, &stabilityBefore, &stabilityAfter, &partition))

	assert.Equal(t, "good", grade)
	assert.InDelta(t, card.Stability, stabilityBefore, 1e-9)
	assert.Greater(t, stabilityAfter, stabilityBefore, "a good answer must raise stability")
	assert.Contains(t, partition, fmt.Sprintf("review_logs_y%04dm%02d", dueDay.Year(), int(dueDay.Month())))

	// 7. The event is in the outbox for the relay to pick up.
	var events int
	const countEvents = `
		SELECT count(*) FROM ops.outbox_events
		WHERE aggregate = 'srs' AND payload->>'user_id' = $1::text`
	require.NoError(t, integrationPool.QueryRow(ctx, countEvents, userID.String()).Scan(&events))
	assert.Equal(t, 1, events, "answering a card must enqueue review.card_answered")

	// 8. Completing the session records the measured time, not an estimate.
	session, err := svc.CompleteSession(ctx, userID, 1, 1)
	require.NoError(t, err)
	assert.Equal(t, 1, session.Reviewed)
	assert.Equal(t, 0, session.Minutes, "2.1 seconds is no whole minutes")

	var reviewsCompleted int
	const readStats = `SELECT reviews_completed FROM learn.review_daily_stats WHERE user_id = $1`
	require.NoError(t, integrationPool.QueryRow(ctx, readStats, userID).Scan(&reviewsCompleted))
	assert.Equal(t, 1, reviewsCompleted)
}

// TestIntegration_DueQueueHonoursTheLearnerTimezone is the §5 trap against the
// real query rather than a fake: two learners, two timezones, one card at one
// instant, and different answers about whether it is due today.
func TestIntegration_DueQueueHonoursTheLearnerTimezone(t *testing.T) {
	// 20:00 UTC is already the 26th in Asia/Ho_Chi_Minh and still the 25th in UTC.
	now := time.Date(2026, 8, 25, 20, 0, 0, 0, time.UTC)
	dueAt := time.Date(2026, 8, 26, 2, 0, 0, 0, time.UTC)

	utcService, utcUser := newIntegrationService(t, now, tzUTC)
	vnService, vnUser := newIntegrationService(t, now, tzVietnam)
	ctx := context.Background()

	const seed = `
		INSERT INTO learn.review_cards (user_id, content_version_id, skill, stability, difficulty, due_at, state)
		VALUES ($1, gen_random_uuid(), 'vocabulary', 2.0, 5.0, $2, 'review')`
	for _, userID := range []uuid.UUID{utcUser, vnUser} {
		_, err := integrationPool.Exec(ctx, seed, userID, dueAt)
		require.NoError(t, err)
	}

	utcCount, err := utcService.DueCount(ctx, utcUser)
	require.NoError(t, err)
	assert.Equal(t, 0, utcCount, "for a UTC learner it is still the 25th, and the card is due on the 26th")

	vnCount, err := vnService.DueCount(ctx, vnUser)
	require.NoError(t, err)
	assert.Equal(t, 1, vnCount, "for a Ho Chi Minh learner it is already the 26th, so the card is due")
}

// TestIntegration_ForecastBucketsByLocalDay checks the projection agrees with
// the due queue about which day a card belongs to.
func TestIntegration_ForecastBucketsByLocalDay(t *testing.T) {
	now := time.Date(2026, 8, 25, 20, 0, 0, 0, time.UTC)
	svc, userID := newIntegrationService(t, now, tzVietnam)
	ctx := context.Background()

	const seed = `
		INSERT INTO learn.review_cards (user_id, content_version_id, skill, stability, difficulty, due_at, state)
		VALUES ($1, gen_random_uuid(), 'vocabulary', 2.0, 5.0, $2, 'review')`
	// 2026-08-27 22:00 UTC is 2026-08-28 05:00 local.
	_, err := integrationPool.Exec(ctx, seed, userID, time.Date(2026, 8, 27, 22, 0, 0, 0, time.UTC))
	require.NoError(t, err)

	days, err := svc.Forecast(ctx, userID, 30)
	require.NoError(t, err)
	require.Len(t, days, 1)
	assert.Equal(t, "2026-08-28", days[0].Date, "05:00 local on the 28th belongs to the 28th")
	assert.Equal(t, 1, days[0].DueCount)
}
