package service_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fluentra/fluentra/internal/generated/gamification/sqlc"
	"github.com/fluentra/fluentra/internal/modules/gamification/contract"
	"github.com/fluentra/fluentra/internal/modules/gamification/domain"
	"github.com/fluentra/fluentra/internal/modules/gamification/service"
	learningcontract "github.com/fluentra/fluentra/internal/modules/learning/contract"
	usercontract "github.com/fluentra/fluentra/internal/modules/user/contract"
	"github.com/fluentra/fluentra/internal/shared/apperr"
)

// fakeRepo is an in-memory stand-in that enforces the two guarantees the real
// schema enforces: the XP idempotency key, and one badge per learner. The rest
// of the surface is the minimum each test needs.
type fakeRepo struct {
	events    []sqlc.LearnXpEvent
	streak    sqlc.LearnStreak
	badges    []sqlc.LearnBadge
	earned    map[uuid.UUID]bool
	quests    []sqlc.ListOpenUserQuestsRow
	broken    bool
	freezes   int
	highWater map[string]sqlc.GetActivityHighWaterRow

	// Recorded for assertions.
	extendCalls int
	awardCalls  int
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{
		streak:    sqlc.LearnStreak{DailyGoalXp: 50, FreezesAvailable: 2},
		earned:    map[uuid.UUID]bool{},
		highWater: map[string]sqlc.GetActivityHighWaterRow{},
	}
}

func (f *fakeRepo) AwardXP(_ context.Context, arg sqlc.AwardXPParams) (sqlc.LearnXpEvent, error) {
	f.awardCalls++
	// uq_xp_events_unscored, in memory.
	for _, existing := range f.events {
		if existing.Source != "activity" &&
			existing.UserID == arg.UserID &&
			existing.Source == arg.Source &&
			existing.SourceID == arg.SourceID {
			return sqlc.LearnXpEvent{}, pgx.ErrNoRows
		}
	}
	event := sqlc.LearnXpEvent{
		ID: uuid.New(), UserID: arg.UserID, Source: arg.Source,
		SourceID: arg.SourceID, Amount: arg.Amount, AwardedAt: time.Now(),
	}
	f.events = append(f.events, event)
	return event, nil
}

func (f *fakeRepo) GetActivityHighWater(
	_ context.Context, userID uuid.UUID, activityID string,
) (sqlc.GetActivityHighWaterRow, error) {
	if f.highWater == nil {
		return sqlc.GetActivityHighWaterRow{}, pgx.ErrNoRows
	}
	key := fmt.Sprintf("%s:%s", userID, activityID)
	hw, ok := f.highWater[key]
	if !ok {
		return sqlc.GetActivityHighWaterRow{}, pgx.ErrNoRows
	}
	return hw, nil
}

func (f *fakeRepo) UpsertActivityHighWater(
	_ context.Context, arg sqlc.UpsertActivityHighWaterParams,
) (sqlc.LearnXpActivityHighWater, error) {
	if f.highWater == nil {
		f.highWater = make(map[string]sqlc.GetActivityHighWaterRow)
	}
	key := fmt.Sprintf("%s:%s", arg.UserID, arg.ActivityID)
	f.highWater[key] = sqlc.GetActivityHighWaterRow{
		BestScore: arg.BestScore,
		XpGranted: arg.XpGranted,
	}
	return sqlc.LearnXpActivityHighWater{
		UserID:     arg.UserID,
		ActivityID: arg.ActivityID,
		BestScore:  arg.BestScore,
		XpGranted:  arg.XpGranted,
	}, nil
}

func (f *fakeRepo) TotalXP(_ context.Context, userID uuid.UUID) (int64, error) {
	total := int64(0)
	for _, e := range f.events {
		if e.UserID == userID {
			total += int64(e.Amount)
		}
	}
	return total, nil
}

func (f *fakeRepo) XPSince(ctx context.Context, userID uuid.UUID, _ time.Time) (int64, error) {
	return f.TotalXP(ctx, userID)
}

func (f *fakeRepo) XPFromSourceSince(
	_ context.Context, userID uuid.UUID, source string, _ time.Time,
) (int64, error) {
	total := int64(0)
	for _, e := range f.events {
		if e.UserID == userID && e.Source == source {
			total += int64(e.Amount)
		}
	}
	return total, nil
}

