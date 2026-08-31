package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/jackc/pgx/v5/pgxpool"
)

// The badge and quest catalogues are authored content, not learner data, so the
// seed upserts them: re-running it corrects the copy rather than failing on a
// unique code, and a deployment that has already run it is unchanged.

type seedBadge struct {
	Code        string
	Name        string
	Description string
	Tier        string
	// Criteria must use a kind the evaluator understands. An unknown kind
	// yields a badge nobody can earn, which TestSeededBadgesAreEvaluable
	// catches rather than leaving to be noticed by a learner who never gets it.
	CriteriaKind string
	Threshold    int
}

type seedQuest struct {
	Code        string
	Name        string
	Description string
	// Steps must use a step code the consumer counts, for the same reason.
	Steps      map[string]int
	WindowDays int
	RewardXP   int
}

// The criteria kinds and step codes the catalogues may use. They mirror
// gamification's domain constants, and the seed's tests assert they match —
// duplicated here rather than imported because `cmd/seed` writes SQL against
// the schema and must not depend on a module's internals.
const (
	criteriaXPTotal      = "xp_total"
	criteriaStreakLength = "streak_length"
	criteriaLevel        = "level"

	stepActivities = "complete_activities"
	stepLessons    = "complete_lessons"
	stepReviews    = "complete_reviews"

	// The tiers ck_badges_tier accepts.
	tierBronze   = "bronze"
	tierSilver   = "silver"
	tierGold     = "gold"
	tierPlatinum = "platinum"
)

var badgeSeedData = []seedBadge{
	{
		Code: "first_steps", Name: "First Steps",
		Description: "Earned your first 50 XP.",
		Tier:        tierBronze, CriteriaKind: criteriaXPTotal, Threshold: 50,
	},
	{
		Code: "getting_serious", Name: "Getting Serious",
		Description: "Reached 500 XP.",
		Tier:        tierBronze, CriteriaKind: criteriaXPTotal, Threshold: 500,
	},
	{
		Code: "two_thousand_club", Name: "Two Thousand Club",
		Description: "Reached 2,000 XP.",
		Tier:        tierSilver, CriteriaKind: criteriaXPTotal, Threshold: 2000,
	},
	{
		Code: "ten_thousand_club", Name: "Ten Thousand Club",
		Description: "Reached 10,000 XP.",
		Tier:        tierGold, CriteriaKind: criteriaXPTotal, Threshold: 10000,
	},
	{
		Code: "level_five", Name: "Level Five",
		Description: "Reached level 5.",
		Tier:        tierBronze, CriteriaKind: criteriaLevel, Threshold: 5,
	},
	{
		Code: "level_ten", Name: "Level Ten",
		Description: "Reached level 10.",
		Tier:        tierSilver, CriteriaKind: criteriaLevel, Threshold: 10,
	},
	{
		Code: "level_twenty", Name: "Level Twenty",
		Description: "Reached level 20.",
		Tier:        tierPlatinum, CriteriaKind: criteriaLevel, Threshold: 20,
	},
	{
		Code: "week_streak", Name: "Seven Days",
		Description: "Studied seven days in a row.",
		Tier:        tierBronze, CriteriaKind: criteriaStreakLength, Threshold: 7,
	},
	{
		Code: "month_streak", Name: "Thirty Days",
		Description: "Studied thirty days in a row.",
		Tier:        tierGold, CriteriaKind: criteriaStreakLength, Threshold: 30,
	},
	{
		Code: "hundred_streak", Name: "One Hundred Days",
		Description: "Studied a hundred days in a row.",
		Tier:        tierPlatinum, CriteriaKind: criteriaStreakLength, Threshold: 100,
	},
}

var questSeedData = []seedQuest{
	{
		Code: "daily_practice", Name: "Daily Practice",
		Description: "Complete three activities today.",
		Steps:       map[string]int{stepActivities: 3},
		WindowDays:  1, RewardXP: 30,
	},
	{
		Code: "daily_review", Name: "Keep It Fresh",
		Description: "Finish a review session today.",
		Steps:       map[string]int{stepReviews: 1},
		WindowDays:  1, RewardXP: 20,
	},
	{
		Code: "weekly_lessons", Name: "Steady Progress",
		Description: "Finish five lessons this week.",
		Steps:       map[string]int{stepLessons: 5},
		WindowDays:  7, RewardXP: 120,
	},
	{
		Code: "weekly_all_round", Name: "Well Rounded",
		Description: "Three lessons, ten activities and three review sessions this week.",
		Steps: map[string]int{
			stepLessons: 3, stepActivities: 10, stepReviews: 3,
		},
		WindowDays: 7, RewardXP: 200,
	},
}

// seedGamification writes the badge and quest catalogues.
//
// It touches no learner state: XP, streaks and earned badges belong to learners
// and are never seeded. A demo account that had badges it did not earn would
// make every screen showing them a lie.
func seedGamification(ctx context.Context, pool *pgxpool.Pool, out io.Writer) error {
	const upsertBadge = `
		INSERT INTO learn.badges (code, name, description, criteria, tier)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (code) DO UPDATE
		SET name = EXCLUDED.name,
		    description = EXCLUDED.description,
		    criteria = EXCLUDED.criteria,
		    tier = EXCLUDED.tier`

	for _, badge := range badgeSeedData {
		criteria, err := json.Marshal(map[string]any{
			"kind": badge.CriteriaKind, "threshold": badge.Threshold,
		})
		if err != nil {
			return fmt.Errorf("encode criteria for badge %s: %w", badge.Code, err)
		}
		if _, err := pool.Exec(ctx, upsertBadge,
			badge.Code, badge.Name, badge.Description, criteria, badge.Tier,
		); err != nil {
			return fmt.Errorf("upsert badge %s: %w", badge.Code, err)
		}
	}

	const upsertQuest = `
		INSERT INTO learn.quests (code, name, description, steps, window_days, reward_xp, active)
		VALUES ($1, $2, $3, $4, $5, $6, true)
		ON CONFLICT (code) DO UPDATE
		SET name = EXCLUDED.name,
		    description = EXCLUDED.description,
		    steps = EXCLUDED.steps,
		    window_days = EXCLUDED.window_days,
		    reward_xp = EXCLUDED.reward_xp,
		    active = true`

	for _, quest := range questSeedData {
		// Stored as a list of {code, target} objects, which is the shape
		// domain.ParseSteps reads. A map would be a second shape to support.
		steps := make([]map[string]any, 0, len(quest.Steps))
		for code, target := range quest.Steps {
			steps = append(steps, map[string]any{"code": code, "target": target})
		}
		encoded, err := json.Marshal(steps)
		if err != nil {
			return fmt.Errorf("encode steps for quest %s: %w", quest.Code, err)
		}
		if _, err := pool.Exec(ctx, upsertQuest,
			quest.Code, quest.Name, quest.Description, encoded, quest.WindowDays, quest.RewardXP,
		); err != nil {
			return fmt.Errorf("upsert quest %s: %w", quest.Code, err)
		}
	}

	_, _ = fmt.Fprintf(out, "  ✓ Gamification: %d badges and %d quests seeded\n",
		len(badgeSeedData), len(questSeedData))
	return nil
}
