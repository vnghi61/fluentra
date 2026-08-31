package domain_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fluentra/fluentra/internal/modules/gamification/domain"
)

// Step codes, named because the linter is right that six copies of a string
// literal is a name waiting to be given.
const (
	stepLessons    = "complete_lessons"
	stepReviews    = "complete_reviews"
	stepActivities = "complete_activities"
)

func day(year int, month time.Month, d int) time.Time {
	return time.Date(year, month, d, 0, 0, 0, 0, time.UTC)
}

// ---------------------------------------------------------------- XP and caps

func TestCalculateAward_PaysTheBaseRateWhenNothingLimitsIt(t *testing.T) {
	award := domain.CalculateAward(domain.SourceActivity, 10, 0, 0)
	assert.Equal(t, 10, award.Amount)
	assert.False(t, award.Capped)
}

func TestCalculateAward_HalvesAfterTenAwardsFromOneSource(t *testing.T) {
	// BR-GAMIFICATION-05. Drilling the same deck still counts, but not as much
	// as breadth does.
	full := domain.CalculateAward(domain.SourceActivity, 10, 0, 9)
	assert.Equal(t, 10, full.Amount)

	diminished := domain.CalculateAward(domain.SourceActivity, 10, 0, 10)
	assert.Equal(t, 5, diminished.Amount)
}

func TestCalculateAward_NeverDiminishesBelowOne(t *testing.T) {
	// A base of 1 halved is 0, and paying zero for real work reads to the
	// learner as the feature being broken.
	award := domain.CalculateAward(domain.SourceActivity, 1, 0, 50)
	assert.Equal(t, 1, award.Amount)
}

func TestCalculateAward_TrimsTheLastAwardToTheCap(t *testing.T) {
	dailyCap := domain.DailyCap(domain.SourceActivity)
	award := domain.CalculateAward(domain.SourceActivity, 10, dailyCap-4, 0)

	// Four left, so four is paid — not ten, and not nothing.
	assert.Equal(t, 4, award.Amount)
	assert.True(t, award.Capped)
}

func TestCalculateAward_PaysNothingOnceTheCapIsReached(t *testing.T) {
	award := domain.CalculateAward(
		domain.SourceActivity, 10, domain.DailyCap(domain.SourceActivity), 0)
	assert.Equal(t, 0, award.Amount)
	assert.True(t, award.Capped, "the caller reports this as DAILY_XP_CAP_REACHED, not as a failure")
}

func TestCalculateAward_AnUnknownSourcePaysNothing(t *testing.T) {
	assert.Equal(t, 0, domain.CalculateAward("something_new", 10, 0, 0).Amount)
}

func TestEverySourceThatPaysAlsoHasACap(t *testing.T) {
	// A source with a base rate and no cap is uncapped, which is the anti-farm
	// rule silently not applying to it.
	sources := []domain.Source{
		domain.SourceActivity, domain.SourceLesson, domain.SourceReviewSession,
		domain.SourceUploadVerified, domain.SourceDailyGoal,
	}
	for _, source := range sources {
		t.Run(string(source), func(t *testing.T) {
			require.Positive(t, domain.BaseAward(source))
			assert.Positive(t, domain.DailyCap(source))
		})
	}
}

// ------------------------------------------------------------------- levels

func TestLevelFor_StartsEveryLearnerAtLevelOne(t *testing.T) {
	assert.Equal(t, 1, domain.LevelFor(0))
	assert.Equal(t, 1, domain.LevelFor(99))
	assert.Equal(t, 1, domain.LevelFor(-5), "a negative total is nonsense, not level zero")
}

func TestLevelFor_ClimbsAndTheStepsGrow(t *testing.T) {
	assert.Equal(t, 2, domain.LevelFor(100))
	assert.Equal(t, 3, domain.LevelFor(300))
	assert.Equal(t, 4, domain.LevelFor(600))

	// Each level costs more than the last, which is the whole point of the curve.
	first := domain.XPForLevel(3) - domain.XPForLevel(2)
	second := domain.XPForLevel(4) - domain.XPForLevel(3)
	assert.Greater(t, second, first)
}

func TestXPForLevel_IsTheInverseOfLevelFor(t *testing.T) {
	for level := 1; level <= 25; level++ {
		start := domain.XPForLevel(level)
		assert.Equal(t, level, domain.LevelFor(start),
			"the XP at which a level begins must resolve back to that level")
		if level > 1 {
			assert.Equal(t, level-1, domain.LevelFor(start-1),
				"one XP short of the threshold is still the previous level")
		}
	}
}

// ------------------------------------------------------------------ streaks

func TestRecordActiveDay_FirstEverDayStartsAtOne(t *testing.T) {
	result := domain.RecordActiveDay(domain.StreakState{}, day(2026, time.March, 1))
	assert.Equal(t, domain.StreakExtended, result.Outcome)
	assert.Equal(t, 1, result.NewLength)
}

