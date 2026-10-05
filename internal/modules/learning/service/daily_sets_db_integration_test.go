//go:build integration

package service_test

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fluentra/fluentra/internal/modules/learning/domain"
	"github.com/fluentra/fluentra/internal/modules/learning/repository"
)

const (
	dailySetsDBLevelA2 = "A2"
	dailySetsDBLevelB1 = "B1"
	dailySetsDBLevelB2 = "B2"
)

type dailySetsDBFixture struct {
	repo   *repository.Repository
	userID uuid.UUID
	date   time.Time
}

func newDailySetsDBFixture(ctx context.Context, t *testing.T, pool *pgxpool.Pool) dailySetsDBFixture {
	t.Helper()
	userID := uuid.NewSHA1(uuid.NameSpaceOID, []byte(t.Name()))
	_, err := pool.Exec(ctx, `
		INSERT INTO core.users (id, email, status, created_at, updated_at)
		VALUES ($1, $2, 'active', now(), now())
	`, userID, "daily-sets-"+userID.String()+"@test.local")
	require.NoError(t, err)
	return dailySetsDBFixture{
		repo: repository.New(pool), userID: userID,
		date: time.Date(2026, 9, 22, 0, 0, 0, 0, time.UTC),
	}
}

func (f dailySetsDBFixture) activities(name string) []uuid.UUID {
	return []uuid.UUID{uuid.NewSHA1(f.userID, []byte(name))}
}

func (f dailySetsDBFixture) create(
	ctx context.Context, t *testing.T, level string, activities []uuid.UUID,
) *domain.DailySet {
	t.Helper()
	set, err := f.repo.CreateDailySet(ctx, f.userID, f.date, activities, level)
	require.NoError(t, err)
	require.NotNil(t, set, "first insert at level %q must succeed", level)
	assert.Equal(t, activities, set.ActivityIDs)
	return set
}

func (f dailySetsDBFixture) assertStored(ctx context.Context, t *testing.T, level string, want *domain.DailySet) {
	t.Helper()
	got, err := f.repo.GetDailySet(ctx, f.userID, f.date, level)
	require.NoError(t, err)
	require.NotNil(t, got, "set at level %q must remain readable", level)
	assert.Equal(t, want.ID, got.ID)
	assert.Equal(t, f.userID, got.UserID)
	assert.Equal(t, f.date, got.LocalDate)
	assert.Equal(t, want.ActivityIDs, got.ActivityIDs)
	assert.Equal(t, want.CreatedAt, got.CreatedAt)
}

func TestDailySetsDB(t *testing.T) {
	// One private gate database, with separate test users and no parallel reset.
	pool := gateDatabase(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	t.Run("HP-02_level_isolation_and_repeats", func(t *testing.T) {
		dailySetsDBLevelIsolation(ctx, t, newDailySetsDBFixture(ctx, t, pool))
	})
	t.Run("BV-02_legacy_empty_level_preserved", func(t *testing.T) {
		dailySetsDBLegacyLevel(ctx, t, newDailySetsDBFixture(ctx, t, pool))
	})
	t.Run("ERR-04_concurrent_creates_converge", func(t *testing.T) {
		dailySetsDBConcurrentCreates(ctx, t, newDailySetsDBFixture(ctx, t, pool))
	})
}

func dailySetsDBLevelIsolation(ctx context.Context, t *testing.T, f dailySetsDBFixture) {
	t.Helper()
	levels := []string{dailySetsDBLevelA2, dailySetsDBLevelB1, dailySetsDBLevelB2}
	stored := make(map[string]*domain.DailySet, len(levels))
	for _, level := range levels {
		stored[level] = f.create(ctx, t, level, f.activities(level))
	}
	assert.NotEqual(t, stored[levels[0]].ID, stored[levels[1]].ID)
	assert.NotEqual(t, stored[levels[0]].ID, stored[levels[2]].ID)
	assert.NotEqual(t, stored[levels[1]].ID, stored[levels[2]].ID)
	for _, level := range levels {
		f.assertStored(ctx, t, level, stored[level])
		repeat, err := f.repo.CreateDailySet(ctx, f.userID, f.date, f.activities("replacement-"+level), level)
		require.NoError(t, err)
		assert.Nil(t, repeat, "repeat insert at %s must lose without overwriting", level)
		f.assertStored(ctx, t, level, stored[level])
	}
}

func dailySetsDBLegacyLevel(ctx context.Context, t *testing.T, f dailySetsDBFixture) {
	t.Helper()
	legacy := f.create(ctx, t, "", f.activities("legacy"))
	for _, level := range []string{dailySetsDBLevelA2, dailySetsDBLevelB1, dailySetsDBLevelB2} {
		labelled, err := f.repo.GetDailySet(ctx, f.userID, f.date, level)
		require.NoError(t, err)
		assert.Nil(t, labelled, "legacy row must not serve labelled level %s", level)
	}
	labelled := f.create(ctx, t, dailySetsDBLevelA2, f.activities("labelled"))
	f.assertStored(ctx, t, dailySetsDBLevelA2, labelled)
	f.assertStored(ctx, t, "", legacy)
}

type dailySetsDBCreateResult struct {
	level      string
	activities []uuid.UUID
	set        *domain.DailySet
	err        error
}

func dailySetsDBConcurrentCreates(ctx context.Context, t *testing.T, f dailySetsDBFixture) {
	t.Helper()
	levels := []string{dailySetsDBLevelB1, dailySetsDBLevelB1, dailySetsDBLevelB1, dailySetsDBLevelB2}
	start := make(chan struct{})
	ready := make(chan struct{}, len(levels))
	results := make(chan dailySetsDBCreateResult, len(levels))
	for i, level := range levels {
		activities := f.activities(level + "-" + strconv.Itoa(i))
		go func() {
			ready <- struct{}{}
			select {
			case <-start:
			case <-ctx.Done():
				results <- dailySetsDBCreateResult{level: level, err: ctx.Err()}
				return
			}
			set, err := f.repo.CreateDailySet(ctx, f.userID, f.date, activities, level)
			results <- dailySetsDBCreateResult{level: level, activities: activities, set: set, err: err}
		}()
	}
	for range levels {
		select {
		case <-ready:
		case <-ctx.Done():
			t.Fatalf("concurrent creators did not reach the barrier: %v", ctx.Err())
		}
	}
	close(start)
	assertDailySetsDBWinners(ctx, t, f, results, len(levels))
}

func assertDailySetsDBWinners(
	ctx context.Context, t *testing.T, f dailySetsDBFixture, results <-chan dailySetsDBCreateResult, count int,
) {
	t.Helper()
	winners := map[string][]*domain.DailySet{}
	losers := 0
	for range count {
		select {
		case result := <-results:
			require.NoError(t, result.err)
			if result.set == nil {
				assert.Equal(t, dailySetsDBLevelB1, result.level, "independent level must not conflict")
				losers++
				continue
			}
			assert.Equal(t, result.activities, result.set.ActivityIDs)
			winners[result.level] = append(winners[result.level], result.set)
		case <-ctx.Done():
			t.Fatalf("concurrent daily-set creates did not finish: %v", ctx.Err())
		}
	}
	assert.Equal(t, 2, losers, "same-level losing inserts must return nil")
	for _, level := range []string{dailySetsDBLevelB1, dailySetsDBLevelB2} {
		require.Len(t, winners[level], 1, "exactly one winner required at %s", level)
		f.assertStored(ctx, t, level, winners[level][0])
	}
}
