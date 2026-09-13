package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strings"

	"github.com/google/uuid"

	contentcontract "github.com/fluentra/fluentra/internal/modules/content/contract"
	learningcontract "github.com/fluentra/fluentra/internal/modules/learning/contract"
	lessoncontract "github.com/fluentra/fluentra/internal/modules/lesson/contract"
	"github.com/fluentra/fluentra/internal/platform/ai"
	"github.com/fluentra/fluentra/internal/shared/apperr"
)

// The exam pool — work order 12 §3.7.
//
// Slots: Eighteen — six kinds across A2, B1, and B2.
// The top-up runs hourly on lock 1_700_000_213.
// Drawing an exam sitting picks 3 listening clips (with audio), 2 reading passages,
// 1 essay prompt + 3 sentence transforms, 2 read-alouds + 2 responses.

const (
	ExamPoolCourseSlug = "pool-exam"

	examPoolCourseTitle = "Exam Pool"
	examPoolDescription = "Auto-generated exam exercises pool"

	kindListeningComprehension = "listening_comprehension"
	kindWritingPrompt          = "writing_prompt"
	kindSpeakingTask           = "speaking_task"

	subTypeReadAloud = "read_aloud"
	subTypeRespond   = "respond"
)

var examLevels = []string{"A2", "B1", "B2"}

type examSlotSpec struct {
	position   int
	kind       string
	taskType   string
	slotName   string
	title      string
	skillFocus string
}

var examSlots = []examSlotSpec{
	{position: 1, kind: kindListeningComprehension, slotName: "listening-comprehension", title: "Listening Comprehension", skillFocus: "listening"},
	{position: 2, kind: kindReadingComprehension, slotName: "reading-comprehension", title: "Reading Comprehension", skillFocus: "reading"},
	{position: 3, kind: kindGrammarSentenceTransform, slotName: "grammar-sentence-transform", title: "Grammar Sentence Transform", skillFocus: "grammar"},
	{position: 4, kind: kindWritingPrompt, slotName: "writing-prompt", title: "Writing Prompt", skillFocus: "writing"},
	{position: 5, kind: kindSpeakingTask, taskType: subTypeReadAloud, slotName: "speaking-task-read-aloud", title: "Speaking Read Aloud", skillFocus: "speaking"},
	{position: 6, kind: kindSpeakingTask, taskType: subTypeRespond, slotName: "speaking-task-respond", title: "Speaking Respond", skillFocus: "speaking"},
}

type examSlotKey struct {
	level    string
	slotName string
}

type examPoolLayout struct {
	courseID uuid.UUID
	lessons  map[examSlotKey]uuid.UUID
}

// EnsureExamPoolStructure ensures the exam pool course, units, and 18 slot lessons exist.
func (s *Service) EnsureExamPoolStructure(ctx context.Context) error {
	_, err := s.examPool(ctx)
	return err
}

func (s *Service) examPool(ctx context.Context) (*examPoolLayout, error) {
	s.examPoolMu.Lock()
	defer s.examPoolMu.Unlock()
	if s.examPoolLayout != nil {
		return s.examPoolLayout, nil
	}
	if s.lessonAuthor == nil {
		return nil, errors.New("lesson author dependency is required for the exam pool")
	}
	layout, err := s.buildExamPool(ctx)
	if err != nil {
		return nil, err
	}
	s.examPoolLayout = layout
	return layout, nil
}

func (s *Service) buildExamPool(ctx context.Context) (*examPoolLayout, error) {
	courseID, err := s.lessonAuthor.EnsureCourse(ctx, lessoncontract.CourseSpec{
		Slug:           ExamPoolCourseSlug,
		Title:          examPoolCourseTitle,
		Description:    examPoolDescription,
		CEFRFrom:       "A2",
		CEFRTo:         "B2",
		EstimatedHours: 100,
	})
	if err != nil {
		return nil, fmt.Errorf("ensure exam pool course: %w", err)
	}

	layout := &examPoolLayout{courseID: courseID, lessons: make(map[examSlotKey]uuid.UUID)}
	for index, level := range examLevels {
		unitID, err := s.lessonAuthor.EnsureUnit(ctx, lessoncontract.UnitSpec{
			CourseID:    courseID,
			Position:    index + 1,
			Title:       level,
			Description: level + " exam pool",
		})
		if err != nil {
			return nil, fmt.Errorf("ensure exam pool unit %s: %w", level, err)
		}
		for _, slot := range examSlots {
			lessonID, err := s.lessonAuthor.EnsureLesson(ctx, lessoncontract.LessonSpec{
				UnitID:           unitID,
				Position:         slot.position,
				Title:            slot.title,
				SkillFocus:       slot.skillFocus,
				EstimatedMinutes: 15,
			})
			if err != nil {
				return nil, fmt.Errorf("ensure exam pool lesson %s in %s: %w", slot.title, level, err)
			}
			layout.lessons[examSlotKey{level: level, slotName: slot.slotName}] = lessonID
		}
	}
	return layout, nil
}

