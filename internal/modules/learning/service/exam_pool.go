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
	// ExamPoolCourseSlug is the course that holds the exam pool's slot lessons.
	ExamPoolCourseSlug = "pool-exam"

	examPoolCourseTitle = "Exam Pool"
	examPoolDescription = "Auto-generated exam exercises pool"

	kindListeningComprehension = "listening_comprehension"
	kindWritingPrompt          = "writing_prompt"
	kindSpeakingTask           = "speaking_task"
	// kindTypedCompletion is a completion question with a typed answer and a
	// word limit (WO 22 D22-25).
	kindTypedCompletion = "typed_completion"

	subTypeReadAloud = "read_aloud"
	subTypeRespond   = "respond"

	defaultSpeakingSeconds = 45

	skillListening = "listening"
	skillReading   = "reading"
	skillGrammar   = "grammar"
	skillWriting   = "writing"
	skillSpeaking  = "speaking"

	varKind         = "Kind"
	varCEFRLevel    = "CEFRLevel"
	varRedactedBody = "RedactedBody"
	keyAnswers      = "answers"

	// Slot lessons the pools share by name, and the titles the lessons carry.
	//
	// Three pools now generate overlapping kinds, so the same slot name and the
	// same lesson title are written in three files. Named here because a typo in
	// one of them does not fail: it makes a fourth slot that nothing draws from,
	// and the pool it belongs to quietly never fills.
	slotListening             = "listening-comprehension"
	slotReadingComprehension  = "reading-comprehension"
	slotGrammarTenseChoice    = "grammar-tense-choice"
	slotGrammarTransform      = "grammar-sentence-transform"
	slotWritingPrompt         = "writing-prompt"
	slotSpeakingReadAloud     = "speaking-task-read-aloud"
	slotSpeakingRespond       = "speaking-task-respond"
	titleListening            = "Listening Comprehension"
	titleReadingComprehension = "Reading Comprehension"
	titleGrammarTenseChoice   = "Grammar Tense Choice"
	titleGrammarTransform     = "Grammar Sentence Transform"
	titleWritingPrompt        = "Writing Prompt"
	titleSpeakingReadAloud    = "Speaking Read Aloud"
	titleSpeakingRespond      = "Speaking Respond"
)

var examLevels = []string{"A2", "B1", "B2"}

// poolSlotSpec describes one slot of a generated pool: what kind of item it
// holds, and how many.
//
// It is shared with the practice pool. The two pools generate the same kinds
// through the same six checks, and the only things that ever differed were the
// slug prefix and how deep each slot is kept. Keeping two slot types meant a
// kind added to one pool silently stayed missing from the other, which is how
// the practice pool ended up generating three of the six kinds a sitting draws.
//
// `slotName` rather than `kind` identifies a slot, because speaking has two:
// read-aloud and respond are one kind with different tasks.
type poolSlotSpec struct {
	position   int
	kind       string
	taskType   string
	slotName   string
	title      string
	skillFocus string
	// target is how many items the slot is filled to, and ceiling how far it
	// may grow for learners who have seen everything. Zero means the defaults.
	//
	// They are per slot because the items are not equally cheap. A listening
	// clip has to be rendered offline before it can be drawn at all, and a
	// writing prompt is answered once a day at most; filling either to the
	// depth a one-line grammar drill needs spends the AI budget on a backlog
	// nobody reaches.
	target  int
	ceiling int
}

// slotTarget and slotCeiling read a slot's depth, falling back to the defaults.
func (slot poolSlotSpec) slotTarget() int {
	if slot.target > 0 {
		return slot.target
	}
	return targetActiveItemsPerSlot
}

func (slot poolSlotSpec) slotCeiling() int {
	if slot.ceiling > 0 {
		return slot.ceiling
	}
	return maxActiveItemsPerSlot
}

