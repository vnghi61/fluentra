// Package service implements business use cases for spaced repetition scheduling (SRS).
package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/fluentra/fluentra/internal/generated/srs/sqlc"
	contentcontract "github.com/fluentra/fluentra/internal/modules/content/contract"
	learningcontract "github.com/fluentra/fluentra/internal/modules/learning/contract"
	"github.com/fluentra/fluentra/internal/modules/srs/contract"
	"github.com/fluentra/fluentra/internal/modules/srs/domain"
	"github.com/fluentra/fluentra/internal/modules/srs/repository"
	usercontract "github.com/fluentra/fluentra/internal/modules/user/contract"
	"github.com/fluentra/fluentra/internal/platform/cache"
	"github.com/fluentra/fluentra/internal/shared/apperr"
	"github.com/fluentra/fluentra/internal/shared/clock"
	"github.com/fluentra/fluentra/internal/shared/dbx"
)

const (
	cacheVersion = 1
	dueCountTTL  = 60 * time.Second

	// schedulerVersion is stamped on every review log. When the FSRS weights or
	// the formulas change, the logs written before the change stay attributable
	// to the scheduler that produced them.
	schedulerVersion = "v4.5"

	defaultDueCardLimit = 20
	maxDueCardLimit     = 100

	// maxForecastDays matches the 30-day projection the module doc describes.
	maxForecastDays = 30
)

// ContentReader resolves the authored material behind a card's content version.
//
// It is the batched form on purpose: a review session is twenty cards, and a
// per-card read is the N+1 the content contract exposes GetManyVersions to
// prevent.
type ContentReader interface {
	GetManyVersions(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]*contentcontract.Version, error)
}

// OutboxTx is the database transaction interface needed to write outbox events.
type OutboxTx interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
}

// EventWriter writes domain events to the outbox.
type EventWriter interface {
	Write(ctx context.Context, tx OutboxTx, aggregate, event string, payload any) (uuid.UUID, error)
}

// SRSCaches holds typed cache clients for the srs service.
type SRSCaches struct {
	DueCount cache.Cache[int]
}

// Deps carries dependencies for constructing the srs Service.
type Deps struct {
	Pool    *pgxpool.Pool
	Repo    repository.Repository
	Users   usercontract.Reader
	Content ContentReader
	Events  EventWriter
	Caches  SRSCaches
	Clock   clock.Clock
	NewID   func() uuid.UUID
	Env     string
}

// Service orchestrates review cards and FSRS scheduling.
type Service struct {
	pool    *pgxpool.Pool
	repo    repository.Repository
	users   usercontract.Reader
	content ContentReader
	events  EventWriter
	caches  SRSCaches
	clock   clock.Clock
	newID   func() uuid.UUID
	env     string
}

// New creates a new srs Service.
func New(deps Deps) *Service {
	clk := deps.Clock
	if clk == nil {
		clk = clock.Real{}
	}
	newID := deps.NewID
	if newID == nil {
		newID = uuid.New
	}
	return &Service{
		pool:    deps.Pool,
		repo:    deps.Repo,
		users:   deps.Users,
		content: deps.Content,
		events:  deps.Events,
		caches:  deps.Caches,
		clock:   clk,
		newID:   newID,
		env:     deps.Env,
	}
}

// AnswerResult represents the response returned after grading a review card.
type AnswerResult struct {
	Card         contract.ReviewCardSummary `json:"card"`
	NextDueAt    time.Time                  `json:"next_due_at"`
	IntervalDays int                        `json:"interval_days"`
}

// SessionResult represents the summary returned when completing a review session.
type SessionResult struct {
	Reviewed    int       `json:"reviewed"`
	Correct     int       `json:"correct"`
	Minutes     int       `json:"minutes"`
	CompletedAt time.Time `json:"completed_at"`
}

// ForecastDay is one bucket of the learner's projected workload.
type ForecastDay struct {
	Date     string `json:"date"`
	DueCount int    `json:"due_count"`
}

// resolveLocalDayCutoff returns the UTC timestamp corresponding to 23:59:59.999999999
// in the learner's local timezone for the current day.
func (s *Service) resolveLocalDayCutoff(ctx context.Context, userID uuid.UUID, now time.Time) time.Time {
	loc := s.resolveLocation(ctx, userID)
	localNow := now.In(loc)
	endOfLocalDay := time.Date(localNow.Year(), localNow.Month(), localNow.Day(), 23, 59, 59, 999999999, loc)
	return endOfLocalDay.UTC()
}

