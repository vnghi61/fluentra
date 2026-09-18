package service

import (
	"context"
	"encoding/json"
	"log/slog"

	sqlc "github.com/fluentra/fluentra/internal/generated/exam/sqlc"
	"github.com/fluentra/fluentra/internal/modules/exam/domain"
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

	for i := range sections {
		for j := range sections[i].Items {
			item := &sections[i].Items[j]
			if raw, ok := answers[item.ActivityID.String()]; ok && !isEmptyAnswer(raw) {
				item.Response = raw
			}
			if s.lesson == nil {
				continue
			}
			activity, err := s.lesson.ResolveActivity(ctx, item.ActivityID)
			if err != nil || activity == nil {
				slog.WarnContext(ctx, "could not load an item for the report review",
					"attempt_id", attempt.ID, "activity_id", item.ActivityID, "error", err)
				continue
			}
			item.Content = activity.Config
		}
	}
}
