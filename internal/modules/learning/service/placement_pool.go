package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/google/uuid"

	contentcontract "github.com/fluentra/fluentra/internal/modules/content/contract"
	"github.com/fluentra/fluentra/internal/modules/learning/domain"
	lessoncontract "github.com/fluentra/fluentra/internal/modules/lesson/contract"
	"github.com/fluentra/fluentra/internal/platform/ai"
)

// The placement pool — work order 13 §3.3.
//
// The same machinery as the practice and exam pools, pointed at a third course:
// append-only slot lessons under pool-placement, one unit per level A1–C1, the
// same checks and blind solve, and the same learn.item_exposures. No item is
// shared with pool-practice or pool-exam: each pool generates into its own
// lessons and draws only from them.

const (
	// PlacementPoolCourseSlug is the course that holds the placement pool's slot lessons.
	PlacementPoolCourseSlug = "pool-placement"

	placementPoolCourseTitle = "Placement Pool"
	placementPoolDescription = "Auto-generated placement test items"

	placementRunningLowThreshold = 5
	maxPlacementItemsToAddPerRun = 5

	// Placement item shapes (§3.3).
	placementChoiceOptions    = 4
	placementPassageQuestions = 3
	placementWritingMinWords  = 60
	placementWritingMaxWords  = 100
	placementSpeakingSeconds  = 45

	// promptKindVocabulary asks the generator for a vocabulary question. The item
	// is published as grammar_tense_choice: see learning/DECISIONS.md.
	promptKindVocabulary = "vocabulary"
)

type placementSlotSpec struct {
	position int
	// kind is the published activity kind, which picks the grader.
	kind string
	// promptKind is what the generator is asked for.
	promptKind string
	taskType   string
	slotName   string
	title      string
	skill      string
	target     int
	// minimum is what one full test can use at a band; the pool is ready when
	// every band has it in every slot.
	minimum int
}

var placementSlots = []placementSlotSpec{
	{
		position: 1, kind: kindGrammarTenseChoice, promptKind: promptKindVocabulary,
		slotName: "vocabulary", title: "Vocabulary Multiple Choice", skill: domain.SkillVocabulary,
		target: 40, minimum: domain.PlacementStageOneMax / 2,
	},
	{
		position: 2, kind: kindGrammarTenseChoice, promptKind: kindGrammarTenseChoice,
		slotName: "grammar-tense-choice", title: "Grammar Tense Choice", skill: domain.SkillGrammar,
		target: 40, minimum: domain.PlacementStageOneMax / 2,
	},
	{
		position: 3, kind: kindReadingComprehension, promptKind: kindReadingComprehension,
		slotName: slotReadingComprehension, title: titleReadingComprehension, skill: domain.SkillReading,
		target: 15, minimum: 3,
	},
	{
		position: 4, kind: kindListeningComprehension, promptKind: kindListeningComprehension,
		slotName: slotListening, title: "Listening Comprehension", skill: domain.SkillListening,
		target: 15, minimum: 3,
	},
	{
		position: 5, kind: kindWritingPrompt, promptKind: kindWritingPrompt,
		slotName: slotWritingPrompt, title: "Writing Prompt", skill: domain.SkillWriting,
		target: 10, minimum: 1,
	},
	{
		position: 6, kind: kindSpeakingTask, promptKind: kindSpeakingTask, taskType: subTypeRespond,
		slotName: slotSpeakingRespond, title: "Speaking Task", skill: domain.SkillSpeaking,
		target: 10, minimum: 1,
	},
}

// placementLengths are the word counts a passage and a clip's script may have at
// each level: passages of 60–180 words, clips of 20–60 seconds at about 2.3 words
// a second.
var placementLengths = map[string]struct{ passageMin, passageMax, scriptMin, scriptMax int }{
	domain.LevelA1: {passageMin: 60, passageMax: 90, scriptMin: 45, scriptMax: 70},
	domain.LevelA2: {passageMin: 80, passageMax: 110, scriptMin: 55, scriptMax: 85},
	domain.LevelB1: {passageMin: 100, passageMax: 140, scriptMin: 70, scriptMax: 105},
	domain.LevelB2: {passageMin: 120, passageMax: 160, scriptMin: 85, scriptMax: 125},
	domain.LevelC1: {passageMin: 140, passageMax: 180, scriptMin: 100, scriptMax: 140},
}

