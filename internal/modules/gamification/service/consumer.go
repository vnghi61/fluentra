package service

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/fluentra/fluentra/internal/modules/gamification/contract"
	"github.com/fluentra/fluentra/internal/modules/gamification/domain"
	learningcontract "github.com/fluentra/fluentra/internal/modules/learning/contract"
	srscontract "github.com/fluentra/fluentra/internal/modules/srs/contract"
	vocabularycontract "github.com/fluentra/fluentra/internal/modules/vocabulary/contract"
)

// Delivery is one outbox event as this consumer needs it.
//
// A local type rather than eventbus.Message, so the consumer can be tested
// without the bus and the composition root stays the only place that knows
// which dispatcher is in use — the same reasoning as audit's.
type Delivery struct {
	// ID is the deduplication key. Delivery is at-least-once, and every path
	// below is idempotent on the event's own source_id rather than on this,
	// because a source_id survives a redelivery under a new outbox row.
	ID      uuid.UUID
	Topic   string
	Payload json.RawMessage
}

// SubscribedTopics is what the composition root subscribes this consumer to.
//
// Only the events that already exist. The module spec also lists
// `writing.graded`, `speaking.scored` and `exam.attempt_finished`; those
// modules are Phase 3 and 4 and publish nothing yet, and subscribing to a topic
// no one writes is a line of code that looks like a feature.
func SubscribedTopics() []string {
	return []string{
		learningcontract.EventActivityCompleted,
		learningcontract.EventLessonCompleted,
		learningcontract.EventLearningSessionCompleted,
		srscontract.EventReviewSessionCompleted,
		vocabularycontract.EventWordsVerified,
	}
}

// Consume handles one delivered event.
//
// Every path returns nil on a business-level miss — an unparseable payload, an
// unknown topic, a capped award. Returning an error nacks the message and asks
// for redelivery, and redelivering an event that gamification simply had
// nothing to do with is a loop, not a retry.
//
// BR-GAMIFICATION-08 in its strongest form: nothing this function does can
// fail a learning action, because by the time it runs the learning action is
// already committed.
func (s *Service) Consume(ctx context.Context, delivery Delivery) error {
	switch delivery.Topic {
	case learningcontract.EventActivityCompleted:
		return s.onActivityCompleted(ctx, delivery)
	case learningcontract.EventLessonCompleted:
		return s.onLessonCompleted(ctx, delivery)
	case learningcontract.EventLearningSessionCompleted, srscontract.EventReviewSessionCompleted:
		return s.onSessionCompleted(ctx, delivery)
	case vocabularycontract.EventWordsVerified:
		return s.onWordsVerified(ctx, delivery)
	default:
		return nil
	}
}

func (s *Service) onActivityCompleted(ctx context.Context, delivery Delivery) error {
	var payload learningcontract.ActivityCompleted
	if err := json.Unmarshal(delivery.Payload, &payload); err != nil {
		slog.WarnContext(ctx, "gamification could not read activity.completed", "error", err)
		return nil
	}
	if payload.UserID == uuid.Nil || payload.ActivityID == uuid.Nil {
		return nil
	}

	// The activity id is the idempotency key, not the outbox event id: a
	// learner redoing an activity should not be paid for it twice, and a
	// redelivery must not either. Both are the same key, which is the point.
	score := payload.Score
	if _, err := s.RecordActivity(ctx, contract.AwardRequest{
		UserID:   payload.UserID,
		Source:   string(domain.SourceActivity),
		SourceID: payload.ActivityID.String(),
		Score:    &score,
	}); err != nil {
		return err
	}
	s.advanceQuests(ctx, payload.UserID, questStepActivities)
	return nil
}

func (s *Service) onLessonCompleted(ctx context.Context, delivery Delivery) error {
	var payload learningcontract.LessonCompleted
	if err := json.Unmarshal(delivery.Payload, &payload); err != nil {
		slog.WarnContext(ctx, "gamification could not read lesson.completed", "error", err)
		return nil
	}
	if payload.UserID == uuid.Nil || payload.LessonID == uuid.Nil {
		return nil
	}

	if _, err := s.RecordActivity(ctx, contract.AwardRequest{
		UserID:   payload.UserID,
		Source:   string(domain.SourceLesson),
		SourceID: payload.LessonID.String(),
	}); err != nil {
		return err
	}
	s.advanceQuests(ctx, payload.UserID, questStepLessons)
	return nil
}

// sessionPayload is the intersection of the two session-completed events.
//
// `learning` and `srs` both publish one, with different names and the same two
// fields gamification needs. Reading them structurally means neither module has
// to change to feed the streak.
type sessionPayload struct {
	UserID    uuid.UUID `json:"user_id"`
	SessionID uuid.UUID `json:"session_id"`
}