func TestRecordActiveDay_ConsecutiveDayExtends(t *testing.T) {
	state := domain.StreakState{CurrentLength: 4, LastActiveOn: day(2026, time.March, 1)}
	result := domain.RecordActiveDay(state, day(2026, time.March, 2))

	assert.Equal(t, domain.StreakExtended, result.Outcome)
	assert.Equal(t, 5, result.NewLength)
}

func TestRecordActiveDay_SameDayChangesNothing(t *testing.T) {
	// Two review sessions in one day is one day of streak, not two.
	state := domain.StreakState{CurrentLength: 4, LastActiveOn: day(2026, time.March, 2)}
	result := domain.RecordActiveDay(state, day(2026, time.March, 2))

	assert.Equal(t, domain.StreakUnchanged, result.Outcome)
	assert.Equal(t, 4, result.NewLength)
}

func TestRecordActiveDay_AGapRestartsAtOne(t *testing.T) {
	state := domain.StreakState{CurrentLength: 30, LastActiveOn: day(2026, time.March, 1)}
	result := domain.RecordActiveDay(state, day(2026, time.March, 5))

	assert.Equal(t, domain.StreakRestarted, result.Outcome)
	assert.Equal(t, 1, result.NewLength,
		"the learner studied today, so today counts — restarting at 0 shows no streak on a day they worked")
}

func TestRecordActiveDay_TravellingWestDoesNotBreakAStreak(t *testing.T) {
	// BR-GAMIFICATION-02. A learner who flies from Sydney to Los Angeles can
	// produce a local day earlier than the one already recorded. That is not a
	// missed day.
	state := domain.StreakState{CurrentLength: 12, LastActiveOn: day(2026, time.March, 3)}
	result := domain.RecordActiveDay(state, day(2026, time.March, 2))

	assert.Equal(t, domain.StreakUnchanged, result.Outcome)
	assert.Equal(t, 12, result.NewLength)
}

func TestStreakIsBroken_SurvivesTheWholeOfTheFollowingDay(t *testing.T) {
	state := domain.StreakState{CurrentLength: 3, LastActiveOn: day(2026, time.March, 1)}

	assert.False(t, domain.StreakIsBroken(state, day(2026, time.March, 1)))
	assert.False(t, domain.StreakIsBroken(state, day(2026, time.March, 2)),
		"the day after is still live; the learner may yet study in it")
	assert.True(t, domain.StreakIsBroken(state, day(2026, time.March, 3)))
}

func TestStreakIsBroken_NothingToBreakWithoutAStreak(t *testing.T) {
	assert.False(t, domain.StreakIsBroken(domain.StreakState{}, day(2026, time.March, 3)))
	assert.False(t, domain.StreakIsBroken(
		domain.StreakState{CurrentLength: 0, LastActiveOn: day(2026, time.January, 1)},
		day(2026, time.March, 3)))
}

func TestLocalDay_ResolvesInTheLearnersOwnZone(t *testing.T) {
	// 22:00 UTC on 1 March is already 05:00 on 2 March in Ho Chi Minh City.
	// Computing the day in UTC puts that learner a day behind.
	at := time.Date(2026, time.March, 1, 22, 0, 0, 0, time.UTC)

	assert.Equal(t, day(2026, time.March, 1), domain.LocalDay(at, "UTC"))
	assert.Equal(t, day(2026, time.March, 2), domain.LocalDay(at, "Asia/Ho_Chi_Minh"))
}

func TestLocalDay_FallsBackToUTCRatherThanFailing(t *testing.T) {
	// BR-GAMIFICATION-08: a bad timezone preference must not stop XP being
	// awarded.
	at := time.Date(2026, time.March, 1, 12, 0, 0, 0, time.UTC)
	assert.Equal(t, day(2026, time.March, 1), domain.LocalDay(at, ""))
	assert.Equal(t, day(2026, time.March, 1), domain.LocalDay(at, "Not/AZone"))
}

func TestHoursUntilStreakLost(t *testing.T) {
	state := domain.StreakState{CurrentLength: 5, LastActiveOn: day(2026, time.March, 1)}

	t.Run("nothing at risk once today is recorded", func(t *testing.T) {
		now := time.Date(2026, time.March, 1, 10, 0, 0, 0, time.UTC)
		assert.Zero(t, domain.HoursUntilStreakLost(state, now, "UTC"))
	})

	t.Run("counts down through the day after", func(t *testing.T) {
		now := time.Date(2026, time.March, 2, 20, 0, 0, 0, time.UTC)
		assert.Equal(t, 3, domain.HoursUntilStreakLost(state, now, "UTC"))
	})

	t.Run("zero once already lost", func(t *testing.T) {
		now := time.Date(2026, time.March, 4, 10, 0, 0, 0, time.UTC)
		assert.Zero(t, domain.HoursUntilStreakLost(state, now, "UTC"))
	})
}

