package service

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math/big"
	"strings"
	"time"
	"unicode"

	"github.com/google/uuid"

	contentcontract "github.com/fluentra/fluentra/internal/modules/content/contract"
	learningcontract "github.com/fluentra/fluentra/internal/modules/learning/contract"
	"github.com/fluentra/fluentra/internal/modules/learning/domain"
	lessoncontract "github.com/fluentra/fluentra/internal/modules/lesson/contract"
	"github.com/fluentra/fluentra/internal/platform/ai"
	"github.com/fluentra/fluentra/internal/shared/apperr"
	"github.com/fluentra/fluentra/internal/shared/dbx"
	"github.com/jackc/pgx/v5"
)



const (
	PracticePoolCourseSlug = "pool-practice"
	PracticePoolCourseTitle = "Practice Pool"
	PracticePoolDescription = "Auto-generated practice exercises pool"

	TargetActiveItemsPerSlot = 50
	MaxActiveItemsPerSlot    = 200
	MaxItemsToAddPerRun      = 5
	MaxRetriesPerItem        = 2 // Up to 3 attempts total (initial + 2 retries)

	HoChiMinhTimeZone = "Asia/Ho_Chi_Minh"
)

var (
	PracticeLevels = []string{"A2", "B1", "B2"}
	PracticeKinds  = []string{
		"reading_comprehension",
		"grammar_tense_choice",
		"grammar_sentence_transform",
	}
)

type practiceSlot struct {
	Level       string
	Kind        string
	LessonTitle string
}

func practiceSlots() []practiceSlot {
	return []practiceSlot{
		{Level: "A2", Kind: "reading_comprehension", LessonTitle: "Reading Comprehension"},
		{Level: "A2", Kind: "grammar_tense_choice", LessonTitle: "Grammar Tense Choice"},
		{Level: "A2", Kind: "grammar_sentence_transform", LessonTitle: "Grammar Sentence Transform"},
		{Level: "B1", Kind: "reading_comprehension", LessonTitle: "Reading Comprehension"},
		{Level: "B1", Kind: "grammar_tense_choice", LessonTitle: "Grammar Tense Choice"},
		{Level: "B1", Kind: "grammar_sentence_transform", LessonTitle: "Grammar Sentence Transform"},
		{Level: "B2", Kind: "reading_comprehension", LessonTitle: "Reading Comprehension"},
		{Level: "B2", Kind: "grammar_tense_choice", LessonTitle: "Grammar Tense Choice"},
		{Level: "B2", Kind: "grammar_sentence_transform", LessonTitle: "Grammar Sentence Transform"},
	}
}

// EnsurePracticePoolStructure ensures the course, units (A2, B1, B2), and lessons exist.
func (s *Service) EnsurePracticePoolStructure(ctx context.Context) error {
	if s.lessonAuthor == nil {
		return errors.New("lesson author dependency is required for practice pool structure")
	}

	courseID, err := s.lessonAuthor.EnsureCourse(ctx, lessoncontract.CourseSpec{
		Slug:           PracticePoolCourseSlug,
		Title:          PracticePoolCourseTitle,
		Description:    PracticePoolDescription,
		CEFRFrom:       "A2",
		CEFRTo:         "B2",
		EstimatedHours: 100,
	})
	if err != nil {
		return fmt.Errorf("ensure practice pool course: %w", err)
	}

	units := []struct {
		Position    int
		Title       string
		Description string
	}{
		{Position: 1, Title: "A2", Description: "A2 practice pool"},
		{Position: 2, Title: "B1", Description: "B1 practice pool"},
		{Position: 3, Title: "B2", Description: "B2 practice pool"},
	}

	lessons := []struct {
		Position   int
		Title      string
		SkillFocus string
	}{
		{Position: 1, Title: "Reading Comprehension", SkillFocus: "reading"},
		{Position: 2, Title: "Grammar Tense Choice", SkillFocus: "grammar"},
		{Position: 3, Title: "Grammar Sentence Transform", SkillFocus: "grammar"},
	}

	for _, u := range units {
		unitID, err := s.lessonAuthor.EnsureUnit(ctx, lessoncontract.UnitSpec{
			CourseID:    courseID,
			Position:    u.Position,
			Title:       u.Title,
			Description: u.Description,
		})
		if err != nil {
			return fmt.Errorf("ensure practice pool unit %s: %w", u.Title, err)
		}

		for _, l := range lessons {
			_, err := s.lessonAuthor.EnsureLesson(ctx, lessoncontract.LessonSpec{
				UnitID:           unitID,
				Position:         l.Position,
				Title:            l.Title,
				SkillFocus:       l.SkillFocus,
				EstimatedMinutes: 10,
			})
			if err != nil {
				return fmt.Errorf("ensure practice pool lesson %s in unit %s: %w", l.Title, u.Title, err)
			}
		}
	}

	return nil
}