func (s *Service) onSessionCompleted(ctx context.Context, delivery Delivery) error {
	var payload sessionPayload
	if err := json.Unmarshal(delivery.Payload, &payload); err != nil {
		slog.WarnContext(ctx, "gamification could not read session_completed", "error", err)
		return nil
	}
	if payload.UserID == uuid.Nil {
		return nil
	}

	sourceID := payload.SessionID.String()
	if payload.SessionID == uuid.Nil {
		// A session event without an id cannot be deduplicated on the session,
		// so it is keyed on the outbox row instead. A redelivery of *that* row
		// is caught; a genuinely separate event is not conflated with it.
		sourceID = delivery.ID.String()
	}

	if _, err := s.RecordActivity(ctx, contract.AwardRequest{
		UserID:   payload.UserID,
		Source:   string(domain.SourceReviewSession),
		SourceID: sourceID,
	}); err != nil {
		return err
	}
	s.advanceQuests(ctx, payload.UserID, questStepReviews)
	return nil
}

// onWordsVerified pays for a learner's own vocabulary once it has been checked.
//
// Per word, and at the lowest rate of any source: confirming that a word exists
// is not the same as having practised it, and paying it like an activity would
// make pasting a dictionary the cheapest way to level up. The per-source daily
// cap is the tightest there is for the same reason.
//
// Keyed on the outbox event id rather than on the words: the payload is a count
// from one verification run, and the run is what must not be paid for twice.
func (s *Service) onWordsVerified(ctx context.Context, delivery Delivery) error {
	var payload vocabularycontract.WordsVerified
	if err := json.Unmarshal(delivery.Payload, &payload); err != nil {
		slog.WarnContext(ctx, "gamification could not read words_verified", "error", err)
		return nil
	}
	if payload.UserID == uuid.Nil || payload.Count <= 0 {
		return nil
	}

	base := domain.BaseAward(domain.SourceUploadVerified) * payload.Count
	if _, err := s.RecordActivity(ctx, contract.AwardRequest{
		UserID:   payload.UserID,
		Source:   string(domain.SourceUploadVerified),
		SourceID: delivery.ID.String(),
		Amount:   base,
	}); err != nil {
		return err
	}
	return nil
}

// The step codes quests may count. Constants because they are the contract
// between an authored quest's `steps` and this file, and a typo in either would
// produce a quest that can never be completed.
const (
	questStepActivities = "complete_activities"
	questStepLessons    = "complete_lessons"
	questStepReviews    = "complete_reviews"
)

// advanceQuests moves every open quest that counts this step, and pays the
// reward for any that are now complete.
//
// Best-effort throughout: a quest that cannot be advanced is logged. The XP for
// the action itself is already written, and a learner must not lose it because
// a quest row was contended.
func (s *Service) advanceQuests(ctx context.Context, userID uuid.UUID, step string) {
	today := domain.LocalDay(s.clock.Now(), s.timezoneOf(ctx, userID))

	open, err := s.repo.ListOpenUserQuests(ctx, userID, today)
	if err != nil {
		slog.WarnContext(ctx, "quest progress skipped", "user_id", userID, "error", err)
		return
	}

	for _, row := range open {
		steps := domain.ParseSteps(row.Steps)
		if !countsStep(steps, step) {
			continue
		}

		progress := domain.ParseProgress(row.Progress)
		progress[step]++
		encoded, err := questProgressJSON(progress)
		if err != nil {
			continue
		}
		if _, err := s.repo.UpdateQuestProgress(ctx, row.UserQuestID, userID, encoded); err != nil {
			if !errors.Is(err, pgx.ErrNoRows) {
				slog.WarnContext(ctx, "quest progress not saved",
					"user_quest_id", row.UserQuestID, "error", err)
			}
			continue
		}

		if !domain.QuestComplete(steps, progress) {
			continue
		}
		if _, err := s.repo.CompleteUserQuest(ctx, row.UserQuestID, userID); err != nil {
			// No row means another delivery completed it first, and its reward
			// has already been paid. Not an error, and not a second payout.
			continue
		}

		reward := domain.QuestAward(int(row.RewardXp))
		if reward > 0 {
			if _, err := s.Award(ctx, contract.AwardRequest{
				UserID:   userID,
				Source:   string(domain.SourceQuest),
				SourceID: row.UserQuestID.String(),
				Amount:   reward,
			}); err != nil {
				slog.WarnContext(ctx, "quest reward not paid",
					"user_quest_id", row.UserQuestID, "error", err)
			}
		}
		s.publish(ctx, contract.EventQuestComplete, contract.QuestCompleted{
			UserID: userID, QuestCode: row.Code, RewardXP: reward,
			OccurredAt: s.clock.Now(),
		})
	}
}

func countsStep(steps []domain.QuestStep, code string) bool {
	for _, step := range steps {
		if step.Code == code {
			return true
		}
	}
	return false
}