func (s *Service) examSlotActivities(
	ctx context.Context, layout *examPoolLayout, level, slotName string,
) ([]lessoncontract.Activity, error) {
	lessonID, ok := layout.lessons[examSlotKey{level: level, slotName: slotName}]
	if !ok {
		return nil, fmt.Errorf("no exam pool lesson for %s %s", level, slotName)
	}
	if s.lesson == nil {
		return nil, errors.New("lesson reader dependency is required for the exam pool")
	}
	lesson, err := s.lesson.GetLesson(ctx, lessonID)
	if err != nil {
		return nil, fmt.Errorf("load exam pool lesson %s: %w", lessonID, err)
	}
	if lesson == nil {
		return nil, nil
	}
	return lesson.Activities, nil
}

// --------------------------------------------------------------------------
// Top-up
// --------------------------------------------------------------------------

// TopUpExamPool generates and adds verified items to every exam slot that needs them.
func (s *Service) TopUpExamPool(ctx context.Context) error {
	if s.ai == nil {
		slog.WarnContext(ctx, "exam pool top-up skipped: no AI client")
		return nil
	}
	if s.contentAuthor == nil || s.lessonAuthor == nil {
		return errors.New("content author and lesson author dependencies are required for exam pool top-up")
	}

	author := s.resolveGeneratorAuthor(ctx)
	if author == uuid.Nil {
		slog.WarnContext(ctx, "exam pool top-up skipped: no owner for generated content")
		return nil
	}

	layout, err := s.examPool(ctx)
	if err != nil {
		return fmt.Errorf("ensure exam pool structure: %w", err)
	}

	for _, level := range examLevels {
		for _, slot := range examSlots {
			s.topUpExamSlot(ctx, layout, level, slot, author)
		}
	}
	return nil
}

func (s *Service) resolveGeneratorAuthor(ctx context.Context) uuid.UUID {
	if s.generatorAuthor != uuid.Nil {
		return s.generatorAuthor
	}
	if s.authorResolver != nil {
		if resolved, err := s.authorResolver.FirstHolderOf(ctx, "admin"); err == nil && resolved != uuid.Nil {
			return resolved
		}
	}
	return uuid.Nil
}

func (s *Service) topUpExamSlot(
	ctx context.Context, layout *examPoolLayout, level string, slot examSlotSpec, author uuid.UUID,
) {
	activities, err := s.examSlotActivities(ctx, layout, level, slot.slotName)
	if err != nil {
		slog.ErrorContext(ctx, "could not list exam pool slot", "level", level, "slot", slot.slotName, "error", err)
		return
	}
	toAdd, err := s.itemsToAdd(ctx, activities)
	if err != nil {
		slog.ErrorContext(ctx, "could not size exam pool top-up", "level", level, "slot", slot.slotName, "error", err)
		return
	}

	lessonID := layout.lessons[examSlotKey{level: level, slotName: slot.slotName}]
	added := 0
	for i := 0; i < toAdd; i++ {
		body, err := s.generateAndVerifyExamItem(ctx, level, slot, author, lessonID, activities)
		if err != nil {
			slog.WarnContext(ctx, "exam pool item not added", "level", level, "slot", slot.slotName, "error", err)
			continue
		}
		activities = append(activities, lessoncontract.Activity{Kind: slot.kind, Config: body})
		added++
	}
	if toAdd > 0 {
		slog.InfoContext(ctx, "exam pool slot topped up", "level", level, "slot", slot.slotName, "added", added)
	}
}