// TopUpPracticePool checks every slot and generates up to 5 verified items if needed.
func (s *Service) TopUpPracticePool(ctx context.Context) error {
	if s.ai == nil {
		slog.WarnContext(ctx, "practice pool top-up skipped: AI client is nil")
		return nil
	}
	if s.contentAuthor == nil || s.lessonAuthor == nil {
		return errors.New("content author and lesson author dependencies are required for practice pool top-up")
	}

	if err := s.EnsurePracticePoolStructure(ctx); err != nil {
		return fmt.Errorf("ensure pool structure: %w", err)
	}

	for _, slot := range practiceSlots() {
		count, err := s.repo.CountActivePoolActivitiesForSlot(ctx, slot.Level, slot.Kind)
		if err != nil {
			slog.ErrorContext(ctx, "failed to count active pool activities", "level", slot.Level, "kind", slot.Kind, "error", err)
			continue
		}

		toAdd := 0
		if count < TargetActiveItemsPerSlot {
			toAdd = MaxItemsToAddPerRun
			if remaining := int(TargetActiveItemsPerSlot - count); remaining < toAdd {
				toAdd = remaining
			}
		} else if count < MaxActiveItemsPerSlot {
			hasActiveLearner, err := s.repo.HasActiveUserWithFewUnseenItems(ctx, slot.Level, slot.Kind)
			if err != nil {
				slog.ErrorContext(ctx, "failed to check active learners for slot", "level", slot.Level, "kind", slot.Kind, "error", err)
				continue
			}
			if hasActiveLearner {
				toAdd = MaxItemsToAddPerRun
				if remaining := int(MaxActiveItemsPerSlot - count); remaining < toAdd {
					toAdd = remaining
				}
			}
		}

		if toAdd <= 0 {
			continue
		}

		lessonID, err := s.repo.GetPoolLessonID(ctx, slot.Level, slot.LessonTitle)
		if err != nil {
			slog.ErrorContext(ctx, "failed to find pool lesson ID", "level", slot.Level, "lessonTitle", slot.LessonTitle, "error", err)
			continue
		}

		addedCount := 0
		for i := 0; i < toAdd; i++ {
			added, err := s.generateAndVerifyItem(ctx, slot.Level, slot.Kind, lessonID)
			if err != nil {
				slog.WarnContext(ctx, "item generation failed", "level", slot.Level, "kind", slot.Kind, "error", err)
				continue
			}
			if added {
				addedCount++
			}
		}

		slog.InfoContext(ctx, "practice pool slot top-up complete", "level", slot.Level, "kind", slot.Kind, "added", addedCount)
	}

	return nil
}

// generateAndVerifyItem attempts generation with up to 2 retries (3 total attempts).
func (s *Service) generateAndVerifyItem(ctx context.Context, level, kind string, lessonID uuid.UUID) (bool, error) {
	var lastErr error

	for attempt := 0; attempt <= MaxRetriesPerItem; attempt++ {
		success, err := s.tryGenerateAndVerify(ctx, level, kind, lessonID)
		if success {
			return true, nil
		}
		lastErr = err
		slog.WarnContext(ctx, "practice pool item verification attempt failed",
			"level", level, "kind", kind, "attempt", attempt+1, "reason", err)
	}

	return false, lastErr
}

