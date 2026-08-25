package service_test

import (
	"context"
	"sort"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/fluentra/fluentra/internal/generated/srs/sqlc"
	learningcontract "github.com/fluentra/fluentra/internal/modules/learning/contract"
	"github.com/fluentra/fluentra/internal/modules/srs/domain"
	"github.com/fluentra/fluentra/internal/modules/srs/repository"
	"github.com/fluentra/fluentra/internal/modules/srs/service"
	usercontract "github.com/fluentra/fluentra/internal/modules/user/contract"
	"github.com/fluentra/fluentra/internal/shared/apperr"
	"github.com/fluentra/fluentra/internal/shared/clock"
)

// fakeRepo is an in-memory repository for srs testing.
type fakeRepo struct {
	cards      map[uuid.UUID]sqlc.LearnReviewCard
	logs       []sqlc.LearnReviewLog
	dailyStats map[string]sqlc.LearnReviewDailyStat
}

// tzUTC and tzVietnam are the two timezones the day-boundary tests contrast.
// They live here rather than in the integration file so both builds see them.
const (
	tzUTC     = "UTC"
	tzVietnam = "Asia/Ho_Chi_Minh"

	skillVocabulary = "vocabulary"
	gradeGood       = "good"
)

func newFakeRepo() *fakeRepo {
	return &fakeRepo{
		cards:      make(map[uuid.UUID]sqlc.LearnReviewCard),
		logs:       make([]sqlc.LearnReviewLog, 0),
		dailyStats: make(map[string]sqlc.LearnReviewDailyStat),
	}
}

func (f *fakeRepo) WithTx(_ pgx.Tx) repository.Repository {
	return f
}

func (f *fakeRepo) UpsertReviewCard(_ context.Context, arg sqlc.UpsertReviewCardParams) (sqlc.LearnReviewCard, error) {
	for id, card := range f.cards {
		if card.UserID == arg.UserID && card.ContentVersionID == arg.ContentVersionID {
			// Mirrors the ON CONFLICT clause: an existing card keeps its schedule.
			card.Skill = arg.Skill
			card.UpdatedAt = time.Now().UTC()
			f.cards[id] = card
			return card, nil
		}
	}

	card := sqlc.LearnReviewCard{
		ID:               uuid.New(),
		UserID:           arg.UserID,
		ContentVersionID: arg.ContentVersionID,
		Skill:            arg.Skill,
		Stability:        arg.Stability,
		Difficulty:       arg.Difficulty,
		DueAt:            arg.DueAt,
		Reps:             arg.Reps,
		Lapses:           arg.Lapses,
		State:            arg.State,
		CreatedAt:        time.Now().UTC(),
		UpdatedAt:        time.Now().UTC(),
	}
	f.cards[card.ID] = card
	return card, nil
}

func (f *fakeRepo) GetReviewCardByID(_ context.Context, id, userID uuid.UUID) (sqlc.LearnReviewCard, error) {
	card, ok := f.cards[id]
	if !ok || card.UserID != userID {
		return sqlc.LearnReviewCard{}, apperr.New(apperr.NotFound, "NOT_FOUND", "card not found")
	}
	return card, nil
}

func (f *fakeRepo) GetReviewCardByUserAndContent(
	_ context.Context, userID, contentVersionID uuid.UUID) (sqlc.LearnReviewCard, error,
) {
	for _, card := range f.cards {
		if card.UserID == userID && card.ContentVersionID == contentVersionID {
			return card, nil
		}
	}
	return sqlc.LearnReviewCard{}, apperr.New(apperr.NotFound, "NOT_FOUND", "card not found")
}

func (f *fakeRepo) ListDueCards(
	_ context.Context, userID uuid.UUID, dueBefore time.Time, limit int32) ([]sqlc.LearnReviewCard, error,
) {
	var result []sqlc.LearnReviewCard
	for _, card := range f.cards {
		if card.UserID == userID && card.SuspendedAt == nil && !card.DueAt.After(dueBefore) {
			result = append(result, card)
			if len(result) >= int(limit) {
				break
			}
		}
	}
	return result, nil
}