func (s *Service) generateAndVerifyExamItem(
	ctx context.Context, level string, slot examSlotSpec, author uuid.UUID,
	lessonID uuid.UUID, existing []lessoncontract.Activity,
) (json.RawMessage, error) {
	var lastErr error
	for attempt := 0; attempt <= maxRetriesPerItem; attempt++ {
		body, err := s.tryGenerateAndVerifyExamItem(ctx, level, slot, author, lessonID, existing)
		if err == nil {
			return body, nil
		}
		lastErr = err
		slog.WarnContext(ctx, "exam pool candidate rejected",
			"level", level, "slot", slot.slotName, "attempt", attempt+1, "reason", err)
	}
	return nil, lastErr
}

func (s *Service) tryGenerateAndVerifyExamItem(
	ctx context.Context, level string, slot examSlotSpec, author uuid.UUID,
	lessonID uuid.UUID, existing []lessoncontract.Activity,
) (json.RawMessage, error) {
	var body json.RawMessage
	task := ai.TaskPracticeGenerate
	vars := map[string]any{"Kind": slot.kind, "CEFRLevel": level}

	if slot.kind == kindListeningComprehension {
		task = ai.TaskListeningGenerate
		vars = map[string]any{"CEFRLevel": level}
	} else if slot.kind == kindSpeakingTask {
		vars["TaskType"] = slot.taskType
	}

	if err := ai.CompleteJSON(ctx, s.ai, ai.Request{
		Task: task,
		Vars: vars,
	}, &body); err != nil {
		return nil, fmt.Errorf("ai generate call failed: %w", err)
	}

	body, err := s.checkAndPrepareExamCandidate(ctx, level, slot, body, existing)
	if err != nil {
		return nil, err
	}

	if err := s.publishExamCandidate(ctx, level, slot, author, lessonID, body); err != nil {
		return nil, err
	}
	return body, nil
}

func (s *Service) checkAndPrepareExamCandidate(
	ctx context.Context, level string, slot examSlotSpec, body json.RawMessage,
	existing []lessoncontract.Activity,
) (json.RawMessage, error) {
	switch slot.kind {
	case kindListeningComprehension:
		return s.checkAndPrepareListening(ctx, level, body, existing)
	case kindReadingComprehension, kindGrammarSentenceTransform:
		if err := s.checkCandidate(ctx, level, slot.kind, body, existing); err != nil {
			return nil, err
		}
		return body, nil
	case kindWritingPrompt:
		return s.checkWritingPrompt(ctx, body, existing)
	case kindSpeakingTask:
		return s.checkSpeakingTask(ctx, slot.taskType, body, existing)
	default:
		return nil, fmt.Errorf("unsupported exam kind: %s", slot.kind)
	}
}

// --------------------------------------------------------------------------
// Listening Checks & Preparation
// --------------------------------------------------------------------------

type listeningCand struct {
	Title          string                              `json:"title"`
	Script         string                              `json:"script"`
	Voice          string                              `json:"voice,omitempty"`
	AudioObjectKey string                              `json:"audio_object_key,omitempty"`
	Questions      []candQuestion                      `json:"questions"`
	Explanation    *learningcontract.AnswerExplanation `json:"explanation,omitempty"`
}