func (s *Service) tryGenerateAndVerify(ctx context.Context, level, kind string, lessonID uuid.UUID) (bool, error) {
	// Call AI to generate practice item
	rawResponse, err := s.ai.Complete(ctx, ai.Request{
		Task: ai.TaskPracticeGenerate,
		Vars: map[string]any{
			"Kind":      kind,
			"CEFRLevel": level,
		},
	})
	if err != nil {
		return false, fmt.Errorf("ai generate call failed: %w", err)
	}

	rawJSON := []byte(strings.TrimSpace(rawResponse.Text))

	// CHECK 1: Parse into grader's own body type
	if err := validateParse(kind, rawJSON); err != nil {
		return false, fmt.Errorf("check 1 (parse) failed: %w", err)
	}

	// CHECK 2: Score full marks through real grader
	grader, ok := s.graders.Get(kind)
	if !ok || grader == nil {
		return false, fmt.Errorf("grader not registered for kind: %s", kind)
	}

	tempVersionID := uuid.New()
	tempVer := &contentcontract.Version{
		ID:        tempVersionID,
		Kind:      kind,
		Body:      rawJSON,
		CEFRLevel: level,
		Status:    "published",
	}
	tempCtx := contentcontract.ContextWithTempVersion(ctx, tempVer)

	ownAnswerPayload, err := buildOwnAnswerPayload(kind, rawJSON)
	if err != nil {
		return false, fmt.Errorf("build own answer payload: %w", err)
	}

	gradeRes, err := grader.Grade(tempCtx, learningcontract.GradeRequest{
		ContentVersionID: tempVersionID,
		Response:         ownAnswerPayload,
	})
	if err != nil || gradeRes.Score < 100 || !gradeRes.Correct {
		return false, fmt.Errorf("check 2 (own answer scores full marks) failed: score=%d correct=%v err=%v",
			gradeRes.Score, gradeRes.Correct, err)
	}

	// CHECK 3: Structure
	if err := validateStructure(kind, rawJSON); err != nil {
		return false, fmt.Errorf("check 3 (structure) failed: %w", err)
	}

	// CHECK 4: Blind solve
	redactedJSON := contentcontract.RedactForLearner(rawJSON)
	solveResponse, err := s.ai.Complete(ctx, ai.Request{
		Task: ai.TaskPracticeSolve,
		Vars: map[string]any{
			"Kind":         kind,
			"RedactedBody": string(redactedJSON),
		},
	})
	if err != nil {
		return false, fmt.Errorf("ai blind solve call failed: %w", err)
	}

	solvePayload, err := parseBlindSolvePayload(kind, []byte(strings.TrimSpace(solveResponse.Text)))

	if err != nil {
		return false, fmt.Errorf("check 4 (blind solve response parse) failed: %w", err)
	}

	solveGradeRes, err := grader.Grade(tempCtx, learningcontract.GradeRequest{
		ContentVersionID: tempVersionID,
		Response:         solvePayload,
	})
	if err != nil || solveGradeRes.Score < 100 || !solveGradeRes.Correct {
		return false, fmt.Errorf("check 4 (blind solve agreement) failed: score=%d correct=%v err=%v",
			solveGradeRes.Score, solveGradeRes.Correct, err)
	}

	// CHECK 5: Deduplication against existing items in the slot
	existingItems, err := s.repo.ListPoolActivitiesForSlot(ctx, level, kind)
	if err != nil {
		return false, fmt.Errorf("list existing slot items: %w", err)
	}
	if isDuplicate(kind, rawJSON, existingItems) {
		return false, errors.New("check 5 (deduplication) failed: item matches existing item in slot")
	}

	// CHECK 6: Redaction verification (learner-facing body contains no answers)
	if err := verifyRedaction(redactedJSON); err != nil {
		return false, fmt.Errorf("check 6 (redaction verification) failed: %w", err)
	}

	// All 6 checks passed -> publish item and append activity
	slug := fmt.Sprintf("pool-%s-%s-%s", strings.ToLower(level), strings.ToLower(kind), uuid.New().String()[:8])
	publishedVersionID, err := s.contentAuthor.EnsurePublished(ctx, contentcontract.AuthorSpec{
		Slug:      slug,
		Kind:      kind,
		CEFRLevel: level,
		Body:      rawJSON,
		AuthorID:  uuid.Nil,
	})
	if err != nil {
		return false, fmt.Errorf("publish verified content: %w", err)
	}

	_, err = s.lessonAuthor.AppendActivity(ctx, lessonID, lessoncontract.ActivitySpec{
		Kind:             kind,
		ContentVersionID: publishedVersionID,
		Config:           rawJSON,
		Weight:           1,
	})
	if err != nil {
		return false, fmt.Errorf("append activity to pool lesson: %w", err)
	}

	return true, nil
}