func (f *fakeRepo) CountDueCards(_ context.Context, userID uuid.UUID, dueBefore time.Time) (int64, error) {
	var count int64
	for _, card := range f.cards {
		if card.UserID == userID && card.SuspendedAt == nil && !card.DueAt.After(dueBefore) {
			count++
		}
	}
	return count, nil
}

func (f *fakeRepo) ForecastDueCards(
	_ context.Context, userID uuid.UUID, timezone string, until time.Time,
) ([]sqlc.ForecastDueCardsRow, error) {
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		loc = time.UTC
	}
	buckets := make(map[string]int64)
	for _, card := range f.cards {
		if card.UserID != userID || card.SuspendedAt != nil || !card.DueAt.Before(until) {
			continue
		}
		local := card.DueAt.In(loc)
		buckets[local.Format(time.DateOnly)]++
	}
	dates := make([]string, 0, len(buckets))
	for date := range buckets {
		dates = append(dates, date)
	}
	sort.Strings(dates)
	rows := make([]sqlc.ForecastDueCardsRow, 0, len(dates))
	for _, date := range dates {
		day, _ := time.ParseInLocation(time.DateOnly, date, time.UTC)
		rows = append(rows, sqlc.ForecastDueCardsRow{
			DueDate:  pgtype.Date{Time: day, Valid: true},
			DueCount: buckets[date],
		})
	}
	return rows, nil
}

func (f *fakeRepo) UpdateReviewCardSchedule(
	_ context.Context, arg sqlc.UpdateReviewCardScheduleParams) (sqlc.LearnReviewCard, error,
) {
	card, ok := f.cards[arg.ID]
	if !ok || card.UserID != arg.UserID {
		return sqlc.LearnReviewCard{}, apperr.New(apperr.NotFound, "NOT_FOUND", "card not found")
	}
	card.Stability = arg.Stability
	card.Difficulty = arg.Difficulty
	card.DueAt = arg.DueAt
	card.Reps = arg.Reps
	card.Lapses = arg.Lapses
	card.State = arg.State
	card.UpdatedAt = time.Now().UTC()
	f.cards[arg.ID] = card
	return card, nil
}

func (f *fakeRepo) SuspendReviewCard(_ context.Context, id, userID uuid.UUID) (sqlc.LearnReviewCard, error) {
	card, ok := f.cards[id]
	if !ok || card.UserID != userID {
		return sqlc.LearnReviewCard{}, apperr.New(apperr.NotFound, "NOT_FOUND", "card not found")
	}
	now := time.Now().UTC()
	card.SuspendedAt = &now
	card.UpdatedAt = now
	f.cards[id] = card
	return card, nil
}

func (f *fakeRepo) SetReviewCardsSuspended(
	_ context.Context, userID uuid.UUID, contentVersionIDs []uuid.UUID, suspended bool,
) (int64, error) {
	wanted := make(map[uuid.UUID]struct{}, len(contentVersionIDs))
	for _, id := range contentVersionIDs {
		wanted[id] = struct{}{}
	}
	var affected int64
	for id, card := range f.cards {
		if card.UserID != userID {
			continue
		}
		if _, ok := wanted[card.ContentVersionID]; !ok {
			continue
		}
		if suspended {
			at := time.Now().UTC()
			card.SuspendedAt = &at
		} else {
			card.SuspendedAt = nil
		}
		f.cards[id] = card
		affected++
	}
	return affected, nil
}

func (f *fakeRepo) ResetReviewCard(_ context.Context, arg sqlc.ResetReviewCardParams) (sqlc.LearnReviewCard, error) {
	card, ok := f.cards[arg.ID]
	if !ok || card.UserID != arg.UserID {
		return sqlc.LearnReviewCard{}, apperr.New(apperr.NotFound, "NOT_FOUND", "card not found")
	}
	now := time.Now().UTC()
	card.Stability = arg.Stability
	card.Difficulty = arg.Difficulty
	card.DueAt = arg.DueAt
	card.Reps = 0
	card.Lapses = 0
	card.State = string(domain.StateNew)
	card.SuspendedAt = nil
	card.UpdatedAt = now
	f.cards[arg.ID] = card
	return card, nil
}

