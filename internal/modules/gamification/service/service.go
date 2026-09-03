// Package service implements gamification's use cases: awarding XP, keeping
// streaks, evaluating badges and quests, and building the weekly leaderboard.
package service

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"math/big"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/fluentra/fluentra/internal/generated/gamification/sqlc"
	"github.com/fluentra/fluentra/internal/modules/gamification/contract"
	"github.com/fluentra/fluentra/internal/modules/gamification/domain"
	"github.com/fluentra/fluentra/internal/modules/gamification/repository"
	usercontract "github.com/fluentra/fluentra/internal/modules/user/contract"
	"github.com/fluentra/fluentra/internal/shared/apperr"
	"github.com/fluentra/fluentra/internal/shared/clock"
)

// OutboxTx is the transaction interface needed to write outbox events.
type OutboxTx interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
}

// EventWriter writes domain events to the outbox.
type EventWriter interface {
	Write(ctx context.Context, tx OutboxTx, aggregate, event string, payload any) (uuid.UUID, error)
}

// Deps carries the dependencies of the gamification service.
type Deps struct {
	Pool   *pgxpool.Pool
	Repo   repository.Repository
	Users  usercontract.Reader
	Events EventWriter
	Clock  clock.Clock
}

// Service orchestrates every gamification use case.
type Service struct {
	pool   *pgxpool.Pool
	repo   repository.Repository
	users  usercontract.Reader
	events EventWriter
	clock  clock.Clock
}

// New constructs the gamification service.
func New(deps Deps) *Service {
	timekeeper := deps.Clock
	if timekeeper == nil {
		timekeeper = clock.Real{}
	}
	return &Service{
		pool:   deps.Pool,
		repo:   deps.Repo,
		users:  deps.Users,
		events: deps.Events,
		clock:  timekeeper,
	}
}

// Compile-time checks that the service satisfies its published contract.
var (
	_ contract.Reader  = (*Service)(nil)
	_ contract.Awarder = (*Service)(nil)
)

// leaderboardPageSize caps a standings read. A league is a band, not the whole
// platform, and nobody scrolls past fifty.
const leaderboardPageSize = 50

// Award pays XP for one action, and is the single place XP enters the system.
//
// The order matters and is not arbitrary:
//
//  1. Resolve the amount from the source's base rate, the awards already made
//     today and the daily cap. All of that is domain arithmetic over numbers
//     this function reads first.
//  2. Insert with ON CONFLICT DO NOTHING. A redelivered event inserts nothing
//     and returns no row, which is how a duplicate is recognised — not by
//     checking first, which races.
//  3. Only then publish, evaluate badges and touch the streak, because only
//     then is it certain that something actually happened.
type activityProgress struct {
	base     int
	newBest  int
	newGrant int
	done     bool
	outcome  contract.AwardOutcome
}

func (s *Service) resolveActivityBase(ctx context.Context, req contract.AwardRequest) (activityProgress, error) {
	bestScore := 0
	xpGranted := 0
	hw, err := s.repo.GetActivityHighWater(ctx, req.UserID, req.SourceID)
	if err == nil {
		bestScore = int(hw.BestScore)
		xpGranted = int(hw.XpGranted)
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return activityProgress{}, err
	}

	// A graded award with no score is refused, not guessed.
	//
	// This branch used to reverse-engineer a score from req.Amount and, failing
	// that, assume a perfect 100. Both are corruptions of the record the whole
	// high-water design exists to protect: best_score never falls, so a
	// fabricated 100 is permanent, and every genuine attempt afterwards -- an
	// honest 80, a real improvement to 90 -- grants nothing for ever. The
	// learner's XP would then measure a number nobody earned.
	//
	// The event path always supplies a score: ActivityCompleted.Score is a value
	// type, so it is present even when it is zero, and zero is a real score that
	// correctly grants nothing. Anything reaching here without one is a caller
	// with a bug, and it should hear about it rather than have it papered over.
	if req.Score == nil {
		return activityProgress{}, apperr.New(apperr.Validation, "SCORE_REQUIRED",
			"A graded activity award must carry a score.")
	}
	score := *req.Score

	activityAward, actNewBest, actNewGrant := domain.CalculateActivityAward(score, bestScore, xpGranted)
	if activityAward <= 0 {
		if actNewBest > bestScore {
			_, _ = s.repo.UpsertActivityHighWater(ctx, sqlc.UpsertActivityHighWaterParams{
				UserID:     req.UserID,
				ActivityID: req.SourceID,
				//nolint:gosec // score is bounded between 0 and 100
				BestScore: int32(actNewBest),
				//nolint:gosec // xp is non-negative and bounded
				XpGranted: int32(xpGranted),
			})
		}
		total, totalErr := s.repo.TotalXP(ctx, req.UserID)
		if totalErr != nil {
			return activityProgress{}, totalErr
		}
		return activityProgress{
			done: true,
			outcome: contract.AwardOutcome{
				Awarded: false,
				TotalXP: total,
				Level:   domain.LevelFor(total),
			},
		}, nil
	}

	return activityProgress{
		base:     activityAward,
		newBest:  actNewBest,
		newGrant: actNewGrant,
	}, nil
}

