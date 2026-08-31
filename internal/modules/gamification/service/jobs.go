package service

import (
	"context"
	"log/slog"
	"time"

	"github.com/fluentra/fluentra/internal/generated/gamification/sqlc"
	"github.com/fluentra/fluentra/internal/modules/gamification/contract"
	"github.com/fluentra/fluentra/internal/modules/gamification/domain"
	"github.com/fluentra/fluentra/internal/modules/gamification/repository"
)

// sweepBatch is how many streaks one sweep considers.
//
// Bounded because the sweep runs on a schedule and must not turn into an
// unbounded scan as the platform grows: what it misses this hour it catches
// next hour, and a streak is not lost by being swept an hour late — the
// arithmetic is on dates.
const sweepBatch = 500

// leaderboardCandidates caps the standings build. Beyond this the ranking is
// not meaningfully a leaderboard.
const leaderboardCandidates = 2000

// leaderboardRetentionWeeks is how many weeks of snapshots are kept.
const leaderboardRetentionWeeks = 8

// SweepStreaks breaks lapsed streaks, spends freezes for those that can be
// saved, and flags the ones about to lapse.
//
// Runs hourly rather than at midnight, because there is no single midnight: the
// day boundary is per learner (BR-GAMIFICATION-02), so a job that ran once a
// day would be running at the wrong time for most of the world. Hourly means
// every timezone's boundary is crossed within an hour of it happening.
//
// Every step is idempotent, so a re-run — a retry, a second replica that took
// the advisory lock — costs nothing.
func (s *Service) SweepStreaks(ctx context.Context) error {
	now := s.clock.Now()

	// A streak is only swept once its last active day is two days behind UTC
	// today. That is deliberately generous: a learner in UTC+14 is still inside
	// the day a UTC-11 learner finished yesterday, and a tighter cutoff would
	// break streaks for whoever happens to live east of the server.
	cutoff := domain.LocalDay(now, "UTC").AddDate(0, 0, -1)

	candidates, err := s.repo.ListStreaksAtRisk(ctx, cutoff, sweepBatch)
	if err != nil {
		return err
	}

	for _, streak := range candidates {
		if err := s.sweepOne(ctx, streak, now); err != nil {
			// One learner's streak must not stop the sweep for everyone else.
			slog.WarnContext(ctx, "streak sweep failed for learner",
				"user_id", streak.UserID, "error", err)
		}
	}
	return nil
}

func (s *Service) sweepOne(ctx context.Context, row sqlc.LearnStreak, now time.Time) error {
	timezone := s.timezoneOf(ctx, row.UserID)
	today := domain.LocalDay(now, timezone)

	state := domain.StreakState{
		CurrentLength: int(row.CurrentLength),
		LongestLength: int(row.LongestLength),
		LastActiveOn:  repository.DayOf(row.LastActiveOn),
	}

	if !domain.StreakIsBroken(state, today) {
		// Still live. If it lapses at the end of today, say so — `notification`
		// decides whether that becomes a reminder, and when.
		if hours := domain.HoursUntilStreakLost(state, now, timezone); hours > 0 {
			s.publish(ctx, contract.EventStreakAtRisk, contract.StreakAtRisk{
				UserID:         row.UserID,
				CurrentLength:  state.CurrentLength,
				HoursRemaining: hours,
				OccurredAt:     now,
			})
		}
		return nil
	}

	// Broken — unless a freeze can cover the missed day (BR-GAMIFICATION-04).
	// The freeze is spent automatically here rather than being offered, because
	// by the time a learner could be asked the streak is already gone.
	if row.FreezesAvailable > 0 {
		missed := repository.DayOf(row.LastActiveOn).AddDate(0, 0, 1)
		if _, err := s.repo.ConsumeFreeze(ctx, row.UserID, missed); err == nil {
			slog.InfoContext(ctx, "streak freeze spent automatically",
				"user_id", row.UserID, "day", missed.Format(time.DateOnly))
			return nil
		}
		// A failed freeze falls through to breaking the streak. It is the
		// honest outcome: pretending the streak survived without spending
		// anything would make the count a fiction.
	}

	if _, err := s.repo.BreakStreak(ctx, row.UserID); err != nil {
		return err
	}
	s.publish(ctx, contract.EventStreakBroken, contract.StreakBroken{
		UserID:         row.UserID,
		PreviousLength: state.CurrentLength,
		OccurredAt:     now,
	})
	return nil
}

// BuildLeaderboard materialises this week's standings.
//
// Snapshot rather than a live ranking so the board does not shuffle under a
// learner mid-request, and so ranking never runs a full sort on the request
// path. Rebuilt on a schedule, and the upsert makes each rebuild a correction
// rather than a duplicate.
//
// Opt-in is enforced in the SQL, not here: WeeklyXPStandings joins to streaks
// on `leaderboard_opt_in`, so a learner who has not opted in is never selected,
// never ranked, and never stored (BR-GAMIFICATION-07).
func (s *Service) BuildLeaderboard(ctx context.Context) error {
	now := s.clock.Now()
	weekStart := domain.WeekStart(domain.LocalDay(now, "UTC"))
	weekEnd := weekStart.AddDate(0, 0, 7)

	standings, err := s.repo.WeeklyXPStandings(ctx, weekStart, weekEnd, leaderboardCandidates)
	if err != nil {
		return err
	}

	// Ranked inside their own league, not globally: rank 1 of bronze is a
	// standing a beginner can hold, and a single global ordering would put
	// every one of them at the bottom of one list.
	rankByLeague := map[string]int32{}
	for _, row := range standings {
		league := domain.League(int(row.Xp))
		rankByLeague[league]++

		if _, err := s.repo.UpsertLeaderboardEntry(ctx, sqlc.UpsertLeaderboardEntryParams{
			League:    league,
			WeekStart: repository.Date(weekStart),
			UserID:    row.UserID,
			Xp:        row.Xp,
			Rank:      rankByLeague[league],
		}); err != nil {
			slog.WarnContext(ctx, "leaderboard entry not written",
				"user_id", row.UserID, "error", err)
		}
	}

	// Old snapshots are a display artefact nothing reads; the XP events they
	// were built from remain the durable record.
	cutoff := weekStart.AddDate(0, 0, -7*leaderboardRetentionWeeks)
	if err := s.repo.DeleteLeaderboardBefore(ctx, cutoff); err != nil {
		slog.WarnContext(ctx, "leaderboard snapshot pruning failed", "error", err)
	}
	return nil
}