// The depths are per slot for the same reason as the practice pool's: one
// sitting draws three clips and one essay prompt, and filling every slot to the
// same fifty spent the AI budget on writing prompts and undrawable clips while
// the grammar slot a sitting takes three from stayed short.
var examSlots = []poolSlotSpec{
	{
		position:   1,
		kind:       kindListeningComprehension,
		slotName:   slotListening,
		title:      titleListening,
		skillFocus: skillListening,
		target:     30, ceiling: 120,
	},
	{
		position:   2,
		kind:       kindReadingComprehension,
		slotName:   slotReadingComprehension,
		title:      titleReadingComprehension,
		skillFocus: skillReading,
		target:     30, ceiling: 120,
	},
	{
		position:   3,
		kind:       kindGrammarSentenceTransform,
		slotName:   slotGrammarTransform,
		title:      titleGrammarTransform,
		skillFocus: skillGrammar,
		target:     50, ceiling: 200,
	},
	{
		position:   4,
		kind:       kindWritingPrompt,
		slotName:   slotWritingPrompt,
		title:      titleWritingPrompt,
		skillFocus: skillWriting,
		target:     20, ceiling: 80,
	},
	{
		position:   5,
		kind:       kindSpeakingTask,
		taskType:   subTypeReadAloud,
		slotName:   slotSpeakingReadAloud,
		title:      titleSpeakingReadAloud,
		skillFocus: skillSpeaking,
		target:     25, ceiling: 100,
	},
	{
		position:   6,
		kind:       kindSpeakingTask,
		taskType:   subTypeRespond,
		slotName:   slotSpeakingRespond,
		title:      titleSpeakingRespond,
		skillFocus: skillSpeaking,
		target:     25, ceiling: 100,
	},
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
			if stop := s.topUpExamSlot(ctx, layout, level, slot, author); stop {
				slog.WarnContext(ctx, "exam pool top-up stopped: every AI provider is out of quota or unavailable")
				return nil
			}
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

// topUpExamSlot fills one slot, and reports true when every AI provider is
// refusing so the run stops.
func (s *Service) topUpExamSlot(
	ctx context.Context, layout *examPoolLayout, level string, slot poolSlotSpec, author uuid.UUID,
) (stop bool) {
	activities, err := s.examSlotActivities(ctx, layout, level, slot.slotName)
	if err != nil {
		slog.ErrorContext(ctx, "could not list exam pool slot", "level", level, "slot", slot.slotName, "error", err)
		return false
	}
	toAdd, err := s.itemsToAdd(ctx, activities, slot)
	if err != nil {
		slog.ErrorContext(ctx, "could not size exam pool top-up", "level", level, "slot", slot.slotName, "error", err)
		return false
	}

	lessonID := layout.lessons[examSlotKey{level: level, slotName: slot.slotName}]
	added := 0
	for i := 0; i < toAdd; i++ {
		body, err := s.generateAndVerifyExamItem(ctx, level, slot, author, lessonID, activities)
		if err != nil {
			if errors.Is(err, ai.ErrProvidersUnavailable) {
				return true
			}
			slog.WarnContext(ctx, "exam pool item not added", "level", level, "slot", slot.slotName, "error", err)
			continue
		}
		activities = append(activities, lessoncontract.Activity{Kind: slot.kind, Config: body})
		added++
	}
	if toAdd > 0 {
		slog.InfoContext(ctx, "exam pool slot topped up", "level", level, "slot", slot.slotName, "added", added)
	}
	if slot.kind == kindListeningComprehension {
		s.requestAudioRender(ctx, added)
	}
	return false
}

func (s *Service) generateAndVerifyExamItem(
	ctx context.Context, level string, slot poolSlotSpec, author uuid.UUID,
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

// generatePoolCandidate asks the model for one item of the slot's kind.
//
// Shared by both generated pools: the prompt task and its variables follow from
// the kind, not from which pool asked.
func (s *Service) generatePoolCandidate(
	ctx context.Context, level string, slot poolSlotSpec,
) (json.RawMessage, error) {
	task := ai.TaskPracticeGenerate
	vars := map[string]any{varKind: slot.kind, varCEFRLevel: level}

	switch slot.kind {
	case kindListeningComprehension:
		task = ai.TaskListeningGenerate
		vars = map[string]any{varCEFRLevel: level}
	case kindSpeakingTask:
		vars["TaskType"] = slot.taskType
	}

	var body json.RawMessage
	if err := ai.CompleteJSON(ctx, s.ai, ai.Request{Task: task, Vars: vars}, &body); err != nil {
		return nil, fmt.Errorf("ai generate call failed: %w", err)
	}
	return body, nil
}

func (s *Service) tryGenerateAndVerifyExamItem(
	ctx context.Context, level string, slot poolSlotSpec, author uuid.UUID,
	lessonID uuid.UUID, existing []lessoncontract.Activity,
) (json.RawMessage, error) {
	body, err := s.generatePoolCandidate(ctx, level, slot)
	if err != nil {
		return nil, err
	}

	body, err = s.checkAndPreparePoolCandidate(ctx, level, slot, body, existing)
	if err != nil {
		return nil, err
	}

	if err := s.publishExamCandidate(ctx, level, slot, author, lessonID, body); err != nil {
		return nil, err
	}
	return body, nil
}

func (s *Service) checkAndPreparePoolCandidate(
	ctx context.Context, level string, slot poolSlotSpec, body json.RawMessage,
	existing []lessoncontract.Activity,
) (json.RawMessage, error) {
	switch slot.kind {
	case kindListeningComprehension:
		return s.checkAndPrepareListening(ctx, level, body, existing)
	case kindReadingComprehension, kindGrammarTenseChoice, kindGrammarSentenceTransform:
		// The three kinds whose answer key is in the body: checkCandidate runs
		// all six checks over them unchanged. grammar_tense_choice is here
		// because the practice pool generates it and this switch is now both
		// pools'; routed through the default it was "unsupported kind", and
		// every tense-choice candidate was rejected without being looked at.
		if err := s.checkCandidate(ctx, level, slot.kind, body, existing); err != nil {
			return nil, err
		}
		return body, nil
	case kindWritingPrompt:
		return s.checkWritingPrompt(ctx, body, existing)
	case kindSpeakingTask:
		return s.checkSpeakingTask(ctx, slot.taskType, body, existing)
	default:
		return nil, fmt.Errorf("unsupported pool kind: %s", slot.kind)
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

func (s *Service) verifyListeningCandidate(
	ctx context.Context, level string, raw json.RawMessage, existing []lessoncontract.Activity, blindSolve bool,
) (listeningCand, error) {
	return s.verifyListeningCandidateWithMin(ctx, level, raw, existing, blindSolve, 4)
}

func (s *Service) verifyListeningCandidateWithMin(
	ctx context.Context, level string, raw json.RawMessage,
	existing []lessoncontract.Activity, blindSolve bool, minQuestions int,
) (listeningCand, error) {
	cand, err := parseListeningCandidateWithMin(raw, minQuestions)
	if err != nil {
		return cand, err
	}

	grader, ok := s.graders.Get(kindListeningComprehension)
	if !ok || grader == nil {
		return cand, errors.New("listening grader not registered")
	}

	versionID := uuid.New()
	gradeCtx := contentcontract.ContextWithTempVersion(ctx, &contentcontract.Version{
		ID: versionID, Kind: kindListeningComprehension, Body: raw, CEFRLevel: level, Status: statusPublished,
	})

	// Checks 2 and 3: the item's own answers score full marks, and its questions are well formed.
	if err := checkListeningQuestions(gradeCtx, grader, versionID, cand); err != nil {
		return cand, err
	}

	// Check 4: Blind solve
	if blindSolve {
		if err := s.blindSolveListening(gradeCtx, grader, versionID, cand); err != nil {
			return cand, fmt.Errorf("check 4 (blind solve) failed: %w", err)
		}
	}

	// Checks 5 and 6: not a duplicate, and nothing answer-bearing survives redaction.
	if err := checkListeningNovelAndRedacted(raw, cand, existing); err != nil {
		return cand, err
	}
	return cand, nil
}

func (s *Service) checkAndPrepareListening(
	ctx context.Context, level string, raw json.RawMessage, existing []lessoncontract.Activity,
) (json.RawMessage, error) {
	cand, err := s.verifyListeningCandidate(ctx, level, raw, existing, true)
	if err != nil {
		return nil, err
	}

	// Audio is rendered offline (§4); a clip already in the cache is attached now.
	if s.synthesiser != nil {
		audioKey, err := s.synthesiser.Synthesise(ctx, cand.Script, cand.Voice)
		if err != nil {
			slog.WarnContext(ctx, "listening item published without audio; cmd/tts renders it", "error", err)
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

// parseListeningCandidate is check 1: the reply parses and has a title, a script and at least four questions.
func parseListeningCandidate(raw json.RawMessage) (listeningCand, error) {
	return parseListeningCandidateWithMin(raw, 4)
}

func parseListeningCandidateWithMin(raw json.RawMessage, minQuestions int) (listeningCand, error) {
	var cand listeningCand
	if err := json.Unmarshal(raw, &cand); err != nil {
		return cand, fmt.Errorf("check 1 (parse) failed: %w", err)
	}
	if strings.TrimSpace(cand.Title) == "" {
		return cand, errors.New("check 1 failed: listening title is empty")
	}
	if strings.TrimSpace(cand.Script) == "" {
		return cand, errors.New("check 1 failed: listening script is empty")
	}
	if minQuestions <= 0 {
		minQuestions = 4
	}
	if len(cand.Questions) < minQuestions {
		return cand, fmt.Errorf(
			"check 1 failed: listening must have at least %d questions, got %d",
			minQuestions, len(cand.Questions),
		)
	}
	// The voice is configuration (media.ConfiguredVoice), not the model's to pick.
	// The default written here was a cloud provider's voice name that no engine
	// could render, so no clip for these items could ever be made or found.
	cand.Voice = ""
	return cand, nil
}

// checkListeningQuestions runs checks 2 and 3.
func checkListeningQuestions(
	ctx context.Context, grader learningcontract.ExerciseGrader, versionID uuid.UUID, cand listeningCand,
) error {
	ownAnswers := make(map[string]string, len(cand.Questions))
	for _, q := range cand.Questions {
		key := questionKey(q)
		if key == "" {
			return fmt.Errorf("question %s has no answer key", q.ID)
		}
		ownAnswers[q.ID] = key
	}
	ownPayload, err := json.Marshal(map[string]any{keyAnswers: ownAnswers})
	if err != nil {
		return fmt.Errorf("build own answer payload: %w", err)
	}
	if err := gradesFullMarks(ctx, grader, versionID, ownPayload); err != nil {
		return fmt.Errorf("check 2 (own answer scores full marks) failed: %w", err)
	}

	for _, q := range cand.Questions {
		if err := checkGroupQuestion(q, 0); err != nil {
			return fmt.Errorf("check 3 (structure) failed for question %s: %w", q.ID, err)
		}
		if q.Explanation == nil || strings.TrimSpace(q.Explanation.Vi()) == "" {
			return fmt.Errorf("check 3 (structure) failed: question %s missing explanation_vi", q.ID)
		}
	}
	return nil
}

// checkListeningNovelAndRedacted runs checks 5 and 6.
func checkListeningNovelAndRedacted(raw json.RawMessage, cand listeningCand, existing []lessoncontract.Activity) error {
	normScript := normaliseText(cand.Script)
	for _, act := range existing {
		var old listeningCand
		if err := json.Unmarshal(act.Config, &old); err != nil || old.Script == "" {
			continue
		}
		if normaliseText(old.Script) == normScript {
			return errors.New("check 5 (deduplication) failed: script matches existing item")
		}
	}

	redacted := contentcontract.RedactForLearner(raw)
	if err := verifyRedaction(redacted); err != nil {
		return fmt.Errorf("check 6 (redaction) failed: %w", err)
	}
	var redactedCheck map[string]any
	if err := json.Unmarshal(redacted, &redactedCheck); err == nil {
		if _, leaked := redactedCheck["script"]; leaked {
			return errors.New("check 6 failed: script leaked in redacted body")
		}
	}
	return nil
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
		Vars: map[string]any{varKind: kindListeningComprehension, varRedactedBody: string(redactedBody)},
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
		cand.SpeakingTimeSeconds = defaultSpeakingSeconds
	}
	if cand.TaskType == "" {
		cand.TaskType = taskType
	}
	if cand.TaskType != taskType {
		return nil, fmt.Errorf("check 1 failed: task_type %q in the %s slot", cand.TaskType, taskType)
	}

	// Checks 3 and 5: structure and duplicates, on the text that defines the task.
	checkTask := checkRespondTask
	if taskType == subTypeReadAloud {
		checkTask = checkReadAloudTask
	}
	if err := checkTask(cand, existing); err != nil {
		return nil, err
	}

	// The body published is the checked one, defaults included: the runner reads
	// task_type and speaking_time_seconds, and the model may have omitted both.
	prepared, err := json.Marshal(cand)
	if err != nil {
		return nil, fmt.Errorf("serialize prepared speaking task: %w", err)
	}
	return prepared, nil
}

func checkReadAloudTask(cand speakingTaskCand, existing []lessoncontract.Activity) error {
	if strings.TrimSpace(cand.ReferenceText) == "" {
		return errors.New("check 1 failed: reference_text is empty for read_aloud")
	}
	words := len(strings.Fields(cand.ReferenceText))
	if words < 15 || words > 100 {
		return fmt.Errorf("check 3 failed: reference_text word count (%d) out of range 15-100", words)
	}
	if speakingDuplicate(existing, cand.ReferenceText, func(old speakingTaskCand) string { return old.ReferenceText }) {
		return errors.New("check 5 (deduplication) failed: reference_text matches existing item")
	}
	return nil
}

func checkRespondTask(cand speakingTaskCand, existing []lessoncontract.Activity) error {
	if strings.TrimSpace(cand.Prompt) == "" {
		return errors.New("check 1 failed: prompt is empty for respond task")
	}
	if words := len(strings.Fields(cand.Prompt)); words < 8 {
		return fmt.Errorf("check 3 failed: respond prompt too short (%d words)", words)
	}
	if speakingDuplicate(existing, cand.Prompt, func(old speakingTaskCand) string { return old.Prompt }) {
		return errors.New("check 5 (deduplication) failed: prompt matches existing item")
	}
	return nil
}

// speakingDuplicate reports whether an existing task already has this text.
func speakingDuplicate(existing []lessoncontract.Activity, text string, field func(speakingTaskCand) string) bool {
	norm := normaliseText(text)
	for _, act := range existing {
		var old speakingTaskCand
		if err := json.Unmarshal(act.Config, &old); err != nil {
			continue
		}
		if value := field(old); value != "" && normaliseText(value) == norm {
			return true
		}
	}
	return false
}

func (s *Service) publishExamCandidate(
	ctx context.Context, level string, slot poolSlotSpec, author uuid.UUID,
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

	// Every slot is checked before anything is drawn: a sitting is never short,
	// and a pool that cannot fill one is reported as empty.
	pools := make(map[string][]lessoncontract.Activity)
	for _, section := range examSittingPlan {
		for _, draw := range section.draws {
			activities, err := s.examSlotActivities(ctx, layout, normLevel, draw.slot)
			if err != nil {
				return nil, err
			}
			if draw.slot == slotListening {
				// A listening item without audio is never drawn.
				activities = s.listeningWithAudio(ctx, activities)
			}
			if len(activities) < draw.count {
				return nil, apperr.New(apperr.NotFound, "EXAM_POOL_EMPTY", "exam pool has insufficient "+draw.what)
			}
			pools[draw.slot] = activities
		}
	}

	sections := make([]learningcontract.ExamSectionActivities, 0, len(examSittingPlan))
	for _, section := range examSittingPlan {
		var drawn []lessoncontract.Activity
		for _, draw := range section.draws {
			picked, err := s.drawItemsFromPool(ctx, userID, pools[draw.slot], draw.count)
			if err != nil {
				return nil, err
			}
			drawn = append(drawn, picked...)
		}
		sections = append(sections, learningcontract.ExamSectionActivities{
			SectionPosition: section.position,
			Skill:           section.skill,
			Activities:      s.toExamActivities(drawn),
		})
	}
	return sections, nil
}

type examDraw struct {
	slot  string
	count int
	what  string
}

type examSectionPlan struct {
	position int
	skill    string
	draws    []examDraw
}

// examSittingPlan is the composition of every sitting, in both modes (§3.7).
var examSittingPlan = []examSectionPlan{
	{position: 1, skill: skillListening, draws: []examDraw{
		{slot: slotListening, count: 3, what: "listening items with audio"},
	}},
	{position: 2, skill: skillReading, draws: []examDraw{
		{slot: slotReadingComprehension, count: 2, what: "reading items"},
	}},
	{position: 3, skill: skillWriting, draws: []examDraw{
		{slot: slotWritingPrompt, count: 1, what: "writing prompts"},
		{slot: slotGrammarTransform, count: 3, what: "sentence transform items"},
	}},
	{position: 4, skill: skillSpeaking, draws: []examDraw{
		{slot: slotSpeakingReadAloud, count: 2, what: "speaking read-aloud items"},
		{slot: slotSpeakingRespond, count: 2, what: "speaking respond items"},
	}},
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

// listeningWithAudio keeps the listening items whose clip exists.
//
// A clip exists when the body names its object, or when the TTS cache holds a
// render of its script. The second is the usual case: audio is rendered offline
// by cmd/tts after the item is published, and a pool lesson is append-only, so
// the body never learns the key. Reading only the body left every item the
// top-up wrote undrawable, and no sitting could ever start.
func (s *Service) listeningWithAudio(ctx context.Context, acts []lessoncontract.Activity) []lessoncontract.Activity {
	eligible := make([]lessoncontract.Activity, 0, len(acts))
	for _, act := range acts {
		if len(act.Config) == 0 {
			continue
		}
		var cand listeningCand
		if err := json.Unmarshal(act.Config, &cand); err != nil {
			continue
		}
		if strings.TrimSpace(cand.AudioObjectKey) != "" {
			eligible = append(eligible, act)
			continue
		}
		if s.audio == nil || strings.TrimSpace(cand.Script) == "" {
			continue
		}
		_, found, err := s.audio.AudioKey(ctx, cand.Script, cand.Voice)
		if err != nil {
			slog.WarnContext(ctx, "could not look up listening audio", "activity_id", act.ID, "error", err)
			continue
		}
		if found {
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