// resolveLocation reads the learner's timezone through the user contract. It
// falls back to UTC rather than failing: a due queue that errors because a
// timezone string is unrecognised is worse than one that is a few hours off for
// one learner, and the fallback is visible in the tests.
func (s *Service) resolveLocation(ctx context.Context, userID uuid.UUID) *time.Location {
	if s.users == nil {
		return time.UTC
	}
	summary, err := s.users.GetByID(ctx, userID)
	if err != nil || summary.Timezone == "" {
		return time.UTC
	}
	loc, err := time.LoadLocation(summary.Timezone)
	if err != nil {
		return time.UTC
	}
	return loc
}

// dueCountCacheKey formats the Redis key for due count.
func (s *Service) dueCountCacheKey(userID uuid.UUID) string {
	return fmt.Sprintf("fluentra:%s:srs:due_count:%s:v%d", s.env, userID.String(), cacheVersion)
}

func (s *Service) invalidateDueCountCache(ctx context.Context, userID uuid.UUID) {
	if s.caches.DueCount == nil {
		return
	}
	key := s.dueCountCacheKey(userID)
	if err := s.caches.DueCount.Delete(ctx, key); err != nil {
		slog.WarnContext(ctx, "failed to invalidate srs due count cache", "user_id", userID, "error", err)
	}
}

// UpsertCards allows upstream exercise engines to create or update review cards for a learner.
func (s *Service) UpsertCards(ctx context.Context, userID uuid.UUID, items []learningcontract.ReviewItem) error {
	if len(items) == 0 {
		return nil
	}

	now := s.clock.Now().UTC()
	params := domain.DefaultParameters()

	for _, item := range items {
		rating := domain.Rating(item.InitialGrade)
		if !rating.IsValid() {
			rating = domain.RatingGood
		}

		// The first schedule comes from the same pure function that reschedules
		// every later answer, so a card's due date is the FSRS output for that
		// grade and that `now` from the very first review onwards.
		first := domain.Schedule(domain.CardState{State: domain.StateNew}, rating, now, params)

		arg := sqlc.UpsertReviewCardParams{
			UserID:           userID,
			ContentVersionID: item.ContentVersionID,
			Skill:            item.Skill,
			Stability:        first.Stability,
			Difficulty:       first.Difficulty,
			DueAt:            first.DueAt,
			Reps:             clampInt32(first.Reps),
			Lapses:           clampInt32(first.Lapses),
			State:            string(first.State),
			// Creation is a review: the card is born from a graded answer and
			// Schedule gives it reps = 1. Recording `now` here is what lets the
			// next answer measure real elapsed time instead of zero.
			LastReviewAt: &now,
		}

		if _, err := s.repo.UpsertReviewCard(ctx, arg); err != nil {
			return fmt.Errorf("failed to upsert review card: %w", err)
		}
	}

	s.invalidateDueCountCache(ctx, userID)
	return nil
}

// SetCardsSuspended takes the given content out of the learner's rotation, or
// puts it back. It is the contract path skill modules use: `vocabulary` calls it
// when a learner marks a word known, which is what makes "known" mean anything
// to the due queue rather than only to a column in skill.user_word_state.
func (s *Service) SetCardsSuspended(
	ctx context.Context, userID uuid.UUID, contentVersionIDs []uuid.UUID, suspended bool,
) error {
	if len(contentVersionIDs) == 0 {
		return nil
	}
	if _, err := s.repo.SetReviewCardsSuspended(ctx, userID, contentVersionIDs, suspended); err != nil {
		return fmt.Errorf("failed to set review card suspension: %w", err)
	}
	s.invalidateDueCountCache(ctx, userID)
	return nil
}

// DueCount returns the count of cards currently due for review for the given user.
func (s *Service) DueCount(ctx context.Context, userID uuid.UUID) (int, error) {
	loader := func(ctx context.Context) (int, error) {
		now := s.clock.Now().UTC()
		cutoff := s.resolveLocalDayCutoff(ctx, userID, now)
		count, err := s.repo.CountDueCards(ctx, userID, cutoff)
		if err != nil {
			return 0, fmt.Errorf("failed to count due cards: %w", err)
		}
		return int(count), nil
	}

	if s.caches.DueCount == nil {
		return loader(ctx)
	}

	key := s.dueCountCacheKey(userID)
	return s.caches.DueCount.GetOrLoad(ctx, key, dueCountTTL, loader)
}