func (s *Service) calculateAwardBase(
	ctx context.Context, req contract.AwardRequest, source domain.Source,
) (int, activityProgress, bool, contract.AwardOutcome, error) {
	if source == domain.SourceActivity {
		act, err := s.resolveActivityBase(ctx, req)
		if err != nil {
			return 0, activityProgress{}, false, contract.AwardOutcome{}, err
		}
		if act.done {
			return 0, act, true, act.outcome, nil
		}
		return act.base, act, false, contract.AwardOutcome{}, nil
	}

	base := req.Amount
	if base <= 0 {
		base = domain.BaseAward(source)
	}
	return base, activityProgress{}, false, contract.AwardOutcome{}, nil
}

func (s *Service) readDailyUsage(
	ctx context.Context, userID uuid.UUID, source string,
) (int64, int64, error) {
	dayStart, err := s.startOfLearnerDay(ctx, userID)
	if err != nil {
		return 0, 0, err
	}
	earnedToday, err := s.repo.XPFromSourceSince(ctx, userID, source, dayStart)
	if err != nil {
		return 0, 0, err
	}
	awardsToday, err := s.repo.CountAwardsFromSourceSince(ctx, userID, source, dayStart)
	if err != nil {
		return 0, 0, err
	}
	return earnedToday, awardsToday, nil
}

// Award pays XP for one learning action (BR-GAMIFICATION-08).
// Callers on the event path log and acknowledge rather than failing the learning action behind it.
func (s *Service) Award(ctx context.Context, req contract.AwardRequest) (contract.AwardOutcome, error) {
	if req.UserID == uuid.Nil {
		return contract.AwardOutcome{}, apperr.New(
			apperr.Validation, "GAMIFICATION_USER_REQUIRED", "An award needs a learner.")
	}
	if req.SourceID == "" {
		return contract.AwardOutcome{}, apperr.New(
			apperr.Validation, "GAMIFICATION_SOURCE_ID_REQUIRED",
			"An award needs an idempotency key.")
	}

	source := domain.Source(req.Source)
	base, act, done, outcome, err := s.calculateAwardBase(ctx, req, source)
	if err != nil {
		return contract.AwardOutcome{}, err
	}
	if done {
		return outcome, nil
	}
	if base <= 0 {
		return contract.AwardOutcome{}, nil
	}

	earnedToday, awardsToday, err := s.readDailyUsage(ctx, req.UserID, req.Source)
	if err != nil {
		return contract.AwardOutcome{}, err
	}

	award := domain.CalculateAward(source, base, int(earnedToday), int(awardsToday))
	if award.Amount <= 0 {
		total, totalErr := s.repo.TotalXP(ctx, req.UserID)
		if totalErr != nil {
			return contract.AwardOutcome{}, totalErr
		}
		return contract.AwardOutcome{
			Capped:  award.Capped,
			TotalXP: total,
			Level:   domain.LevelFor(total),
		}, nil
	}

	before, err := s.repo.TotalXP(ctx, req.UserID)
	if err != nil {
		return contract.AwardOutcome{}, err
	}

	event, err := s.repo.AwardXP(ctx, sqlc.AwardXPParams{
		UserID:     req.UserID,
		Source:     req.Source,
		SourceID:   req.SourceID,
		Amount:     int32(award.Amount), //nolint:gosec // bounded by DailyCap, max 500
		Multiplier: oneMultiplier(),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return contract.AwardOutcome{
				TotalXP: before,
				Level:   domain.LevelFor(before),
			}, nil
		}
		return contract.AwardOutcome{}, err
	}

	if source == domain.SourceActivity {
		_, err := s.repo.UpsertActivityHighWater(ctx, sqlc.UpsertActivityHighWaterParams{
			UserID:     req.UserID,
			ActivityID: req.SourceID,
			BestScore:  int32(act.newBest),  //nolint:gosec // score is bounded 0-100
			XpGranted:  int32(act.newGrant), //nolint:gosec // bounded
		})
		if err != nil {
			slog.WarnContext(ctx, "could not update activity high-water mark", "error", err)
		}
	}

	total := before + int64(event.Amount)
	levelBefore, levelAfter := domain.LevelFor(before), domain.LevelFor(total)

	return s.publishAndNotify(
		ctx, req, total, int(event.Amount), levelBefore, levelAfter, award.Capped,
	), nil
}

