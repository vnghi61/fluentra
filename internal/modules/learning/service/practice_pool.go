package service

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math/big"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	contentcontract "github.com/fluentra/fluentra/internal/modules/content/contract"
	learningcontract "github.com/fluentra/fluentra/internal/modules/learning/contract"
	"github.com/fluentra/fluentra/internal/modules/learning/domain"
	lessoncontract "github.com/fluentra/fluentra/internal/modules/lesson/contract"
	"github.com/fluentra/fluentra/internal/platform/ai"
	"github.com/fluentra/fluentra/internal/shared/apperr"
	"github.com/fluentra/fluentra/internal/shared/dbx"
)

// The practice pool — work order 11 §3.11. The model adds a few items at a time
// to slots, one kind at one level, and each learner's daily set is drawn from the
// items that learner has not seen.
//
// The pool's course, units, lessons and activities belong to `lesson` and are
// reached only through its contract. This file used to join learn.activities,
// learn.lessons, learn.course_units and learn.courses from learning's own SQL —
// rule L2, which no linter can see because it is SQL. Learning's queries now touch
// only what learning owns here: item_exposures and daily_sets.

const (
	// PracticePoolCourseSlug identifies the pool's course. The slug is what makes
	// resolving the pool's structure converge rather than create a second copy.
	PracticePoolCourseSlug = "pool-practice"

	practicePoolCourseTitle = "Practice Pool"
	practicePoolDescription = "Auto-generated practice exercises pool"

	targetActiveItemsPerSlot = 50
	maxActiveItemsPerSlot    = 200
	maxItemsToAddPerRun      = 5
	// maxRetriesPerItem counts regenerations after the first attempt: three in all.
	maxRetriesPerItem = 2
	// runningLowThreshold is how few unseen items make a slot grow past its target.
	runningLowThreshold = 10

	hoChiMinhTimeZone    = "Asia/Ho_Chi_Minh"
	defaultPracticeLevel = "B1"

	kindReadingComprehension     = "reading_comprehension"
	kindGrammarTenseChoice       = "grammar_tense_choice"
	kindGrammarSentenceTransform = "grammar_sentence_transform"
)

var practiceLevels = []string{"A2", "B1", "B2"}

// practiceLesson is a slot's lesson inside each level's unit.
type practiceLesson struct {
	position   int
	kind       string
	title      string
	skillFocus string
}

var practiceLessons = []practiceLesson{
	{position: 1, kind: kindReadingComprehension, title: "Reading Comprehension", skillFocus: "reading"},
	{position: 2, kind: kindGrammarTenseChoice, title: "Grammar Tense Choice", skillFocus: "grammar"},
	{position: 3, kind: kindGrammarSentenceTransform, title: "Grammar Sentence Transform", skillFocus: "grammar"},
}

// dailySetComposition is what one day's set draws from each slot at a level.
var dailySetComposition = []struct {
	kind  string
	count int
}{
	{kind: kindReadingComprehension, count: 1},
	{kind: kindGrammarTenseChoice, count: 5},
	{kind: kindGrammarSentenceTransform, count: 3},
}

type slotKey struct{ level, kind string }

// practicePoolLayout is the pool's course and the lesson behind each slot.
type practicePoolLayout struct {
	courseID uuid.UUID
	lessons  map[slotKey]uuid.UUID
}

// EnsurePracticePoolStructure makes sure the pool's course, a unit per level and a
// lesson per kind exist, and remembers their ids for the life of the process.
func (s *Service) EnsurePracticePoolStructure(ctx context.Context) error {
	_, err := s.practicePool(ctx)
	return err
}

func (s *Service) practicePool(ctx context.Context) (*practicePoolLayout, error) {
	s.poolMu.Lock()
	defer s.poolMu.Unlock()
	if s.poolLayout != nil {
		return s.poolLayout, nil
	}
	if s.lessonAuthor == nil {
		return nil, errors.New("lesson author dependency is required for the practice pool")
	}
	layout, err := s.buildPracticePool(ctx)
	if err != nil {
		return nil, err
	}
	s.poolLayout = layout
	return layout, nil
}