func placementSlotForSkill(skill string) (placementSlotSpec, bool) {
	for _, slot := range placementSlots {
		if slot.skill == skill {
			return slot, true
		}
	}
	return placementSlotSpec{}, false
}

type placementSlotKey struct {
	level    string
	slotName string
}

type placementPoolLayout struct {
	courseID uuid.UUID
	lessons  map[placementSlotKey]uuid.UUID
}

// EnsurePlacementPoolStructure ensures the placement pool course, units and 30 slot lessons exist.
func (s *Service) EnsurePlacementPoolStructure(ctx context.Context) error {
	_, err := s.placementPool(ctx)
	return err
}

func (s *Service) placementPool(ctx context.Context) (*placementPoolLayout, error) {
	s.placementPoolMu.Lock()
	defer s.placementPoolMu.Unlock()
	if s.placementPoolLayout != nil {
		return s.placementPoolLayout, nil
	}
	if s.lessonAuthor == nil {
		return nil, errors.New("lesson author dependency is required for the placement pool")
	}
	layout, err := s.buildPlacementPool(ctx)
	if err != nil {
		return nil, err
	}
	s.placementPoolLayout = layout
	return layout, nil
}

func (s *Service) buildPlacementPool(ctx context.Context) (*placementPoolLayout, error) {
	courseID, err := s.lessonAuthor.EnsureCourse(ctx, lessoncontract.CourseSpec{
		Slug:           PlacementPoolCourseSlug,
		Title:          placementPoolCourseTitle,
		Description:    placementPoolDescription,
		CEFRFrom:       domain.LevelA1,
		CEFRTo:         domain.LevelC1,
		EstimatedHours: 1,
	})
	if err != nil {
		return nil, fmt.Errorf("ensure placement pool course: %w", err)
	}

	layout := &placementPoolLayout{courseID: courseID, lessons: make(map[placementSlotKey]uuid.UUID)}
	for index, level := range domain.PlacementBands {
		unitID, err := s.lessonAuthor.EnsureUnit(ctx, lessoncontract.UnitSpec{
			CourseID:    courseID,
			Position:    index + 1,
			Title:       level,
			Description: level + " placement pool",
		})
		if err != nil {
			return nil, fmt.Errorf("ensure placement pool unit %s: %w", level, err)
		}
		for _, slot := range placementSlots {
			// A pool lesson keeps its level null: placement opens only curriculum lessons.
			lessonID, err := s.lessonAuthor.EnsureLesson(ctx, lessoncontract.LessonSpec{
				UnitID:           unitID,
				Position:         slot.position,
				Title:            slot.title,
				SkillFocus:       slot.skill,
				EstimatedMinutes: 1,
			})
			if err != nil {
				return nil, fmt.Errorf("ensure placement pool lesson %s in %s: %w", slot.title, level, err)
			}
			layout.lessons[placementSlotKey{level: level, slotName: slot.slotName}] = lessonID
		}
	}
	return layout, nil
}

func (s *Service) placementSlotActivities(
	ctx context.Context, layout *placementPoolLayout, level, slotName string,
) ([]lessoncontract.Activity, error) {
	lessonID, ok := layout.lessons[placementSlotKey{level: level, slotName: slotName}]
	if !ok {
		return nil, fmt.Errorf("no placement pool lesson for %s %s", level, slotName)
	}
	if s.lesson == nil {
		return nil, errors.New("lesson reader dependency is required for the placement pool")
	}
	lesson, err := s.lesson.GetLesson(ctx, lessonID)
	if err != nil {
		return nil, fmt.Errorf("load placement pool lesson %s: %w", lessonID, err)
	}
	if lesson == nil {
		return nil, nil
	}
	return lesson.Activities, nil
}

// placementPoolReady reports whether every band can serve a full test: a test
// that runs out of unseen items at a band stops estimating and starts guessing,
// so a thin pool is refused rather than quietly served (§3.3, §8).
func (s *Service) placementPoolReady(ctx context.Context, layout *placementPoolLayout) (bool, error) {
	for _, level := range domain.PlacementBands {
		for _, slot := range placementSlots {
			activities, err := s.placementSlotActivities(ctx, layout, level, slot.slotName)
			if err != nil {
				return false, err
			}
			if slot.kind == kindListeningComprehension {
				activities = s.listeningWithAudio(ctx, activities)
			}
			if len(activities) < slot.minimum {
				return false, nil
			}
		}
	}
	return true, nil
}