// DueCards returns active due cards up to the specified limit.
func (s *Service) DueCards(ctx context.Context, userID uuid.UUID, limit int32) ([]contract.ReviewCardSummary, error) {
	if limit <= 0 {
		limit = defaultDueCardLimit
	}
	if limit > maxDueCardLimit {
		limit = maxDueCardLimit
	}

	now := s.clock.Now().UTC()
	cutoff := s.resolveLocalDayCutoff(ctx, userID, now)

	rows, err := s.repo.ListDueCards(ctx, userID, cutoff, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to list due cards: %w", err)
	}

	result := make([]contract.ReviewCardSummary, 0, len(rows))
	for _, row := range rows {
		result = append(result, mapReviewCardSummary(row))
	}
	s.attachContent(ctx, result)
	return result, nil
}

// attachContent resolves each card's content version in one batched read.
//
// A failure is logged and leaves Content nil rather than failing the session: a
// learner with twenty due cards and one archived version should review the other
// nineteen. The client renders the missing one as an explicit state, which is why
// nothing here substitutes a placeholder.
func (s *Service) attachContent(ctx context.Context, cards []contract.ReviewCardSummary) {
	if s.content == nil || len(cards) == 0 {
		return
	}

	ids := make([]uuid.UUID, 0, len(cards))
	seen := make(map[uuid.UUID]struct{}, len(cards))
	for _, card := range cards {
		if _, ok := seen[card.ContentVersionID]; ok {
			continue
		}
		seen[card.ContentVersionID] = struct{}{}
		ids = append(ids, card.ContentVersionID)
	}

	versions, err := s.content.GetManyVersions(ctx, ids)
	if err != nil {
		slog.WarnContext(ctx, "failed to resolve review card content", "error", err)
		return
	}

	for i := range cards {
		version, ok := versions[cards[i].ContentVersionID]
		if !ok || version == nil {
			continue
		}
		cards[i].Content = &contract.ReviewCardContent{
			Kind:      version.Kind,
			CEFRLevel: version.CEFRLevel,
			Body:      version.Body,
		}
	}
}

// Forecast projects the learner's workload for the next `days` calendar days in
// their own timezone. Days with nothing due are omitted rather than padded with
// zeroes: the client renders a calendar and knows which dates it asked about.
func (s *Service) Forecast(ctx context.Context, userID uuid.UUID, days int) ([]ForecastDay, error) {
	if days <= 0 || days > maxForecastDays {
		days = maxForecastDays
	}

	now := s.clock.Now().UTC()
	loc := s.resolveLocation(ctx, userID)
	localNow := now.In(loc)
	startOfDay := time.Date(localNow.Year(), localNow.Month(), localNow.Day(), 0, 0, 0, 0, loc)
	until := startOfDay.AddDate(0, 0, days).UTC()

	rows, err := s.repo.ForecastDueCards(ctx, userID, loc.String(), until)
	if err != nil {
		return nil, fmt.Errorf("failed to forecast due cards: %w", err)
	}

	forecast := make([]ForecastDay, 0, len(rows))
	for _, row := range rows {
		if !row.DueDate.Valid {
			continue
		}
		forecast = append(forecast, ForecastDay{
			Date:     row.DueDate.Time.Format(time.DateOnly),
			DueCount: int(row.DueCount),
		})
	}
	return forecast, nil
}