// ------------------------------------------------------- leagues and weeks

func TestLeague_BandsByWeeklyEffort(t *testing.T) {
	assert.Equal(t, "bronze", domain.League(0))
	assert.Equal(t, "bronze", domain.League(249))
	assert.Equal(t, "silver", domain.League(250))
	assert.Equal(t, "gold", domain.League(800))
	assert.Equal(t, "diamond", domain.League(2000))
}

func TestWeekStart_IsAlwaysTheMonday(t *testing.T) {
	// 4 March 2026 is a Wednesday; the Monday is the 2nd.
	assert.Equal(t, day(2026, time.March, 2), domain.WeekStart(day(2026, time.March, 4)))
	assert.Equal(t, day(2026, time.March, 2), domain.WeekStart(day(2026, time.March, 2)))
	// Sunday belongs to the week that started six days earlier, not the next one.
	assert.Equal(t, day(2026, time.March, 2), domain.WeekStart(day(2026, time.March, 8)))
}

// ------------------------------------------------------------------ badges

func TestEarned_MatchesEachCriteriaKind(t *testing.T) {
	facts := domain.BadgeFacts{TotalXP: 1000, Level: 5, StreakLength: 7, WordsVerified: 40}

	assert.True(t, domain.Earned(domain.BadgeCriteria{Kind: domain.CriteriaXPTotal, Threshold: 1000}, facts))
	assert.False(t, domain.Earned(domain.BadgeCriteria{Kind: domain.CriteriaXPTotal, Threshold: 1001}, facts))
	assert.True(t, domain.Earned(domain.BadgeCriteria{Kind: domain.CriteriaLevel, Threshold: 5}, facts))
	assert.True(t, domain.Earned(domain.BadgeCriteria{Kind: domain.CriteriaStreakLength, Threshold: 7}, facts))
	assert.True(t, domain.Earned(domain.BadgeCriteria{Kind: domain.CriteriaWordsVerified, Threshold: 40}, facts))
}

func TestEarned_AnUnknownKindAwardsNothing(t *testing.T) {
	// A badge nobody can earn is a content fault to fix. An evaluator that
	// errors on every event for every learner is an outage.
	facts := domain.BadgeFacts{TotalXP: 1_000_000}
	assert.False(t, domain.Earned(domain.BadgeCriteria{Kind: "invented", Threshold: 1}, facts))
	assert.False(t, domain.Earned(domain.BadgeCriteria{Kind: domain.CriteriaXPTotal}, facts),
		"a zero threshold would be earned by everyone the moment the badge is created")
}

func TestParseCriteria_SurvivesRubbish(t *testing.T) {
	assert.Equal(t, domain.BadgeCriteria{}, domain.ParseCriteria(nil))
	assert.Equal(t, domain.BadgeCriteria{}, domain.ParseCriteria([]byte("not json")))
	assert.Equal(t,
		domain.BadgeCriteria{Kind: domain.CriteriaLevel, Threshold: 10},
		domain.ParseCriteria([]byte(`{"kind":"level","threshold":10}`)))
}

// ------------------------------------------------------------------ quests

func TestQuestComplete_NeedsEveryStep(t *testing.T) {
	steps := []domain.QuestStep{
		{Code: stepLessons, Target: 2},
		{Code: stepReviews, Target: 1},
	}

	assert.False(t, domain.QuestComplete(steps, domain.QuestProgress{stepLessons: 2}))
	assert.True(t, domain.QuestComplete(steps, domain.QuestProgress{
		stepLessons: 2, stepReviews: 1,
	}))
	assert.True(t, domain.QuestComplete(steps, domain.QuestProgress{
		stepLessons: 5, stepReviews: 3,
	}), "overshooting a target still completes it")
}

func TestQuestComplete_AStepListIsRequired(t *testing.T) {
	// An empty step list is an authoring fault. Treating it as satisfied would
	// pay the reward to everyone who was handed the quest.
	assert.False(t, domain.QuestComplete(nil, domain.QuestProgress{}))
	assert.False(t, domain.QuestComplete([]domain.QuestStep{}, domain.QuestProgress{"x": 9}))
}

func TestQuestComplete_AStepWithNoTargetIsNeverMet(t *testing.T) {
	steps := []domain.QuestStep{{Code: stepLessons, Target: 0}}
	assert.False(t, domain.QuestComplete(steps, domain.QuestProgress{stepLessons: 100}))
}

func TestQuestAward_NeverTakesXPAway(t *testing.T) {
	assert.Equal(t, 0, domain.QuestAward(-50))
	assert.Equal(t, 75, domain.QuestAward(75))
}
