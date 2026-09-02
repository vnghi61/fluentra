package main

import (
	"encoding/json"
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// The catalogue now exists twice, and only one of the two reaches a learner.
//
// `badgeSeedData` is what `cmd/seed` writes, and it is thoroughly checked: every
// badge must name a criteria kind the evaluator understands, carry a real tier
// and a threshold above zero. But `cmd/seed` refuses to run when APP_ENV is
// production, so what production actually gets is
// db/migrations/gamification/1700000300_seed_gamification_catalogues.sql -- and
// nothing looked at that file at all.
//
// That inverts the safety the existing tests provide: the copy under test is the
// one that does not matter, and the copy that ships is unexamined. A badge added
// to the migration with a criteria kind the evaluator does not evaluate is a
// badge no learner can ever earn, and it looks completely healthy in the
// database. That is the exact fault TestSeededBadges_UseCriteriaTheEvaluator
// Understands exists to prevent, aimed at the wrong file.
//
// So this reads the migration and holds it to the Go data. Keeping two copies is
// a decision -- a migration cannot call Go, and production needs SQL -- but
// letting them drift is not.

const catalogueMigrationPath = "../../db/migrations/gamification/1700000300_seed_gamification_catalogues.sql"

// badgeRowPattern matches one VALUES row of the badge insert:
//
//	('code', 'Name', 'Description', '{"kind": "...", "threshold": N}'::jsonb, 'tier')
//
// The description is skipped rather than compared, because it is prose and a
// wording change in one file is not the kind of drift that breaks a learner.
var badgeRowPattern = regexp.MustCompile(
	`\('([a-z_]+)',\s*'((?:[^']|'')*)',\s*'(?:[^']|'')*',\s*'(\{[^']*\})'::jsonb,\s*'([a-z]+)'\)`,
)

// questRowPattern matches one VALUES row of the quest insert.
var questRowPattern = regexp.MustCompile(
	`\('([a-z_]+)',\s*'((?:[^']|'')*)',\s*'(?:[^']|'')*',\s*'(\[[^']*\])'::jsonb,\s*(\d+),\s*(\d+),\s*(true|false)\)`,
)

func readCatalogueMigration(t *testing.T) string {
	t.Helper()
	body, err := os.ReadFile(catalogueMigrationPath)
	if err != nil {
		t.Fatalf("read catalogue migration: %v", err)
	}
	return string(body)
}

// unquote undoes SQL's doubled single quote.
func unquote(value string) string { return strings.ReplaceAll(value, "''", "'") }

func TestCatalogueMigration_MatchesTheSeededBadges(t *testing.T) {
	sql := readCatalogueMigration(t)

	matches := badgeRowPattern.FindAllStringSubmatch(sql, -1)
	if len(matches) != len(badgeSeedData) {
		t.Fatalf("migration declares %d badges, badgeSeedData has %d; one was added without the other",
			len(matches), len(badgeSeedData))
	}

	wanted := make(map[string]seedBadge, len(badgeSeedData))
	for _, badge := range badgeSeedData {
		wanted[badge.Code] = badge
	}

	for _, row := range matches {
		code, name, criteriaJSON, tier := row[1], unquote(row[2]), row[3], row[4]

		badge, ok := wanted[code]
		if !ok {
			t.Errorf("migration seeds badge %q, which badgeSeedData does not; it is never checked "+
				"against the evaluator", code)
			continue
		}
		if name != badge.Name {
			t.Errorf("badge %q: migration name %q, badgeSeedData %q", code, name, badge.Name)
		}
		if tier != badge.Tier {
			t.Errorf("badge %q: migration tier %q, badgeSeedData %q", code, tier, badge.Tier)
		}

		var criteria struct {
			Kind      string `json:"kind"`
			Threshold int    `json:"threshold"`
		}
		if err := json.Unmarshal([]byte(criteriaJSON), &criteria); err != nil {
			t.Errorf("badge %q: criteria is not valid JSON: %v", code, err)
			continue
		}
		if criteria.Kind != badge.CriteriaKind {
			t.Errorf("badge %q: migration criteria kind %q, badgeSeedData %q -- the evaluator only "+
				"understands the second, so this badge cannot be earned",
				code, criteria.Kind, badge.CriteriaKind)
		}
		if criteria.Threshold != badge.Threshold {
			t.Errorf("badge %q: migration threshold %d, badgeSeedData %d",
				code, criteria.Threshold, badge.Threshold)
		}
	}
}

func TestCatalogueMigration_MatchesTheSeededQuests(t *testing.T) {
	sql := readCatalogueMigration(t)

	matches := questRowPattern.FindAllStringSubmatch(sql, -1)
	if len(matches) != len(questSeedData) {
		t.Fatalf("migration declares %d quests, questSeedData has %d; one was added without the other",
			len(matches), len(questSeedData))
	}

	wanted := make(map[string]seedQuest, len(questSeedData))
	for _, quest := range questSeedData {
		wanted[quest.Code] = quest
	}

	for _, row := range matches {
		code, name, stepsJSON := row[1], unquote(row[2]), row[3]

		quest, ok := wanted[code]
		if !ok {
			t.Errorf("migration seeds quest %q, which questSeedData does not; its step codes are "+
				"never checked against the consumer that counts them", code)
			continue
		}
		if name != quest.Name {
			t.Errorf("quest %q: migration name %q, questSeedData %q", code, name, quest.Name)
		}

		windowDays, _ := strconv.Atoi(row[4])
		if windowDays != quest.WindowDays {
			t.Errorf("quest %q: migration window_days %d, questSeedData %d",
				code, windowDays, quest.WindowDays)
		}
		rewardXP, _ := strconv.Atoi(row[5])
		if rewardXP != quest.RewardXP {
			t.Errorf("quest %q: migration reward_xp %d, questSeedData %d", code, rewardXP, quest.RewardXP)
		}

		var steps []struct {
			Code   string `json:"code"`
			Target int    `json:"target"`
		}
		if err := json.Unmarshal([]byte(stepsJSON), &steps); err != nil {
			t.Errorf("quest %q: steps are not valid JSON: %v", code, err)
			continue
		}
		if len(steps) != len(quest.Steps) {
			t.Errorf("quest %q: migration has %d steps, questSeedData %d",
				code, len(steps), len(quest.Steps))
			continue
		}
		for _, step := range steps {
			target, known := quest.Steps[step.Code]
			if !known {
				t.Errorf("quest %q: migration step %q is not in questSeedData, so nothing checks that "+
					"the consumer counts it", code, step.Code)
				continue
			}
			if target != step.Target {
				t.Errorf("quest %q step %q: migration target %d, questSeedData %d",
					code, step.Code, step.Target, target)
			}
		}
	}
}
