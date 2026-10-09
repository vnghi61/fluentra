package service

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/google/uuid"

	sqlc "github.com/fluentra/fluentra/internal/generated/exam/sqlc"
	"github.com/fluentra/fluentra/internal/modules/exam/domain"
	lessoncontract "github.com/fluentra/fluentra/internal/modules/lesson/contract"
)

// attachReview adds to each item of a submitted sitting's report what the learner
// answered and the item as authored.
//
// The report used to say "1/4 questions correct" and nothing else: not which
// question, not what was chosen, not the right answer or why. A learner reviewing
// a sitting needs all four. The item body carries the answers, the script and the
// explanations, which is exactly why it is attached only here: a report exists for
// a submitted sitting, so nothing reaches the learner before grading (ADR-0025).
// Read at request time and never stored, so a report does not grow a copy of
// every item it draws.
func (s *Service) attachReview(ctx context.Context, attempt *sqlc.AssessExamAttempt, sections []domain.SectionOutcome) {
	if attempt == nil || attempt.Status == domain.StatusInProgress {
		return
	}
	var answers map[string]json.RawMessage
	if len(attempt.DraftAnswers) > 0 {
		_ = json.Unmarshal(attempt.DraftAnswers, &answers)
	}

	var ids []uuid.UUID
	for _, section := range sections {
		for _, item := range section.Items {
			ids = append(ids, item.ActivityID)
		}
	}
	resolved := s.resolveActivities(ctx, ids)

	for i := range sections {
		items := sections[i].Items // shares the backing array, so writes land in sections
		for j := range items {
			item := &items[j]
			if raw, ok := answers[item.ActivityID.String()]; ok && !isEmptyAnswer(raw) {
				item.Response = raw
			}
			if s.lesson == nil {
				continue
			}
			activity := resolved[item.ActivityID]
			if activity == nil {
				slog.WarnContext(ctx, "could not load an item for the report review",
					"attempt_id", attempt.ID, "activity_id", item.ActivityID)
				continue
			}
			item.Content = activity.Config
		}
	}
}

// resolveActivities reads every activity in one call. A failure leaves the map
// empty, so each item keeps the defaults it had before resolution.
func (s *Service) resolveActivities(
	ctx context.Context, ids []uuid.UUID,
) map[uuid.UUID]*lessoncontract.ActivityHierarchy {
	if s.lesson == nil || len(ids) == 0 {
		return nil
	}
	found, err := s.lesson.ResolveActivities(ctx, ids)
	if err != nil {
		slog.WarnContext(ctx, "could not resolve exam activities", "count", len(ids), "error", err)
		return nil
	}
	return found
}
