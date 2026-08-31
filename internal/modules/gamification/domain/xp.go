// Package domain holds gamification's pure rules: the level curve, the XP caps
// and the streak's day arithmetic. Nothing here does I/O, and nothing here
// imports a layer above it.
package domain

import "time"

// Source names the kind of action that earned XP. It is the first half of the
// idempotency key, and the axis the daily cap applies along.
type Source string

// The sources that pay XP today. Adding one means adding a cap below: the
// default is zero, and a source with no cap earns nothing, which fails loudly
// in a test rather than quietly in production.
const (
	SourceActivity       Source = "activity"
	SourceLesson         Source = "lesson"
	SourceReviewSession  Source = "review_session"
	SourceUploadVerified Source = "upload_verified"
	SourceQuest          Source = "quest"
	SourceDailyGoal      Source = "daily_goal"
)

// BaseAward is the XP a source pays before caps and diminishing returns.
func BaseAward(source Source) int {
	switch source {
	case SourceActivity:
		return 10
	case SourceLesson:
		return 40
	case SourceReviewSession:
		return 25
	case SourceUploadVerified:
		// Per verified word. Deliberately below an activity: confirming a word
		// exists is not the same as having practised it, and paying it the same
		// would make uploading a dictionary the cheapest way to level up.
		return 5
	case SourceQuest:
		return 0 // The quest's own reward_xp is the amount; see QuestAward.
	case SourceDailyGoal:
		return 20
	default:
		return 0
	}
}

// DailyCap is the most XP one source may pay a learner in a day
// (BR-GAMIFICATION-05).
//
// Per source rather than in total, so a learner who has exhausted one way of
// earning has not exhausted all of them — an overall cap would make the last
// hour of study worth nothing whatever they chose to do in it.
func DailyCap(source Source) int {
	switch source {
	case SourceActivity:
		return 300
	case SourceLesson:
		return 400
	case SourceReviewSession:
		return 200
	case SourceUploadVerified:
		// The tightest cap, because it is the cheapest action: a learner can
		// paste a thousand words in one go, and the verification job would
		// otherwise pay for all of them.
		return 100
	case SourceQuest:
		return 500
	case SourceDailyGoal:
		return 20
	default:
		return 0
	}
}

// diminishAfter is how many awards from one source in a day are paid in full.
const diminishAfter = 10

// Award is the outcome of applying the rules to one earning event.
type Award struct {
	// Amount actually payable, after diminishing returns and the cap.
	Amount int
	// Capped reports that the daily cap, not the base rate, decided the amount.
	// The API surfaces it as DAILY_XP_CAP_REACHED, which is information rather
	// than an error: the learning happened either way.
	Capped bool
}

// CalculateAward applies BR-GAMIFICATION-05 to one earning event.
//
// `earnedToday` is the XP this source has already paid the learner today, and
// `awardsToday` how many times it has paid. Both come from the store, so this
// stays a pure function of numbers and can be reasoned about without a database.
func CalculateAward(source Source, base, earnedToday, awardsToday int) Award {
	if base <= 0 {
		return Award{}
	}

	// Diminishing returns: after the tenth award from a source in a day, it
	// pays half. Repetition still counts for something — a learner drilling
	// the same deck is still learning — but not as much as breadth.
	amount := base
	if awardsToday >= diminishAfter {
		amount = base / 2
		if amount < 1 {
			amount = 1
		}
	}

	remaining := DailyCap(source) - earnedToday
	if remaining <= 0 {
		return Award{Amount: 0, Capped: true}
	}
	if amount > remaining {
		return Award{Amount: remaining, Capped: true}
	}
	return Award{Amount: amount}
}

// levelStep is the XP between consecutive levels at the bottom of the curve.
const levelStep = 100

// LevelFor is the level a cumulative XP total earns.
//
// Quadratic, not linear: level n costs n*100 XP more than level n-1, so early
// levels arrive quickly and later ones stay meaningful. Level 1 starts at 0 XP,
// which means every learner has a level from their first minute rather than
// spending their first session at "level 0".
func LevelFor(totalXP int64) int {
	if totalXP < 0 {
		return 1
	}
	// The step to the next threshold is `level * levelStep`, matching
	// XPForLevel's sum exactly. It was `(level+1) * levelStep`, which overshot
	// by one step every level and put a learner one level below where their XP
	// placed them from level 3 upwards — invisible in the early levels the
	// first tests covered, and wrong for everyone past them.
	level := 1
	for threshold := int64(levelStep); totalXP >= threshold; threshold += int64(level) * levelStep {
		level++
	}
	return level
}

// XPForLevel is the cumulative XP at which a level begins. LevelFor's inverse,
// used to draw the progress bar between two levels.
func XPForLevel(level int) int64 {
	if level <= 1 {
		return 0
	}
	total := int64(0)
	for n := 1; n < level; n++ {
		total += int64(n) * levelStep
	}
	return total
}

// Progress describes where a learner stands between two levels.
type Progress struct {
	Level        int
	TotalXP      int64
	LevelStartXP int64
	NextLevelXP  int64
}

// ProgressFor derives the whole level position from a cumulative total.
func ProgressFor(totalXP int64) Progress {
	level := LevelFor(totalXP)
	return Progress{
		Level:        level,
		TotalXP:      totalXP,
		LevelStartXP: XPForLevel(level),
		NextLevelXP:  XPForLevel(level + 1),
	}
}

// QuestAward is a quest's reward, floored at zero so an ill-authored quest
// cannot take XP away.
func QuestAward(rewardXP int) int {
	if rewardXP < 0 {
		return 0
	}
	return rewardXP
}

// LocalDay is the calendar day a moment falls on in a learner's own timezone.
//
// The whole of BR-GAMIFICATION-02 rests on this being the learner's day and not
// the server's. A learner in Ho Chi Minh City studying at 8am has been active
// on that date; computing it in UTC puts them on the day before and breaks a
// streak they kept.
//
// An unparseable or empty timezone falls back to UTC rather than erroring: a
// bad preference must not stop XP being awarded (BR-GAMIFICATION-08).
func LocalDay(at time.Time, timezone string) time.Time {
	location := time.UTC
	if timezone != "" {
		if loaded, err := time.LoadLocation(timezone); err == nil {
			location = loaded
		}
	}
	local := at.In(location)
	return time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, time.UTC)
}