func (s *Service) buildPracticePool(ctx context.Context) (*practicePoolLayout, error) {
	courseID, err := s.lessonAuthor.EnsureCourse(ctx, lessoncontract.CourseSpec{
		Slug:           PracticePoolCourseSlug,
		Title:          practicePoolCourseTitle,
		Description:    practicePoolDescription,
		CEFRFrom:       "A2",
		CEFRTo:         "B2",
		EstimatedHours: 100,
	})
	if err != nil {
		return nil, fmt.Errorf("ensure practice pool course: %w", err)
	}

	layout := &practicePoolLayout{courseID: courseID, lessons: make(map[slotKey]uuid.UUID)}
	for index, level := range practiceLevels {
		unitID, err := s.lessonAuthor.EnsureUnit(ctx, lessoncontract.UnitSpec{
			CourseID:    courseID,
			Position:    index + 1,
			Title:       level,
			Description: level + " practice pool",
		})
		if err != nil {
			return nil, fmt.Errorf("ensure practice pool unit %s: %w", level, err)
		}
		for _, lesson := range practiceLessons {
			lessonID, err := s.lessonAuthor.EnsureLesson(ctx, lessoncontract.LessonSpec{
				UnitID:           unitID,
				Position:         lesson.position,
				Title:            lesson.title,
				SkillFocus:       lesson.skillFocus,
				EstimatedMinutes: 10,
			})
			if err != nil {
				return nil, fmt.Errorf("ensure practice pool lesson %s in %s: %w", lesson.title, level, err)
			}
			layout.lessons[slotKey{level: level, kind: lesson.kind}] = lessonID
		}
	}
	return layout, nil
}

// slotActivities lists a slot's active items through lesson's contract.
func (s *Service) slotActivities(
	ctx context.Context, layout *practicePoolLayout, level, kind string,
) ([]lessoncontract.Activity, error) {
	lessonID, ok := layout.lessons[slotKey{level: level, kind: kind}]
	if !ok {
		return nil, fmt.Errorf("no practice pool lesson for %s %s", level, kind)
	}
	if s.lesson == nil {
		return nil, errors.New("lesson reader dependency is required for the practice pool")
	}
	lesson, err := s.lesson.GetLesson(ctx, lessonID)
	if err != nil {
		return nil, fmt.Errorf("load practice pool lesson %s: %w", lessonID, err)
	}
	if lesson == nil {
		return nil, nil
	}
	return lesson.Activities, nil
}

// --------------------------------------------------------------------------
// Top-up
// --------------------------------------------------------------------------

// TopUpPracticePool adds verified items to every slot that needs them.
func (s *Service) TopUpPracticePool(ctx context.Context) error {
	if s.ai == nil {
		slog.WarnContext(ctx, "practice pool top-up skipped: no AI client")
		return nil
	}
	if s.contentAuthor == nil || s.lessonAuthor == nil {
		return errors.New("content author and lesson author dependencies are required for practice pool top-up")
	}
	// content_items.owner_id is required, and EnsurePublished refuses uuid.Nil. The
	// top-up used to publish with no owner, so every item that passed all six checks
	// was refused at the last step and the pool never held a single item.
	if s.generatorAuthor == uuid.Nil {
		slog.WarnContext(ctx, "practice pool top-up skipped: no owner for generated content")
		return nil
	}

	layout, err := s.practicePool(ctx)
	if err != nil {
		return fmt.Errorf("ensure pool structure: %w", err)
	}
	for _, level := range practiceLevels {
		for _, lesson := range practiceLessons {
			s.topUpSlot(ctx, layout, level, lesson.kind)
		}
	}
	return nil
}