// --------------------------------------------------------------------------
// Top-up
// --------------------------------------------------------------------------

// TopUpPlacementPool generates and adds verified items to every placement slot that needs them.
func (s *Service) TopUpPlacementPool(ctx context.Context) error {
	if s.ai == nil {
		slog.WarnContext(ctx, "placement pool top-up skipped: no AI client")
		return nil
	}
	if s.contentAuthor == nil || s.lessonAuthor == nil {
		return errors.New("content author and lesson author dependencies are required for placement pool top-up")
	}

	// The author is resolved when the job runs, not when the worker starts.
	author := s.resolveGeneratorAuthor(ctx)
	if author == uuid.Nil {
		slog.WarnContext(ctx, "placement pool top-up skipped: no owner for generated content")
		return nil
	}

	layout, err := s.placementPool(ctx)
	if err != nil {
		return fmt.Errorf("ensure placement pool structure: %w", err)
	}

	for _, level := range domain.PlacementBands {
		for _, slot := range placementSlots {
			if stop := s.topUpPlacementSlot(ctx, layout, level, slot, author); stop {
				slog.WarnContext(ctx, "placement pool top-up stopped: every AI provider is out of quota or unavailable")
				return nil
			}
		}
	}
	return nil
}

// topUpPlacementSlot fills one slot, and reports true when every AI provider is
// refusing so the run stops.
func (s *Service) topUpPlacementSlot(
	ctx context.Context, layout *placementPoolLayout, level string, slot placementSlotSpec, author uuid.UUID,
) (stop bool) {
	activities, err := s.placementSlotActivities(ctx, layout, level, slot.slotName)
	if err != nil {
		slog.ErrorContext(ctx, "could not list placement pool slot", "level", level, "slot", slot.slotName, "error", err)
		return false
	}
	toAdd, err := s.placementItemsToAdd(ctx, activities, slot.target)
	if err != nil {
		slog.ErrorContext(ctx, "could not size placement pool top-up",
			"level", level, "slot", slot.slotName, "error", err)
		return false
	}

	lessonID := layout.lessons[placementSlotKey{level: level, slotName: slot.slotName}]
	added := 0
	for i := 0; i < toAdd; i++ {
		body, err := s.generateAndVerifyPlacementItem(ctx, level, slot, author, lessonID, activities)
		if err != nil {
			if errors.Is(err, ai.ErrProvidersUnavailable) {
				return true
			}
			slog.WarnContext(ctx, "placement pool item not added", "level", level, "slot", slot.slotName, "error", err)
			continue
		}
		activities = append(activities, lessoncontract.Activity{Kind: slot.kind, Config: body})
		added++
	}
	if toAdd > 0 {
		slog.InfoContext(ctx, "placement pool slot topped up", "level", level, "slot", slot.slotName, "added", added)
	}
	if slot.kind == kindListeningComprehension {
		s.requestAudioRender(ctx, added)
	}
	return false
}

// placementItemsToAdd is five per run until the target, then five only when a
// learner active recently has fewer than five unseen items in the slot.
func (s *Service) placementItemsToAdd(
	ctx context.Context, activities []lessoncontract.Activity, target int,
) (int, error) {
	count := len(activities)
	if count < target {
		return min(maxPlacementItemsToAddPerRun, target-count), nil
	}
	low, err := s.repo.HasActiveLearnerRunningLow(ctx, idsOf(activities), placementRunningLowThreshold)
	if err != nil {
		return 0, err
	}
	if !low {
		return 0, nil
	}
	return maxPlacementItemsToAddPerRun, nil
}

