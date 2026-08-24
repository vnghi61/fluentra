package domain

import (
	"math"
	"strings"
	"time"

	"github.com/google/uuid"
)

// CEFR Level constants.
const (
	LevelA1 = "A1"
	LevelA2 = "A2"
	LevelB1 = "B1"
	LevelB2 = "B2"
	LevelC1 = "C1"
	LevelC2 = "C2"
)

// ValidSkills is the set of skills accepted by learn.skill_mastery.
var ValidSkills = map[string]struct{}{
	"vocabulary": {},
	"grammar":    {},
	"reading":    {},
	"listening":  {},
	"speaking":   {},
	"writing":    {},
}

// NormalizeSkill sanitizes and checks if a skill string matches a known skill domain.
// Unrecognized skills return ("", false) so the caller can safely skip mastery updates.
func NormalizeSkill(raw string) (string, bool) {
	norm := strings.ToLower(strings.TrimSpace(raw))
	if _, ok := ValidSkills[norm]; ok {
		return norm, true
	}
	return "", false
}

// SkillMastery models a learner's mastery level in an English competency skill.
type SkillMastery struct {
	ID         uuid.UUID `json:"id"`
	UserID     uuid.UUID `json:"user_id"`
	Skill      string    `json:"skill"`
	Level      string    `json:"level"`
	Confidence float64   `json:"confidence"`
	UpdatedAt  time.Time `json:"updated_at"`
	CreatedAt  time.Time `json:"created_at"`
}

// EstimateMastery implements BR-LEARNING-09:
// Exponentially weighted moving average over attempts where recent attempts dominate.
//
// If existing is nil (first attempt), the estimate is initialized directly to the attempt score.
// Otherwise, it updates: newEstimate = alpha * newScore + (1 - alpha) * currentEstimate.
//
// The result is projected onto the CEFR bands (A1-C2) and confidence is incremented towards 1.00.
func EstimateMastery(
	existing *SkillMastery, attemptScore float64,
) (level string, confidence float64, estimate float64) {
	const alpha = 0.35 // Recency weighting factor

	currentConf := 0.20
	estimate = attemptScore

	if existing != nil {
		// The prior estimate is read back from the stored band, because the band is
		// all learn.skill_mastery keeps — P8.2 created no column for a raw estimate
		// and P8.4 adds no column. The blend therefore starts from the band's
		// midpoint, which loses precision within a band but not direction.
		currentEstimate := ScoreFromCEFR(existing.Level)
		estimate = (alpha * attemptScore) + ((1.0 - alpha) * currentEstimate)
		currentConf = existing.Confidence + 0.10
	}

	if currentConf > 1.00 {
		currentConf = 1.00
	}
	confidence = math.Round(currentConf*100) / 100.0

	level = CEFRFromScore(estimate)
	return level, confidence, estimate
}

// CEFRFromScore maps a 0-100 score to the CEFR band.
func CEFRFromScore(score float64) string {
	switch {
	case score < 30.0:
		return LevelA1
	case score < 50.0:
		return LevelA2
	case score < 70.0:
		return LevelB1
	case score < 85.0:
		return LevelB2
	case score < 95.0:
		return LevelC1
	default:
		return LevelC2
	}
}

// ScoreFromCEFR returns the midpoint score for a CEFR band.
func ScoreFromCEFR(level string) float64 {
	switch level {
	case LevelA1:
		return 20.0
	case LevelA2:
		return 40.0
	case LevelB1:
		return 60.0
	case LevelB2:
		return 77.5
	case LevelC1:
		return 90.0
	case LevelC2:
		return 97.5
	default:
		return 50.0
	}
}