func (s *Service) checkAndPrepareListening(
	ctx context.Context, level string, raw json.RawMessage, existing []lessoncontract.Activity,
) (json.RawMessage, error) {
	var cand listeningCand
	if err := json.Unmarshal(raw, &cand); err != nil {
		return nil, fmt.Errorf("check 1 (parse) failed: %w", err)
	}
	if strings.TrimSpace(cand.Title) == "" {
		return nil, errors.New("check 1 failed: listening title is empty")
	}
	if strings.TrimSpace(cand.Script) == "" {
		return nil, errors.New("check 1 failed: listening script is empty")
	}
	if len(cand.Questions) < 4 {
		return nil, fmt.Errorf("check 1 failed: listening must have at least 4 questions, got %d", len(cand.Questions))
	}
	if cand.Voice == "" {
		cand.Voice = "en-US-Standard-C"
	}

	grader, ok := s.graders.Get(kindListeningComprehension)
	if !ok || grader == nil {
		return nil, errors.New("listening grader not registered")
	}

	versionID := uuid.New()
	gradeCtx := contentcontract.ContextWithTempVersion(ctx, &contentcontract.Version{
		ID: versionID, Kind: kindListeningComprehension, Body: raw, CEFRLevel: level, Status: "published",
	})

	// Check 2: Own answer scores full marks
	ownAnswers := make(map[string]string, len(cand.Questions))
	for _, q := range cand.Questions {
		if q.CorrectOptionID == "" {
			return nil, fmt.Errorf("question %s missing correct_option_id", q.ID)
		}
		ownAnswers[q.ID] = q.CorrectOptionID
	}
	ownPayload, err := json.Marshal(map[string]any{"answers": ownAnswers})
	if err != nil {
		return nil, fmt.Errorf("build own answer payload: %w", err)
	}
	if err := gradesFullMarks(gradeCtx, grader, versionID, ownPayload); err != nil {
		return nil, fmt.Errorf("check 2 (own answer scores full marks) failed: %w", err)
	}

	// Check 3: Structure
	for _, q := range cand.Questions {
		if err := checkOptions(q.Options, q.CorrectOptionID); err != nil {
			return nil, fmt.Errorf("check 3 (structure) failed for question %s: %w", q.ID, err)
		}
		if q.Explanation == nil || strings.TrimSpace(q.Explanation.Vi()) == "" {
			return nil, fmt.Errorf("check 3 (structure) failed: question %s missing explanation_vi", q.ID)
		}
	}

	// Check 4: Blind solve
	if err := s.blindSolveListening(gradeCtx, grader, versionID, cand); err != nil {
		return nil, fmt.Errorf("check 4 (blind solve) failed: %w", err)
	}

	// Check 5: Deduplication
	normScript := normaliseText(cand.Script)
	for _, act := range existing {
		var old listeningCand
		if err := json.Unmarshal(act.Config, &old); err == nil && old.Script != "" {
			if normaliseText(old.Script) == normScript {
				return nil, errors.New("check 5 (deduplication) failed: script matches existing item")
			}
		}
	}

	// Check 6: Redaction verification
	redacted := contentcontract.RedactForLearner(raw)
	if err := verifyRedaction(redacted); err != nil {
		return nil, fmt.Errorf("check 6 (redaction) failed: %w", err)
	}
	var redactedCheck map[string]any
	if err := json.Unmarshal(redacted, &redactedCheck); err == nil {
		if _, leaked := redactedCheck["script"]; leaked {
			return nil, errors.New("check 6 failed: script leaked in redacted body")
		}
	}

	// Synthesise audio if synthesiser is configured
	if s.synthesiser != nil {
		audioKey, err := s.synthesiser.Synthesise(ctx, cand.Script, cand.Voice)
		if err != nil {
			slog.WarnContext(ctx, "could not pre-render audio for listening item; audio_object_key left empty", "error", err)
		} else {
			cand.AudioObjectKey = audioKey
		}
	}

	preparedRaw, err := json.Marshal(cand)
	if err != nil {
		return nil, fmt.Errorf("serialize prepared listening item: %w", err)
	}
	return preparedRaw, nil
}

func (s *Service) blindSolveListening(
	ctx context.Context, grader learningcontract.ExerciseGrader, versionID uuid.UUID,
	cand listeningCand,
) error {
	redactedQuestions := make([]map[string]any, 0, len(cand.Questions))
	for _, q := range cand.Questions {
		opts := make([]map[string]any, 0, len(q.Options))
		for _, opt := range q.Options {
			opts = append(opts, map[string]any{"id": opt.ID, "text": opt.Text})
		}
		redactedQuestions = append(redactedQuestions, map[string]any{
			"id":      q.ID,
			"type":    q.Type,
			"prompt":  q.Prompt,
			"options": opts,
		})
	}
	redactedBody, err := json.Marshal(map[string]any{
		"title":     cand.Title,
		"passage":   cand.Script, // blind solve reads script as passage
		"questions": redactedQuestions,
	})
	if err != nil {
		return err
	}

	var reply json.RawMessage
	if err := ai.CompleteJSON(ctx, s.ai, ai.Request{
		Task: ai.TaskPracticeSolve,
		Vars: map[string]any{"Kind": kindListeningComprehension, "RedactedBody": string(redactedBody)},
	}, &reply); err != nil {
		return fmt.Errorf("ai blind solve call failed: %w", err)
	}
	payload, err := parseBlindSolvePayload(kindReadingComprehension, reply)
	if err != nil {
		return fmt.Errorf("parse blind solve response: %w", err)
	}
	return gradesFullMarks(ctx, grader, versionID, payload)
}

