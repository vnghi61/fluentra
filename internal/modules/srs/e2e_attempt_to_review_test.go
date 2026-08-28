package srs_test

import (
	"context"
	"encoding/json"
	"sort"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fluentra/fluentra/internal/generated/srs/sqlc"
	contentcontract "github.com/fluentra/fluentra/internal/modules/content/contract"
	learningcontract "github.com/fluentra/fluentra/internal/modules/learning/contract"
	"github.com/fluentra/fluentra/internal/modules/srs/repository"
	"github.com/fluentra/fluentra/internal/modules/srs/service"
	usercontract "github.com/fluentra/fluentra/internal/modules/user/contract"
	vocabservice "github.com/fluentra/fluentra/internal/modules/vocabulary/service"
	"github.com/fluentra/fluentra/internal/shared/apperr"
	"github.com/fluentra/fluentra/internal/shared/clock"
)

type fakeContentReader struct {
	versions map[uuid.UUID]*contentcontract.Version
}

func (f *fakeContentReader) GetVersion(_ context.Context, id uuid.UUID) (*contentcontract.Version, error) {
	if v, ok := f.versions[id]; ok {
		return v, nil
	}
	return nil, nil
}

type fakeSRSRepo struct {
	cards      map[uuid.UUID]sqlc.LearnReviewCard
	logs       []sqlc.LearnReviewLog
	dailyStats map[string]sqlc.LearnReviewDailyStat
}

// fakeRepoNow is the instant this fake stamps rows with; see the note on fakeNow
// in service/service_test.go. The service runs on a frozen clock, so the
// repository must not stamp the real one.
var fakeRepoNow = time.Date(2026, 8, 25, 10, 0, 0, 0, time.UTC)

func newFakeSRSRepo() *fakeSRSRepo {
	return &fakeSRSRepo{
		cards:      make(map[uuid.UUID]sqlc.LearnReviewCard),
		logs:       make([]sqlc.LearnReviewLog, 0),
		dailyStats: make(map[string]sqlc.LearnReviewDailyStat),
	}
}

func (f *fakeSRSRepo) WithTx(_ pgx.Tx) repository.Repository {
	return f
}