func (f *fakeRepo) CountAwardsFromSourceSince(
	_ context.Context, userID uuid.UUID, source string, _ time.Time,
) (int64, error) {
	count := int64(0)
	for _, e := range f.events {
		if e.UserID == userID && e.Source == source {
			count++
		}
	}
	return count, nil
}

func (f *fakeRepo) WeeklyXPStandings(
	_ context.Context, _, _ time.Time, _ int32,
) ([]sqlc.WeeklyXPStandingsRow, error) {
	return nil, nil
}

func (f *fakeRepo) GetStreak(_ context.Context, _ uuid.UUID) (sqlc.LearnStreak, error) {
	return f.streak, nil
}

func (f *fakeRepo) EnsureStreak(_ context.Context, userID uuid.UUID) (sqlc.LearnStreak, error) {
	f.streak.UserID = userID
	return f.streak, nil
}

func (f *fakeRepo) ExtendStreak(
	_ context.Context, _ uuid.UUID, newLength int32, day time.Time,
) (sqlc.LearnStreak, error) {
	f.extendCalls++
	f.streak.CurrentLength = newLength
	if newLength > f.streak.LongestLength {
		f.streak.LongestLength = newLength
	}
	f.streak.LastActiveOn = pgtype.Date{Time: day, Valid: true}
	return f.streak, nil
}

func (f *fakeRepo) BreakStreak(_ context.Context, _ uuid.UUID) (sqlc.LearnStreak, error) {
	f.broken = true
	f.streak.CurrentLength = 0
	return f.streak, nil
}

func (f *fakeRepo) ConsumeFreeze(
	_ context.Context, _ uuid.UUID, day time.Time,
) (sqlc.LearnStreak, error) {
	// The real statement's guards: a freeze must be available and not already
	// spent today.
	if f.streak.FreezesAvailable <= 0 {
		return sqlc.LearnStreak{}, pgx.ErrNoRows
	}
	if f.streak.FreezeUsedOn.Valid && !f.streak.FreezeUsedOn.Time.Before(day) {
		return sqlc.LearnStreak{}, pgx.ErrNoRows
	}
	f.freezes++
	f.streak.FreezesAvailable--
	f.streak.FreezeUsedOn = pgtype.Date{Time: day, Valid: true}
	f.streak.LastActiveOn = pgtype.Date{Time: day, Valid: true}
	return f.streak, nil
}

func (f *fakeRepo) SetDailyGoal(_ context.Context, _ uuid.UUID, goal int32) (sqlc.LearnStreak, error) {
	f.streak.DailyGoalXp = goal
	return f.streak, nil
}

func (f *fakeRepo) SetLeaderboardOptIn(_ context.Context, _ uuid.UUID, optIn bool) (sqlc.LearnStreak, error) {
	f.streak.LeaderboardOptIn = optIn
	return f.streak, nil
}

func (f *fakeRepo) ListStreaksAtRisk(_ context.Context, _ time.Time, _ int32) ([]sqlc.LearnStreak, error) {
	return []sqlc.LearnStreak{f.streak}, nil
}

func (f *fakeRepo) ListUnearnedBadges(_ context.Context, _ uuid.UUID) ([]sqlc.LearnBadge, error) {
	out := []sqlc.LearnBadge{}
	for _, badge := range f.badges {
		if !f.earned[badge.ID] {
			out = append(out, badge)
		}
	}
	return out, nil
}

func (f *fakeRepo) ListEarnedBadges(_ context.Context, _ uuid.UUID) ([]sqlc.ListEarnedBadgesRow, error) {
	out := []sqlc.ListEarnedBadgesRow{}
	for _, badge := range f.badges {
		if f.earned[badge.ID] {
			out = append(out, sqlc.ListEarnedBadgesRow{
				ID: badge.ID, Code: badge.Code, Name: badge.Name, Tier: badge.Tier,
			})
		}
	}
	return out, nil
}

func (f *fakeRepo) AwardBadge(
	_ context.Context, _ uuid.UUID, badgeID uuid.UUID,
) (sqlc.LearnBadgesEarned, error) {
	// uq_badges_earned, in memory.
	if f.earned[badgeID] {
		return sqlc.LearnBadgesEarned{}, pgx.ErrNoRows
	}
	f.earned[badgeID] = true
	return sqlc.LearnBadgesEarned{BadgeID: badgeID}, nil
}