func (f *fakeRepo) InsertReviewLog(_ context.Context, arg sqlc.InsertReviewLogParams) (sqlc.LearnReviewLog, error) {
	log := sqlc.LearnReviewLog{
		ID:               uuid.New(),
		CardID:           arg.CardID,
		UserID:           arg.UserID,
		Grade:            arg.Grade,
		ElapsedMs:        arg.ElapsedMs,
		StabilityBefore:  arg.StabilityBefore,
		StabilityAfter:   arg.StabilityAfter,
		DifficultyBefore: arg.DifficultyBefore,
		DifficultyAfter:  arg.DifficultyAfter,
		ScheduledDays:    arg.ScheduledDays,
		SchedulerVersion: arg.SchedulerVersion,
		ReviewedAt:       arg.ReviewedAt,
	}
	f.logs = append(f.logs, log)
	return log, nil
}

func (f *fakeRepo) ListReviewLogsByCard(
	_ context.Context, cardID, userID uuid.UUID, limit int32) ([]sqlc.LearnReviewLog, error,
) {
	var result []sqlc.LearnReviewLog
	for _, l := range f.logs {
		if l.CardID == cardID && l.UserID == userID {
			result = append(result, l)
			if len(result) >= int(limit) {
				break
			}
		}
	}
	return result, nil
}

func (f *fakeRepo) SumRecentReviewElapsedMs(
	_ context.Context, userID uuid.UUID, since time.Time, limit int32,
) (int64, error) {
	recent := make([]sqlc.LearnReviewLog, 0, len(f.logs))
	for _, entry := range f.logs {
		if entry.UserID == userID && !entry.ReviewedAt.Before(since) {
			recent = append(recent, entry)
		}
	}
	sort.Slice(recent, func(i, j int) bool { return recent[i].ReviewedAt.After(recent[j].ReviewedAt) })
	if int(limit) < len(recent) {
		recent = recent[:limit]
	}
	var total int64
	for _, entry := range recent {
		total += int64(entry.ElapsedMs)
	}
	return total, nil
}

func (f *fakeRepo) UpsertReviewDailyStats(
	_ context.Context, arg sqlc.UpsertReviewDailyStatsParams) (sqlc.LearnReviewDailyStat, error,
) {
	key := arg.UserID.String() + ":" + arg.StatDate.Time.Format("2006-01-02")
	stat, ok := f.dailyStats[key]
	if !ok {
		stat = sqlc.LearnReviewDailyStat{
			ID:               uuid.New(),
			UserID:           arg.UserID,
			StatDate:         arg.StatDate,
			ReviewsCompleted: arg.ReviewsCompleted,
			NewCardsLearned:  arg.NewCardsLearned,
			TotalMinutes:     arg.TotalMinutes,
			CreatedAt:        time.Now().UTC(),
			UpdatedAt:        time.Now().UTC(),
		}
	} else {
		stat.ReviewsCompleted += arg.ReviewsCompleted
		stat.NewCardsLearned += arg.NewCardsLearned
		stat.TotalMinutes += arg.TotalMinutes
		stat.UpdatedAt = time.Now().UTC()
	}
	f.dailyStats[key] = stat
	return stat, nil
}

type fakeUserReader struct {
	users map[uuid.UUID]usercontract.Summary
}

func (u *fakeUserReader) GetByID(_ context.Context, id uuid.UUID) (usercontract.Summary, error) {
	s, ok := u.users[id]
	if !ok {
		return usercontract.Summary{}, apperr.New(apperr.NotFound, "USER_NOT_FOUND", "user not found")
	}
	return s, nil
}