// --------------------------------------------------------------------------
// Validation helpers for the 6 checks
// --------------------------------------------------------------------------

type candExplanation struct {
	Text          string `json:"text,omitempty"`
	TextVi        string `json:"text_vi,omitempty"`
	ExplanationEn string `json:"explanation_en,omitempty"`
	ExplanationVi string `json:"explanation_vi,omitempty"`
}

func (e *candExplanation) Vi() string {
	if e == nil {
		return ""
	}
	if strings.TrimSpace(e.TextVi) != "" {
		return strings.TrimSpace(e.TextVi)
	}
	return strings.TrimSpace(e.ExplanationVi)
}

type candOption struct {
	ID   string `json:"id"`
	Text string `json:"text"`
}

type candQuestion struct {
	ID              string           `json:"id"`
	Type            string           `json:"type"`
	Prompt          string           `json:"prompt"`
	Options         []candOption     `json:"options,omitempty"`
	CorrectOptionID string           `json:"correct_option_id,omitempty"`
	CorrectAnswer   string           `json:"correct_answer,omitempty"`
	Acceptable      []string         `json:"acceptable,omitempty"`
	Explanation     *candExplanation `json:"explanation,omitempty"`
}

type readingComprehensionCand struct {
	PassageTitle string         `json:"passage_title,omitempty"`
	Passage      string         `json:"passage"`
	Questions    []candQuestion `json:"questions"`
}

type grammarTenseChoiceCand struct {
	Prompt          string           `json:"prompt"`
	Options         []candOption     `json:"options"`
	CorrectOptionID string           `json:"correct_option_id"`
	Explanation     *candExplanation `json:"explanation,omitempty"`
}

type grammarSentenceTransformCand struct {
	Prompt        string           `json:"prompt"`
	CorrectAnswer string           `json:"correct_answer"`
	Acceptable    []string         `json:"acceptable,omitempty"`
	Explanation   *candExplanation `json:"explanation,omitempty"`
}


func validateParse(kind string, raw []byte) error {
	switch kind {
	case "reading_comprehension":
		var body readingComprehensionCand
		if err := json.Unmarshal(raw, &body); err != nil {
			return err
		}
		if strings.TrimSpace(body.Passage) == "" {
			return errors.New("passage is empty")
		}
		if len(body.Questions) < 4 || len(body.Questions) > 6 {
			return fmt.Errorf("expected 4-6 questions, got %d", len(body.Questions))
		}
		for i, q := range body.Questions {
			if strings.TrimSpace(q.Prompt) == "" {
				return fmt.Errorf("question %d prompt is empty", i)
			}
			if len(q.Options) != 4 {
				return fmt.Errorf("question %d expected 4 options, got %d", i, len(q.Options))
			}
			if strings.TrimSpace(q.CorrectOptionID) == "" {
				return fmt.Errorf("question %d correct_option_id is empty", i)
			}
		}
		return nil

	case "grammar_tense_choice":
		var body grammarTenseChoiceCand
		if err := json.Unmarshal(raw, &body); err != nil {
			return err
		}
		if strings.TrimSpace(body.Prompt) == "" {
			return errors.New("prompt is empty")
		}
		if len(body.Options) != 4 {
			return fmt.Errorf("expected 4 options, got %d", len(body.Options))
		}
		if strings.TrimSpace(body.CorrectOptionID) == "" {
			return errors.New("correct_option_id is empty")
		}
		return nil

	case "grammar_sentence_transform":
		var body grammarSentenceTransformCand
		if err := json.Unmarshal(raw, &body); err != nil {
			return err
		}
		if strings.TrimSpace(body.Prompt) == "" {
			return errors.New("prompt is empty")
		}
		if strings.TrimSpace(body.CorrectAnswer) == "" {
			return errors.New("correct_answer is empty")
		}
		return nil

	default:
		return fmt.Errorf("unsupported kind: %s", kind)
	}
}