func (s *Service) publishAndNotify(
	ctx context.Context,
	req contract.AwardRequest,
	total int64,
	amount int,
	levelBefore, levelAfter int,
	capped bool,
) contract.AwardOutcome {
	s.publish(ctx, contract.EventXPAwarded, contract.XPAwarded{
		UserID:     req.UserID,
		Amount:     amount,
		Source:     req.Source,
		SourceID:   req.SourceID,
		TotalXP:    total,
		OccurredAt: s.clock.Now(),
	})
	if levelAfter > levelBefore {
		s.publish(ctx, contract.EventLevelUp, contract.LevelUp{
			UserID: req.UserID, Level: levelAfter, OccurredAt: s.clock.Now(),
		})
	}

	// Badges are evaluated after every award because their conditions are
	// thresholds on totals, and an award is the only thing that moves one.
	// Failure is logged, not returned: a learner must not lose XP because a
	// badge could not be written.
	if err := s.evaluateBadges(ctx, req.UserID, total, levelAfter); err != nil {
		slog.WarnContext(ctx, "badge evaluation failed", "user_id", req.UserID, "error", err)
	}

	return contract.AwardOutcome{
		Awarded: true,
		Amount:  amount,
		Capped:  capped,
		TotalXP: total,
		Level:   levelAfter,
		LevelUp: levelAfter > levelBefore,
	}
}

// RecordActivity is the whole of what one learning action does to gamification:
// pay XP, then decide whether the day now counts towards the streak.
//
// The two are one call because the streak depends on the XP: BR-GAMIFICATION-03
// says a streak extends when the daily goal is met, not when the app is opened,
// and the goal is measured in the XP just awarded.
func (s *Service) RecordActivity(ctx context.Context, req contract.AwardRequest) (contract.AwardOutcome, error) {
	outcome, err := s.Award(ctx, req)
	if err != nil {
		return outcome, err
	}
	if err := s.maybeExtendStreak(ctx, req.UserID); err != nil {
		// Logged rather than returned for the same reason as badges: the XP is
		// already written, and failing here would make the caller retry an
		// award that has already happened.
		slog.WarnContext(ctx, "streak update failed", "user_id", req.UserID, "error", err)
	}
	return outcome, nil
}

// maybeExtendStreak records today as active when the learner has met their goal.
func (s *Service) maybeExtendStreak(ctx context.Context, userID uuid.UUID) error {
	streak, err := s.repo.EnsureStreak(ctx, userID)
	if err != nil {
		return err
	}

	dayStart, err := s.startOfLearnerDay(ctx, userID)
	if err != nil {
		return err
	}
	todayXP, err := s.repo.XPSince(ctx, userID, dayStart)
	if err != nil {
		return err
	}
	if todayXP < int64(streak.DailyGoalXp) {
		return nil // The day is not earned yet.
	}

	day := domain.LocalDay(s.clock.Now(), s.timezoneOf(ctx, userID))
	result := domain.RecordActiveDay(domain.StreakState{
		CurrentLength: int(streak.CurrentLength),
		LongestLength: int(streak.LongestLength),
		LastActiveOn:  repository.DayOf(streak.LastActiveOn),
	}, day)

	if result.Outcome == domain.StreakUnchanged {
		return nil
	}
	//nolint:gosec // a streak length is a day count; ck_streaks_current keeps it sane
	if _, err := s.repo.ExtendStreak(ctx, userID, int32(result.NewLength), day); err != nil {
		// The statement is guarded on last_active_on, so a concurrent writer
		// that got there first leaves no row — which means the day is already
		// recorded, not that anything failed.
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		return err
	}

	// The daily-goal bonus, paid once per day by its own idempotency key.
	if _, err := s.Award(ctx, contract.AwardRequest{
		UserID:   userID,
		Source:   string(domain.SourceDailyGoal),
		SourceID: day.Format("2006-01-02"),
	}); err != nil {
		slog.WarnContext(ctx, "daily goal award failed", "user_id", userID, "error", err)
	}
	return nil
}