func (u *fakeUserReader) GetManyByIDs(_ context.Context, ids []uuid.UUID) (map[uuid.UUID]usercontract.Summary, error) {
	res := make(map[uuid.UUID]usercontract.Summary)
	for _, id := range ids {
		if s, ok := u.users[id]; ok {
			res[id] = s
		}
	}
	return res, nil
}

func (u *fakeUserReader) Exists(_ context.Context, id uuid.UUID) (bool, error) {
	_, ok := u.users[id]
	return ok, nil
}

func TestSRS_TimezoneMidnightDayBoundary(t *testing.T) {
	// Fixed UTC time: 2026-08-25 20:00:00 UTC
	nowUTC := time.Date(2026, 8, 25, 20, 0, 0, 0, time.UTC)
	frozenClock := clock.NewFake(nowUTC)

	// User 1 in UTC: current local day is 2026-08-25. End of local day is 2026-08-25 23:59:59.999999999 UTC.
	userUTC := uuid.New()
	// User 2 in Asia/Ho_Chi_Minh (UTC+7): at 20:00 UTC the local time is already
	// 2026-08-26 03:00, so the end of their local day is 2026-08-26 23:59:59.999999999
	// +07:00 — 2026-08-26 16:59:59 UTC.
	userVN := uuid.New()

	userReader := &fakeUserReader{
		users: map[uuid.UUID]usercontract.Summary{
			userUTC: {ID: userUTC, Timezone: tzUTC},
			userVN:  {ID: userVN, Timezone: tzVietnam},
		},
	}

	repo := newFakeRepo()
	svc := service.New(service.Deps{
		Repo:  repo,
		Users: userReader,
		Clock: frozenClock,
	})

	// Add card for userUTC due at 2026-08-26 02:00:00 UTC (tomorrow for UTC learner)
	repo.cards[uuid.New()] = sqlc.LearnReviewCard{
		ID:               uuid.New(),
		UserID:           userUTC,
		ContentVersionID: uuid.New(),
		Skill:            skillVocabulary,
		Stability:        2.0,
		Difficulty:       5.0,
		DueAt:            time.Date(2026, 8, 26, 2, 0, 0, 0, time.UTC),
		State:            string(domain.StateReview),
	}

	// Add card for userVN due at 2026-08-26 02:00:00 UTC (today for VN learner because it's Aug 26 in VN)
	repo.cards[uuid.New()] = sqlc.LearnReviewCard{
		ID:               uuid.New(),
		UserID:           userVN,
		ContentVersionID: uuid.New(),
		Skill:            skillVocabulary,
		Stability:        2.0,
		Difficulty:       5.0,
		DueAt:            time.Date(2026, 8, 26, 2, 0, 0, 0, time.UTC),
		State:            string(domain.StateReview),
	}

	ctx := context.Background()

	// UTC learner: card is NOT due today
	dueCountUTC, err := svc.DueCount(ctx, userUTC)
	require.NoError(t, err)
	assert.Equal(t, 0, dueCountUTC, "Card scheduled for Aug 26 02:00 UTC is not due for UTC user on Aug 25")

	// VN learner: card IS due today
	dueCountVN, err := svc.DueCount(ctx, userVN)
	require.NoError(t, err)
	assert.Equal(t, 1, dueCountVN,
		"Aug 26 02:00 UTC is due for the VN learner, whose local date is already Aug 26")
}