func (f *fakeRepo) UpsertBadge(_ context.Context, _ sqlc.UpsertBadgeParams) (sqlc.LearnBadge, error) {
	return sqlc.LearnBadge{}, nil
}

func (f *fakeRepo) ListActiveQuests(_ context.Context) ([]sqlc.LearnQuest, error) { return nil, nil }

func (f *fakeRepo) UpsertQuest(_ context.Context, _ sqlc.UpsertQuestParams) (sqlc.LearnQuest, error) {
	return sqlc.LearnQuest{}, nil
}

func (f *fakeRepo) StartUserQuest(
	_ context.Context, _, _ uuid.UUID, _, _ time.Time,
) (sqlc.LearnUserQuest, error) {
	return sqlc.LearnUserQuest{}, nil
}

func (f *fakeRepo) ListOpenUserQuests(
	_ context.Context, _ uuid.UUID, _ time.Time,
) ([]sqlc.ListOpenUserQuestsRow, error) {
	return f.quests, nil
}

func (f *fakeRepo) UpdateQuestProgress(
	_ context.Context, id, _ uuid.UUID, progress []byte,
) (sqlc.LearnUserQuest, error) {
	for i := range f.quests {
		if f.quests[i].UserQuestID == id {
			f.quests[i].Progress = progress
		}
	}
	return sqlc.LearnUserQuest{ID: id}, nil
}

func (f *fakeRepo) CompleteUserQuest(_ context.Context, id, _ uuid.UUID) (sqlc.LearnUserQuest, error) {
	for i := range f.quests {
		if f.quests[i].UserQuestID == id {
			if f.quests[i].CompletedAt != nil {
				return sqlc.LearnUserQuest{}, pgx.ErrNoRows
			}
			now := time.Now()
			f.quests[i].CompletedAt = &now
		}
	}
	return sqlc.LearnUserQuest{ID: id}, nil
}

func (f *fakeRepo) UpsertLeaderboardEntry(
	_ context.Context, _ sqlc.UpsertLeaderboardEntryParams,
) (sqlc.LearnLeaderboardSnapshot, error) {
	return sqlc.LearnLeaderboardSnapshot{}, nil
}

func (f *fakeRepo) ListLeaderboard(
	_ context.Context, _ string, _ time.Time, _ int32,
) ([]sqlc.LearnLeaderboardSnapshot, error) {
	return nil, nil
}

func (f *fakeRepo) GetLeaderboardEntry(
	_ context.Context, _ uuid.UUID, _ time.Time,
) (sqlc.LearnLeaderboardSnapshot, error) {
	return sqlc.LearnLeaderboardSnapshot{}, pgx.ErrNoRows
}

func (f *fakeRepo) DeleteLeaderboardBefore(_ context.Context, _ time.Time) error { return nil }

// fakeUsers hands back a fixed timezone.
type fakeUsers struct{ timezone string }

func (f fakeUsers) GetByID(_ context.Context, id uuid.UUID) (usercontract.Summary, error) {
	return usercontract.Summary{ID: id, DisplayName: "Learner", Timezone: f.timezone}, nil
}

func (f fakeUsers) GetManyByIDs(
	_ context.Context, ids []uuid.UUID,
) (map[uuid.UUID]usercontract.Summary, error) {
	out := map[uuid.UUID]usercontract.Summary{}
	for _, id := range ids {
		out[id] = usercontract.Summary{ID: id, DisplayName: "Learner"}
	}
	return out, nil
}

func (f fakeUsers) Exists(_ context.Context, _ uuid.UUID) (bool, error) { return true, nil }

// fixedClock keeps the day boundary arithmetic deterministic.
type fixedClock struct{ at time.Time }

func (c fixedClock) Now() time.Time { return c.at }

// codeOf reads the machine-readable code off an apperr. Asserting on the code
// rather than the message keeps the tests from breaking when the copy changes.
func codeOf(t *testing.T, err error) string {
	t.Helper()
	var appErr *apperr.Error
	require.ErrorAs(t, err, &appErr)
	return appErr.Code
}