// AnswerCard records a review grade, reschedules the card using FSRS, and logs the attempt.
func (s *Service) AnswerCard(
	ctx context.Context, userID, cardID uuid.UUID, gradeStr string, elapsedMs int) (AnswerResult, error,
) {
	rating := domain.Rating(gradeStr)
	if !rating.IsValid() {
		return AnswerResult{}, apperr.New(
			apperr.Validation, "INVALID_GRADE", "Grade must be one of: again, hard, good, easy.",
		)
	}

	cardRow, err := s.repo.GetReviewCardByID(ctx, cardID, userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return AnswerResult{}, apperr.New(apperr.NotFound, "REVIEW_CARD_NOT_FOUND", "Review card not found.")
		}
		return AnswerResult{}, fmt.Errorf("failed to fetch review card: %w", err)
	}

	if cardRow.SuspendedAt != nil {
		return AnswerResult{}, apperr.New(apperr.Conflict, "REVIEW_CARD_SUSPENDED", "Cannot review a suspended card.")
	}

	now := s.clock.Now().UTC()
	params := domain.DefaultParameters()

	currentCardState := domain.CardState{
		Stability:  cardRow.Stability,
		Difficulty: cardRow.Difficulty,
		State:      domain.State(cardRow.State),
		Reps:       int(cardRow.Reps),
		Lapses:     int(cardRow.Lapses),
		// The card's own record of when it was last answered, not `updated_at`.
		// `updated_at` is written by the database's clock and by things that are
		// not reviews — suspend, reset — each of which used to reset the
		// baseline FSRS measures elapsed time from, and with it the learner's
		// interval growth. Nil means never reviewed, which elapsedDays already
		// reads as no elapsed time.
		LastReviewAt: derefTime(cardRow.LastReviewAt),
		DueAt:        cardRow.DueAt,
	}

	nextState := domain.Schedule(currentCardState, rating, now, params)
	intervalDays := domain.NextInterval(nextState.Stability, params.RequestRetention, params.MaxInterval)

	var updatedCard sqlc.LearnReviewCard

	// writeAnswer is the whole answer path: reschedule the card, append the log
	// that makes later parameter tuning possible, and — when there is a
	// transaction to hang it on — enqueue the event. It is written once and run
	// either inside InTx or directly, so the two paths cannot drift apart.
	writeAnswer := func(txCtx context.Context, repo repository.Repository, outboxTx OutboxTx) error {
		var err error
		updatedCard, err = repo.UpdateReviewCardSchedule(txCtx, sqlc.UpdateReviewCardScheduleParams{
			ID:         cardID,
			UserID:     userID,
			Stability:  nextState.Stability,
			Difficulty: nextState.Difficulty,
			DueAt:      nextState.DueAt,
			Reps:       clampInt32(nextState.Reps),
			Lapses:     clampInt32(nextState.Lapses),
			State:      string(nextState.State),
			// `now` is the injected clock, which is the whole reason the clock is
			// injected: this is the value every later elapsed-time calculation
			// is measured against.
			LastReviewAt: &now,
		})
		if err != nil {
			return fmt.Errorf("failed to update review card: %w", err)
		}

		if _, err := repo.InsertReviewLog(txCtx, sqlc.InsertReviewLogParams{
			CardID:           cardID,
			UserID:           userID,
			Grade:            string(rating),
			ElapsedMs:        clampInt32(elapsedMs),
			StabilityBefore:  cardRow.Stability,
			StabilityAfter:   nextState.Stability,
			DifficultyBefore: cardRow.Difficulty,
			DifficultyAfter:  nextState.Difficulty,
			ScheduledDays:    clampInt32(intervalDays),
			SchedulerVersion: schedulerVersion,
			ReviewedAt:       now,
		}); err != nil {
			return fmt.Errorf("failed to insert review log: %w", err)
		}

		if s.events == nil || outboxTx == nil {
			return nil
		}
		payload := contract.CardAnswered{
			UserID:       userID,
			CardID:       cardID,
			Grade:        string(rating),
			IntervalDays: intervalDays,
			OccurredAt:   now,
		}
		if _, err := s.events.Write(
			txCtx, outboxTx, contract.Aggregate, contract.EventReviewCardAnswered, payload,
		); err != nil {
			return fmt.Errorf("failed to write review.card_answered event: %w", err)
		}
		return nil
	}

	if s.pool == nil {
		if err := writeAnswer(ctx, s.repo, nil); err != nil {
			return AnswerResult{}, err
		}
	} else if err := dbx.InTx(ctx, s.pool, func(txCtx context.Context, tx pgx.Tx) error {
		outboxTx, _ := tx.(OutboxTx)
		return writeAnswer(txCtx, s.repo.WithTx(tx), outboxTx)
	}); err != nil {
		return AnswerResult{}, err
	}

	s.invalidateDueCountCache(ctx, userID)

	return AnswerResult{
		Card:         mapReviewCardSummary(updatedCard),
		NextDueAt:    nextState.DueAt,
		IntervalDays: intervalDays,
	}, nil
}

// SuspendCard stops scheduling a review card.
func (s *Service) SuspendCard(ctx context.Context, userID, cardID uuid.UUID) (contract.ReviewCardSummary, error) {
	row, err := s.repo.SuspendReviewCard(ctx, cardID, userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return contract.ReviewCardSummary{}, apperr.New(apperr.NotFound, "REVIEW_CARD_NOT_FOUND", "Review card not found.")
		}
		return contract.ReviewCardSummary{}, fmt.Errorf("failed to suspend review card: %w", err)
	}
	s.invalidateDueCountCache(ctx, userID)
	return mapReviewCardSummary(row), nil
}