// --------------------------------------------------------------------------
// Writing & Speaking Checks
// --------------------------------------------------------------------------

type writingPromptCand struct {
	Prompt           string                              `json:"prompt"`
	ModelAnswer      string                              `json:"model_answer"`
	MinWords         int                                 `json:"min_words,omitempty"`
	TimeLimitMinutes int                                 `json:"time_limit_minutes,omitempty"`
	Topic            string                              `json:"topic,omitempty"`
	Explanation      *learningcontract.AnswerExplanation `json:"explanation,omitempty"`
}

func (s *Service) checkWritingPrompt(
	_ context.Context, raw json.RawMessage, existing []lessoncontract.Activity,
) (json.RawMessage, error) {
	var cand writingPromptCand
	if err := json.Unmarshal(raw, &cand); err != nil {
		return nil, fmt.Errorf("check 1 (parse) failed: %w", err)
	}
	if strings.TrimSpace(cand.Prompt) == "" {
		return nil, errors.New("check 1 failed: prompt is empty")
	}
	if strings.TrimSpace(cand.ModelAnswer) == "" {
		return nil, errors.New("check 1 failed: model_answer is empty")
	}

	// Check 3: Structure
	promptWords := len(strings.Fields(cand.Prompt))
	if promptWords < 15 {
		return nil, fmt.Errorf("check 3 failed: prompt too short (%d words)", promptWords)
	}
	answerWords := len(strings.Fields(cand.ModelAnswer))
	if answerWords < 80 {
		return nil, fmt.Errorf("check 3 failed: model_answer too short (%d words)", answerWords)
	}

	// Check 5: Deduplication
	normPrompt := normaliseText(cand.Prompt)
	for _, act := range existing {
		var old writingPromptCand
		if err := json.Unmarshal(act.Config, &old); err == nil && old.Prompt != "" {
			if normaliseText(old.Prompt) == normPrompt {
				return nil, errors.New("check 5 (deduplication) failed: prompt matches existing writing prompt")
			}
		}
	}

	// Check 6: Redaction
	redacted := contentcontract.RedactForLearner(raw)
	var check map[string]any
	if err := json.Unmarshal(redacted, &check); err == nil {
		if _, leaked := check["model_answer"]; leaked {
			return nil, errors.New("check 6 failed: model_answer leaked in redacted body")
		}
	}
	return raw, nil
}

type speakingTaskCand struct {
	TaskType            string                              `json:"task_type"`
	Prompt              string                              `json:"prompt"`
	ReferenceText       string                              `json:"reference_text,omitempty"`
	SpeakingTimeSeconds int                                 `json:"speaking_time_seconds,omitempty"`
	Explanation         *learningcontract.AnswerExplanation `json:"explanation,omitempty"`
}

func (s *Service) checkSpeakingTask(
	_ context.Context, taskType string, raw json.RawMessage, existing []lessoncontract.Activity,
) (json.RawMessage, error) {
	var cand speakingTaskCand
	if err := json.Unmarshal(raw, &cand); err != nil {
		return nil, fmt.Errorf("check 1 (parse) failed: %w", err)
	}
	if cand.SpeakingTimeSeconds <= 0 {
		cand.SpeakingTimeSeconds = 45
	}
	if cand.TaskType == "" {
		cand.TaskType = taskType
	}

	// Check 3: Structure
	if taskType == subTypeReadAloud {
		if strings.TrimSpace(cand.ReferenceText) == "" {
			return nil, errors.New("check 1 failed: reference_text is empty for read_aloud")
		}
		words := len(strings.Fields(cand.ReferenceText))
		if words < 15 || words > 100 {
			return nil, fmt.Errorf("check 3 failed: reference_text word count (%d) out of range 15-100", words)
		}
		normRef := normaliseText(cand.ReferenceText)
		for _, act := range existing {
			var old speakingTaskCand
			if err := json.Unmarshal(act.Config, &old); err == nil && old.ReferenceText != "" {
				if normaliseText(old.ReferenceText) == normRef {
					return nil, errors.New("check 5 (deduplication) failed: reference_text matches existing item")
				}
			}
		}
	} else {
		if strings.TrimSpace(cand.Prompt) == "" {
			return nil, errors.New("check 1 failed: prompt is empty for respond task")
		}
		words := len(strings.Fields(cand.Prompt))
		if words < 8 {
			return nil, fmt.Errorf("check 3 failed: respond prompt too short (%d words)", words)
		}
		normPrompt := normaliseText(cand.Prompt)
		for _, act := range existing {
			var old speakingTaskCand
			if err := json.Unmarshal(act.Config, &old); err == nil && old.Prompt != "" {
				if normaliseText(old.Prompt) == normPrompt {
					return nil, errors.New("check 5 (deduplication) failed: prompt matches existing item")
				}
			}
		}
	}

	return raw, nil
}