func newService(repo *fakeRepo) *service.Service {
	return service.New(service.Deps{
		Repo:  repo,
		Users: fakeUsers{timezone: "UTC"},
		Clock: fixedClock{at: time.Date(2026, time.March, 2, 12, 0, 0, 0, time.UTC)},
		// No Pool and no Events: publishing is a no-op, which is what these
		// tests want. What is published is asserted through the consumer's
		// behaviour, not by intercepting the outbox.
	})
}

// ------------------------------------------------------------------- awards

func TestAward_PaysOnceForOneAction(t *testing.T) {
	repo := newFakeRepo()
	svc := newService(repo)
	user, activity := uuid.New(), uuid.New().String()

	first, err := svc.Award(context.Background(), contract.AwardRequest{
		UserID: user, Source: string(domain.SourceActivity), SourceID: activity, Score: perfectScore(),
	})
	require.NoError(t, err)
	assert.True(t, first.Awarded)
	assert.Equal(t, 10, first.Amount)

	// BR-GAMIFICATION-01: the same event redelivered.
	second, err := svc.Award(context.Background(), contract.AwardRequest{
		UserID: user, Source: string(domain.SourceActivity), SourceID: activity, Score: perfectScore(),
	})
	require.NoError(t, err)
	assert.False(t, second.Awarded, "a redelivered event must not award again")
	assert.Equal(t, int64(10), second.TotalXP, "and must not move the total")
}

func TestAward_RefusesAnAwardWithNoIdempotencyKey(t *testing.T) {
	svc := newService(newFakeRepo())
	_, err := svc.Award(context.Background(), contract.AwardRequest{
		UserID: uuid.New(), Source: string(domain.SourceActivity),
	})

	require.Error(t, err)
	assert.Equal(t, "GAMIFICATION_SOURCE_ID_REQUIRED", codeOf(t, err))
}

func TestAward_AnUnknownSourceIsNotAnError(t *testing.T) {
	// A module publishing an event gamification has no rate for must not have
	// its action fail.
	svc := newService(newFakeRepo())
	outcome, err := svc.Award(context.Background(), contract.AwardRequest{
		UserID: uuid.New(), Source: "something_new", SourceID: "x",
	})

	require.NoError(t, err)
	assert.False(t, outcome.Awarded)
}

func TestAward_StopsAtTheDailyCapWithoutFailing(t *testing.T) {
	repo := newFakeRepo()
	svc := newService(repo)
	user := uuid.New()

	// Fill the activity cap.
	repo.events = append(repo.events, sqlc.LearnXpEvent{
		UserID: user, Source: string(domain.SourceActivity), SourceID: "prior",
		//nolint:gosec // a daily cap is a small authored constant
		Amount: int32(domain.DailyCap(domain.SourceActivity)),
	})

	outcome, err := svc.Award(context.Background(), contract.AwardRequest{
		UserID: user, Source: string(domain.SourceActivity), SourceID: uuid.New().String(), Score: perfectScore(),
	})
	require.NoError(t, err, "a capped award is information, not a failure")
	assert.False(t, outcome.Awarded)
	assert.True(t, outcome.Capped)
	assert.Equal(t, 0, outcome.Amount)
}

func TestAward_ReportsALevelUp(t *testing.T) {
	repo := newFakeRepo()
	svc := newService(repo)
	user := uuid.New()

	// 95 XP: one lesson award of 40 crosses the level-2 threshold at 100.
	repo.events = append(repo.events, sqlc.LearnXpEvent{
		UserID: user, Source: "seed", SourceID: "seed", Amount: 95,
	})

	outcome, err := svc.Award(context.Background(), contract.AwardRequest{
		UserID: user, Source: string(domain.SourceLesson), SourceID: uuid.New().String(),
	})
	require.NoError(t, err)
	assert.True(t, outcome.LevelUp)
	assert.Equal(t, 2, outcome.Level)
}

// ------------------------------------------------------------------ badges

func TestAward_EarnsABadgeExactlyOnce(t *testing.T) {
	repo := newFakeRepo()
	badgeID := uuid.New()
	repo.badges = []sqlc.LearnBadge{{
		ID: badgeID, Code: "first_steps", Name: "First Steps", Tier: "bronze",
		Criteria: json.RawMessage(`{"kind":"xp_total","threshold":10}`),
	}}
	svc := newService(repo)
	user := uuid.New()

	for i := 0; i < 3; i++ {
		_, err := svc.Award(context.Background(), contract.AwardRequest{
			UserID: user, Source: string(domain.SourceActivity), SourceID: uuid.New().String(), Score: perfectScore(),
		})
		require.NoError(t, err)
	}

	// BR-GAMIFICATION-06. The evaluator ran three times; the badge is held once.
	assert.True(t, repo.earned[badgeID])
	earned, err := repo.ListEarnedBadges(context.Background(), user)
	require.NoError(t, err)
	assert.Len(t, earned, 1)
}