// ResetCard resets a card to its new initial state.
func (s *Service) ResetCard(ctx context.Context, userID, cardID uuid.UUID) (contract.ReviewCardSummary, error) {
	now := s.clock.Now().UTC()
	params := domain.DefaultParameters()
	initStability := domain.InitStability(domain.RatingGood, params)
	initDifficulty := domain.InitDifficulty(domain.RatingGood, params)

	arg := sqlc.ResetReviewCardParams{
		ID:         cardID,
		UserID:     userID,
		Stability:  initStability,
		Difficulty: initDifficulty,
		DueAt:      now,
	}

	row, err := s.repo.ResetReviewCard(ctx, arg)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return contract.ReviewCardSummary{}, apperr.New(apperr.NotFound, "REVIEW_CARD_NOT_FOUND", "Review card not found.")
		}
		return contract.ReviewCardSummary{}, fmt.Errorf("failed to reset review card: %w", err)
	}
	s.invalidateDueCountCache(ctx, userID)
	return mapReviewCardSummary(row), nil
}

// sessionMinutes is the time the learner actually spent, read back from the
// `elapsed_ms` every answer already writes to review_logs. It is deliberately
// not estimated from the card count: review_daily_stats.total_minutes is a
// figure a learner will eventually be shown, and a guess dressed as a
// measurement is worse than no measurement.
func (s *Service) sessionMinutes(ctx context.Context, userID uuid.UUID, since time.Time, reviewed int) int {
	if reviewed <= 0 {
		return 0
	}
	totalMs, err := s.repo.SumRecentReviewElapsedMs(ctx, userID, since, clampInt32(reviewed))
	if err != nil {
		slog.WarnContext(ctx, "failed to sum review elapsed time", "user_id", userID, "error", err)
		return 0
	}
	return int(totalMs / int64(time.Minute/time.Millisecond))
}

// CompleteSession closes the review session and records daily statistics.
func (s *Service) CompleteSession(ctx context.Context, userID uuid.UUID, reviewed, correct int) (SessionResult, error) {
	if reviewed < 0 {
		reviewed = 0
	}
	if correct < 0 {
		correct = 0
	}
	if correct > reviewed {
		correct = reviewed
	}

	now := s.clock.Now().UTC()
	statDate := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	minutes := s.sessionMinutes(ctx, userID, statDate, reviewed)

	arg := sqlc.UpsertReviewDailyStatsParams{
		UserID:           userID,
		StatDate:         pgtype.Date{Time: statDate, Valid: true},
		ReviewsCompleted: clampInt32(reviewed),
		NewCardsLearned:  0,
		TotalMinutes:     clampInt32(minutes),
	}

	if _, err := s.repo.UpsertReviewDailyStats(ctx, arg); err != nil {
		slog.WarnContext(ctx, "failed to record review daily stats", "user_id", userID, "error", err)
	}

	if s.events != nil && s.pool != nil {
		payload := contract.SessionCompleted{
			UserID:     userID,
			Reviewed:   reviewed,
			Correct:    correct,
			Minutes:    minutes,
			OccurredAt: now,
		}
		_ = dbx.InTx(ctx, s.pool, func(txCtx context.Context, tx pgx.Tx) error {
			if outboxTx, ok := tx.(OutboxTx); ok {
				_, _ = s.events.Write(txCtx, outboxTx, contract.Aggregate, contract.EventReviewSessionCompleted, payload)
			}
			return nil
		})
	}

	return SessionResult{
		Reviewed:    reviewed,
		Correct:     correct,
		Minutes:     minutes,
		CompletedAt: now,
	}, nil
}

// clampInt32 narrows a counter to the column width without wrapping. Every
// caller here is a small, non-negative count; a negative or absurd value can
// only come from a caller bug or a hostile payload, and saturating is safer
// than the silent overflow a bare conversion would produce.
// derefTime reads a nullable timestamp as a value, with the zero time standing
// for absent. FSRS already treats a zero LastReviewAt as "no time has elapsed",
// which is the right reading for a card that has been scheduled and never
// answered.
func derefTime(value *time.Time) time.Time {
	if value == nil {
		return time.Time{}
	}
	return *value
}

func clampInt32(v int) int32 {
	switch {
	case v < 0:
		return 0
	case v > math.MaxInt32:
		return math.MaxInt32
	default:
		return int32(v)
	}
}

func mapReviewCardSummary(row sqlc.LearnReviewCard) contract.ReviewCardSummary {
	return contract.ReviewCardSummary{
		ID:               row.ID,
		UserID:           row.UserID,
		ContentVersionID: row.ContentVersionID,
		Skill:            row.Skill,
		Stability:        row.Stability,
		Difficulty:       row.Difficulty,
		DueAt:            row.DueAt,
		Reps:             int(row.Reps),
		Lapses:           int(row.Lapses),
		State:            row.State,
		SuspendedAt:      row.SuspendedAt,
	}
}