func (s *Service) publishExamCandidate(
	ctx context.Context, level string, slot examSlotSpec, author uuid.UUID,
	lessonID uuid.UUID, body json.RawMessage,
) error {
	// Kebab-case slug: pool-exam-<level>-<slot-name>-<random>
	slug := fmt.Sprintf("pool-exam-%s-%s-%s",
		strings.ToLower(level), slot.slotName, uuid.New().String()[:8])

	versionID, err := s.contentAuthor.EnsurePublished(ctx, contentcontract.AuthorSpec{
		Slug:      slug,
		Kind:      slot.kind,
		CEFRLevel: level,
		Body:      body,
		AuthorID:  author,
	})
	if err != nil {
		return fmt.Errorf("publish verified content: %w", err)
	}

	if _, err := s.lessonAuthor.AppendActivity(ctx, lessonID, lessoncontract.ActivitySpec{
		Kind:             slot.kind,
		ContentVersionID: versionID,
		Config:           body,
		Weight:           1,
	}); err != nil {
		return fmt.Errorf("append activity to exam pool lesson: %w", err)
	}
	return nil
}

// --------------------------------------------------------------------------
// Draw Sitting
// --------------------------------------------------------------------------

// DrawExamSitting draws a complete 4-skill sitting for a learner at the requested level.
func (s *Service) DrawExamSitting(
	ctx context.Context, userID uuid.UUID, level string,
) ([]learningcontract.ExamSectionActivities, error) {
	normLevel := practiceLevel(level)
	layout, err := s.examPool(ctx)
	if err != nil {
		return nil, fmt.Errorf("load exam pool layout: %w", err)
	}

	// Section 1: Listening (3 clips with pre-rendered audio)
	listeningActs, err := s.examSlotActivities(ctx, layout, normLevel, "listening-comprehension")
	if err != nil {
		return nil, err
	}
	eligibleListening := filterListeningWithAudio(listeningActs)
	if len(eligibleListening) < 3 {
		return nil, apperr.New(apperr.NotFound, "EXAM_POOL_EMPTY", "exam pool has insufficient listening items with audio")
	}
	drawnListening, err := s.drawItemsFromPool(ctx, userID, eligibleListening, 3)
	if err != nil {
		return nil, err
	}

	// Section 2: Reading (2 passages)
	readingActs, err := s.examSlotActivities(ctx, layout, normLevel, "reading-comprehension")
	if err != nil {
		return nil, err
	}
	if len(readingActs) < 2 {
		return nil, apperr.New(apperr.NotFound, "EXAM_POOL_EMPTY", "exam pool has insufficient reading items")
	}
	drawnReading, err := s.drawItemsFromPool(ctx, userID, readingActs, 2)
	if err != nil {
		return nil, err
	}

	// Section 3: Writing (1 essay prompt + 3 sentence transforms)
	writingPromptActs, err := s.examSlotActivities(ctx, layout, normLevel, "writing-prompt")
	if err != nil {
		return nil, err
	}
	if len(writingPromptActs) < 1 {
		return nil, apperr.New(apperr.NotFound, "EXAM_POOL_EMPTY", "exam pool has insufficient writing prompts")
	}
	transformActs, err := s.examSlotActivities(ctx, layout, normLevel, "grammar-sentence-transform")
	if err != nil {
		return nil, err
	}
	if len(transformActs) < 3 {
		return nil, apperr.New(apperr.NotFound, "EXAM_POOL_EMPTY", "exam pool has insufficient sentence transform items")
	}

	drawnEssay, err := s.drawItemsFromPool(ctx, userID, writingPromptActs, 1)
	if err != nil {
		return nil, err
	}
	drawnTransforms, err := s.drawItemsFromPool(ctx, userID, transformActs, 3)
	if err != nil {
		return nil, err
	}
	drawnWriting := append(drawnEssay, drawnTransforms...)

	// Section 4: Speaking (2 read-aloud + 2 responses)
	readAloudActs, err := s.examSlotActivities(ctx, layout, normLevel, "speaking-task-read-aloud")
	if err != nil {
		return nil, err
	}
	if len(readAloudActs) < 2 {
		return nil, apperr.New(apperr.NotFound, "EXAM_POOL_EMPTY", "exam pool has insufficient speaking read-aloud items")
	}
	respondActs, err := s.examSlotActivities(ctx, layout, normLevel, "speaking-task-respond")
	if err != nil {
		return nil, err
	}
	if len(respondActs) < 2 {
		return nil, apperr.New(apperr.NotFound, "EXAM_POOL_EMPTY", "exam pool has insufficient speaking respond items")
	}

	drawnReadAloud, err := s.drawItemsFromPool(ctx, userID, readAloudActs, 2)
	if err != nil {
		return nil, err
	}
	drawnRespond, err := s.drawItemsFromPool(ctx, userID, respondActs, 2)
	if err != nil {
		return nil, err
	}
	drawnSpeaking := append(drawnReadAloud, drawnRespond...)

	sections := []learningcontract.ExamSectionActivities{
		{
			SectionPosition: 1,
			Skill:           "listening",
			Activities:      s.toExamActivities(drawnListening),
		},
		{
			SectionPosition: 2,
			Skill:           "reading",
			Activities:      s.toExamActivities(drawnReading),
		},
		{
			SectionPosition: 3,
			Skill:           "writing",
			Activities:      s.toExamActivities(drawnWriting),
		},
		{
			SectionPosition: 4,
			Skill:           "speaking",
			Activities:      s.toExamActivities(drawnSpeaking),
		},
	}
	return sections, nil
}