func (s *Service) generateAndVerifyPlacementItem(
	ctx context.Context, level string, slot placementSlotSpec, author uuid.UUID,
	lessonID uuid.UUID, existing []lessoncontract.Activity,
) (json.RawMessage, error) {
	var lastErr error
	for attempt := 0; attempt <= maxRetriesPerItem; attempt++ {
		body, err := s.tryGenerateAndVerifyPlacementItem(ctx, level, slot, author, lessonID, existing)
		if err == nil {
			return body, nil
		}
		lastErr = err
		slog.WarnContext(ctx, "placement pool candidate rejected",
			"level", level, "slot", slot.slotName, "attempt", attempt+1, "reason", err)
	}
	return nil, lastErr
}

func (s *Service) tryGenerateAndVerifyPlacementItem(
	ctx context.Context, level string, slot placementSlotSpec, author uuid.UUID,
	lessonID uuid.UUID, existing []lessoncontract.Activity,
) (json.RawMessage, error) {
	var raw json.RawMessage
	if err := ai.CompleteJSON(ctx, s.ai, ai.Request{
		Task: ai.TaskPlacementGenerate,
		Vars: map[string]any{varKind: slot.promptKind, varCEFRLevel: level},
	}, &raw); err != nil {
		return nil, fmt.Errorf("ai generate call failed: %w", err)
	}

	body, err := s.checkPlacementCandidate(ctx, level, slot, raw, existing)
	if err != nil {
		return nil, err
	}
	if err := s.publishPlacementCandidate(ctx, level, slot, author, lessonID, body); err != nil {
		return nil, err
	}
	return body, nil
}

func (s *Service) checkPlacementCandidate(
	ctx context.Context, level string, slot placementSlotSpec,
	raw json.RawMessage, existing []lessoncontract.Activity,
) (json.RawMessage, error) {
	switch slot.kind {
	case kindGrammarTenseChoice:
		return s.checkPlacementChoice(ctx, slot, raw, existing)
	case kindReadingComprehension:
		return s.checkPlacementReading(ctx, level, raw, existing)
	case kindListeningComprehension:
		return s.checkPlacementListening(ctx, level, raw, existing)
	case kindWritingPrompt:
		return s.checkPlacementWriting(ctx, raw, existing)
	case kindSpeakingTask:
		return s.checkPlacementSpeaking(ctx, raw, existing)
	default:
		return nil, fmt.Errorf("unsupported placement slot kind: %s", slot.kind)
	}
}

// checkPlacementChoice checks one question with four options, a Vietnamese
// explanation, no duplicate, nothing answer-bearing after redaction, and a blind
// solve that agrees with the key.
func (s *Service) checkPlacementChoice(
	ctx context.Context, slot placementSlotSpec, raw json.RawMessage, existing []lessoncontract.Activity,
) (json.RawMessage, error) {
	var cand grammarTenseChoiceCand
	if err := json.Unmarshal(raw, &cand); err != nil {
		return nil, fmt.Errorf("check 1 (parse) failed: %w", err)
	}
	if strings.TrimSpace(cand.Prompt) == "" || cand.Explanation.Vi() == "" {
		return nil, errors.New("check 1 failed: prompt or explanation_vi is empty")
	}
	if len(cand.Options) != placementChoiceOptions {
		return nil, fmt.Errorf("check 3 failed: %d options, want %d", len(cand.Options), placementChoiceOptions)
	}
	if err := checkOptions(cand.Options, cand.CorrectOptionID); err != nil {
		return nil, fmt.Errorf("check 3 (structure) failed: %w", err)
	}
	if duplicateText(existing, cand.Prompt, func(body json.RawMessage) string {
		var old grammarTenseChoiceCand
		_ = json.Unmarshal(body, &old)
		return old.Prompt
	}) {
		return nil, errors.New("check 5 (deduplication) failed: prompt matches existing item")
	}
	if err := verifyRedaction(contentcontract.RedactForLearner(raw)); err != nil {
		return nil, fmt.Errorf("check 6 (redaction) failed: %w", err)
	}
	if err := s.blindSolvePlacementChoice(ctx, slot.kind, raw, cand.CorrectOptionID); err != nil {
		return nil, fmt.Errorf("check 7 (blind solve) failed: %w", err)
	}
	return raw, nil
}