// evaluateBadges awards every badge the learner now qualifies for.
func (s *Service) evaluateBadges(ctx context.Context, userID uuid.UUID, totalXP int64, level int) error {
	candidates, err := s.repo.ListUnearnedBadges(ctx, userID)
	if err != nil {
		return err
	}
	if len(candidates) == 0 {
		return nil
	}

	streak, err := s.repo.EnsureStreak(ctx, userID)
	if err != nil {
		return err
	}
	facts := domain.BadgeFacts{
		TotalXP:      totalXP,
		Level:        level,
		StreakLength: int(streak.CurrentLength),
	}

	for _, badge := range candidates {
		if !domain.Earned(domain.ParseCriteria(badge.Criteria), facts) {
			continue
		}
		if _, err := s.repo.AwardBadge(ctx, userID, badge.ID); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				continue // Already held; a concurrent evaluation won.
			}
			return err
		}
		s.publish(ctx, contract.EventBadgeEarned, contract.BadgeEarned{
			UserID: userID, BadgeCode: badge.Code, BadgeName: badge.Name,
			OccurredAt: s.clock.Now(),
		})
	}
	return nil
}

// SummaryOf implements contract.Reader.
func (s *Service) SummaryOf(ctx context.Context, userID uuid.UUID) (contract.Summary, error) {
	streak, err := s.repo.EnsureStreak(ctx, userID)
	if err != nil {
		return contract.Summary{}, err
	}
	total, err := s.repo.TotalXP(ctx, userID)
	if err != nil {
		return contract.Summary{}, err
	}

	timezone := s.timezoneOf(ctx, userID)
	now := s.clock.Now()
	dayStart := domain.LocalDay(now, timezone)
	todayXP, err := s.repo.XPSince(ctx, userID, dayStart)
	if err != nil {
		return contract.Summary{}, err
	}
	weekXP, err := s.repo.XPSince(ctx, userID, domain.WeekStart(dayStart))
	if err != nil {
		return contract.Summary{}, err
	}

	badges, err := s.badgesOf(ctx, userID)
	if err != nil {
		return contract.Summary{}, err
	}
	quests, err := s.questsOf(ctx, userID, dayStart)
	if err != nil {
		return contract.Summary{}, err
	}

	progress := domain.ProgressFor(total)
	state := domain.StreakState{
		CurrentLength: int(streak.CurrentLength),
		LongestLength: int(streak.LongestLength),
		LastActiveOn:  repository.DayOf(streak.LastActiveOn),
	}

	summary := contract.Summary{
		UserID:       userID,
		TotalXP:      total,
		Level:        progress.Level,
		LevelStartXP: progress.LevelStartXP,
		NextLevelXP:  progress.NextLevelXP,
		XPToday:      todayXP,
		DailyGoalXP:  int(streak.DailyGoalXp),
		Streak: contract.Streak{
			Current:          state.CurrentLength,
			Longest:          state.LongestLength,
			FreezesAvailable: int(streak.FreezesAvailable),
			HoursRemaining:   domain.HoursUntilStreakLost(state, now, timezone),
		},
		Badges: badges,
		Quests: quests,
		League: domain.League(int(weekXP)),
	}
	if !state.LastActiveOn.IsZero() {
		day := state.LastActiveOn
		summary.Streak.LastActiveOn = &day
	}
	return summary, nil
}

func (s *Service) badgesOf(ctx context.Context, userID uuid.UUID) ([]contract.Badge, error) {
	rows, err := s.repo.ListEarnedBadges(ctx, userID)
	if err != nil {
		return nil, err
	}
	badges := make([]contract.Badge, 0, len(rows))
	for _, row := range rows {
		badges = append(badges, contract.Badge{
			Code: row.Code, Name: row.Name, Description: row.Description,
			Tier: row.Tier, EarnedAt: row.EarnedAt,
		})
	}
	return badges, nil
}