func (s *Service) toExamActivities(acts []lessoncontract.Activity) []learningcontract.ExamActivity {
	out := make([]learningcontract.ExamActivity, 0, len(acts))
	for _, act := range acts {
		redacted := contentcontract.RedactForLearner(act.Config)
		out = append(out, learningcontract.ExamActivity{
			ID:               act.ID,
			Kind:             act.Kind,
			ContentVersionID: act.ContentVersionID,
			Config:           redacted,
			Weight:           1,
		})
	}
	return out
}

func filterListeningWithAudio(acts []lessoncontract.Activity) []lessoncontract.Activity {
	eligible := make([]lessoncontract.Activity, 0, len(acts))
	for _, act := range acts {
		if len(act.Config) == 0 {
			continue
		}
		var cand listeningCand
		if err := json.Unmarshal(act.Config, &cand); err == nil && strings.TrimSpace(cand.AudioObjectKey) != "" {
			eligible = append(eligible, act)
		}
	}
	return eligible
}

func (s *Service) drawItemsFromPool(
	ctx context.Context, userID uuid.UUID, activities []lessoncontract.Activity, needed int,
) ([]lessoncontract.Activity, error) {
	if len(activities) < needed {
		return nil, errors.New("insufficient items in slot")
	}

	ids := idsOf(activities)
	exposures, err := s.repo.ListItemExposures(ctx, userID, ids)
	if err != nil {
		return nil, fmt.Errorf("list item exposures: %w", err)
	}

	activityMap := make(map[uuid.UUID]lessoncontract.Activity, len(activities))
	for _, act := range activities {
		activityMap[act.ID] = act
	}

	var unseen, seen []uuid.UUID
	for _, id := range ids {
		if _, served := exposures[id]; served {
			seen = append(seen, id)
		} else {
			unseen = append(unseen, id)
		}
	}

	pickedIDs := firstN(shuffledIDs(unseen), needed)
	if shortfall := needed - len(pickedIDs); shortfall > 0 && len(seen) > 0 {
		sort.Slice(seen, func(i, j int) bool {
			return exposures[seen[i]].Before(exposures[seen[j]])
		})
		repeated := firstN(seen, shortfall)
		pickedIDs = append(pickedIDs, repeated...)
		slog.InfoContext(ctx, "exam sitting repeated items the learner had seen",
			"user_id", userID, "repeated", len(repeated))
	}

	pickedActs := make([]lessoncontract.Activity, 0, len(pickedIDs))
	for _, id := range pickedIDs {
		if act, ok := activityMap[id]; ok {
			pickedActs = append(pickedActs, act)
		}
	}
	return pickedActs, nil
}
