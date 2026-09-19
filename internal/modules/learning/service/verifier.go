package service

import (
	"context"
	"errors"
	"strings"

	learningcontract "github.com/fluentra/fluentra/internal/modules/learning/contract"
	lessoncontract "github.com/fluentra/fluentra/internal/modules/lesson/contract"
)

// VerifyItem runs the six checks that stand between a generated or authored item
// and a learner. A community submission is checked by exactly the same machine, for
// exactly the same reason: a wrong answer key is invisible to a reader and
// wrong for everyone who meets it.
func (s *Service) VerifyItem(ctx context.Context, req learningcontract.VerifyItemRequest) error {
	if strings.TrimSpace(req.Kind) == "" {
		return errors.New("check 1 (parse) failed: kind is empty")
	}
	if len(req.Body) == 0 {
		return errors.New("check 1 (parse) failed: body is empty")
	}

	existingActivities := make([]lessoncontract.Activity, 0, len(req.Existing))
	for _, raw := range req.Existing {
		existingActivities = append(existingActivities, lessoncontract.Activity{
			Config: raw,
		})
	}

	switch req.Kind {
	case kindListeningComprehension:
		_, err := s.verifyListeningCandidate(ctx, req.CEFRLevel, req.Body, existingActivities, req.BlindSolve)
		return err
	case kindReadingComprehension, kindGrammarTenseChoice, kindGrammarSentenceTransform:
		return s.checkCandidateWithBlindSolve(ctx, req.CEFRLevel, req.Kind, req.Body, existingActivities, req.BlindSolve)
	case kindWritingPrompt:
		_, err := s.checkWritingPrompt(ctx, req.Body, existingActivities)
		return err
	case kindSpeakingTask:
		_, err := s.checkSpeakingTask(ctx, req.TaskType, req.Body, existingActivities)
		return err
	default:
		// Vocabulary kinds and others
		return s.checkCandidateWithBlindSolve(ctx, req.CEFRLevel, req.Kind, req.Body, existingActivities, req.BlindSolve)
	}
}
