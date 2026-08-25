//go:build integration

package srs_test

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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fluentra/fluentra/db/migrations"
	"github.com/fluentra/fluentra/internal/modules/srs"
)

// The partition rotation job is the reason `review_logs` keeps working on the
// first of a month. P8.2 paid to learn that shipping monthly partitioning without
// the rotation job is a production outage on a date nobody is watching, so the
// job ships in the same task — and a job nothing ever ran is not a job.
//
// These tests run the real module against a real database. TestMain lives here
// rather than in the package's other test file because only this one needs a
// database; the unit tests in e2e_attempt_to_review_test.go run with fakes and
// are unaffected.

const moduleDatabase = "fluentra_srs_module_test"

var modulePool *pgxpool.Pool

func TestMain(m *testing.M) {
	base := os.Getenv("TEST_DATABASE_URL")
	if base == "" {
		os.Exit(m.Run())
	}

	dsn, dropDatabase, err := createModuleDatabase(base, moduleDatabase)
	if err != nil {
		fmt.Fprintf(os.Stderr, "prepare %s: %v\n", moduleDatabase, err)
		os.Exit(1)
	}
	if err := migrateModuleUp(dsn); err != nil {
		dropDatabase()
		fmt.Fprintf(os.Stderr, "migrate %s: %v\n", moduleDatabase, err)
		os.Exit(1)
	}

	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		dropDatabase()
		fmt.Fprintf(os.Stderr, "pool for %s: %v\n", moduleDatabase, err)
		os.Exit(1)
	}
	modulePool = pool

	code := m.Run()

	pool.Close()
	dropDatabase()
	os.Exit(code)
}

func createModuleDatabase(base, name string) (string, func(), error) {
	maintenance, err := replaceModuleDatabase(base, "postgres")
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

	dsn, err := replaceModuleDatabase(base, name)
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

func migrateModuleUp(dsn string) error {
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

func replaceModuleDatabase(dsn, database string) (string, error) {
	parsed, err := url.Parse(dsn)
	if err != nil {
		return "", fmt.Errorf("parse TEST_DATABASE_URL: %w", err)
	}
	parsed.Path = "/" + database
	return parsed.String(), nil
}

func newModule(t *testing.T) *srs.Module {
	t.Helper()
	if modulePool == nil {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	return srs.New(srs.Deps{Pool: modulePool, Guard: permissiveGuard{}, Env: "test"})
}

type permissiveGuard struct{}

func (permissiveGuard) Require(_ context.Context, _ string) error { return nil }

// TestModuleIntegration_RotatePartitionsRuns is the proof that the job works
// against the function the migration actually created. A rename on either side
// — the SQL function or the sqlc query — leaves the job erroring every six hours
// into a log nobody reads until an insert fails on the first of a month.
func TestModuleIntegration_RotatePartitionsRuns(t *testing.T) {
	module := newModule(t)
	ctx := context.Background()

	require.NoError(t, module.RotatePartitions(ctx))

	// The migration pre-created them, so a rotation right after migrating creates
	// nothing — and must still succeed.
	require.NoError(t, module.RotatePartitions(ctx), "rotation must be idempotent")

	var partitions int
	const countPartitions = `
		SELECT count(*) FROM pg_class c
		JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE n.nspname = 'learn' AND c.relname ~ '^review_logs_y\d{4}m\d{2}$'`
	require.NoError(t, modulePool.QueryRow(ctx, countPartitions).Scan(&partitions))
	assert.GreaterOrEqual(t, partitions, 4,
		"the current month plus three ahead must exist after migrating and rotating")
}

// TestModuleIntegration_CronJobUsesItsOwnLockID guards the convention from §6 of
// the work-package brief. learning.rotate_partitions holds 1_700_000_210; two
// jobs sharing a lock id means one of them silently never runs, and nothing
// else in the build would notice.
func TestModuleIntegration_CronJobUsesItsOwnLockID(t *testing.T) {
	module := newModule(t)

	jobs := module.CronJobs()
	require.Len(t, jobs, 1)

	job := jobs[0]
	assert.Equal(t, "srs.rotate_partitions", job.Name)
	assert.Equal(t, int64(1_700_000_220), job.LockID,
		"the lock id is the migration timestamp, and must not collide with learning's 1700000210")
	assert.Positive(t, job.Interval)
	assert.NotNil(t, job.Task)
}

// TestModuleIntegration_ContractsAreWired: cmd/api hands these two to `learning`
// and `vocabulary`, and a nil either side is a nil-pointer panic on the first
// graded attempt rather than a boot failure.
func TestModuleIntegration_ContractsAreWired(t *testing.T) {
	module := newModule(t)

	assert.NotNil(t, module.CardWriter())
	assert.NotNil(t, module.QueueReader())
}

// TestModuleIntegration_RotatePartitionsWithoutAPoolFails: the worker calls this
// at start-up, and a misconfigured process must say so rather than report
// success having done nothing.
func TestModuleIntegration_RotatePartitionsWithoutAPoolFails(t *testing.T) {
	module := srs.New(srs.Deps{Guard: permissiveGuard{}})

	require.Error(t, module.RotatePartitions(context.Background()))
}
