// Package contract exposes the public interfaces, DTOs and event payloads of
// the gamification module. It is the only package other modules may import.
package contract

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// Aggregate is the outbox aggregate name every event below is written under.
const Aggregate = "gamification"

// Events published by this module.
const (
	EventXPAwarded     = "gamification.xp_awarded"
	EventLevelUp       = "gamification.level_up"
	EventBadgeEarned   = "gamification.badge_earned"
	EventStreakAtRisk  = "gamification.streak_at_risk"
	EventStreakBroken  = "gamification.streak_broken"
	EventQuestComplete = "gamification.quest_completed"
)

// XPAwarded is published when a learner is actually paid XP.
//
// Not published when an award is capped to zero or deduplicated away: a
// consumer counting these is counting real awards, and an event for "nothing
// happened" is a notification a learner should never receive.
type XPAwarded struct {
	UserID     uuid.UUID `json:"user_id"`
	Amount     int       `json:"amount"`
	Source     string    `json:"source"`
	SourceID   string    `json:"source_id"`
	TotalXP    int64     `json:"total_xp"`
	OccurredAt time.Time `json:"occurred_at"`
}

// LevelUp is published when an award carries a learner over a level boundary.
type LevelUp struct {
	UserID     uuid.UUID `json:"user_id"`
	Level      int       `json:"level"`
	OccurredAt time.Time `json:"occurred_at"`
}

// BadgeEarned is published once per learner per badge.
type BadgeEarned struct {
	UserID     uuid.UUID `json:"user_id"`
	BadgeCode  string    `json:"badge_code"`
	BadgeName  string    `json:"badge_name"`
	OccurredAt time.Time `json:"occurred_at"`
}

// StreakAtRisk drives the reminder `notification` sends. Quiet hours are that
// module's business, not this one's (BR-GAMIFICATION-09).
type StreakAtRisk struct {
	UserID         uuid.UUID `json:"user_id"`
	CurrentLength  int       `json:"current_length"`
	HoursRemaining int       `json:"hours_remaining"`
	OccurredAt     time.Time `json:"occurred_at"`
}

// StreakBroken is published when the sweep finds a lapsed streak.
type StreakBroken struct {
	UserID         uuid.UUID `json:"user_id"`
	PreviousLength int       `json:"previous_length"`
	OccurredAt     time.Time `json:"occurred_at"`
}

// QuestCompleted is published when every step of a quest is met.
type QuestCompleted struct {
	UserID     uuid.UUID `json:"user_id"`
	QuestCode  string    `json:"quest_code"`
	RewardXP   int       `json:"reward_xp"`
	OccurredAt time.Time `json:"occurred_at"`
}

// Badge is one entry in a learner's collection.
type Badge struct {
	Code        string    `json:"code"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Tier        string    `json:"tier"`
	EarnedAt    time.Time `json:"earned_at"`
}

// Streak is a learner's streak with everything the screen needs to explain it.
type Streak struct {
	Current          int        `json:"current"`
	Longest          int        `json:"longest"`
	LastActiveOn     *time.Time `json:"last_active_on,omitempty"`
	FreezesAvailable int        `json:"freezes_available"`
	// HoursRemaining is how long the learner has left today to keep the streak.
	// Zero when nothing is at risk — they have already studied, or have no
	// streak to lose.
	HoursRemaining int `json:"hours_remaining"`
}

// QuestSummary is one quest in flight.
type QuestSummary struct {
	Code        string         `json:"code"`
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Progress    map[string]int `json:"progress"`
	Steps       map[string]int `json:"steps"`
	RewardXP    int            `json:"reward_xp"`
	ExpiresOn   time.Time      `json:"expires_on"`
}

// Summary is the whole of a learner's gamification state in one read.
//
// One DTO rather than several because the dashboard needs all of it at once,
// and `notification` composing a message needs the same. Splitting it would
// make the common case several round trips.
type Summary struct {
	UserID       uuid.UUID      `json:"user_id"`
	TotalXP      int64          `json:"total_xp"`
	Level        int            `json:"level"`
	LevelStartXP int64          `json:"level_start_xp"`
	NextLevelXP  int64          `json:"next_level_xp"`
	XPToday      int64          `json:"xp_today"`
	DailyGoalXP  int            `json:"daily_goal_xp"`
	Streak       Streak         `json:"streak"`
	Badges       []Badge        `json:"badges"`
	Quests       []QuestSummary `json:"quests"`
	League       string         `json:"league"`
}

// AwardRequest asks for XP to be paid for one action.
//
// `SourceID` is the idempotency key's second half and is required: an award
// without one cannot be deduplicated, and BR-GAMIFICATION-01 has no exceptions.
type AwardRequest struct {
	UserID   uuid.UUID
	Source   string
	SourceID string
	// Score is populated for graded items (SourceActivity) to calculate high-water delta awards.
	Score *int
	// Amount overrides the source's base rate when positive. Used by quests,
	// whose reward is authored per quest, and by the verification job, which
	// pays per word.
	Amount int
}

// AwardOutcome reports what an award actually did.
type AwardOutcome struct {
	// Awarded is false for a duplicate or a fully capped award. Both are normal.
	Awarded bool
	Amount  int
	// Capped is true when the daily cap decided the amount. Surfaced as
	// DAILY_XP_CAP_REACHED, which is information rather than a failure.
	Capped  bool
	TotalXP int64
	Level   int
	LevelUp bool
}

// Awarder is how other modules pay XP without going through an event.
//
// Most XP arrives on the event path and should keep doing so — gamification is
// downstream by design, and a synchronous call from a learning flow is exactly
// what §14 of the module's AGENT.md forbids. This exists for the one caller
// that is already asynchronous and already owns the transaction boundary: the
// vocabulary verification job, which knows how many words it confirmed and has
// no learning request to block.
type Awarder interface {
	Award(ctx context.Context, req AwardRequest) (AwardOutcome, error)
}

// Reader is the read side, used by the dashboard, `notification` and `admin`.
type Reader interface {
	SummaryOf(ctx context.Context, userID uuid.UUID) (Summary, error)
}