func (f *fakeSRSRepo) UpsertReviewCard(
	_ context.Context, arg sqlc.UpsertReviewCardParams) (sqlc.LearnReviewCard, error,
) {
	for id, card := range f.cards {
		if card.UserID == arg.UserID && card.ContentVersionID == arg.ContentVersionID {
			// Mirrors the ON CONFLICT clause: an existing card keeps its schedule.
			card.Skill = arg.Skill
			card.UpdatedAt = fakeRepoNow
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
		CreatedAt:        fakeRepoNow,
		UpdatedAt:        fakeRepoNow,
	}
	f.cards[card.ID] = card
	return card, nil
}

func (f *fakeSRSRepo) GetReviewCardByID(_ context.Context, id, userID uuid.UUID) (sqlc.LearnReviewCard, error) {
	card, ok := f.cards[id]
	if !ok || card.UserID != userID {
		return sqlc.LearnReviewCard{}, apperr.New(apperr.NotFound, "NOT_FOUND", "card not found")
	}
	return card, nil
}

func (f *fakeSRSRepo) GetReviewCardByUserAndContent(
	_ context.Context, userID, contentVersionID uuid.UUID) (sqlc.LearnReviewCard, error,
) {
	for _, card := range f.cards {
		if card.UserID == userID && card.ContentVersionID == contentVersionID {
			return card, nil
		}
	}
	return sqlc.LearnReviewCard{}, apperr.New(apperr.NotFound, "NOT_FOUND", "card not found")
}

func (f *fakeSRSRepo) ListDueCards(
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

func (f *fakeSRSRepo) CountDueCards(_ context.Context, userID uuid.UUID, dueBefore time.Time) (int64, error) {
	var count int64
	for _, card := range f.cards {
		if card.UserID == userID && card.SuspendedAt == nil && !card.DueAt.After(dueBefore) {
			count++
		}
	}
	return count, nil
}

func (f *fakeSRSRepo) ForecastDueCards(
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

func (f *fakeSRSRepo) UpdateReviewCardSchedule(
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
	card.UpdatedAt = fakeRepoNow
	f.cards[arg.ID] = card
	return card, nil
}

func (f *fakeSRSRepo) SuspendReviewCard(_ context.Context, id, userID uuid.UUID) (sqlc.LearnReviewCard, error) {
	card, ok := f.cards[id]
	if !ok || card.UserID != userID {
		return sqlc.LearnReviewCard{}, apperr.New(apperr.NotFound, "NOT_FOUND", "card not found")
	}
	now := fakeRepoNow
	card.SuspendedAt = &now
	card.UpdatedAt = now
	f.cards[id] = card
	return card, nil
}

func (f *fakeSRSRepo) SetReviewCardsSuspended(
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
			at := fakeRepoNow
			card.SuspendedAt = &at
		} else {
			card.SuspendedAt = nil
		}
		f.cards[id] = card
		affected++
	}
	return affected, nil
}

func (f *fakeSRSRepo) ResetReviewCard(_ context.Context, arg sqlc.ResetReviewCardParams) (sqlc.LearnReviewCard, error) {
	card, ok := f.cards[arg.ID]
	if !ok || card.UserID != arg.UserID {
		return sqlc.LearnReviewCard{}, apperr.New(apperr.NotFound, "NOT_FOUND", "card not found")
	}
	now := fakeRepoNow
	card.Stability = arg.Stability
	card.Difficulty = arg.Difficulty
	card.DueAt = arg.DueAt
	card.Reps = 0
	card.Lapses = 0
	card.State = "new"
	card.SuspendedAt = nil
	card.UpdatedAt = now
	f.cards[card.ID] = card
	return card, nil
}

func (f *fakeSRSRepo) InsertReviewLog(_ context.Context, arg sqlc.InsertReviewLogParams) (sqlc.LearnReviewLog, error) {
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

func (f *fakeSRSRepo) ListReviewLogsByCard(
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

func (f *fakeSRSRepo) SumRecentReviewElapsedMs(
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

func (f *fakeSRSRepo) UpsertReviewDailyStats(
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
			CreatedAt:        fakeRepoNow,
			UpdatedAt:        fakeRepoNow,
		}
	} else {
		stat.ReviewsCompleted += arg.ReviewsCompleted
		stat.NewCardsLearned += arg.NewCardsLearned
		stat.TotalMinutes += arg.TotalMinutes
		stat.UpdatedAt = fakeRepoNow
	}
	f.dailyStats[key] = stat
	return stat, nil
}

type fakeUserReader struct{}

func (fakeUserReader) GetByID(_ context.Context, id uuid.UUID) (usercontract.Summary, error) {
	return usercontract.Summary{ID: id, Timezone: "UTC"}, nil
}

func (fakeUserReader) GetManyByIDs(_ context.Context, ids []uuid.UUID) (map[uuid.UUID]usercontract.Summary, error) {
	res := make(map[uuid.UUID]usercontract.Summary)
	for _, id := range ids {
		res[id] = usercontract.Summary{ID: id, Timezone: "UTC"}
	}
	return res, nil
}

func (fakeUserReader) Exists(_ context.Context, _ uuid.UUID) (bool, error) { return true, nil }

// The word this journey is about; goconst objects to the literal repeating.
const lemmaMeticulous = "meticulous"

// fakeSenses stands in for the vocabulary repository's lemma lookup.
type fakeSenses struct{ byLemma map[string]uuid.UUID }

func (f fakeSenses) GetSenseContentVersionByLemma(
	_ context.Context, lemma string,
) (*uuid.UUID, error) {
	id, ok := f.byLemma[lemma]
	if !ok {
		return nil, nil
	}
	return &id, nil
}

func TestE2E_AttemptToReview_Pipeline(t *testing.T) {
	// Setup deterministic clock
	fixedTime := time.Date(2026, 8, 25, 14, 0, 0, 0, time.UTC)
	frozenClock := clock.NewFake(fixedTime)

	userID := uuid.New()
	contentID := uuid.New()
	senseID := uuid.New()

	// 1. Two content versions, because there are two different things here.
	//
	// The activity's body is an answer key: what this exercise accepts. The
	// sense's body is the dictionary entry: what a flashcard shows. Conflating
	// them is what left every review card rendering "This card has no content
	// yet" — the schedule pointed at the quiz, and a quiz has no front and back.
	bodyJSON, _ := json.Marshal(map[string]any{
		"correct_answer": lemmaMeticulous,
		"prompt":         "Showing great attention to detail.",
	})
	senseBodyJSON, _ := json.Marshal(map[string]any{
		"word":             lemmaMeticulous,
		"pos":              "adjective",
		"ipa":              "/məˈtɪkjələs/",
		"definition":       "Showing great attention to detail.",
		"example_sentence": "She kept meticulous records.",
		"correct_answer":   lemmaMeticulous,
	})
	contentReader := &fakeContentReader{
		versions: map[uuid.UUID]*contentcontract.Version{
			contentID: {
				ID:        contentID,
				Kind:      "vocabulary_quiz",
				Body:      bodyJSON,
				CEFRLevel: "B2",
			},
			senseID: {
				ID:        senseID,
				Kind:      "vocab_flashcard",
				Body:      senseBodyJSON,
				CEFRLevel: "B2",
			},
		},
	}

	// 2. Grader evaluates learner's attempt
	vocabGrader := vocabservice.NewGrader(contentReader, &fakeSenses{
		byLemma: map[string]uuid.UUID{lemmaMeticulous: senseID},
	})
	userResp, _ := json.Marshal(map[string]string{
		"answer": lemmaMeticulous,
	})

	ctx := context.Background()
	gradeResult, err := vocabGrader.Grade(ctx, learningcontract.GradeRequest{
		AttemptID:        uuid.New(),
		ActivityID:       uuid.New(),
		ContentVersionID: contentID,
		UserID:           userID,
		Response:         userResp,
	})
	require.NoError(t, err)
	assert.True(t, gradeResult.Correct)
	assert.Equal(t, 100, gradeResult.Score)
	require.Len(t, gradeResult.ReviewItems, 1)
	assert.Equal(t, "good", gradeResult.ReviewItems[0].InitialGrade)
	assert.Equal(t, senseID, gradeResult.ReviewItems[0].ContentVersionID,
		"the card schedules the word, not the exercise that asked about it")

	// 3. SRS CardWriter receives review items and upserts card
	srsRepo := newFakeSRSRepo()
	srsService := service.New(service.Deps{
		Repo:  srsRepo,
		Users: fakeUserReader{},
		Clock: frozenClock,
	})

	err = srsService.UpsertCards(ctx, userID, gradeResult.ReviewItems)
	require.NoError(t, err)

	// 4. Advance time by 4 days so the scheduled card is due for review
	frozenClock.Advance(4 * 24 * time.Hour)
	nowDue := frozenClock.Now()

	dueCount, err := srsService.DueCount(ctx, userID)
	require.NoError(t, err)
	assert.Equal(t, 1, dueCount, "Card is due after interval elapsed")

	dueCards, err := srsService.DueCards(ctx, userID, 20)
	require.NoError(t, err)
	require.Len(t, dueCards, 1)
	card := dueCards[0]
	assert.Equal(t, senseID, card.ContentVersionID)
	assert.Equal(t, "vocabulary", card.Skill)

	// The assertion this journey was missing.
	//
	// Every step above passed while the card was unrenderable: the pipeline
	// proved a card existed and was due, and never that there was anything on
	// it. web/src/features/review/model/flashcard.ts yields null unless `word`
	// and `definition` are both present, and null is the "This card has no
	// content yet" screen — so these two keys are the difference between a
	// review session and an apology.
	resolved, err := contentReader.GetVersion(ctx, card.ContentVersionID)
	require.NoError(t, err)
	require.NotNil(t, resolved, "the card points at content that does not exist")

	var front map[string]any
	require.NoError(t, json.Unmarshal(resolved.Body, &front))
	assert.NotEmpty(t, front["word"], "a card with no `word` renders as unavailable")
	assert.NotEmpty(t, front["definition"], "a card with no `definition` has no back")

	// 5. Learner answers review session card with "good" rating
	answerRes, err := srsService.AnswerCard(ctx, userID, card.ID, "good", 2100)
	require.NoError(t, err)

	// Invariant checks:
	// - State is now "review"
	// - NextDueAt is scheduled in the future (> nowDue)
	// - IntervalDays > 0
	// - Reps incremented to 2
	assert.Equal(t, "review", answerRes.Card.State)
	assert.Greater(t, answerRes.IntervalDays, 0)
	assert.True(t, answerRes.NextDueAt.After(nowDue))
	assert.Equal(t, 2, answerRes.Card.Reps)

	// 6. Review log is recorded in review_logs
	require.Len(t, srsRepo.logs, 1)
	log := srsRepo.logs[0]
	assert.Equal(t, card.ID, log.CardID)
	assert.Equal(t, userID, log.UserID)
	assert.Equal(t, "good", log.Grade)
	assert.Equal(t, 2100, int(log.ElapsedMs))
	assert.Equal(t, answerRes.IntervalDays, int(log.ScheduledDays))

	// 7. Complete review session updates daily stats
	sessionResult, err := srsService.CompleteSession(ctx, userID, 1, 1)
	require.NoError(t, err)
	assert.Equal(t, 1, sessionResult.Reviewed)
	assert.Equal(t, 1, sessionResult.Correct)
}