func TestAward_ABadgeWithAnUnknownCriteriaKindIsNotAwarded(t *testing.T) {
	repo := newFakeRepo()
	badgeID := uuid.New()
	repo.badges = []sqlc.LearnBadge{{
		ID: badgeID, Code: "mystery", Criteria: json.RawMessage(`{"kind":"invented","threshold":1}`),
	}}
	svc := newService(repo)

	_, err := svc.Award(context.Background(), contract.AwardRequest{
		UserID: uuid.New(), Source: string(domain.SourceActivity), SourceID: "a", Score: perfectScore(),
	})
	require.NoError(t, err, "an unearnable badge must not fail the award")
	assert.False(t, repo.earned[badgeID])
}

// ----------------------------------------------------------------- streaks

func TestRecordActivity_ExtendsTheStreakOnlyOnceTheGoalIsMet(t *testing.T) {
	repo := newFakeRepo()
	repo.streak.DailyGoalXp = 30 // Three activity awards.
	svc := newService(repo)
	user := uuid.New()

	// 10 XP: short of the goal.
	_, err := svc.RecordActivity(context.Background(), contract.AwardRequest{
		UserID: user, Source: string(domain.SourceActivity), SourceID: uuid.New().String(), Score: perfectScore(),
	})
	require.NoError(t, err)
	assert.Zero(t, repo.extendCalls, "BR-GAMIFICATION-03: opening the app is not a streak day")

	// Two more clears it.
	for i := 0; i < 2; i++ {
		_, err := svc.RecordActivity(context.Background(), contract.AwardRequest{
			UserID: user, Source: string(domain.SourceActivity), SourceID: uuid.New().String(), Score: perfectScore(),
		})
		require.NoError(t, err)
	}
	assert.Positive(t, repo.extendCalls)
	assert.Equal(t, int32(1), repo.streak.CurrentLength)
}

func TestRecordActivity_DoesNotExtendTwiceInOneDay(t *testing.T) {
	repo := newFakeRepo()
	repo.streak.DailyGoalXp = 10 // One award clears it.
	svc := newService(repo)
	user := uuid.New()

	for i := 0; i < 4; i++ {
		_, err := svc.RecordActivity(context.Background(), contract.AwardRequest{
			UserID: user, Source: string(domain.SourceActivity), SourceID: uuid.New().String(), Score: perfectScore(),
		})
		require.NoError(t, err)
	}
	assert.Equal(t, int32(1), repo.streak.CurrentLength,
		"four sessions in one day is one day of streak")
}

func TestUseFreeze_RefusesWhenNoneRemain(t *testing.T) {
	repo := newFakeRepo()
	repo.streak.FreezesAvailable = 0
	svc := newService(repo)

	_, err := svc.UseFreeze(context.Background(), uuid.New())
	require.Error(t, err)
	assert.Equal(t, "NO_FREEZES_AVAILABLE", codeOf(t, err))
}

func TestUseFreeze_SpendsOneAndOnlyOnePerDay(t *testing.T) {
	repo := newFakeRepo()
	svc := newService(repo)
	user := uuid.New()

	_, err := svc.UseFreeze(context.Background(), user)
	require.NoError(t, err)
	assert.Equal(t, 1, repo.freezes)

	_, err = svc.UseFreeze(context.Background(), user)
	require.Error(t, err, "a freeze already spent today cannot be spent again")
	assert.Equal(t, 1, repo.freezes)
}

func TestSetDailyGoal_RejectsAnImpossibleGoal(t *testing.T) {
	svc := newService(newFakeRepo())

	require.Error(t, svc.SetDailyGoal(context.Background(), uuid.New(), 0))
	require.Error(t, svc.SetDailyGoal(context.Background(), uuid.New(), 5000))
	require.NoError(t, svc.SetDailyGoal(context.Background(), uuid.New(), 100))
}

// -------------------------------------------------------------- the sweep