func (s *Service) blindSolvePlacementChoice(
	ctx context.Context, kind string, raw json.RawMessage, wantOptionID string,
) error {
	var reply struct {
		SelectedOptionID string `json:"selected_option_id"`
	}
	if err := ai.CompleteJSON(ctx, s.ai, ai.Request{
		Task: ai.TaskPlacementSolve,
		Vars: map[string]any{varKind: kind, varRedactedBody: string(contentcontract.RedactForLearner(raw))},
	}, &reply); err != nil {
		return fmt.Errorf("ai blind solve call failed: %w", err)
	}
	if !strings.EqualFold(strings.TrimSpace(reply.SelectedOptionID), strings.TrimSpace(wantOptionID)) {
		return fmt.Errorf("blind solve chose %q, want %q", reply.SelectedOptionID, wantOptionID)
	}
	return nil
}

func (s *Service) checkPlacementReading(
	ctx context.Context, level string, raw json.RawMessage, existing []lessoncontract.Activity,
) (json.RawMessage, error) {
	var cand readingComprehensionCand
	if err := json.Unmarshal(raw, &cand); err != nil {
		return nil, fmt.Errorf("check 1 (parse) failed: %w", err)
	}
	if strings.TrimSpace(cand.Passage) == "" {
		return nil, errors.New("check 1 failed: passage is empty")
	}
	bounds := placementLengths[level]
	if words := len(strings.Fields(cand.Passage)); words < bounds.passageMin || words > bounds.passageMax {
		return nil, fmt.Errorf("check 3 failed: passage has %d words, want %d–%d at %s",
			words, bounds.passageMin, bounds.passageMax, level)
	}
	if err := checkPlacementQuestions(cand.Questions); err != nil {
		return nil, err
	}
	if duplicateText(existing, cand.Passage, func(body json.RawMessage) string {
		var old readingComprehensionCand
		_ = json.Unmarshal(body, &old)
		return old.Passage
	}) {
		return nil, errors.New("check 5 (deduplication) failed: passage matches existing item")
	}
	if err := verifyRedaction(contentcontract.RedactForLearner(raw)); err != nil {
		return nil, fmt.Errorf("check 6 (redaction) failed: %w", err)
	}
	if err := s.blindSolvePlacementQuestions(ctx, kindReadingComprehension, cand.Passage, cand.Questions); err != nil {
		return nil, fmt.Errorf("check 7 (blind solve) failed: %w", err)
	}
	return raw, nil
}

func (s *Service) checkPlacementListening(
	ctx context.Context, level string, raw json.RawMessage, existing []lessoncontract.Activity,
) (json.RawMessage, error) {
	var cand listeningCand
	if err := json.Unmarshal(raw, &cand); err != nil {
		return nil, fmt.Errorf("check 1 (parse) failed: %w", err)
	}
	if strings.TrimSpace(cand.Script) == "" {
		return nil, errors.New("check 1 failed: script is empty")
	}
	bounds := placementLengths[level]
	if words := len(strings.Fields(cand.Script)); words < bounds.scriptMin || words > bounds.scriptMax {
		return nil, fmt.Errorf("check 3 failed: script has %d words, want %d–%d at %s",
			words, bounds.scriptMin, bounds.scriptMax, level)
	}
	if err := checkPlacementQuestions(cand.Questions); err != nil {
		return nil, err
	}
	if duplicateText(existing, cand.Script, func(body json.RawMessage) string {
		var old listeningCand
		_ = json.Unmarshal(body, &old)
		return old.Script
	}) {
		return nil, errors.New("check 5 (deduplication) failed: script matches existing item")
	}
	if err := verifyRedaction(contentcontract.RedactForLearner(raw)); err != nil {
		return nil, fmt.Errorf("check 6 (redaction) failed: %w", err)
	}
	if err := s.blindSolvePlacementQuestions(ctx, kindListeningComprehension, cand.Script, cand.Questions); err != nil {
		return nil, fmt.Errorf("check 7 (blind solve) failed: %w", err)
	}
	return raw, nil
}

// checkPlacementQuestions checks a passage's or clip's three questions.
func checkPlacementQuestions(questions []candQuestion) error {
	if len(questions) != placementPassageQuestions {
		return fmt.Errorf("check 3 failed: %d questions, want %d", len(questions), placementPassageQuestions)
	}
	for i, q := range questions {
		if strings.TrimSpace(q.ID) == "" || strings.TrimSpace(q.Prompt) == "" {
			return fmt.Errorf("check 1 failed: question %d has no id or prompt", i)
		}
		if q.Explanation.Vi() == "" {
			return fmt.Errorf("check 1 failed: question %d explanation_vi is empty", i)
		}
		if err := checkOptions(q.Options, q.CorrectOptionID); err != nil {
			return fmt.Errorf("check 3 (structure) failed for question %d: %w", i, err)
		}
	}
	return nil
}

