//go:build integration

package exam_test

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fluentra/fluentra/db/migrations"
	"github.com/fluentra/fluentra/internal/modules/exam/domain"
	"github.com/fluentra/fluentra/internal/modules/exam/repository"
)

const gateDatabaseName = "fluentra_exam_gate_test"

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

func TestStageG_Gate_DatabaseVerification(t *testing.T) {
	pool := gateDatabase(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	repo := repository.New(pool)

	// 1. Check all versions have https source
	versions, err := repo.ListExamVersions(ctx)
	require.NoError(t, err)
	require.NotEmpty(t, versions, "Database must contain seeded exam versions")

	versionMap := make(map[string]*domain.ExamVersion)
	for _, v := range versions {
		assert.True(t, strings.HasPrefix(v.SourceURL, "https://"),
			"Version %s source URL %q must begin with https://", v.Code, v.SourceURL)
		versionMap[v.Code] = v
	}

	// Ensure all 6 families exist
	require.Contains(t, versionMap, "TOEIC_LR_2026")
	require.Contains(t, versionMap, "VSTEP_3_5")
	require.Contains(t, versionMap, "IELTS_ACADEMIC_2026")
	require.Contains(t, versionMap, "TOEFL_IBT_2026")
	require.Contains(t, versionMap, "CAMBRIDGE_B2_FIRST")
	require.Contains(t, versionMap, "VN_THPT_2026")

	// TOEFL iBT must have is_current = false
	assert.False(t, versionMap["TOEFL_IBT_2026"].IsCurrent, "TOEFL iBT must have is_current = false")

	// 2. Verify TOEIC parts: 6 / 25 / 39 / 30 and 30 / 16 / 54
	toeicParts, err := repo.ListExamPartsByVersionID(ctx, versionMap["TOEIC_LR_2026"].ID)
	require.NoError(t, err)
	require.Len(t, toeicParts, 7, "TOEIC must have 7 parts")

	expectedToeicCounts := map[int]int{
		1: 6,
		2: 25,
		3: 39,
		4: 30,
		5: 30,
		6: 16,
		7: 54,
	}
	for _, p := range toeicParts {
		expectedCount, ok := expectedToeicCounts[p.PartNumber]
		require.True(t, ok, "Unexpected part number: %d", p.PartNumber)
		assert.Equal(t, expectedCount, p.QuestionCount, "Part %d question count mismatch", p.PartNumber)
		assert.Equal(t, 0, p.QuestionCount%p.GroupSize, "Part %d group size must divide question count", p.PartNumber)
	}

	// 3. Verify VSTEP parts: 35 listening and 40 reading
	vstepParts, err := repo.ListExamPartsByVersionID(ctx, versionMap["VSTEP_3_5"].ID)
	require.NoError(t, err)
	require.NotEmpty(t, vstepParts)

	var vstepListeningCount, vstepReadingCount int
	for _, p := range vstepParts {
		switch p.Section {
		case "listening":
			vstepListeningCount += p.QuestionCount
		case "reading":
			vstepReadingCount += p.QuestionCount
		}
		assert.Equal(t, 0, p.QuestionCount%p.GroupSize, "Part %d group size must divide question count", p.PartNumber)
	}
	assert.Equal(t, 35, vstepListeningCount, "VSTEP listening parts must sum to 35")
	assert.Equal(t, 40, vstepReadingCount, "VSTEP reading parts must sum to 40")

	// 4. Verify app-role table privileges
	tables := []string{
		"assess.exam_versions",
		"assess.exam_parts",
		"assess.blueprints",
	}
	for _, tbl := range tables {
		for _, priv := range []string{"SELECT", "INSERT", "UPDATE", "DELETE"} {
			var hasPriv bool
			err := pool.QueryRow(ctx,
				"SELECT has_table_privilege('fluentra_app', $1, $2)", tbl, priv,
			).Scan(&hasPriv)
			require.NoError(t, err, "Check privilege %s on %s", priv, tbl)
			assert.True(t, hasPriv, "fluentra_app must have %s on %s", priv, tbl)
		}
	}
}

// Live DB Verification for Stage H tables & privileges
func TestStageH_Gate_DatabaseVerification(t *testing.T) {
	pool := gateDatabase(t)
	var err error
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	for _, priv := range []string{"SELECT", "INSERT", "UPDATE", "DELETE"} {
		var hasPriv bool
		err := pool.QueryRow(ctx,
			"SELECT has_table_privilege('fluentra_app', 'assess.mock_tests', $1)", priv,
		).Scan(&hasPriv)
		require.NoError(t, err, "Check privilege %s on assess.mock_tests", priv)
		assert.True(t, hasPriv, "fluentra_app must have %s on assess.mock_tests", priv)
	}

	var colExists bool
	err = pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM information_schema.columns
			WHERE table_schema = 'assess' AND table_name = 'exam_attempts' AND column_name = 'mock_test_id'
		)
	`).Scan(&colExists)
	require.NoError(t, err)
	assert.True(t, colExists, "assess.exam_attempts must have mock_test_id column")

	repo := repository.New(pool)
	vstepExam, err := repo.GetExamBySlug(ctx, "vstep-3-5")
	require.NoError(t, err)
	require.NotNil(t, vstepExam)
	assert.NotNil(t, vstepExam.VersionID)

	toeicExam, err := repo.GetExamBySlug(ctx, "toeic-lr-2026")
	require.NoError(t, err)
	require.NotNil(t, toeicExam)
	assert.NotNil(t, toeicExam.VersionID)
}