func TestSRS_Lifecycle_UpsertAnswerSuspendReset(t *testing.T) {
	now := time.Date(2026, 8, 25, 10, 0, 0, 0, time.UTC)
	frozenClock := clock.NewFake(now)
	userID := uuid.New()
	contentID := uuid.New()

	repo := newFakeRepo()
	svc := service.New(service.Deps{
		Repo:  repo,
		Clock: frozenClock,
	})
	ctx := context.Background()

	// 1. Upsert card from exercise completion
	err := svc.UpsertCards(ctx, userID, []learningcontract.ReviewItem{
		{
			ContentVersionID: contentID,
			Skill:            skillVocabulary,
			InitialGrade:     gradeGood,
		},
	})
	require.NoError(t, err)

	card, err := repo.GetReviewCardByUserAndContent(ctx, userID, contentID)
	require.NoError(t, err)
	assert.Equal(t, "learning", card.State)
	assert.Equal(t, int32(1), card.Reps)

	// 2. Answer card with Good
	res, err := svc.AnswerCard(ctx, userID, card.ID, gradeGood, 1200)
	require.NoError(t, err)
	assert.Equal(t, "review", res.Card.State)
	assert.Greater(t, res.Card.Stability, card.Stability)
	assert.GreaterOrEqual(t, res.IntervalDays, 1)

	// Invariant: Review log written with stability before and after
	require.Len(t, repo.logs, 1)
	assert.Equal(t, card.Stability, repo.logs[0].StabilityBefore)
	assert.Equal(t, res.Card.Stability, repo.logs[0].StabilityAfter)

	// 3. Suspend card
	suspended, err := svc.SuspendCard(ctx, userID, card.ID)
	require.NoError(t, err)
	assert.NotNil(t, suspended.SuspendedAt)

	// Cannot answer suspended card
	_, err = svc.AnswerCard(ctx, userID, card.ID, gradeGood, 1000)
	require.Error(t, err)

	// 4. Reset card
	reset, err := svc.ResetCard(ctx, userID, card.ID)
	require.NoError(t, err)
	assert.Nil(t, reset.SuspendedAt)
	assert.Equal(t, "new", reset.State)
	assert.Equal(t, 0, reset.Reps)
	assert.Equal(t, 0, reset.Lapses)

	// 5. Complete session. Minutes are read back from the elapsed_ms the answer
	// in step 2 recorded, not estimated from the card count, so a session with
	// one 1.2-second answer reports zero whole minutes rather than a plausible
	// number nobody measured.
	sessionRes, err := svc.CompleteSession(ctx, userID, 10, 8)
	require.NoError(t, err)
	assert.Equal(t, 10, sessionRes.Reviewed)
	assert.Equal(t, 8, sessionRes.Correct)
	assert.Equal(t, 0, sessionRes.Minutes)
}

// TestSRS_CompleteSession_MinutesComeFromTheLogs proves the reported time on
// task is measured rather than guessed.
func TestSRS_CompleteSession_MinutesComeFromTheLogs(t *testing.T) {
	now := time.Date(2026, 8, 25, 10, 0, 0, 0, time.UTC)
	userID := uuid.New()

	repo := newFakeRepo()
	svc := service.New(service.Deps{Repo: repo, Clock: clock.NewFake(now)})
	ctx := context.Background()

	// Three answers taking 90 seconds each: 4 minutes 30 seconds of real work.
	for range 3 {
		repo.logs = append(repo.logs, sqlc.LearnReviewLog{
			ID:         uuid.New(),
			UserID:     userID,
			CardID:     uuid.New(),
			Grade:      gradeGood,
			ElapsedMs:  90_000,
			ReviewedAt: now,
		})
	}

	res, err := svc.CompleteSession(ctx, userID, 3, 3)
	require.NoError(t, err)
	assert.Equal(t, 4, res.Minutes, "270 seconds is four whole minutes")
}

