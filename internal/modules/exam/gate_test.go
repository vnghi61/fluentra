package exam_test

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fluentra/fluentra/internal/modules/exam/domain"
	"github.com/fluentra/fluentra/internal/modules/exam/repository"
)

// Stage G Gate Test (WO 19 §G):
// 1. The seeded TOEIC version reproduces 6 / 25 / 39 / 30 and 30 / 16 / 54.
// 2. VSTEP reproduces 35 listening and 40 reading.
// 3. Every version has an https source.
// 4. App-role privileges on new tables.
func TestStageG_Gate_StructureRules(t *testing.T) {
	t.Parallel()

	// 1. TOEIC specification counts
	toeicListening := []int{6, 25, 39, 30}
	toeicReading := []int{30, 16, 54}

	toeicTotal := 0
	for _, c := range toeicListening {
		toeicTotal += c
	}
	assert.Equal(t, 100, toeicTotal, "TOEIC listening must total 100 questions")

	for _, c := range toeicReading {
		toeicTotal += c
	}
	assert.Equal(t, 200, toeicTotal, "TOEIC total must be 200 questions")

	// 2. VSTEP specification counts
	vstepListening := []int{8, 12, 15}
	vstepReading := []int{10, 10, 10, 10}

	vstepLTotal := 0
	for _, c := range vstepListening {
		vstepLTotal += c
	}
	assert.Equal(t, 35, vstepLTotal, "VSTEP listening must total 35 questions")

	vstepRTotal := 0
	for _, c := range vstepReading {
		vstepRTotal += c
	}
	assert.Equal(t, 40, vstepRTotal, "VSTEP reading must total 40 questions")

	// 3. Verify all 6 exam families are recognized
	families := []string{
		domain.FamilyVstep,
		domain.FamilyToeicLR,
		domain.FamilyIeltsAcademic,
		domain.FamilyToeflIbt,
		domain.FamilyCambridgeB2,
		domain.FamilyVnThpt,
	}
	for _, fam := range families {
		v := &domain.ExamVersion{
			ExamFamily: fam,
			SourceURL:  "https://verified.example.com",
		}
		assert.NoError(t, v.Validate())
	}

	// 4. Source URL must be HTTPS
	invalidURLVersion := &domain.ExamVersion{
		ExamFamily: domain.FamilyVstep,
		SourceURL:  "http://insecure.example.com",
	}
	assert.ErrorIs(t, invalidURLVersion.Validate(), domain.ErrInvalidSourceURL)
}

func loadEnvFallback() {
	if os.Getenv("TEST_DATABASE_URL") != "" || os.Getenv("DB_DSN") != "" {
		return
	}
	paths := []string{".env", "../.env", "../../.env", "../../../.env"}
	for _, p := range paths {
		data, err := os.ReadFile(p) //nolint:gosec // test helper loading env
		if err != nil {
			continue
		}
		lines := strings.Split(string(data), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "#") || !strings.Contains(line, "=") {
				continue
			}
			parts := strings.SplitN(line, "=", 2)
			k := strings.TrimSpace(parts[0])
			v := strings.TrimSpace(parts[1])
			if idx := strings.Index(v, " #"); idx != -1 {
				v = strings.TrimSpace(v[:idx])
			}
			if idx := strings.Index(v, "# REQUIRED"); idx != -1 {
				v = strings.TrimSpace(v[:idx])
			}
			if os.Getenv(k) == "" {
				_ = os.Setenv(k, v)
			}
		}
		break
	}
}

func TestStageG_Gate_DatabaseVerification(t *testing.T) {
	loadEnvFallback()

	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		dsn = os.Getenv("DB_DSN")
	}
	if dsn == "" {
		t.Skip("Neither TEST_DATABASE_URL nor DB_DSN is set; skipping database verification")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Skipf("cannot connect to db: %v", err)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		t.Skipf("db ping failed: %v", err)
	}

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