func TestSweepStreaks_SpendsAFreezeBeforeBreakingAStreak(t *testing.T) {
	repo := newFakeRepo()
	repo.streak.CurrentLength = 9
	repo.streak.FreezesAvailable = 1
	// Last active two days ago: broken unless a freeze covers it.
	repo.streak.LastActiveOn = pgtype.Date{
		Time: time.Date(2026, time.February, 28, 0, 0, 0, 0, time.UTC), Valid: true,
	}
	svc := newService(repo)

	require.NoError(t, svc.SweepStreaks(context.Background()))
	assert.Equal(t, 1, repo.freezes, "BR-GAMIFICATION-04: the freeze is spent automatically")
	assert.False(t, repo.broken)
	assert.Equal(t, int32(9), repo.streak.CurrentLength)
}

func TestSweepStreaks_BreaksAStreakWithNoFreezeLeft(t *testing.T) {
	repo := newFakeRepo()
	repo.streak.CurrentLength = 9
	repo.streak.FreezesAvailable = 0
	repo.streak.LastActiveOn = pgtype.Date{
		Time: time.Date(2026, time.February, 28, 0, 0, 0, 0, time.UTC), Valid: true,
	}
	svc := newService(repo)

	require.NoError(t, svc.SweepStreaks(context.Background()))
	assert.True(t, repo.broken)
	assert.Zero(t, repo.streak.CurrentLength)
}

func TestSweepStreaks_LeavesALiveStreakAlone(t *testing.T) {
	repo := newFakeRepo()
	repo.streak.CurrentLength = 4
	// Active yesterday: today is still live.
	repo.streak.LastActiveOn = pgtype.Date{
		Time: time.Date(2026, time.March, 1, 0, 0, 0, 0, time.UTC), Valid: true,
	}
	svc := newService(repo)

	require.NoError(t, svc.SweepStreaks(context.Background()))
	assert.False(t, repo.broken)
	assert.Zero(t, repo.freezes, "a live streak must not cost a freeze")
	assert.Equal(t, int32(4), repo.streak.CurrentLength)
}

// ---------------------------------------------------------------- consumer

const testSkillVocabulary = "vocabulary"

func TestConsume_ActivityCompletedAwardsOncePerActivity(t *testing.T) {
	repo := newFakeRepo()
	svc := newService(repo)
	user, activity := uuid.New(), uuid.New()

	payload, err := json.Marshal(learningcontract.ActivityCompleted{
		UserID: user, ActivityID: activity, Score: 100, Skill: testSkillVocabulary,
	})
	require.NoError(t, err)

	delivery := service.Delivery{
		ID: uuid.New(), Topic: learningcontract.EventActivityCompleted, Payload: payload,
	}
	require.NoError(t, svc.Consume(context.Background(), delivery))

	// Redelivered under a *new* outbox id — which is what at-least-once looks
	// like — and keyed on the activity, so it still does not double-award.
	delivery.ID = uuid.New()
	require.NoError(t, svc.Consume(context.Background(), delivery))

	total, err := repo.TotalXP(context.Background(), user)
	require.NoError(t, err)
	assert.Equal(t, int64(10), total)
}