// TestSRS_UpsertCards_DoesNotResetAnExistingSchedule guards the ON CONFLICT
// clause in UpsertReviewCard. Redoing a lesson activity re-runs the grader,
// which emits the same ReviewItem again; if the conflict path copied the initial
// stability over the stored one, a learner who revisits a unit would silently
// lose every review they had accumulated for those words.
func TestSRS_UpsertCards_DoesNotResetAnExistingSchedule(t *testing.T) {
	now := time.Date(2026, 8, 25, 10, 0, 0, 0, time.UTC)
	userID := uuid.New()
	contentID := uuid.New()

	repo := newFakeRepo()
	svc := service.New(service.Deps{Repo: repo, Clock: clock.NewFake(now)})
	ctx := context.Background()

	matureID := uuid.New()
	matured := sqlc.LearnReviewCard{
		ID:               matureID,
		UserID:           userID,
		ContentVersionID: contentID,
		Skill:            skillVocabulary,
		Stability:        42.0,
		Difficulty:       3.5,
		DueAt:            now.AddDate(0, 0, 40),
		Reps:             9,
		Lapses:           1,
		State:            string(domain.StateReview),
	}
	repo.cards[matureID] = matured

	err := svc.UpsertCards(ctx, userID, []learningcontract.ReviewItem{
		{ContentVersionID: contentID, Skill: skillVocabulary, InitialGrade: gradeGood},
	})
	require.NoError(t, err)

	after := repo.cards[matureID]
	assert.Equal(t, matured.Stability, after.Stability, "stability must survive a repeated attempt")
	assert.Equal(t, matured.Difficulty, after.Difficulty, "difficulty must survive a repeated attempt")
	assert.Equal(t, matured.DueAt, after.DueAt, "the schedule must survive a repeated attempt")
	assert.Equal(t, matured.Reps, after.Reps, "reps must survive a repeated attempt")
	assert.Equal(t, matured.Lapses, after.Lapses, "lapses must survive a repeated attempt")
	assert.Len(t, repo.cards, 1, "a repeated attempt must not create a second card")
}

// TestSRS_UpsertCards_FirstScheduleMatchesThePureFunction is the P9.5 acceptance
// criterion: the due date a graded attempt writes is exactly what the FSRS
// function returns for that grade and that `now`.
func TestSRS_UpsertCards_FirstScheduleMatchesThePureFunction(t *testing.T) {
	now := time.Date(2026, 8, 25, 23, 55, 0, 0, time.UTC)
	userID := uuid.New()

	for _, grade := range []string{"again", "hard", "good", "easy"} {
		t.Run(grade, func(t *testing.T) {
			contentID := uuid.New()
			repo := newFakeRepo()
			svc := service.New(service.Deps{Repo: repo, Clock: clock.NewFake(now)})

			err := svc.UpsertCards(context.Background(), userID, []learningcontract.ReviewItem{
				{ContentVersionID: contentID, Skill: skillVocabulary, InitialGrade: grade},
			})
			require.NoError(t, err)

			want := domain.Schedule(
				domain.CardState{State: domain.StateNew},
				domain.Rating(grade),
				now,
				domain.DefaultParameters(),
			)

			require.Len(t, repo.cards, 1)
			for _, got := range repo.cards {
				assert.Equal(t, want.DueAt, got.DueAt)
				assert.InDelta(t, want.Stability, got.Stability, 1e-9)
				assert.InDelta(t, want.Difficulty, got.Difficulty, 1e-9)
				assert.Equal(t, string(want.State), got.State)
			}
		})
	}
}

// TestSRS_SuspendedCardsLeaveTheDueQueue is the other half of the P9.4
// acceptance criterion: a card suspended through the contract — which is what
// `vocabulary` does when a learner marks a word known — stops being counted and
// stops being served, and resuming brings it back.
func TestSRS_SuspendedCardsLeaveTheDueQueue(t *testing.T) {
	now := time.Date(2026, 8, 25, 10, 0, 0, 0, time.UTC)
	userID := uuid.New()
	contentID := uuid.New()

	repo := newFakeRepo()
	cardID := uuid.New()
	repo.cards[cardID] = sqlc.LearnReviewCard{
		ID:               cardID,
		UserID:           userID,
		ContentVersionID: contentID,
		Skill:            skillVocabulary,
		Stability:        5,
		Difficulty:       5,
		DueAt:            now.Add(-time.Hour),
		State:            string(domain.StateReview),
	}

	svc := service.New(service.Deps{Repo: repo, Clock: clock.NewFake(now)})
	ctx := context.Background()

	count, err := svc.DueCount(ctx, userID)
	require.NoError(t, err)
	require.Equal(t, 1, count, "the card is due before anyone suspends it")

	require.NoError(t, svc.SetCardsSuspended(ctx, userID, []uuid.UUID{contentID}, true))

	count, err = svc.DueCount(ctx, userID)
	require.NoError(t, err)
	assert.Equal(t, 0, count, "a suspended card is not due")

	cards, err := svc.DueCards(ctx, userID, 20)
	require.NoError(t, err)
	assert.Empty(t, cards, "a suspended card is not served in a session")

	require.NoError(t, svc.SetCardsSuspended(ctx, userID, []uuid.UUID{contentID}, false))

	count, err = svc.DueCount(ctx, userID)
	require.NoError(t, err)
	assert.Equal(t, 1, count, "resuming puts the card back in the queue")
}

