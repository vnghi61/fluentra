package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	contentcontract "github.com/fluentra/fluentra/internal/modules/content/contract"
	learningcontract "github.com/fluentra/fluentra/internal/modules/learning/contract"
	lessoncontract "github.com/fluentra/fluentra/internal/modules/lesson/contract"
	"github.com/fluentra/fluentra/internal/platform/ai"
)

// VerifyItem runs the eight checks that stand between a generated or authored item
// and a learner:
// 1. Schema & parse
// 2. Answer key scores full marks with own answer
// 3. Structure
// 4. Blind solve (if requested)
// 5. Deduplication against existing activities
// 6. Redaction verification
// 7. CEFR calibration check (if requested)
// 8. Exam structure & provenance checks (if requested)
func (s *Service) VerifyItem(ctx context.Context, req learningcontract.VerifyItemRequest) error {
	if strings.TrimSpace(req.Kind) == "" {
		return errors.New("check 1 (parse) failed: kind is empty")
	}
	if len(req.Body) == 0 {
		return errors.New("check 1 (parse) failed: body is empty")
	}

	// Structural Provenance Check (Brief §8 check 8)
	if req.CheckProvenance {
		if err := validateProvenance(req.Body); err != nil {
			return fmt.Errorf("check 8 (provenance) failed: %w", err)
		}
	}

	// Exam Structure Check (Stage E.1)
	if req.ExamConstraints != nil {
		if err := validateExamStructure(req.Kind, req.Body, req.ExamConstraints); err != nil {
			return err
		}
	}

	// CEFR Calibration Check (Stage E.1)
	if req.CheckCEFR && s.ai != nil {
		if err := s.validateCEFRLevel(ctx, req.Kind, req.CEFRLevel, req.Body); err != nil {
			return err
		}
	}

	existingActivities := make([]lessoncontract.Activity, 0, len(req.Existing))
	for _, raw := range req.Existing {
		existingActivities = append(existingActivities, lessoncontract.Activity{
			Config: raw,
		})
	}

	return s.dispatchItemVerification(ctx, req, existingActivities)
}

func (s *Service) dispatchItemVerification(
	ctx context.Context,
	req learningcontract.VerifyItemRequest,
	existing []lessoncontract.Activity,
) error {
	switch req.Kind {
	case kindListeningComprehension:
		minQ := 4
		if req.ExamConstraints != nil && req.ExamConstraints.QuestionsPerGroup > 0 {
			minQ = req.ExamConstraints.QuestionsPerGroup
		}
		_, err := s.verifyListeningCandidateWithMin(ctx, req.CEFRLevel, req.Body, existing, req.BlindSolve, minQ)
		return err
	case kindReadingComprehension:
		minQ := 4
		if req.ExamConstraints != nil && req.ExamConstraints.QuestionsPerGroup > 0 {
			minQ = req.ExamConstraints.QuestionsPerGroup
		}
		return s.checkReadingCandidateWithMin(ctx, req.CEFRLevel, req.Body, existing, req.BlindSolve, minQ)
	case kindGrammarTenseChoice, kindGrammarSentenceTransform:
		return s.checkCandidateWithBlindSolve(ctx, req.CEFRLevel, req.Kind, req.Body, existing, req.BlindSolve)
	case kindWritingPrompt:
		_, err := s.checkWritingPrompt(ctx, req.Body, existing)
		return err
	case kindSpeakingTask:
		_, err := s.checkSpeakingTask(ctx, req.TaskType, req.Body, existing)
		return err
	default:
		// Vocabulary kinds and others
		return s.checkCandidateWithBlindSolve(ctx, req.CEFRLevel, req.Kind, req.Body, existing, req.BlindSolve)
	}
}

func validateProvenance(raw json.RawMessage) error {
	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return fmt.Errorf("decode json body: %w", err)
	}
	provRaw, exists := decoded["_provenance"]
	if !exists || provRaw == nil {
		return errors.New("_provenance top-level object is missing")
	}
	prov, ok := provRaw.(map[string]any)
	if !ok {
		return errors.New("_provenance must be a JSON object")
	}
	if str, _ := prov["prompt_version"].(string); strings.TrimSpace(str) == "" {
		return errors.New("_provenance.prompt_version is missing or empty")
	}
	if str, _ := prov["model"].(string); strings.TrimSpace(str) == "" {
		return errors.New("_provenance.model is missing or empty")
	}
	if str, _ := prov["ai_request_id"].(string); strings.TrimSpace(str) == "" {
		return errors.New("_provenance.ai_request_id is missing or empty")
	}
	return nil
}

func validateListeningExamStructure(raw json.RawMessage, c *learningcontract.ExamPartConstraints) error {
	var cand listeningCand
	if err := json.Unmarshal(raw, &cand); err != nil {
		return fmt.Errorf("check (exam structure) failed: %w", err)
	}
	if c.QuestionsPerGroup > 0 && len(cand.Questions) != c.QuestionsPerGroup {
		return fmt.Errorf("check (exam structure) failed: questions per group is %d, want %d",
			len(cand.Questions), c.QuestionsPerGroup)
	}
	if c.OptionCount > 0 {
		for i, q := range cand.Questions {
			if len(q.Options) != c.OptionCount {
				return fmt.Errorf("check (exam structure) failed: question %d has %d options, want %d",
					i+1, len(q.Options), c.OptionCount)
			}
		}
	}
	if c.AudioRequired && strings.TrimSpace(cand.Script) == "" {
		return errors.New("check (exam structure) failed: audio script is required")
	}
	wc := len(strings.Fields(cand.Script))
	if c.MinWords > 0 && wc < c.MinWords {
		return fmt.Errorf("check (exam structure) failed: script has %d words, want at least %d", wc, c.MinWords)
	}
	if c.MaxWords > 0 && wc > c.MaxWords {
		return fmt.Errorf("check (exam structure) failed: script has %d words, want at most %d", wc, c.MaxWords)
	}
	return nil
}