func buildOwnAnswerPayload(kind string, raw []byte) (json.RawMessage, error) {
	switch kind {
	case "reading_comprehension":
		var body readingComprehensionCand
		if err := json.Unmarshal(raw, &body); err != nil {
			return nil, err
		}
		answers := make(map[string]string, len(body.Questions))
		for _, q := range body.Questions {
			answers[q.ID] = q.CorrectOptionID
		}
		return json.Marshal(map[string]any{"answers": answers})

	case "grammar_tense_choice":
		var body grammarTenseChoiceCand
		if err := json.Unmarshal(raw, &body); err != nil {
			return nil, err
		}
		return json.Marshal(map[string]any{"selected_option_id": body.CorrectOptionID})

	case "grammar_sentence_transform":
		var body grammarSentenceTransformCand
		if err := json.Unmarshal(raw, &body); err != nil {
			return nil, err
		}
		return json.Marshal(map[string]any{"answer": body.CorrectAnswer})

	default:
		return nil, fmt.Errorf("unsupported kind: %s", kind)
	}
}

func validateStructure(kind string, raw []byte) error {
	switch kind {
	case "reading_comprehension":
		var body readingComprehensionCand
		if err := json.Unmarshal(raw, &body); err != nil {
			return err
		}
		for i, q := range body.Questions {
			if q.Explanation == nil || q.Explanation.Vi() == "" {
				return fmt.Errorf("question %d explanation_vi is empty", i)
			}
			foundCorrect := false
			seenOptions := make(map[string]bool)
			for _, opt := range q.Options {
				if opt.ID == q.CorrectOptionID {
					foundCorrect = true
				}
				norm := normaliseText(opt.Text)
				if seenOptions[norm] {
					return fmt.Errorf("question %d has duplicate option: %q", i, opt.Text)
				}
				seenOptions[norm] = true
			}
			if !foundCorrect {
				return fmt.Errorf("question %d correct_option_id %q not found in options", i, q.CorrectOptionID)
			}
		}
		return nil

	case "grammar_tense_choice":
		var body grammarTenseChoiceCand
		if err := json.Unmarshal(raw, &body); err != nil {
			return err
		}
		if body.Explanation == nil || body.Explanation.Vi() == "" {
			return errors.New("explanation_vi is empty")
		}
		foundCorrect := false
		seenOptions := make(map[string]bool)
		for _, opt := range body.Options {
			if opt.ID == body.CorrectOptionID {
				foundCorrect = true
			}
			norm := normaliseText(opt.Text)
			if seenOptions[norm] {
				return fmt.Errorf("duplicate option: %q", opt.Text)
			}
			seenOptions[norm] = true
		}
		if !foundCorrect {
			return fmt.Errorf("correct_option_id %q not found in options", body.CorrectOptionID)
		}
		return nil

	case "grammar_sentence_transform":
		var body grammarSentenceTransformCand
		if err := json.Unmarshal(raw, &body); err != nil {
			return err
		}
		if body.Explanation == nil || body.Explanation.Vi() == "" {
			return errors.New("explanation_vi is empty")
		}

		// A transform answer must not appear verbatim in prompt
		normPrompt := normaliseText(body.Prompt)
		normAnswer := normaliseText(body.CorrectAnswer)
		if strings.Contains(normPrompt, normAnswer) {
			return errors.New("correct_answer appears verbatim in prompt")
		}
		return nil

	default:
		return fmt.Errorf("unsupported kind: %s", kind)
	}
}