func (s *Service) topUpSlot(ctx context.Context, layout *practicePoolLayout, level, kind string) {
	activities, err := s.slotActivities(ctx, layout, level, kind)
	if err != nil {
		slog.ErrorContext(ctx, "could not list practice pool slot", "level", level, "kind", kind, "error", err)
		return
	}
	toAdd, err := s.itemsToAdd(ctx, activities)
	if err != nil {
		slog.ErrorContext(ctx, "could not size practice pool top-up", "level", level, "kind", kind, "error", err)
		return
	}

	lessonID := layout.lessons[slotKey{level: level, kind: kind}]
	added := 0
	for i := 0; i < toAdd; i++ {
		body, err := s.generateAndVerifyItem(ctx, level, kind, lessonID, activities)
		if err != nil {
			slog.WarnContext(ctx, "practice pool item not added", "level", level, "kind", kind, "error", err)
			continue
		}
		// Later candidates in this run are checked for duplicates against it too.
		activities = append(activities, lessoncontract.Activity{Kind: kind, Config: body})
		added++
	}
	if toAdd > 0 {
		slog.InfoContext(ctx, "practice pool slot topped up", "level", level, "kind", kind, "added", added)
	}
}

// itemsToAdd is §3.11's rule: fill to the target five at a time, then grow only
// while an active learner is running low, and never past the ceiling.
func (s *Service) itemsToAdd(ctx context.Context, activities []lessoncontract.Activity) (int, error) {
	count := len(activities)
	switch {
	case count < targetActiveItemsPerSlot:
		return min(maxItemsToAddPerRun, targetActiveItemsPerSlot-count), nil
	case count >= maxActiveItemsPerSlot:
		return 0, nil
	}
	low, err := s.repo.HasActiveLearnerRunningLow(ctx, idsOf(activities), runningLowThreshold)
	if err != nil {
		return 0, err
	}
	if !low {
		return 0, nil
	}
	return min(maxItemsToAddPerRun, maxActiveItemsPerSlot-count), nil
}

// generateAndVerifyItem tries a candidate up to three times and returns the body
// of the one it published.
func (s *Service) generateAndVerifyItem(
	ctx context.Context, level, kind string, lessonID uuid.UUID, existing []lessoncontract.Activity,
) (json.RawMessage, error) {
	var lastErr error
	for attempt := 0; attempt <= maxRetriesPerItem; attempt++ {
		body, err := s.tryGenerateAndVerify(ctx, level, kind, lessonID, existing)
		if err == nil {
			return body, nil
		}
		lastErr = err
		slog.WarnContext(ctx, "practice pool candidate rejected",
			"level", level, "kind", kind, "attempt", attempt+1, "reason", err)
	}
	return nil, lastErr
}

func (s *Service) tryGenerateAndVerify(
	ctx context.Context, level, kind string, lessonID uuid.UUID, existing []lessoncontract.Activity,
) (json.RawMessage, error) {
	response, err := s.ai.Complete(ctx, ai.Request{
		Task: ai.TaskPracticeGenerate,
		Vars: map[string]any{"Kind": kind, "CEFRLevel": level},
	})
	if err != nil {
		return nil, fmt.Errorf("ai generate call failed: %w", err)
	}
	body := json.RawMessage(strings.TrimSpace(response.Text))

	if err := s.checkCandidate(ctx, level, kind, body, existing); err != nil {
		return nil, err
	}
	if err := s.publishCandidate(ctx, level, kind, lessonID, body); err != nil {
		return nil, err
	}
	return body, nil
}