func validateReadingExamStructure(raw json.RawMessage, c *learningcontract.ExamPartConstraints) error {
	var cand readingComprehensionCand
	if err := json.Unmarshal(raw, &cand); err != nil {
		return fmt.Errorf("check (exam structure) failed: %w", err)
	}
	if c.QuestionsPerGroup > 0 && len(cand.Questions) != c.QuestionsPerGroup {
		return fmt.Errorf("check (exam structure) failed: questions per group is %d, want %d",
			len(cand.Questions), c.QuestionsPerGroup)
	}
	if c.OptionCount > 0 {
		for i, q := range cand.Questions {
			if len(q.Options) != c.OptionCount {
				return fmt.Errorf("check (exam structure) failed: question %d has %d options, want %d",
					i+1, len(q.Options), c.OptionCount)
			}
		}
	}
	wc := len(strings.Fields(cand.Passage))
	if c.MinWords > 0 && wc < c.MinWords {
		return fmt.Errorf("check (exam structure) failed: passage has %d words, want at least %d", wc, c.MinWords)
	}
	if c.MaxWords > 0 && wc > c.MaxWords {
		return fmt.Errorf("check (exam structure) failed: passage has %d words, want at most %d", wc, c.MaxWords)
	}
	return nil
}

func validateGrammarExamStructure(raw json.RawMessage, c *learningcontract.ExamPartConstraints) error {
	var cand grammarTenseChoiceCand
	if err := json.Unmarshal(raw, &cand); err != nil {
		return fmt.Errorf("check (exam structure) failed: %w", err)
	}
	if c.OptionCount > 0 && len(cand.Options) != c.OptionCount {
		return fmt.Errorf("check (exam structure) failed: option count is %d, want %d",
			len(cand.Options), c.OptionCount)
	}
	return nil
}

func validateExamStructure(kind string, raw json.RawMessage, c *learningcontract.ExamPartConstraints) error {
	if c == nil {
		return nil
	}
	switch kind {
	case kindListeningComprehension:
		return validateListeningExamStructure(raw, c)
	case kindReadingComprehension:
		return validateReadingExamStructure(raw, c)
	case kindGrammarTenseChoice:
		return validateGrammarExamStructure(raw, c)
	default:
		return nil
	}
}

func (s *Service) validateCEFRLevel(ctx context.Context, kind, requestedLevel string, raw json.RawMessage) error {
	judgedLevel, reasoning, err := s.evaluateCEFR(ctx, kind, requestedLevel, raw)
	if err != nil {
		return fmt.Errorf("check 7 (cefr) failed: %w", err)
	}

	reqIdx, ok1 := cefrBandIndex(requestedLevel)
	judgedIdx, ok2 := cefrBandIndex(judgedLevel)
	if !ok1 || !ok2 {
		return fmt.Errorf("check 7 (cefr) failed: invalid CEFR level comparison: requested %q, judged %q",
			requestedLevel, judgedLevel)
	}
	diff := reqIdx - judgedIdx
	if diff < 0 {
		diff = -diff
	}
	if diff > 1 {
		return fmt.Errorf("check 7 (cefr) failed: item judged %s, requested %s (>1 band difference): %s",
			judgedLevel, requestedLevel, reasoning)
	}
	return nil
}

func (s *Service) evaluateCEFR(
	ctx context.Context, kind, requestedLevel string, raw json.RawMessage,
) (string, string, error) {
	if s.ai == nil {
		return requestedLevel, "", nil
	}
	redacted := contentcontract.RedactForLearner(raw)
	var reply struct {
		CEFRLevel  string  `json:"cefr_level"`
		Reasoning  string  `json:"reasoning"`
		Confidence float64 `json:"confidence"`
	}
	err := ai.CompleteJSON(ctx, s.ai, ai.Request{
		Task: ai.TaskItemLevel,
		Vars: map[string]any{
			varKind:          kind,
			"RequestedLevel": requestedLevel,
			"RedactedBody":   string(redacted),
		},
	}, &reply)
	if err != nil {
		return "", "", fmt.Errorf("ai item_level call failed: %w", err)
	}
	if reply.CEFRLevel == "" {
		reply.CEFRLevel = requestedLevel
	}
	return reply.CEFRLevel, reply.Reasoning, nil
}

func cefrBandIndex(level string) (int, bool) {
	switch strings.ToUpper(strings.TrimSpace(level)) {
	case "A1":
		return 1, true
	case "A2":
		return 2, true
	case "B1":
		return 3, true
	case "B2":
		return 4, true
	case "C1":
		return 5, true
	case "C2":
		return 6, true
	default:
		return 0, false
	}
}