// TestSRS_Forecast_BucketsByLearnerLocalDay is the forecast counterpart of the
// timezone test: the same card in the same UTC instant belongs to different
// calendar days for two learners, and the projection has to agree with the due
// queue about which day that is.
func TestSRS_Forecast_BucketsByLearnerLocalDay(t *testing.T) {
	nowUTC := time.Date(2026, 8, 25, 20, 0, 0, 0, time.UTC)
	userUTC := uuid.New()
	userVN := uuid.New()

	repo := newFakeRepo()
	// Due 2026-08-26 02:00 UTC, which is 2026-08-26 09:00 in Asia/Ho_Chi_Minh.
	dueAt := time.Date(2026, 8, 26, 2, 0, 0, 0, time.UTC)
	for _, owner := range []uuid.UUID{userUTC, userVN} {
		id := uuid.New()
		repo.cards[id] = sqlc.LearnReviewCard{
			ID:               id,
			UserID:           owner,
			ContentVersionID: uuid.New(),
			Skill:            skillVocabulary,
			DueAt:            dueAt,
			State:            string(domain.StateReview),
		}
	}

	svc := service.New(service.Deps{
		Repo: repo,
		Users: &fakeUserReader{users: map[uuid.UUID]usercontract.Summary{
			userUTC: {ID: userUTC, Timezone: tzUTC},
			userVN:  {ID: userVN, Timezone: tzVietnam},
		}},
		Clock: clock.NewFake(nowUTC),
	})
	ctx := context.Background()

	forUTC, err := svc.Forecast(ctx, userUTC, 30)
	require.NoError(t, err)
	require.Len(t, forUTC, 1)
	assert.Equal(t, "2026-08-26", forUTC[0].Date)
	assert.Equal(t, 1, forUTC[0].DueCount)

	forVN, err := svc.Forecast(ctx, userVN, 30)
	require.NoError(t, err)
	require.Len(t, forVN, 1)
	assert.Equal(t, "2026-08-26", forVN[0].Date, "09:00 local on the 26th is the 26th, not the 25th")
}

// TestSRS_Forecast_StopsAtTheHorizon: a card due next year is not next month's
// workload.
func TestSRS_Forecast_StopsAtTheHorizon(t *testing.T) {
	now := time.Date(2026, 8, 25, 10, 0, 0, 0, time.UTC)
	userID := uuid.New()

	repo := newFakeRepo()
	for _, due := range []time.Time{now.AddDate(0, 0, 3), now.AddDate(0, 0, 400)} {
		id := uuid.New()
		repo.cards[id] = sqlc.LearnReviewCard{
			ID: id, UserID: userID, ContentVersionID: uuid.New(),
			Skill: skillVocabulary, DueAt: due, State: string(domain.StateReview),
		}
	}

	svc := service.New(service.Deps{Repo: repo, Clock: clock.NewFake(now)})

	days, err := svc.Forecast(context.Background(), userID, 30)
	require.NoError(t, err)
	require.Len(t, days, 1, "only the card inside the 30-day horizon counts")
	assert.Equal(t, "2026-08-28", days[0].Date)
}