func TestActivity_GradedHighWaterMark(t *testing.T) {
	repo := newFakeRepo()
	svc := newService(repo)
	user, activity := uuid.New(), uuid.New()

	// 1. 80/100 grants 8
	payload80, err := json.Marshal(learningcontract.ActivityCompleted{
		UserID: user, ActivityID: activity, Score: 80, Skill: testSkillVocabulary,
	})
	require.NoError(t, err)
	require.NoError(t, svc.Consume(context.Background(), service.Delivery{
		ID: uuid.New(), Topic: learningcontract.EventActivityCompleted, Payload: payload80,
	}))
	total, err := repo.TotalXP(context.Background(), user)
	require.NoError(t, err)
	assert.Equal(t, int64(8), total)

	// 2. Redelivered 80/100 grants 0 (still 8)
	require.NoError(t, svc.Consume(context.Background(), service.Delivery{
		ID: uuid.New(), Topic: learningcontract.EventActivityCompleted, Payload: payload80,
	}))
	total, err = repo.TotalXP(context.Background(), user)
	require.NoError(t, err)
	assert.Equal(t, int64(8), total)

	// 3. Retaking to 100/100 grants 2, not 10 (total becomes 10)
	payload100, err := json.Marshal(learningcontract.ActivityCompleted{
		UserID: user, ActivityID: activity, Score: 100, Skill: testSkillVocabulary,
	})
	require.NoError(t, err)
	require.NoError(t, svc.Consume(context.Background(), service.Delivery{
		ID: uuid.New(), Topic: learningcontract.EventActivityCompleted, Payload: payload100,
	}))
	total, err = repo.TotalXP(context.Background(), user)
	require.NoError(t, err)
	assert.Equal(t, int64(10), total)

	// 4. A third attempt at 100/100 grants nothing
	require.NoError(t, svc.Consume(context.Background(), service.Delivery{
		ID: uuid.New(), Topic: learningcontract.EventActivityCompleted, Payload: payload100,
	}))
	total, err = repo.TotalXP(context.Background(), user)
	require.NoError(t, err)
	assert.Equal(t, int64(10), total)

	// 5. A worse retake (70/100) grants 0 and takes nothing back
	payload70, err := json.Marshal(learningcontract.ActivityCompleted{
		UserID: user, ActivityID: activity, Score: 70, Skill: testSkillVocabulary,
	})
	require.NoError(t, err)
	require.NoError(t, svc.Consume(context.Background(), service.Delivery{
		ID: uuid.New(), Topic: learningcontract.EventActivityCompleted, Payload: payload70,
	}))
	total, err = repo.TotalXP(context.Background(), user)
	require.NoError(t, err)
	assert.Equal(t, int64(10), total)
}

func TestConsume_IgnoresAnUnknownTopic(t *testing.T) {
	svc := newService(newFakeRepo())
	require.NoError(t, svc.Consume(context.Background(), service.Delivery{
		ID: uuid.New(), Topic: "something.else", Payload: json.RawMessage(`{}`),
	}))
}

func TestConsume_AnUnreadablePayloadIsAcknowledged(t *testing.T) {
	// Returning an error nacks the message and asks for redelivery, and
	// redelivering a payload that will never parse is a loop, not a retry.
	svc := newService(newFakeRepo())
	require.NoError(t, svc.Consume(context.Background(), service.Delivery{
		ID:      uuid.New(),
		Topic:   learningcontract.EventActivityCompleted,
		Payload: json.RawMessage(`{"user_id": "not-a-uuid"`),
	}))
}

func TestConsume_AdvancesAndPaysAQuest(t *testing.T) {
	repo := newFakeRepo()
	userQuestID := uuid.New()
	repo.quests = []sqlc.ListOpenUserQuestsRow{{
		UserQuestID: userQuestID,
		Code:        "daily_three",
		Name:        "Three activities",
		Steps:       json.RawMessage(`[{"code":"complete_activities","target":2}]`),
		Progress:    json.RawMessage(`{}`),
		RewardXp:    30,
	}}
	svc := newService(repo)
	user := uuid.New()

	send := func() {
		payload, err := json.Marshal(learningcontract.ActivityCompleted{
			UserID: user, ActivityID: uuid.New(), Score: 100,
		})
		require.NoError(t, err)
		require.NoError(t, svc.Consume(context.Background(), service.Delivery{
			ID: uuid.New(), Topic: learningcontract.EventActivityCompleted, Payload: payload,
		}))
	}

	send()
	assert.Nil(t, repo.quests[0].CompletedAt, "one of two steps is not a completed quest")

	send()
	require.NotNil(t, repo.quests[0].CompletedAt)

	// 10 + 10 for the activities, 30 for the quest reward.
	total, err := repo.TotalXP(context.Background(), user)
	require.NoError(t, err)
	assert.Equal(t, int64(50), total)
}

// perfectScore is the graded result these tests assume when they are about
// something else -- the daily cap, a badge, a streak.
//
// A graded award has to carry one. The service used to invent a score when none
// was given, defaulting to a perfect 100, and that default was what kept these
// tests passing after the flat-10 rule was replaced: 100/10 is 10, the number
// they were originally written against. It also wrote best_score = 100 into a
// table whose whole purpose is that the value never falls, so an invented
// perfect score silenced every genuine attempt afterwards. Stating it here
// keeps the tests honest about what they are assuming.
func perfectScore() *int {
	score := 100
	return &score
}
