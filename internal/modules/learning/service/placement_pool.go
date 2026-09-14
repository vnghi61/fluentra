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
	lessoncontract "github.com/fluentra/fluentra/internal/modules/lesson/contract"
	"github.com/fluentra/fluentra/internal/platform/ai"
)

// The placement pool — work order 13 §3.3.
//
// Slots: 30 — 6 kinds across A1, A2, B1, B2, and C1.
// Top-up runs hourly on lock 1_700_000_216.
// Course: pool-placement.
// Append-only lessons, no items shared with pool-practice or pool-exam.

const (
	// PlacementPoolCourseSlug is the course that holds the placement pool's slot lessons.
	PlacementPoolCourseSlug = "pool-placement"

	placementPoolCourseTitle = "Placement Pool"
	placementPoolDescription = "Auto-generated placement test items pool"

	// Lock ID for placement pool top-up cron job (WO13 §5).
	topUpPlacementPoolLockID int64 = 1_700_000_216

	placementRunningLowThreshold = 5
	maxPlacementItemsToAddPerRun = 5
)

var placementLevels = []string{"A1", "A2", "B1", "B2", "C1"}

type placementSlotSpec struct {
	position   int
	kind       string
	taskType   string
	slotName   string
	title      string
	skillFocus string
	target     int
}

var placementSlots = []placementSlotSpec{
	{
		position:   1,
		kind:       "vocabulary",
		slotName:   "vocabulary",
		title:      "Vocabulary Multiple Choice",
		skillFocus: "vocabulary",
		target:     40,
	},
	{
		position:   2,
		kind:       kindGrammarTenseChoice,
		slotName:   "grammar-tense-choice",
		title:      "Grammar Tense Choice",
		skillFocus: skillGrammar,
		target:     40,
	},
	{
		position:   3,
		kind:       kindReadingComprehension,
		slotName:   "reading-comprehension",
		title:      "Reading Comprehension",
		skillFocus: skillReading,
		target:     15,
	},
	{
		position:   4,
		kind:       kindListeningComprehension,
		slotName:   "listening-comprehension",
		title:      "Listening Comprehension",
		skillFocus: skillListening,
		target:     15,
	},
	{
		position:   5,
		kind:       kindWritingPrompt,
		slotName:   "writing-prompt",
		title:      "Writing Prompt",
		skillFocus: skillWriting,
		target:     10,
	},
	{
		position:   6,
		kind:       kindSpeakingTask,
		taskType:   subTypeRespond,
		slotName:   "speaking-task-respond",
		title:      "Speaking Task",
		skillFocus: skillSpeaking,
		target:     10,
	},
}

type placementSlotKey struct {
	level    string
	slotName string
}

type placementPoolLayout struct {
	courseID uuid.UUID
	lessons  map[placementSlotKey]uuid.UUID
}