func parseBlindSolvePayload(kind string, raw []byte) (json.RawMessage, error) {
	switch kind {
	case "reading_comprehension":
		var resp struct {
			Answers map[string]string `json:"answers"`
		}
		if err := json.Unmarshal(raw, &resp); err != nil {
			return nil, err
		}
		if len(resp.Answers) == 0 {
			return nil, errors.New("empty blind solve answers")
		}
		return json.Marshal(map[string]any{"answers": resp.Answers})

	case "grammar_tense_choice":
		var resp struct {
			SelectedOptionID string `json:"selected_option_id"`
			Answer           string `json:"answer"`
		}
		if err := json.Unmarshal(raw, &resp); err != nil {
			return nil, err
		}
		ans := resp.SelectedOptionID
		if ans == "" {
			ans = resp.Answer
		}
		if ans == "" {
			return nil, errors.New("empty blind solve option ID")
		}
		return json.Marshal(map[string]any{"selected_option_id": ans})

	case "grammar_sentence_transform":
		var resp struct {
			Answer string `json:"answer"`
		}
		if err := json.Unmarshal(raw, &resp); err != nil {
			return nil, err
		}
		if strings.TrimSpace(resp.Answer) == "" {
			return nil, errors.New("empty blind solve sentence transform answer")
		}
		return json.Marshal(map[string]any{"answer": resp.Answer})

	default:
		return nil, fmt.Errorf("unsupported kind: %s", kind)
	}
}

func isDuplicate(kind string, newJSON []byte, existing []domain.PoolActivity) bool {
	normTarget := extractComparisonText(kind, newJSON)
	if normTarget == "" {
		return false
	}

	for _, item := range existing {
		normExisting := extractComparisonText(kind, item.Config)
		if normExisting == normTarget {
			return true
		}
	}
	return false
}

func extractComparisonText(kind string, raw []byte) string {
	if len(raw) == 0 {
		return ""
	}
	switch kind {
	case "reading_comprehension":
		var body struct {
			Passage string `json:"passage"`
		}
		_ = json.Unmarshal(raw, &body)
		return normaliseText(body.Passage)

	default:
		var body struct {
			Prompt string `json:"prompt"`
		}
		_ = json.Unmarshal(raw, &body)
		return normaliseText(body.Prompt)
	}
}

func verifyRedaction(redacted []byte) error {
	redactedStr := string(redacted)
	forbiddenKeys := []string{
		`"correct_option_id"`,
		`"correct_answer"`,
		`"acceptable"`,
		`"answers"`,
		`"answer"`,
		`"solution"`,
	}
	for _, k := range forbiddenKeys {
		if strings.Contains(redactedStr, k) {
			return fmt.Errorf("redacted JSON contains forbidden key: %s", k)
		}
	}
	return nil
}

func normaliseText(s string) string {
	fields := strings.FieldsFunc(strings.ToLower(s), func(r rune) bool {
		return unicode.IsSpace(r) || unicode.IsPunct(r)
	})
	return strings.Join(fields, " ")
}

// --------------------------------------------------------------------------
// Daily Practice Set Generation and Fetching
// --------------------------------------------------------------------------