// checkCandidate runs §3.11's six checks, in order.
func (s *Service) checkCandidate(
	ctx context.Context, level, kind string, body json.RawMessage, existing []lessoncontract.Activity,
) error {
	if err := validateParse(kind, body); err != nil {
		return fmt.Errorf("check 1 (parse) failed: %w", err)
	}

	grader, ok := s.graders.Get(kind)
	if !ok || grader == nil {
		return fmt.Errorf("grader not registered for kind: %s", kind)
	}
	versionID := uuid.New()
	gradeCtx := contentcontract.ContextWithTempVersion(ctx, &contentcontract.Version{
		ID: versionID, Kind: kind, Body: body, CEFRLevel: level, Status: "published",
	})

	ownAnswer, err := buildOwnAnswerPayload(kind, body)
	if err != nil {
		return fmt.Errorf("build own answer payload: %w", err)
	}
	if err := gradesFullMarks(gradeCtx, grader, versionID, ownAnswer); err != nil {
		return fmt.Errorf("check 2 (own answer scores full marks) failed: %w", err)
	}

	if err := validateStructure(kind, body); err != nil {
		return fmt.Errorf("check 3 (structure) failed: %w", err)
	}

	redacted := contentcontract.RedactForLearner(body)
	if err := s.blindSolve(gradeCtx, grader, versionID, kind, redacted); err != nil {
		return fmt.Errorf("check 4 (blind solve) failed: %w", err)
	}

	if isDuplicate(kind, body, existing) {
		return errors.New("check 5 (deduplication) failed: item matches an item already in the slot")
	}

	if err := verifyRedaction(redacted); err != nil {
		return fmt.Errorf("check 6 (redaction verification) failed: %w", err)
	}
	return nil
}

// blindSolve asks the model to answer the redacted item and grades that answer.
// Disagreement with the item's own key is what catches a wrong key.
func (s *Service) blindSolve(
	ctx context.Context, grader learningcontract.ExerciseGrader, versionID uuid.UUID, kind string,
	redacted json.RawMessage,
) error {
	response, err := s.ai.Complete(ctx, ai.Request{
		Task: ai.TaskPracticeSolve,
		Vars: map[string]any{"Kind": kind, "RedactedBody": string(redacted)},
	})
	if err != nil {
		return fmt.Errorf("ai blind solve call failed: %w", err)
	}
	payload, err := parseBlindSolvePayload(kind, []byte(strings.TrimSpace(response.Text)))
	if err != nil {
		return fmt.Errorf("parse blind solve response: %w", err)
	}
	return gradesFullMarks(ctx, grader, versionID, payload)
}

func gradesFullMarks(
	ctx context.Context, grader learningcontract.ExerciseGrader, versionID uuid.UUID, response json.RawMessage,
) error {
	result, err := grader.Grade(ctx, learningcontract.GradeRequest{
		ContentVersionID: versionID,
		Response:         response,
	})
	if err != nil {
		return err
	}
	if result.Score < 100 || !result.Correct {
		return fmt.Errorf("score=%d correct=%v", result.Score, result.Correct)
	}
	return nil
}

// publishCandidate publishes an item that passed every check and appends it to its
// slot's lesson. Appending, never replacing: see work order 11 §3.0.
func (s *Service) publishCandidate(
	ctx context.Context, level, kind string, lessonID uuid.UUID, body json.RawMessage,
) error {
	slug := fmt.Sprintf("pool-%s-%s-%s", strings.ToLower(level), kind, uuid.New().String()[:8])
	versionID, err := s.contentAuthor.EnsurePublished(ctx, contentcontract.AuthorSpec{
		Slug:      slug,
		Kind:      kind,
		CEFRLevel: level,
		Body:      body,
		AuthorID:  s.generatorAuthor,
	})
	if err != nil {
		return fmt.Errorf("publish verified content: %w", err)
	}

	if _, err := s.lessonAuthor.AppendActivity(ctx, lessonID, lessoncontract.ActivitySpec{
		Kind:             kind,
		ContentVersionID: versionID,
		Config:           body,
		Weight:           1,
	}); err != nil {
		return fmt.Errorf("append activity to pool lesson: %w", err)
	}
	return nil
}

// --------------------------------------------------------------------------
// Candidate bodies and the checks over them
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
	case kindReadingComprehension:
		return parseReading(raw)
	case kindGrammarTenseChoice:
		var body grammarTenseChoiceCand
		if err := json.Unmarshal(raw, &body); err != nil {
			return err
		}
		return parseChoice(body.Prompt, body.Options, body.CorrectOptionID)
	case kindGrammarSentenceTransform:
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

func parseReading(raw []byte) error {
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
		if err := parseChoice(q.Prompt, q.Options, q.CorrectOptionID); err != nil {
			return fmt.Errorf("question %d: %w", i, err)
		}
	}
	return nil
}

