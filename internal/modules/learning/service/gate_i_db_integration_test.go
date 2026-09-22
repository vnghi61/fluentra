//go:build integration

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
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fluentra/fluentra/db/migrations"
	"github.com/fluentra/fluentra/internal/modules/learning/domain"
	"github.com/fluentra/fluentra/internal/modules/learning/repository"
)

const gateDatabaseName = "fluentra_learning_gate_test"

// gateDatabase migrates a private database from scratch and returns a pool on
// it. The gate checks used to read whatever database .env pointed at, so in CI
// they met an empty one: no schema, no fluentra_app role. Migrating here tests
// what the migrations produce, which is the claim the gate makes.
func gateDatabase(t *testing.T) *pgxpool.Pool {
	t.Helper()
	base := os.Getenv("TEST_DATABASE_URL")
	if base == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	maintenance, err := withDatabase(base, "postgres")
	require.NoError(t, err)
	dsn, err := withDatabase(base, gateDatabaseName)
	require.NoError(t, err)

	admin, err := sql.Open("pgx", maintenance)
	require.NoError(t, err)
	t.Cleanup(func() { _ = admin.Close() })
	drop := fmt.Sprintf("DROP DATABASE IF EXISTS %q WITH (FORCE)", gateDatabaseName)
	_, err = admin.ExecContext(context.Background(), drop)
	require.NoError(t, err)
	_, err = admin.ExecContext(context.Background(), fmt.Sprintf("CREATE DATABASE %q", gateDatabaseName))
	require.NoError(t, err)
	t.Cleanup(func() { _, _ = admin.ExecContext(context.Background(), drop) })

	db, err := sql.Open("pgx", dsn)
	require.NoError(t, err)
	sources, err := migrations.Flattened()
	require.NoError(t, err)
	provider, err := goose.NewProvider(goose.DialectPostgres, db, sources)
	require.NoError(t, err)
	_, err = provider.Up(context.Background())
	require.NoError(t, err)
	require.NoError(t, provider.Close())

	pool, err := pgxpool.New(context.Background(), dsn)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	return pool
}

func withDatabase(dsn, database string) (string, error) {
	parsed, err := url.Parse(dsn)
	if err != nil {
		return "", fmt.Errorf("parse TEST_DATABASE_URL: %w", err)
	}
	parsed.Path = "/" + database
	return parsed.String(), nil
}

// TestGateI_LiveDatabaseVerification verifies §2 live database checks:
// 1. learn.node_mastery table existence and schema.
// 2. fluentra_app permissions (SELECT, INSERT, UPDATE, DELETE).
// 3. User erasure drops records for user_id.
func TestGateI_LiveDatabaseVerification(t *testing.T) {
	pool := gateDatabase(t)
	var err error
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 1. Check fluentra_app privileges on learn.node_mastery
	for _, priv := range []string{"SELECT", "INSERT", "UPDATE", "DELETE"} {
		var hasPriv bool
		err := pool.QueryRow(ctx,
			"SELECT has_table_privilege('fluentra_app', 'learn.node_mastery', $1)", priv,
		).Scan(&hasPriv)
		require.NoError(t, err, "Check privilege %s on learn.node_mastery", priv)
		assert.True(t, hasPriv, "fluentra_app must have %s on learn.node_mastery", priv)
	}

	// 2. Check table columns
	var exists bool
	err = pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM information_schema.tables
			WHERE table_schema = 'learn' AND table_name = 'node_mastery'
		)
	`).Scan(&exists)
	require.NoError(t, err)
	assert.True(t, exists, "learn.node_mastery must exist")

	// 3. Test CRUD and erasure via Repository
	repo := repository.New(pool)
	testUser := uuid.New()
	testNode := uuid.New()

	// Ensure core.users row exists for FK
	_, err = pool.Exec(ctx, `
		INSERT INTO core.users (id, email, status, created_at, updated_at)
		VALUES ($1, $2, 'active', now(), now())
		ON CONFLICT (id) DO NOTHING
	`, testUser, "gate-i-"+testUser.String()[:8]+"@test.local")
	require.NoError(t, err)

	defer func() {
		_, _ = pool.Exec(ctx, "DELETE FROM learn.node_mastery WHERE user_id = $1", testUser)
		_, _ = pool.Exec(ctx, "DELETE FROM core.users WHERE id = $1", testUser)
	}()

	now := time.Now().UTC().Truncate(time.Microsecond)
	m := domain.NodeMastery{
		UserID:     testUser,
		NodeID:     testNode,
		Attempts:   3,
		Correct:    1,
		Score:      0.333,
		LastSeenAt: &now,
	}

	upserted, err := repo.UpsertNodeMastery(ctx, m)
	require.NoError(t, err)
	require.NotNil(t, upserted)
	assert.Equal(t, testUser, upserted.UserID)
	assert.Equal(t, testNode, upserted.NodeID)
	assert.Equal(t, 3, upserted.Attempts)
	assert.Equal(t, 1, upserted.Correct)
	assert.InDelta(t, 0.333, upserted.Score, 0.001)

	// Read back
	readBack, err := repo.GetNodeMastery(ctx, testUser, testNode)
	require.NoError(t, err)
	require.NotNil(t, readBack)
	assert.Equal(t, 3, readBack.Attempts)

	// List weak nodes
	weak, err := repo.ListWeakNodesByUser(ctx, testUser, domain.MinAttemptsForWeakNode)
	require.NoError(t, err)
	require.NotEmpty(t, weak)
	assert.Equal(t, testNode, weak[0].NodeID)

	// User erasure
	err = repo.DeleteNodeMasteryByUser(ctx, testUser)
	require.NoError(t, err)

	afterErasure, err := repo.GetNodeMastery(ctx, testUser, testNode)
	require.NoError(t, err)
	assert.Nil(t, afterErasure, "record must be deleted after user erasure")
}
