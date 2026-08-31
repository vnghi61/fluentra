package main

import (
	"testing"

	"github.com/fluentra/fluentra/internal/modules/gamification/domain"
)

// The catalogues are data, and data that names a kind the evaluator does not
// know is a badge nobody can ever earn or a quest nobody can ever finish. Both
// look complete in the database and fail silently for every learner, which is
// exactly the class of fault worth a test rather than a review.

func TestSeededBadges_UseCriteriaTheEvaluatorUnderstands(t *testing.T) {
	known := map[string]bool{
		domain.CriteriaXPTotal:       true,
		domain.CriteriaStreakLength:  true,
		domain.CriteriaLevel:         true,
		domain.CriteriaWordsVerified: true,
	}

	seen := map[string]bool{}
	for _, badge := range badgeSeedData {
		if badge.Code == "" || badge.Name == "" {
			t.Errorf("badge %+v has no code or name", badge)
		}
		if seen[badge.Code] {
			t.Errorf("badge code %q is seeded twice; the unique constraint would reject the second", badge.Code)
		}
		seen[badge.Code] = true

		if !known[badge.CriteriaKind] {
			t.Errorf("badge %q uses criteria kind %q, which domain.Earned does not evaluate",
				badge.Code, badge.CriteriaKind)
		}
		if badge.Threshold <= 0 {
			t.Errorf("badge %q has threshold %d; a zero threshold is earned by everyone at once",
				badge.Code, badge.Threshold)
		}
		if badge.Tier != tierBronze && badge.Tier != tierSilver &&
			badge.Tier != tierGold && badge.Tier != tierPlatinum {
			t.Errorf("badge %q has tier %q, which ck_badges_tier rejects", badge.Code, badge.Tier)
		}
	}
}

func TestSeededBadges_AreActuallyEarnable(t *testing.T) {
	// Every badge must be reachable by some learner. A criteria the evaluator
	// parses but never satisfies is the same fault one layer down.
	generous := domain.BadgeFacts{
		TotalXP: 1_000_000, Level: 500, StreakLength: 3650, WordsVerified: 100_000,
	}
	for _, badge := range badgeSeedData {
		criteria := domain.BadgeCriteria{Kind: badge.CriteriaKind, Threshold: badge.Threshold}
		if !domain.Earned(criteria, generous) {
			t.Errorf("badge %q cannot be earned by any learner", badge.Code)
		}
	}
}

func TestSeededQuests_UseStepCodesTheConsumerCounts(t *testing.T) {
	// The step codes gamification's consumer increments. Written out rather
	// than imported because they are unexported constants of that package —
	// which is the point: if one is renamed there and not here, this fails.
	counted := map[string]bool{
		stepActivities: true,
		stepLessons:    true,
		stepReviews:    true,
	}

	seen := map[string]bool{}
	for _, quest := range questSeedData {
		if seen[quest.Code] {
			t.Errorf("quest code %q is seeded twice", quest.Code)
		}
		seen[quest.Code] = true

		if len(quest.Steps) == 0 {
			t.Errorf("quest %q has no steps; domain.QuestComplete never completes it", quest.Code)
		}
		for code, target := range quest.Steps {
			if !counted[code] {
				t.Errorf("quest %q counts step %q, which nothing in the consumer increments",
					quest.Code, code)
			}
			if target <= 0 {
				t.Errorf("quest %q step %q has target %d; a step with no target is never met",
					quest.Code, code, target)
			}
		}
		if quest.WindowDays <= 0 || quest.WindowDays > 365 {
			t.Errorf("quest %q has a %d-day window, which ck_quests_window rejects",
				quest.Code, quest.WindowDays)
		}
		if quest.RewardXP < 0 {
			t.Errorf("quest %q has a negative reward", quest.Code)
		}
	}
}

func TestSeededQuests_RewardsStayUnderTheDailyCap(t *testing.T) {
	// A quest whose reward exceeds the quest source's daily cap pays less than
	// it advertises, and the learner sees a number that does not match what
	// arrives.
	dailyCap := domain.DailyCap(domain.SourceQuest)
	for _, quest := range questSeedData {
		if quest.RewardXP > dailyCap {
			t.Errorf("quest %q advertises %d XP but the daily quest cap is %d",
				quest.Code, quest.RewardXP, dailyCap)
		}
	}
}

func TestSeedConstantsMatchTheDomain(t *testing.T) {
	// The seed writes SQL against the schema and must not import a module's
	// internals, so these constants are duplicated. That is only safe while
	// they agree.
	if criteriaXPTotal != domain.CriteriaXPTotal {
		t.Errorf("criteriaXPTotal is %q, domain says %q", criteriaXPTotal, domain.CriteriaXPTotal)
	}
	if criteriaStreakLength != domain.CriteriaStreakLength {
		t.Errorf("criteriaStreakLength is %q, domain says %q",
			criteriaStreakLength, domain.CriteriaStreakLength)
	}
	if criteriaLevel != domain.CriteriaLevel {
		t.Errorf("criteriaLevel is %q, domain says %q", criteriaLevel, domain.CriteriaLevel)
	}
}