func parseChoice(prompt string, options []candOption, correctOptionID string) error {
	if strings.TrimSpace(prompt) == "" {
		return errors.New("prompt is empty")
	}
	if len(options) != 4 {
		return fmt.Errorf("expected 4 options, got %d", len(options))
	}
	if strings.TrimSpace(correctOptionID) == "" {
		return errors.New("correct_option_id is empty")
	}
	return nil
}

func buildOwnAnswerPayload(kind string, raw []byte) (json.RawMessage, error) {
	switch kind {
	case kindReadingComprehension:
		var body readingComprehensionCand
		if err := json.Unmarshal(raw, &body); err != nil {
			return nil, err
		}
		answers := make(map[string]string, len(body.Questions))
		for _, q := range body.Questions {
			answers[q.ID] = q.CorrectOptionID
		}
		return json.Marshal(map[string]any{"answers": answers})
	case kindGrammarTenseChoice:
		var body grammarTenseChoiceCand
		if err := json.Unmarshal(raw, &body); err != nil {
			return nil, err
		}
		return json.Marshal(map[string]any{"selected_option_id": body.CorrectOptionID})
	case kindGrammarSentenceTransform:
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
	case kindReadingComprehension:
		var body readingComprehensionCand
		if err := json.Unmarshal(raw, &body); err != nil {
			return err
		}
		for i, q := range body.Questions {
			if q.Explanation.Vi() == "" {
				return fmt.Errorf("question %d explanation_vi is empty", i)
			}
			if err := checkOptions(q.Options, q.CorrectOptionID); err != nil {
				return fmt.Errorf("question %d: %w", i, err)
			}
		}
		return nil
	case kindGrammarTenseChoice:
		var body grammarTenseChoiceCand
		if err := json.Unmarshal(raw, &body); err != nil {
			return err
		}
		if body.Explanation.Vi() == "" {
			return errors.New("explanation_vi is empty")
		}
		return checkOptions(body.Options, body.CorrectOptionID)
	case kindGrammarSentenceTransform:
		return structureSentenceTransform(raw)
	default:
		return fmt.Errorf("unsupported kind: %s", kind)
	}
}

func structureSentenceTransform(raw []byte) error {
	var body grammarSentenceTransformCand
	if err := json.Unmarshal(raw, &body); err != nil {
		return err
	}
	if body.Explanation.Vi() == "" {
		return errors.New("explanation_vi is empty")
	}
	// A transform answer must not appear verbatim in its own prompt.
	if strings.Contains(normaliseText(body.Prompt), normaliseText(body.CorrectAnswer)) {
		return errors.New("correct_answer appears verbatim in prompt")
	}
	return nil
}

// checkOptions requires the correct option to be one of the options, and the
// options to be distinct once normalised.
func checkOptions(options []candOption, correctOptionID string) error {
	foundCorrect := false
	seen := make(map[string]bool, len(options))
	for _, opt := range options {
		if opt.ID == correctOptionID {
			foundCorrect = true
		}
		norm := normaliseText(opt.Text)
		if seen[norm] {
			return fmt.Errorf("duplicate option: %q", opt.Text)
		}
		seen[norm] = true
	}
	if !foundCorrect {
		return fmt.Errorf("correct_option_id %q not found in options", correctOptionID)
	}
	return nil
}