func (s *Service) questsOf(ctx context.Context, userID uuid.UUID, today time.Time) ([]contract.QuestSummary, error) {
	rows, err := s.repo.ListOpenUserQuests(ctx, userID, today)
	if err != nil {
		return nil, err
	}
	quests := make([]contract.QuestSummary, 0, len(rows))
	for _, row := range rows {
		steps := map[string]int{}
		for _, step := range domain.ParseSteps(row.Steps) {
			steps[step.Code] = step.Target
		}
		quests = append(quests, contract.QuestSummary{
			Code: row.Code, Name: row.Name, Description: row.Description,
			Progress: domain.ParseProgress(row.Progress),
			Steps:    steps,
			RewardXP: int(row.RewardXp),
			// A quest row always carries an expires_on; the column is NOT NULL.
			ExpiresOn: repository.DayOf(row.ExpiresOn),
		})
	}
	return quests, nil
}

// UseFreeze spends a streak freeze for today.
func (s *Service) UseFreeze(ctx context.Context, userID uuid.UUID) (contract.Streak, error) {
	if _, err := s.repo.EnsureStreak(ctx, userID); err != nil {
		return contract.Streak{}, err
	}
	timezone := s.timezoneOf(ctx, userID)
	day := domain.LocalDay(s.clock.Now(), timezone)

	streak, err := s.repo.ConsumeFreeze(ctx, userID, day)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// The statement's guards did not hold: no freezes left, or one has
			// already been used today. Either way the answer to the learner is
			// the same.
			return contract.Streak{}, apperr.New(
				apperr.Conflict, "NO_FREEZES_AVAILABLE",
				"You have no streak freeze available today.")
		}
		return contract.Streak{}, err
	}

	state := domain.StreakState{
		CurrentLength: int(streak.CurrentLength),
		LongestLength: int(streak.LongestLength),
		LastActiveOn:  repository.DayOf(streak.LastActiveOn),
	}
	return contract.Streak{
		Current:          state.CurrentLength,
		Longest:          state.LongestLength,
		FreezesAvailable: int(streak.FreezesAvailable),
		HoursRemaining:   domain.HoursUntilStreakLost(state, s.clock.Now(), timezone),
	}, nil
}

// SetDailyGoal changes the XP a learner must earn for a day to count.
func (s *Service) SetDailyGoal(ctx context.Context, userID uuid.UUID, goal int) error {
	if goal <= 0 || goal > 1000 {
		return apperr.New(apperr.Validation, "GAMIFICATION_GOAL_OUT_OF_RANGE",
			"A daily goal must be between 1 and 1000 XP.")
	}
	if _, err := s.repo.EnsureStreak(ctx, userID); err != nil {
		return err
	}
	_, err := s.repo.SetDailyGoal(ctx, userID, int32(goal))
	return err
}

// SetLeaderboardOptIn records whether a learner may be ranked.
func (s *Service) SetLeaderboardOptIn(ctx context.Context, userID uuid.UUID, optIn bool) error {
	if _, err := s.repo.EnsureStreak(ctx, userID); err != nil {
		return err
	}
	_, err := s.repo.SetLeaderboardOptIn(ctx, userID, optIn)
	return err
}

// LeaderboardEntry is one standing, with the display name and avatar resolved.
type LeaderboardEntry struct {
	Rank        int       `json:"rank"`
	UserID      uuid.UUID `json:"user_id"`
	DisplayName string    `json:"display_name"`
	AvatarURL   *string   `json:"avatar_url,omitempty"`
	XP          int       `json:"xp"`
	IsSelf      bool      `json:"is_self"`
}

// Leaderboard returns the current week's standings for a learner's league.
//
// Opt-in is checked here as well as in the snapshot build (BR-GAMIFICATION-07):
// the build decides who appears, and this decides who may look.
func (s *Service) Leaderboard(ctx context.Context, userID uuid.UUID) ([]LeaderboardEntry, error) {
	streak, err := s.repo.EnsureStreak(ctx, userID)
	if err != nil {
		return nil, err
	}
	if !streak.LeaderboardOptIn {
		return nil, apperr.New(apperr.Forbidden, "LEADERBOARD_NOT_OPTED_IN",
			"Join the leaderboard to see the standings.")
	}

	week := domain.WeekStart(domain.LocalDay(s.clock.Now(), s.timezoneOf(ctx, userID)))
	own, err := s.repo.GetLeaderboardEntry(ctx, userID, week)
	var league string
	switch {
	case err == nil:
		league = own.League
	case errors.Is(err, pgx.ErrNoRows):
		// Not yet ranked this week — opted in after the build, or no XP yet.
		// Their own league is computed from this week's XP so they see the
		// board they will join rather than an empty screen.
		weekXP, xpErr := s.repo.XPSince(ctx, userID, week)
		if xpErr != nil {
			return nil, xpErr
		}
		league = domain.League(int(weekXP))
	default:
		return nil, err
	}

	rows, err := s.repo.ListLeaderboard(ctx, league, week, leaderboardPageSize)
	if err != nil {
		return nil, err
	}
	return s.resolveNames(ctx, rows, userID), nil
}