// EnsurePlacementPoolStructure ensures the placement pool course, units, and 30 slot lessons exist.
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
		CEFRFrom:       "A1",
		CEFRTo:         "C1",
		EstimatedHours: 100,
	})
	if err != nil {
		return nil, fmt.Errorf("ensure placement pool course: %w", err)
	}

	layout := &placementPoolLayout{courseID: courseID, lessons: make(map[placementSlotKey]uuid.UUID)}
	for index, level := range placementLevels {
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
			lessonID, err := s.lessonAuthor.EnsureLesson(ctx, lessoncontract.LessonSpec{
				UnitID:           unitID,
				Position:         slot.position,
				Title:            slot.title,
				SkillFocus:       slot.skillFocus,
				EstimatedMinutes: 15,
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

	author := s.resolveGeneratorAuthor(ctx)
	if author == uuid.Nil {
		slog.WarnContext(ctx, "placement pool top-up skipped: no owner for generated content")
		return nil
	}

	layout, err := s.placementPool(ctx)
	if err != nil {
		return fmt.Errorf("ensure placement pool structure: %w", err)
	}

	for _, level := range placementLevels {
		for _, slot := range placementSlots {
			s.topUpPlacementSlot(ctx, layout, level, slot, author)
		}
	}
	return nil
}

func (s *Service) topUpPlacementSlot(
	ctx context.Context, layout *placementPoolLayout, level string, slot placementSlotSpec, author uuid.UUID,
) {
	activities, err := s.placementSlotActivities(ctx, layout, level, slot.slotName)
	if err != nil {
		slog.ErrorContext(ctx, "could not list placement pool slot", "level", level, "slot", slot.slotName, "error", err)
		return
	}
	toAdd, err := s.placementItemsToAdd(ctx, activities, slot.target)
	if err != nil {
		slog.ErrorContext(ctx, "could not size placement pool top-up", "level", level, "slot", slot.slotName, "error", err)
		return
	}

	lessonID := layout.lessons[placementSlotKey{level: level, slotName: slot.slotName}]
	added := 0
	for i := 0; i < toAdd; i++ {
		body, err := s.generateAndVerifyPlacementItem(ctx, level, slot, author, lessonID, activities)
		if err != nil {
			slog.WarnContext(ctx, "placement pool item not added", "level", level, "slot", slot.slotName, "error", err)
			continue
		}
		activities = append(activities, lessoncontract.Activity{Kind: slot.kind, Config: body})
		added++
	}
	if toAdd > 0 {
		slog.InfoContext(ctx, "placement pool slot topped up", "level", level, "slot", slot.slotName, "added", added)
	}
}

func (s *Service) placementItemsToAdd(ctx context.Context, activities []lessoncontract.Activity, target int) (int, error) {
	count := len(activities)
	maxCeiling := target * 3
	switch {
	case count < target:
		return min(maxPlacementItemsToAddPerRun, target-count), nil
	case count >= maxCeiling:
		return 0, nil
	}

	low, err := s.repo.HasActiveLearnerRunningLow(ctx, idsOf(activities), placementRunningLowThreshold)
	if err != nil {
		return 0, err
	}
	if !low {
		return 0, nil
	}
	return min(maxPlacementItemsToAddPerRun, maxCeiling-count), nil
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
	var body json.RawMessage
	task := ai.TaskPlacementGenerate
	vars := map[string]any{varKind: slot.kind, varCEFRLevel: level}

	if err := ai.CompleteJSON(ctx, s.ai, ai.Request{
		Task: task,
		Vars: vars,
	}, &body); err != nil {
		return nil, fmt.Errorf("ai generate call failed: %w", err)
	}

	body, err := s.checkAndPreparePlacementCandidate(ctx, level, slot, body, existing)
	if err != nil {
		return nil, err
	}

	if err := s.publishPlacementCandidate(ctx, level, slot, author, lessonID, body); err != nil {
		return nil, err
	}
	return body, nil
}

func (s *Service) checkAndPreparePlacementCandidate(
	ctx context.Context, level string, slot placementSlotSpec,
	raw json.RawMessage, existing []lessoncontract.Activity,
) (json.RawMessage, error) {
	switch slot.kind {
	case "vocabulary", kindGrammarTenseChoice:
		return s.checkPlacementMultipleChoice(ctx, level, slot, raw, existing)
	case kindReadingComprehension:
		return s.checkPlacementReadingComprehension(ctx, level, slot, raw, existing)
	case kindListeningComprehension:
		return s.checkPlacementListeningComprehension(ctx, level, slot, raw, existing)
	case kindWritingPrompt:
		return s.checkWritingPrompt(ctx, raw, existing)
	case kindSpeakingTask:
		return s.checkSpeakingTask(ctx, slot.taskType, raw, existing)
	default:
		return nil, fmt.Errorf("unsupported placement slot kind: %s", slot.kind)
	}
}

func (s *Service) checkPlacementMultipleChoice(
	ctx context.Context, _ string, slot placementSlotSpec,
	raw json.RawMessage, existing []lessoncontract.Activity,
) (json.RawMessage, error) {
	var cand grammarTenseChoiceCand
	if err := json.Unmarshal(raw, &cand); err != nil {
		return nil, fmt.Errorf("check 1 (parse) failed: %w", err)
	}
	if strings.TrimSpace(cand.Prompt) == "" {
		return nil, errors.New("check 1 failed: prompt is empty")
	}
	if cand.Explanation.Vi() == "" {
		return nil, errors.New("check 1 failed: explanation_vi is empty")
	}
	if err := checkOptions(cand.Options, cand.CorrectOptionID); err != nil {
		return nil, fmt.Errorf("check 3 (structure) failed: %w", err)
	}

	// Check 5: Deduplication
	normPrompt := normaliseText(cand.Prompt)
	for _, act := range existing {
		var old grammarTenseChoiceCand
		if err := json.Unmarshal(act.Config, &old); err == nil && old.Prompt != "" {
			if normaliseText(old.Prompt) == normPrompt {
				return nil, errors.New("check 5 (deduplication) failed: prompt matches existing item")
			}
		}
	}

	// Check 6: Redaction
	redacted := contentcontract.RedactForLearner(raw)
	var check map[string]any
	if err := json.Unmarshal(redacted, &check); err == nil {
		if _, leaked := check["correct_option_id"]; leaked {
			return nil, errors.New("check 6 failed: correct_option_id leaked in redacted body")
		}
	}

	// Check 7: Blind solve
	if err := s.blindSolvePlacementMultipleChoice(ctx, slot.kind, raw, cand.CorrectOptionID); err != nil {
		return nil, fmt.Errorf("check 7 (blind solve) failed: %w", err)
	}

	return raw, nil
}

func (s *Service) blindSolvePlacementMultipleChoice(
	ctx context.Context, kind string, raw json.RawMessage, wantOptionID string,
) error {
	redacted := contentcontract.RedactForLearner(raw)
	var reply json.RawMessage
	if err := ai.CompleteJSON(ctx, s.ai, ai.Request{
		Task: ai.TaskPlacementSolve,
		Vars: map[string]any{"Kind": kind, "RedactedBody": string(redacted)},
	}, &reply); err != nil {
		return fmt.Errorf("ai blind solve call failed: %w", err)
	}

	var ans struct {
		SelectedOptionID string `json:"selected_option_id"`
	}
	if err := json.Unmarshal(reply, &ans); err != nil {
		return fmt.Errorf("unmarshal blind solve answer: %w", err)
	}
	if !strings.EqualFold(strings.TrimSpace(ans.SelectedOptionID), strings.TrimSpace(wantOptionID)) {
		return fmt.Errorf("blind solve chose %q, want %q", ans.SelectedOptionID, wantOptionID)
	}
	return nil
}

func (s *Service) checkPlacementReadingComprehension(
	ctx context.Context, _ string, _ placementSlotSpec,
	raw json.RawMessage, existing []lessoncontract.Activity,
) (json.RawMessage, error) {
	var cand readingComprehensionCand
	if err := json.Unmarshal(raw, &cand); err != nil {
		return nil, fmt.Errorf("check 1 (parse) failed: %w", err)
	}
	if strings.TrimSpace(cand.PassageTitle) == "" || strings.TrimSpace(cand.Passage) == "" {
		return nil, errors.New("check 1 failed: passage_title or passage is empty")
	}
	if len(cand.Questions) < 3 {
		return nil, fmt.Errorf("check 3 failed: expected at least 3 questions, got %d", len(cand.Questions))
	}
	for i, q := range cand.Questions {
		if q.Explanation.Vi() == "" {
			return nil, fmt.Errorf("check 1 failed: question %d explanation_vi is empty", i)
		}
		if err := checkOptions(q.Options, q.CorrectOptionID); err != nil {
			return nil, fmt.Errorf("check 3 (structure) failed for question %d: %w", i, err)
		}
	}

	normPassage := normaliseText(cand.Passage)
	for _, act := range existing {
		var old readingComprehensionCand
		if err := json.Unmarshal(act.Config, &old); err == nil && old.Passage != "" {
			if normaliseText(old.Passage) == normPassage {
				return nil, errors.New("check 5 (deduplication) failed: passage matches existing item")
			}
		}
	}

	// Blind solve
	if err := s.blindSolvePlacementQuestions(ctx, kindReadingComprehension, cand.PassageTitle, cand.Passage, cand.Questions); err != nil {
		return nil, fmt.Errorf("check 7 (blind solve) failed: %w", err)
	}

	return raw, nil
}

func (s *Service) checkPlacementListeningComprehension(
	ctx context.Context, _ string, _ placementSlotSpec,
	raw json.RawMessage, existing []lessoncontract.Activity,
) (json.RawMessage, error) {
	var cand listeningCand
	if err := json.Unmarshal(raw, &cand); err != nil {
		return nil, fmt.Errorf("check 1 (parse) failed: %w", err)
	}
	if strings.TrimSpace(cand.Title) == "" || strings.TrimSpace(cand.Script) == "" {
		return nil, errors.New("check 1 failed: title or script is empty")
	}
	if len(cand.Questions) < 3 {
		return nil, fmt.Errorf("check 3 failed: expected at least 3 questions, got %d", len(cand.Questions))
	}
	for i, q := range cand.Questions {
		if q.Explanation.Vi() == "" {
			return nil, fmt.Errorf("check 1 failed: question %d explanation_vi is empty", i)
		}
		if err := checkOptions(q.Options, q.CorrectOptionID); err != nil {
			return nil, fmt.Errorf("check 3 (structure) failed for question %d: %w", i, err)
		}
	}

	normScript := normaliseText(cand.Script)
	for _, act := range existing {
		var old listeningCand
		if err := json.Unmarshal(act.Config, &old); err == nil && old.Script != "" {
			if normaliseText(old.Script) == normScript {
				return nil, errors.New("check 5 (deduplication) failed: script matches existing item")
			}
		}
	}

	// Blind solve
	if err := s.blindSolvePlacementQuestions(ctx, kindListeningComprehension, cand.Title, cand.Script, cand.Questions); err != nil {
		return nil, fmt.Errorf("check 7 (blind solve) failed: %w", err)
	}

	return raw, nil
}

func (s *Service) blindSolvePlacementQuestions(
	ctx context.Context, kind, title, text string, questions []candQuestion,
) error {
	redactedQuestions := make([]map[string]any, 0, len(questions))
	for _, q := range questions {
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
		"title":     title,
		"passage":   text,
		"questions": redactedQuestions,
	})
	if err != nil {
		return err
	}

	var reply json.RawMessage
	if err := ai.CompleteJSON(ctx, s.ai, ai.Request{
		Task: ai.TaskPlacementSolve,
		Vars: map[string]any{"Kind": kind, "RedactedBody": string(redactedBody)},
	}, &reply); err != nil {
		return fmt.Errorf("ai blind solve call failed: %w", err)
	}

	var ans struct {
		Answers map[string]string `json:"answers"`
	}
	if err := json.Unmarshal(reply, &ans); err != nil {
		return fmt.Errorf("parse blind solve response: %w", err)
	}

	for _, q := range questions {
		got, ok := ans.Answers[q.ID]
		if !ok || !strings.EqualFold(strings.TrimSpace(got), strings.TrimSpace(q.CorrectOptionID)) {
			return fmt.Errorf("question %s: blind solve chose %q, want %q", q.ID, got, q.CorrectOptionID)
		}
	}
	return nil
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

// --------------------------------------------------------------------------
// Placement Pool Inspection & Invitation Status
// --------------------------------------------------------------------------

// HasSufficientPlacementPool reports whether every band (A1..C1) has enough items
// to serve complete placement tests (§3.3, §3.5).
func (s *Service) HasSufficientPlacementPool(ctx context.Context) (bool, error) {
	layout, err := s.placementPool(ctx)
	if err != nil {
		return false, fmt.Errorf("load placement pool layout: %w", err)
	}

	// Minimal threshold per slot required to conduct a test without exhausting unseen items.
	// Vocab & Grammar: at least 14 items per level.
	// Reading & Listening: at least 2 items per level.
	for _, level := range placementLevels {
		for _, slot := range placementSlots {
			acts, err := s.placementSlotActivities(ctx, layout, level, slot.slotName)
			if err != nil {
				return false, err
			}
			minRequired := 2
			if slot.kind == "vocabulary" || slot.kind == kindGrammarTenseChoice {
				minRequired = 14
			}
			if len(acts) < minRequired {
				return false, nil
			}
		}
	}
	return true, nil
}

// GetUnseenPlacementItems returns activities in the placement pool for a given level and kind
// that the learner has not yet been exposed to.
func (s *Service) GetUnseenPlacementItems(
	ctx context.Context, userID uuid.UUID, level, slotName string,
) ([]lessoncontract.Activity, error) {
	layout, err := s.placementPool(ctx)
	if err != nil {
		return nil, fmt.Errorf("load placement pool layout: %w", err)
	}

	activities, err := s.placementSlotActivities(ctx, layout, level, slotName)
	if err != nil {
		return nil, err
	}
	if len(activities) == 0 {
		return nil, nil
	}

	ids := idsOf(activities)
	exposures, err := s.repo.ListItemExposures(ctx, userID, ids)
	if err != nil {
		return nil, fmt.Errorf("list item exposures: %w", err)
	}

	unseen := make([]lessoncontract.Activity, 0, len(activities))
	for _, act := range activities {
		if _, served := exposures[act.ID]; !served {
			unseen = append(unseen, act)
		}
	}
	return unseen, nil
}