func parseBlindSolvePayload(kind string, raw []byte) (json.RawMessage, error) {
	switch kind {
	case kindReadingComprehension:
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
	case kindGrammarTenseChoice:
		var resp struct {
			SelectedOptionID string `json:"selected_option_id"`
			Answer           string `json:"answer"`
		}
		if err := json.Unmarshal(raw, &resp); err != nil {
			return nil, err
		}
		answer := resp.SelectedOptionID
		if answer == "" {
			answer = resp.Answer
		}
		if answer == "" {
			return nil, errors.New("empty blind solve option ID")
		}
		return json.Marshal(map[string]any{"selected_option_id": answer})
	case kindGrammarSentenceTransform:
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

func isDuplicate(kind string, body []byte, existing []lessoncontract.Activity) bool {
	target := extractComparisonText(kind, body)
	if target == "" {
		return false
	}
	for _, activity := range existing {
		if extractComparisonText(kind, activity.Config) == target {
			return true
		}
	}
	return false
}

func extractComparisonText(kind string, raw []byte) string {
	if len(raw) == 0 {
		return ""
	}
	if kind == kindReadingComprehension {
		var body struct {
			Passage string `json:"passage"`
		}
		_ = json.Unmarshal(raw, &body)
		return normaliseText(body.Passage)
	}
	var body struct {
		Prompt string `json:"prompt"`
	}
	_ = json.Unmarshal(raw, &body)
	return normaliseText(body.Prompt)
}

func verifyRedaction(redacted []byte) error {
	serialized := string(redacted)
	for _, key := range []string{
		`"correct_option_id"`, `"correct_answer"`, `"acceptable"`, `"answers"`, `"answer"`, `"solution"`,
	} {
		if strings.Contains(serialized, key) {
			return fmt.Errorf("redacted JSON contains forbidden key: %s", key)
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
// The daily set
// --------------------------------------------------------------------------

// GetDailySet builds or retrieves the caller's practice set for today in
// Asia/Ho_Chi_Minh.
func (s *Service) GetDailySet(
	ctx context.Context, userID uuid.UUID, levelOverride string,
) (*domain.DailySetDTO, error) {
	if s.lesson == nil {
		return nil, errors.New("lesson reader dependency is required for the daily set")
	}
	localDate := learnerLocalDate(s.clock.Now())
	level := practiceLevel(levelOverride)

	layout, err := s.practicePool(ctx)
	if err != nil {
		return nil, fmt.Errorf("resolve practice pool: %w", err)
	}
	s.ensurePoolEnrollment(ctx, userID, layout.courseID)

	existing, err := s.repo.GetDailySet(ctx, userID, localDate)
	if err != nil {
		return nil, fmt.Errorf("check existing daily set: %w", err)
	}
	if existing != nil {
		return s.assembleDailySetDTO(ctx, existing.ID, localDate, level, existing.ActivityIDs)
	}

	chosen, err := s.drawDailySet(ctx, userID, level, layout)
	if err != nil {
		return nil, err
	}
	if len(chosen) == 0 {
		return nil, apperr.New(apperr.NotFound, "PRACTICE_POOL_EMPTY", "Practice pool contains no activities.")
	}

	stored, err := s.saveDailySet(ctx, userID, localDate, chosen)
	if err != nil {
		return nil, err
	}
	return s.assembleDailySetDTO(ctx, stored.ID, localDate, level, stored.ActivityIDs)
}

func (s *Service) drawDailySet(
	ctx context.Context, userID uuid.UUID, level string, layout *practicePoolLayout,
) ([]uuid.UUID, error) {
	var chosen []uuid.UUID
	for _, part := range dailySetComposition {
		activities, err := s.slotActivities(ctx, layout, level, part.kind)
		if err != nil {
			return nil, err
		}
		picked, err := s.drawFromSlot(ctx, userID, activities, part.count)
		if err != nil {
			return nil, fmt.Errorf("draw from %s %s: %w", level, part.kind, err)
		}
		chosen = append(chosen, picked...)
	}
	return chosen, nil
}

// drawFromSlot picks at random among items the learner has not seen, then fills
// from the ones they were served longest ago. A set is never short while the slot
// has anything in it.
func (s *Service) drawFromSlot(
	ctx context.Context, userID uuid.UUID, activities []lessoncontract.Activity, needed int,
) ([]uuid.UUID, error) {
	if len(activities) == 0 || needed <= 0 {
		return nil, nil
	}
	ids := idsOf(activities)
	exposures, err := s.repo.ListItemExposures(ctx, userID, ids)
	if err != nil {
		return nil, fmt.Errorf("list item exposures: %w", err)
	}

	var unseen, seen []uuid.UUID
	for _, id := range ids {
		if _, served := exposures[id]; served {
			seen = append(seen, id)
		} else {
			unseen = append(unseen, id)
		}
	}

	picked := firstN(shuffledIDs(unseen), needed)
	if shortfall := needed - len(picked); shortfall > 0 && len(seen) > 0 {
		sort.Slice(seen, func(i, j int) bool { return exposures[seen[i]].Before(exposures[seen[j]]) })
		repeated := firstN(seen, shortfall)
		picked = append(picked, repeated...)
		slog.InfoContext(ctx, "daily practice set repeated items the learner had seen",
			"user_id", userID, "repeated", len(repeated))
	}
	return picked, nil
}

// saveDailySet stores a set and its exposures in one transaction.
//
// When another request stored the learner's set for the day first, that set is
// returned and nothing from this request's draw is recorded. The insert used to
// be ON CONFLICT DO UPDATE, which never failed: the losing request carried on,
// marked its own randomly drawn items as seen — items the learner was never shown
// — and answered with a set different from the one stored.
func (s *Service) saveDailySet(
	ctx context.Context, userID uuid.UUID, localDate time.Time, chosen []uuid.UUID,
) (*domain.DailySet, error) {
	write := func(ctx context.Context, repo Repository) (*domain.DailySet, error) {
		created, err := repo.CreateDailySet(ctx, userID, localDate, chosen)
		if err != nil || created == nil {
			return created, err
		}
		for _, id := range created.ActivityIDs {
			if err := repo.RecordItemExposure(ctx, userID, id); err != nil {
				return nil, fmt.Errorf("record exposure for %s: %w", id, err)
			}
		}
		return created, nil
	}

	var stored *domain.DailySet
	if s.pool == nil {
		created, err := write(ctx, s.repo)
		if err != nil {
			return nil, fmt.Errorf("save daily set: %w", err)
		}
		stored = created
	} else if err := dbx.InTx(ctx, s.pool, func(txCtx context.Context, tx pgx.Tx) error {
		created, err := write(txCtx, s.repo.WithTx(tx))
		stored = created
		return err
	}); err != nil {
		return nil, fmt.Errorf("save daily set: %w", err)
	}
	if stored != nil {
		return stored, nil
	}

	winner, err := s.repo.GetDailySet(ctx, userID, localDate)
	if err != nil {
		return nil, fmt.Errorf("read the day's set after a concurrent save: %w", err)
	}
	if winner == nil {
		return nil, errors.New("daily set missing after a concurrent save")
	}
	return winner, nil
}

func (s *Service) assembleDailySetDTO(
	ctx context.Context, setID uuid.UUID, localDate time.Time, level string, activityIDs []uuid.UUID,
) (*domain.DailySetDTO, error) {
	resolved := make([]*lessoncontract.ActivityHierarchy, 0, len(activityIDs))
	versionIDs := make([]uuid.UUID, 0, len(activityIDs))
	for _, id := range activityIDs {
		activity, err := s.lesson.ResolveActivity(ctx, id)
		if err != nil || activity == nil {
			// An item that can no longer be resolved is left out rather than
			// failing the whole day's set.
			slog.WarnContext(ctx, "daily set activity could not be resolved", "activity_id", id, "error", err)
			continue
		}
		resolved = append(resolved, activity)
		versionIDs = append(versionIDs, activity.ContentVersionID)
	}

	versions := s.loadVersions(ctx, versionIDs)
	dtos := make([]domain.DailySetActivityDTO, 0, len(resolved))
	for position, activity := range resolved {
		dtos = append(dtos, domain.DailySetActivityDTO{
			ID:               activity.ActivityID,
			LessonID:         activity.LessonID,
			Position:         position + 1,
			Kind:             activity.Kind,
			ContentVersionID: activity.ContentVersionID,
			Config:           redactedMap(activity.Config),
			Content:          redactedVersionMap(versions[activity.ContentVersionID]),
			Weight:           activity.Weight,
		})
	}

	return &domain.DailySetDTO{
		ID:         setID,
		LocalDate:  localDate,
		Level:      level,
		Activities: dtos,
	}, nil
}

func (s *Service) loadVersions(ctx context.Context, ids []uuid.UUID) map[uuid.UUID]*contentcontract.Version {
	if s.content == nil || len(ids) == 0 {
		return map[uuid.UUID]*contentcontract.Version{}
	}
	versions, err := s.content.GetManyVersions(ctx, ids)
	if err != nil {
		slog.WarnContext(ctx, "failed to batch load content versions for daily set", "error", err)
		return map[uuid.UUID]*contentcontract.Version{}
	}
	return versions
}

// ensurePoolEnrollment enrols the learner in the pool's course, which StartAttempt
// requires before it will record an attempt. The pool is not a course the learner
// chose, so withoutPoolCourse keeps it off the dashboard and the progress page.
func (s *Service) ensurePoolEnrollment(ctx context.Context, userID, courseID uuid.UUID) {
	existing, err := s.repo.GetEnrollmentByUserCourse(ctx, userID, courseID)
	if err != nil || existing != nil {
		return
	}
	if _, err := s.repo.CreateEnrollment(
		ctx, userID, courseID, domain.StatusEnrollmentActive, s.clock.Now().UTC(),
	); err != nil {
		slog.WarnContext(ctx, "could not enrol learner in the practice pool", "user_id", userID, "error", err)
	}
}

// withoutPoolCourse drops the practice pool's enrolment. The dashboard continues
// the newest active enrolment, so without this, opening today's practice turned
// "continue learning" into a machine-made drill and put "Practice Pool" among the
// learner's courses.
func (s *Service) withoutPoolCourse(ctx context.Context, enrollments []domain.Enrollment) []domain.Enrollment {
	if s.lessonAuthor == nil {
		return enrollments
	}
	layout, err := s.practicePool(ctx)
	if err != nil {
		return enrollments
	}
	kept := make([]domain.Enrollment, 0, len(enrollments))
	for _, enrollment := range enrollments {
		if enrollment.CourseID != layout.courseID {
			kept = append(kept, enrollment)
		}
	}
	return kept
}

// --------------------------------------------------------------------------
// Small helpers
// --------------------------------------------------------------------------

func learnerLocalDate(now time.Time) time.Time {
	loc, err := time.LoadLocation(hoChiMinhTimeZone)
	if err != nil {
		loc = time.FixedZone(hoChiMinhTimeZone, 7*3600)
	}
	local := now.In(loc)
	return time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, time.UTC)
}

func practiceLevel(requested string) string {
	for _, level := range practiceLevels {
		if requested == level {
			return level
		}
	}
	return defaultPracticeLevel
}

func idsOf(activities []lessoncontract.Activity) []uuid.UUID {
	ids := make([]uuid.UUID, 0, len(activities))
	for _, activity := range activities {
		if activity.ID != uuid.Nil {
			ids = append(ids, activity.ID)
		}
	}
	return ids
}

func firstN(ids []uuid.UUID, n int) []uuid.UUID {
	if len(ids) > n {
		return ids[:n]
	}
	return ids
}

func shuffledIDs(src []uuid.UUID) []uuid.UUID {
	dest := append([]uuid.UUID(nil), src...)
	for i := len(dest) - 1; i > 0; i-- {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(i+1)))
		if err != nil {
			continue
		}
		j := int(n.Int64())
		dest[i], dest[j] = dest[j], dest[i]
	}
	return dest
}

func redactedMap(raw json.RawMessage) map[string]any {
	redacted := contentcontract.RedactForLearner(raw)
	if len(redacted) == 0 {
		return nil
	}
	var out map[string]any
	if err := json.Unmarshal(redacted, &out); err != nil {
		return nil
	}
	return out
}

func redactedVersionMap(version *contentcontract.Version) map[string]any {
	if version == nil {
		return nil
	}
	redacted := contentcontract.RedactVersionForLearner(version)
	if redacted == nil {
		return nil
	}
	encoded, err := json.Marshal(redacted)
	if err != nil {
		return nil
	}
	var out map[string]any
	if err := json.Unmarshal(encoded, &out); err != nil {
		return nil
	}
	return out
}
