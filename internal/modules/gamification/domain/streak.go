package domain

import "time"

// StreakOutcome is what a day of activity does to a streak.
type StreakOutcome int

// The three things a recorded day can mean.
const (
	// StreakUnchanged: the day is already recorded. A second review session on
	// the same day does not make the streak two days long.
	StreakUnchanged StreakOutcome = iota
	// StreakExtended: the day after the last active one.
	StreakExtended
	// StreakRestarted: a gap. The streak begins again at 1 — the learner is
	// active today, so today counts, and starting them at 0 would mean a
	// learner returning after a break shows no streak on a day they studied.
	StreakRestarted
)

// StreakState is the part of a streak row the rules need.
type StreakState struct {
	CurrentLength int
	LongestLength int
	// LastActiveOn is the zero time when the learner has never been active.
	LastActiveOn time.Time
}

// StreakResult is the decision, ready to be written.
type StreakResult struct {
	Outcome StreakOutcome
	// NewLength is what current_length becomes. Meaningless when the outcome
	// is StreakUnchanged, and equal to the existing length there.
	NewLength int
}

// RecordActiveDay decides what an active day does to a streak.
//
// `day` is already the learner's local day — see LocalDay. Passing a moment
// instead is the mistake this signature exists to prevent: the comparison below
// is a date comparison, and a date computed in the wrong zone is off by one for
// half the world.
//
// Deliberately not a rule about opening the app. BR-GAMIFICATION-03 says the
// streak extends when the daily goal is met, and the caller decides that before
// getting here.
func RecordActiveDay(state StreakState, day time.Time) StreakResult {
	day = truncateDay(day)

	if state.LastActiveOn.IsZero() {
		return StreakResult{Outcome: StreakExtended, NewLength: 1}
	}
	last := truncateDay(state.LastActiveOn)

	switch {
	case !day.After(last):
		// Today is already recorded, or the clock went backwards — a learner
		// who flies west can produce a local day earlier than the one already
		// stored, and losing their streak for it would be exactly the
		// travel-punishing behaviour BR-GAMIFICATION-02 forbids.
		return StreakResult{Outcome: StreakUnchanged, NewLength: state.CurrentLength}
	case day.Equal(last.AddDate(0, 0, 1)):
		return StreakResult{Outcome: StreakExtended, NewLength: state.CurrentLength + 1}
	default:
		return StreakResult{Outcome: StreakRestarted, NewLength: 1}
	}
}

// StreakIsBroken reports whether a streak has lapsed as of the learner's local
// today. A streak survives the whole of the day after its last active day —
// that day is still live, and the learner may yet study in it.
func StreakIsBroken(state StreakState, today time.Time) bool {
	if state.CurrentLength <= 0 || state.LastActiveOn.IsZero() {
		return false
	}
	return truncateDay(today).After(truncateDay(state.LastActiveOn).AddDate(0, 0, 1))
}

// HoursUntilStreakLost is how long a learner has left to keep a streak alive,
// as of `now` in their own timezone. Zero when nothing is at risk.
//
// Drives gamification.streak_at_risk, which `notification` turns into a
// reminder — subject to quiet hours, which are its business and not this
// module's (BR-GAMIFICATION-09).
func HoursUntilStreakLost(state StreakState, now time.Time, timezone string) int {
	if state.CurrentLength <= 0 || state.LastActiveOn.IsZero() {
		return 0
	}
	today := LocalDay(now, timezone)
	last := truncateDay(state.LastActiveOn)
	if !today.After(last) {
		// Already active today; nothing is at risk until tomorrow.
		return 0
	}
	if today.After(last.AddDate(0, 0, 1)) {
		return 0 // Already lost.
	}

	location := time.UTC
	if loaded, err := time.LoadLocation(timezone); err == nil && timezone != "" {
		location = loaded
	}
	local := now.In(location)
	endOfDay := time.Date(local.Year(), local.Month(), local.Day(), 23, 59, 59, 0, location)
	remaining := int(endOfDay.Sub(local).Hours())
	if remaining < 0 {
		return 0
	}
	return remaining
}

// League places a learner by their weekly XP (BR-GAMIFICATION-07).
//
// Bands rather than a global ranking so a learner competes with people studying
// at their own intensity. A beginner has no useful contest against someone
// doing four hours a day, and putting them in one is how a leaderboard
// demotivates the learner it was added to motivate.
func League(weeklyXP int) string {
	switch {
	case weeklyXP >= 2000:
		return "diamond"
	case weeklyXP >= 800:
		return "gold"
	case weeklyXP >= 250:
		return "silver"
	default:
		return "bronze"
	}
}

// WeekStart is the Monday of the week a day falls in, which is how a
// leaderboard snapshot is keyed.
func WeekStart(day time.Time) time.Time {
	day = truncateDay(day)
	// Go numbers Sunday 0; ISO weeks start on Monday, so Sunday is 6 days in.
	offset := (int(day.Weekday()) + 6) % 7
	return day.AddDate(0, 0, -offset)
}

func truncateDay(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}
