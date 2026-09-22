package domain

import (
	"time"

	"github.com/google/uuid"
)

// Constants for node mastery and prerequisite gating per WO 19 §I and §J.
const (
	// MinAttemptsForWeakNode is the minimum number of attempts before a node can be considered weak (Trap I.1).
	// A node with two attempts is not "weak", it is unknown.
	MinAttemptsForWeakNode = 3

	// MinAttemptsForMastery is the minimum number of attempts required before a node can be marked mastered (Stage J).
	// This threshold is a first guess to be tuned from data.
	MinAttemptsForMastery = 5

	// MasteryScoreThreshold is the threshold (0.8) at or above which a node is considered mastered (Stage J).
	// This threshold is a first guess to be tuned from data.
	MasteryScoreThreshold = 0.8

	// NodeMasteryAlpha is the exponentially weighted factor for updating node mastery (same as skill mastery).
	NodeMasteryAlpha = 0.35
)

// FoundationPathNode is one topic in a learner's foundation learning path,
// annotated with their mastery and indicating whether it is the next topic to study.
type FoundationPathNode struct {
	ID        uuid.UUID `json:"id"`
	Namespace string    `json:"namespace"`
	Code      string    `json:"code"`
	Label     string    `json:"label"`
	CEFRLevel *string   `json:"cefr_level,omitempty"`
	Attempts  int       `json:"attempts"`
	Score     float64   `json:"score"`
	Mastered  bool      `json:"mastered"`
	Next      bool      `json:"next"`
}

// LearnerFoundationPath is the response for GET /me/foundation/path?target=CODE.
type LearnerFoundationPath struct {
	Target    string               `json:"target"`
	Namespace string               `json:"namespace"`
	Items     []FoundationPathNode `json:"items"`
}

// NodeMastery tracks a learner's performance on a specific spine taxonomy node.
type NodeMastery struct {
	UserID     uuid.UUID  `json:"user_id"`
	NodeID     uuid.UUID  `json:"node_id"`
	Attempts   int        `json:"attempts"`
	Correct    int        `json:"correct"`
	Score      float64    `json:"score"`
	LastSeenAt *time.Time `json:"last_seen_at,omitempty"`
}

// CalculateNodeMasteryUpdate computes the updated attempts, correct count, and exponentially weighted score.
func CalculateNodeMasteryUpdate(
	existing *NodeMastery, isCorrect bool, attemptScore float64,
) (attempts, correct int, newScore float64) {
	if existing == nil {
		attempts = 1
		if isCorrect {
			correct = 1
		}
		newScore = attemptScore
		if newScore < 0 {
			newScore = 0
		} else if newScore > 1 {
			newScore = 1
		}
		return attempts, correct, newScore
	}

	attempts = existing.Attempts + 1
	correct = existing.Correct
	if isCorrect {
		correct++
	}

	newScore = (NodeMasteryAlpha * attemptScore) + ((1.0 - NodeMasteryAlpha) * existing.Score)
	if newScore < 0 {
		newScore = 0
	} else if newScore > 1 {
		newScore = 1
	}
	return attempts, correct, newScore
}

// IsNodeMastered checks if a node meets the mastery criteria (score >= 0.8 with at least 5 attempts).
func (m *NodeMastery) IsNodeMastered() bool {
	return m != nil && m.Attempts >= MinAttemptsForMastery && m.Score >= MasteryScoreThreshold
}

// IsNodePrerequisiteMet checks if a prerequisite node has been adequately met (at least 3 attempts and score >= 0.8).
func (m *NodeMastery) IsNodePrerequisiteMet() bool {
	return m != nil && m.Attempts >= MinAttemptsForWeakNode && m.Score >= MasteryScoreThreshold
}
