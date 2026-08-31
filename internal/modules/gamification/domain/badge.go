package domain

import "encoding/json"

// BadgeCriteria is the authored unlock condition, stored as jsonb on the badge.
//
// One struct with a `kind` discriminator rather than a per-kind type, because
// the catalogue is data: adding a badge must be an insert, not a deploy. An
// unrecognised kind evaluates to false — a badge nobody can earn is a content
// fault to fix, and is strictly better than an evaluator that errors on every
// event for every learner (BR-GAMIFICATION-08).
type BadgeCriteria struct {
	Kind      string `json:"kind"`
	Threshold int    `json:"threshold"`
}

// The criteria kinds the evaluator understands.
const (
	CriteriaXPTotal       = "xp_total"
	CriteriaStreakLength  = "streak_length"
	CriteriaLevel         = "level"
	CriteriaWordsVerified = "words_verified"
)

// BadgeFacts is everything the evaluator may look at. A struct rather than a
// callback so evaluation is a pure function of a snapshot: the same facts
// always yield the same badges, which is what makes re-running it safe.
type BadgeFacts struct {
	TotalXP       int64
	Level         int
	StreakLength  int
	WordsVerified int
}

// ParseCriteria decodes an authored criteria blob. A blob that does not parse
// yields a zero criteria, which earns nothing.
func ParseCriteria(raw []byte) BadgeCriteria {
	var criteria BadgeCriteria
	if len(raw) == 0 {
		return criteria
	}
	_ = json.Unmarshal(raw, &criteria)
	return criteria
}

// Earned reports whether the facts satisfy the criteria.
//
// Idempotent by construction: it reads a snapshot and returns a boolean, so
// running it on every event is safe and the "award once" guarantee lives in the
// unique constraint rather than here (BR-GAMIFICATION-06).
func Earned(criteria BadgeCriteria, facts BadgeFacts) bool {
	if criteria.Threshold <= 0 {
		return false
	}
	switch criteria.Kind {
	case CriteriaXPTotal:
		return facts.TotalXP >= int64(criteria.Threshold)
	case CriteriaStreakLength:
		return facts.StreakLength >= criteria.Threshold
	case CriteriaLevel:
		return facts.Level >= criteria.Threshold
	case CriteriaWordsVerified:
		return facts.WordsVerified >= criteria.Threshold
	default:
		return false
	}
}

// QuestProgress counts a learner's progress through a quest's steps.
type QuestProgress map[string]int

// QuestStep is one authored requirement.
type QuestStep struct {
	Code   string `json:"code"`
	Target int    `json:"target"`
}

// ParseSteps decodes a quest's authored steps.
func ParseSteps(raw []byte) []QuestStep {
	var steps []QuestStep
	if len(raw) == 0 {
		return nil
	}
	_ = json.Unmarshal(raw, &steps)
	return steps
}

// ParseProgress decodes a learner's counters.
func ParseProgress(raw []byte) QuestProgress {
	progress := QuestProgress{}
	if len(raw) == 0 {
		return progress
	}
	_ = json.Unmarshal(raw, &progress)
	return progress
}

// QuestComplete reports whether every step has met its target. A quest with no
// steps is never complete: an empty step list is an authoring fault, and
// treating it as satisfied would pay its reward to everyone.
func QuestComplete(steps []QuestStep, progress QuestProgress) bool {
	if len(steps) == 0 {
		return false
	}
	for _, step := range steps {
		if step.Target <= 0 || progress[step.Code] < step.Target {
			return false
		}
	}
	return true
}