func (s *Service) blindSolvePlacementQuestions(
	ctx context.Context, kind, text string, questions []candQuestion,
) error {
	redacted := make([]map[string]any, 0, len(questions))
	for _, q := range questions {
		options := make([]map[string]any, 0, len(q.Options))
		for _, opt := range q.Options {
			options = append(options, map[string]any{"id": opt.ID, "text": opt.Text})
		}
		redacted = append(redacted, map[string]any{"id": q.ID, "prompt": q.Prompt, "options": options})
	}
	body, err := json.Marshal(map[string]any{"passage": text, "questions": redacted})
	if err != nil {
		return err
	}

	var reply struct {
		Answers map[string]string `json:"answers"`
	}
	if err := ai.CompleteJSON(ctx, s.ai, ai.Request{
		Task: ai.TaskPlacementSolve,
		Vars: map[string]any{varKind: kind, varRedactedBody: string(body)},
	}, &reply); err != nil {
		return fmt.Errorf("ai blind solve call failed: %w", err)
	}
	for _, q := range questions {
		got := reply.Answers[q.ID]
		if !strings.EqualFold(strings.TrimSpace(got), strings.TrimSpace(q.CorrectOptionID)) {
			return fmt.Errorf("question %s: blind solve chose %q, want %q", q.ID, got, q.CorrectOptionID)
		}
	}
	return nil
}

// checkPlacementWriting checks a 60–100 word task.
func (s *Service) checkPlacementWriting(
	ctx context.Context, raw json.RawMessage, existing []lessoncontract.Activity,
) (json.RawMessage, error) {
	var cand writingPromptCand
	if err := json.Unmarshal(raw, &cand); err != nil {
		return nil, fmt.Errorf("check 1 (parse) failed: %w", err)
	}
	if cand.MinWords < placementWritingMinWords || cand.MinWords > placementWritingMaxWords {
		return nil, fmt.Errorf("check 3 failed: min_words %d, want %d–%d",
			cand.MinWords, placementWritingMinWords, placementWritingMaxWords)
	}
	if words := len(strings.Fields(cand.ModelAnswer)); words < placementWritingMinWords {
		return nil, fmt.Errorf("check 3 failed: model_answer has %d words, want at least %d",
			words, placementWritingMinWords)
	}
	return s.checkWritingPrompt(ctx, raw, existing)
}

// checkPlacementSpeaking checks a respond task of 45 seconds.
func (s *Service) checkPlacementSpeaking(
	ctx context.Context, raw json.RawMessage, existing []lessoncontract.Activity,
) (json.RawMessage, error) {
	var cand speakingTaskCand
	if err := json.Unmarshal(raw, &cand); err != nil {
		return nil, fmt.Errorf("check 1 (parse) failed: %w", err)
	}
	if cand.SpeakingTimeSeconds != 0 && cand.SpeakingTimeSeconds != placementSpeakingSeconds {
		return nil, fmt.Errorf("check 3 failed: speaking_time_seconds %d, want %d",
			cand.SpeakingTimeSeconds, placementSpeakingSeconds)
	}
	return s.checkSpeakingTask(ctx, subTypeRespond, raw, existing)
}

func duplicateText(existing []lessoncontract.Activity, text string, field func(json.RawMessage) string) bool {
	norm := normaliseText(text)
	for _, activity := range existing {
		if old := field(activity.Config); old != "" && normaliseText(old) == norm {
			return true
		}
	}
	return false
}

func (s *Service) publishPlacementCandidate(
	ctx context.Context, level string, slot placementSlotSpec, author uuid.UUID,
	lessonID uuid.UUID, body json.RawMessage,
) error {
	// Kebab-case slug: pool-placement-<level>-<slot-name>-<random>
	slug := fmt.Sprintf("pool-placement-%s-%s-%s",
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
		return fmt.Errorf("append activity to placement pool lesson: %w", err)
	}
	return nil
}