// GetDailySet builds or retrieves the daily practice set for the caller for today in Asia/Ho_Chi_Minh.
func (s *Service) GetDailySet(ctx context.Context, userID uuid.UUID, levelOverride string) (*domain.DailySetDTO, error) {
	loc, err := time.LoadLocation(HoChiMinhTimeZone)
	if err != nil {
		loc = time.FixedZone(HoChiMinhTimeZone, 7*3600)
	}

	now := s.clock.Now().In(loc)
	localDate := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)

	level := "B1"
	if levelOverride == "A2" || levelOverride == "B1" || levelOverride == "B2" {
		level = levelOverride
	}

	// Ensure learner is enrolled in pool-practice so attempts can be recorded
	if poolCourseID, pErr := s.repo.GetPoolPracticeCourseID(ctx); pErr == nil && poolCourseID != uuid.Nil {
		if existing, _ := s.repo.GetEnrollmentByUserCourse(ctx, userID, poolCourseID); existing == nil {
			_, _ = s.repo.CreateEnrollment(ctx, userID, poolCourseID, domain.StatusEnrollmentActive, s.clock.Now().UTC())
		}
	}

	// 1. Check if daily set already exists
	existingSet, err := s.repo.GetDailySet(ctx, userID, localDate)
	if err != nil {
		return nil, fmt.Errorf("check existing daily set: %w", err)
	}

	if existingSet != nil {
		return s.assembleDailySetDTO(ctx, existingSet.ID, localDate, level, existingSet.ActivityIDs)
	}

	// 2. Build a new set: 1 passage, 5 grammar tense, 3 grammar sentence transform
	neededByKind := map[string]int{
		"reading_comprehension":      1,
		"grammar_tense_choice":       5,
		"grammar_sentence_transform": 3,
	}

	var chosenActivityIDs []uuid.UUID

	for _, kind := range []string{"reading_comprehension", "grammar_tense_choice", "grammar_sentence_transform"} {
		needed := neededByKind[kind]
		unseen, err := s.repo.ListUnseenPoolActivitiesForSlot(ctx, level, kind, userID)
		if err != nil {
			return nil, fmt.Errorf("list unseen items for slot %s/%s: %w", level, kind, err)
		}

		shuffledUnseen := shuffleActivities(unseen)
		if len(shuffledUnseen) >= needed {
			for i := 0; i < needed; i++ {
				chosenActivityIDs = append(chosenActivityIDs, shuffledUnseen[i].ID)
			}
		} else {
			// Take all unseen
			for _, act := range shuffledUnseen {
				chosenActivityIDs = append(chosenActivityIDs, act.ID)
			}
			shortfall := needed - len(shuffledUnseen)
			slog.InfoContext(ctx, "daily practice set slot drew from oldest exposures",
				"user_id", userID, "level", level, "kind", kind, "shortfall", shortfall)

			seen, err := s.repo.ListSeenPoolActivitiesForSlotOldestFirst(ctx, level, kind, userID)
			if err != nil {
				return nil, fmt.Errorf("list seen items for slot %s/%s: %w", level, kind, err)
			}

			for i := 0; i < shortfall && i < len(seen); i++ {
				chosenActivityIDs = append(chosenActivityIDs, seen[i].ID)
			}
		}
	}

	// If the practice pool has fewer than 9 items total (e.g. fresh installation or test),
	// we still record what we have rather than hard failing.
	if len(chosenActivityIDs) == 0 {
		return nil, apperr.New(apperr.NotFound, "PRACTICE_POOL_EMPTY", "Practice pool contains no activities.")
	}

	// 3. Persist daily set and exposures in transaction
	var createdID uuid.UUID
	if s.pool == nil {
		created, err := s.repo.CreateDailySet(ctx, userID, localDate, chosenActivityIDs)
		if err != nil {
			raceSet, rErr := s.repo.GetDailySet(ctx, userID, localDate)
			if rErr == nil && raceSet != nil {
				return s.assembleDailySetDTO(ctx, raceSet.ID, localDate, level, raceSet.ActivityIDs)
			}
			return nil, err
		}
		createdID = created.ID
		for _, actID := range chosenActivityIDs {
			if err := s.repo.RecordItemExposure(ctx, userID, actID); err != nil {
				return nil, fmt.Errorf("record exposure for %s: %w", actID, err)
			}
		}
	} else {
		err = dbx.InTx(ctx, s.pool, func(txCtx context.Context, tx pgx.Tx) error {
			txRepo := s.repo.WithTx(tx)

			created, err := txRepo.CreateDailySet(txCtx, userID, localDate, chosenActivityIDs)
			if err != nil {
				// Idempotence: if concurrent request won the race, return nil to fallback to reading
				return err
			}
			createdID = created.ID

			for _, actID := range chosenActivityIDs {
				if err := txRepo.RecordItemExposure(txCtx, userID, actID); err != nil {
					return fmt.Errorf("record exposure for %s: %w", actID, err)
				}
			}
			return nil
		})

		if err != nil {
			// Race fallback: re-read set
			raceSet, rErr := s.repo.GetDailySet(ctx, userID, localDate)
			if rErr == nil && raceSet != nil {
				return s.assembleDailySetDTO(ctx, raceSet.ID, localDate, level, raceSet.ActivityIDs)
			}
			return nil, fmt.Errorf("save daily set: %w", err)
		}
	}

	return s.assembleDailySetDTO(ctx, createdID, localDate, level, chosenActivityIDs)
}