// resolveNames turns snapshot rows into display entries.
//
// Display names and avatars only (BR-GAMIFICATION-07): no email or other
// private profile attributes.
func (s *Service) resolveNames(
	ctx context.Context, rows []sqlc.LearnLeaderboardSnapshot, self uuid.UUID,
) []LeaderboardEntry {
	ids := make([]uuid.UUID, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.UserID)
	}

	names := map[uuid.UUID]usercontract.Summary{}
	if s.users != nil && len(ids) > 0 {
		if resolved, err := s.users.GetManyByIDs(ctx, ids); err == nil {
			names = resolved
		} else {
			// A name lookup failure degrades to anonymous rows rather than
			// failing the screen: the standings are the point.
			slog.WarnContext(ctx, "leaderboard name lookup failed", "error", err)
		}
	}

	entries := make([]LeaderboardEntry, 0, len(rows))
	for _, row := range rows {
		name := "Learner"
		var avatarURL *string
		if summary, ok := names[row.UserID]; ok {
			if summary.DisplayName != "" {
				name = summary.DisplayName
			}
			avatarURL = summary.AvatarURL
		}
		entries = append(entries, LeaderboardEntry{
			Rank: int(row.Rank), UserID: row.UserID, DisplayName: name,
			AvatarURL: avatarURL,
			XP:        int(row.Xp), IsSelf: row.UserID == self,
		})
	}
	return entries
}

// startOfLearnerDay is midnight today in the learner's own timezone, expressed
// as an instant the XP queries can compare against.
func (s *Service) startOfLearnerDay(ctx context.Context, userID uuid.UUID) (time.Time, error) {
	timezone := s.timezoneOf(ctx, userID)
	now := s.clock.Now()
	location := time.UTC
	if timezone != "" {
		if loaded, err := time.LoadLocation(timezone); err == nil {
			location = loaded
		}
	}
	local := now.In(location)
	return time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, location), nil
}

// timezoneOf reads a learner's timezone, falling back to UTC.
//
// A failure here is not propagated. BR-GAMIFICATION-08: a learner whose profile
// cannot be read must still earn their XP, on a UTC day boundary, rather than
// having the award fail.
func (s *Service) timezoneOf(ctx context.Context, userID uuid.UUID) string {
	if s.users == nil {
		return "UTC"
	}
	summary, err := s.users.GetByID(ctx, userID)
	if err != nil || summary.Timezone == "" {
		return "UTC"
	}
	return summary.Timezone
}

// publish writes an event to the outbox on its own transaction.
//
// Not joined to the award's transaction, and deliberately so: the award is a
// single statement whose idempotency is enforced by a unique constraint, and
// wrapping it in an explicit transaction to share with the outbox would buy
// nothing that the constraint does not already guarantee. A publish that fails
// is logged and dropped rather than rolling back XP the learner has earned.
func (s *Service) publish(ctx context.Context, event string, payload any) {
	if s.events == nil || s.pool == nil {
		return
	}
	if _, err := s.events.Write(ctx, s.pool, contract.Aggregate, event, payload); err != nil {
		slog.WarnContext(ctx, "gamification event not published",
			"event", event, "error", err)
	}
}

// oneMultiplier is the numeric(4,2) value 1.00.
//
// Multipliers are stored per award so a future double-XP weekend is a data
// change rather than a migration. Nothing sets anything but 1.00 today, and the
// column exists because retrofitting it onto historic awards would mean
// guessing what they were worth.
func oneMultiplier() pgtype.Numeric {
	return pgtype.Numeric{Int: big.NewInt(100), Exp: -2, Valid: true}
}

// questProgressJSON encodes a progress map for storage.
func questProgressJSON(progress domain.QuestProgress) ([]byte, error) {
	return json.Marshal(progress)
}