func (s *Service) assembleDailySetDTO(
	ctx context.Context, setID uuid.UUID, localDate time.Time, level string, activityIDs []uuid.UUID,
) (*domain.DailySetDTO, error) {
	activities, err := s.repo.ListActivitiesByIDs(ctx, activityIDs)
	if err != nil {
		return nil, fmt.Errorf("list activities by ids: %w", err)
	}

	// Index activities by ID to preserve the daily set's ordering
	actMap := make(map[uuid.UUID]domain.PoolActivity, len(activities))
	var versionIDs []uuid.UUID
	for _, act := range activities {
		actMap[act.ID] = act
		if act.ContentVersionID != uuid.Nil {
			versionIDs = append(versionIDs, act.ContentVersionID)
		}
	}

	// Batch resolve content versions
	versions := make(map[uuid.UUID]*contentcontract.Version)
	if s.content != nil && len(versionIDs) > 0 {
		var vErr error
		versions, vErr = s.content.GetManyVersions(ctx, versionIDs)
		if vErr != nil {
			slog.WarnContext(ctx, "failed to batch load content versions for daily set", "error", vErr)
		}
	}

	actDTOs := make([]domain.DailySetActivityDTO, 0, len(activityIDs))
	for pos, actID := range activityIDs {
		act, ok := actMap[actID]
		if !ok {
			continue
		}

		redactedConfigRaw := contentcontract.RedactForLearner(act.Config)
		var configMap map[string]interface{}
		if len(redactedConfigRaw) > 0 {
			_ = json.Unmarshal(redactedConfigRaw, &configMap)
		}

		var contentMap map[string]interface{}
		if ver, exists := versions[act.ContentVersionID]; exists && ver != nil {
			redactedVer := contentcontract.RedactVersionForLearner(ver)
			if redactedVer != nil {
				marshaled, mErr := json.Marshal(redactedVer)
				if mErr == nil {
					_ = json.Unmarshal(marshaled, &contentMap)
				}
			}
		}

		actDTOs = append(actDTOs, domain.DailySetActivityDTO{
			ID:               act.ID,
			LessonID:         act.LessonID,
			Position:         pos + 1,
			Kind:             act.Kind,
			ContentVersionID: act.ContentVersionID,
			Config:           configMap,
			Content:          contentMap,
			Weight:           act.Weight,
		})
	}

	return &domain.DailySetDTO{
		ID:         setID,
		LocalDate:  localDate,
		Level:      level,
		Activities: actDTOs,
	}, nil
}

func shuffleActivities(src []domain.PoolActivity) []domain.PoolActivity {
	dest := make([]domain.PoolActivity, len(src))
	copy(dest, src)
	for i := len(dest) - 1; i > 0; i-- {
		nBig, err := rand.Int(rand.Reader, big.NewInt(int64(i+1)))
		if err != nil {
			continue
		}
		j := int(nBig.Int64())
		dest[i], dest[j] = dest[j], dest[i]
	}
	return dest
}
