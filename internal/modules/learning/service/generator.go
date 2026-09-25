package service

import (
	"context"
	"encoding/json"
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

const (
	purposePractice   = "practice"
	purposeFoundation = "foundation"
	purposeBank       = "bank"
	purposeResource   = "resource"
)

// Generate produces verified, spine-tagged educational content items.
//
// Conforms to Work Order 19 §C:
//  1. Tags content items with resolved taxonomy spine nodes in content.content_tags.
//  2. Authors through EnsureDraft for foundation, bank, and resource purposes;
//     EnsurePublished for practice pool items.
//  3. Runs ItemVerifier with blind solve on for foundation and bank items.
//  4. Injects provenance under _provenance in the body, which is stripped by RedactForLearner.
func (s *Service) Generate(
	ctx context.Context, req learningcontract.GenerateRequest,
) ([]learningcontract.GeneratedItem, error) {
	if err := validateGenerateRequest(&req); err != nil {
		return nil, err
	}

	tagRefs, spineNodeStrings, err := s.resolveSpineNodes(ctx, req)
	if err != nil {
		return nil, err
	}

	authorID := s.resolveAuthorForRequest(ctx, req)
	blindSolve := (req.Purpose == purposeBank || req.Purpose == purposeFoundation)
	// Not preallocated from req.Count: the size would come from a request body.
	var results []learningcontract.GeneratedItem

	for i := 0; i < req.Count; i++ {
		authoredItem, err := s.retryGenerateSingleItem(ctx, req, spineNodeStrings, tagRefs, authorID, blindSolve, i)
		if err != nil {
			return nil, fmt.Errorf("generate item %d of %d failed after %d attempts: %w",
				i+1, req.Count, maxRetriesPerItem+1, err)
		}
		results = append(results, *authoredItem)
	}

	return results, nil
}

// maxGenerateCount is the most items one Generate call may produce; it matches
// GenerateQuestionsRequest.count's maximum in the OpenAPI spec.
const maxGenerateCount = 100

func validateGenerateRequest(req *learningcontract.GenerateRequest) error {
	if strings.TrimSpace(req.Kind) == "" {
		return apperr.New(apperr.Validation, "GENERATOR_KIND_REQUIRED", "Activity kind is required.")
	}
	if strings.TrimSpace(req.CEFRLevel) == "" {
		return apperr.New(apperr.Validation, "GENERATOR_CEFR_REQUIRED", "CEFR level is required.")
	}
	if len(req.NodeCodes) == 0 {
		return apperr.New(
			apperr.Validation,
			"GENERATOR_NODE_CODES_REQUIRED",
			"At least one spine node code is required.",
		)
	}
	if req.Count <= 0 {
		req.Count = 1
	}
	// The count reaches here from an admin request body, and it sizes an
	// allocation and a loop of model calls; the spec's maximum is enforced here
	// because nothing validates request bodies against the spec.
	if req.Count > maxGenerateCount {
		return apperr.New(apperr.Validation, "GENERATOR_COUNT_TOO_LARGE",
			fmt.Sprintf("At most %d items can be generated per request.", maxGenerateCount))
	}
	switch req.Purpose {
	case purposePractice, purposeFoundation, purposeBank, purposeResource:
	default:
		return apperr.New(apperr.Validation, "GENERATOR_PURPOSE_INVALID",
			fmt.Sprintf("Purpose %q is invalid; must be practice, foundation, bank, or resource.", req.Purpose))
	}
	if req.Purpose == purposeResource && (req.OwnerID == nil || *req.OwnerID == uuid.Nil) {
		return apperr.New(apperr.Validation, "GENERATOR_OWNER_REQUIRED", "Resource purpose requires an owner ID.")
	}
	return nil
}

func (s *Service) resolveSpineNodes(
	ctx context.Context, req learningcontract.GenerateRequest,
) ([]contentcontract.TagRef, []string, error) {
	tagRefs := make([]contentcontract.TagRef, 0, len(req.NodeCodes))
	spineNodeStrings := make([]string, 0, len(req.NodeCodes))
	for _, code := range req.NodeCodes {
		trimmed := strings.TrimSpace(code)
		if trimmed == "" {
			continue
		}
		if s.taxonomies != nil {
			node, err := s.taxonomies.GetTaxonomyByCode(ctx, trimmed)
			if err != nil || node == nil {
				return nil, nil, apperr.New(apperr.Validation, "GENERATOR_UNKNOWN_SPINE_CODE",
					fmt.Sprintf("Spine taxonomy node %q is unknown.", trimmed))
			}
			tagRefs = append(tagRefs, contentcontract.TagRef{Namespace: node.Namespace, Code: node.Code})
			spineNodeStrings = append(spineNodeStrings, fmt.Sprintf("%s (%s - %s)", node.Code, node.Label, node.Namespace))
		} else {
			ns := "grammar"
			if strings.HasPrefix(req.Kind, "reading") || strings.HasPrefix(req.Kind, "listening") {
				ns = "skill"
			}
			tagRefs = append(tagRefs, contentcontract.TagRef{Namespace: ns, Code: trimmed})
			spineNodeStrings = append(spineNodeStrings, trimmed)
		}
	}
	return tagRefs, spineNodeStrings, nil
}

func (s *Service) resolveAuthorForRequest(ctx context.Context, req learningcontract.GenerateRequest) uuid.UUID {
	if req.OwnerID != nil && *req.OwnerID != uuid.Nil {
		return *req.OwnerID
	}
	if author := s.resolveGeneratorAuthor(ctx); author != uuid.Nil {
		return author
	}
	if id, err := s.newID(); err == nil {
		return id
	}
	return uuid.New()
}

func (s *Service) retryGenerateSingleItem(
	ctx context.Context,
	req learningcontract.GenerateRequest,
	spineNodeStrings []string,
	tagRefs []contentcontract.TagRef,
	authorID uuid.UUID,
	blindSolve bool,
	itemIndex int,
) (*learningcontract.GeneratedItem, error) {
	var lastErr error
	var retryNote string
	for attempt := 0; attempt <= maxRetriesPerItem; attempt++ {
		item, err := s.generateSingleItem(ctx, req, spineNodeStrings, tagRefs, authorID, blindSolve, itemIndex, retryNote)
		if err == nil {
			return item, nil
		}
		lastErr = err
		// The next attempt is told why the last one was rejected. Without this
		// the model repeated the same defect three times — a duplicate option,
		// an answer in the prompt — and the item was lost.
		retryNote = err.Error()
		slog.WarnContext(ctx, "generator candidate rejected",
			"purpose", req.Purpose, "kind", req.Kind, "level", req.CEFRLevel,
			"attempt", attempt+1, "reason", err)
	}
	return nil, lastErr
}

func buildGenerateVars(req learningcontract.GenerateRequest, spineNodes []string, retryNote string) map[string]any {
	vars := map[string]any{
		varKind:      req.Kind,
		"CEFRLevel":  req.CEFRLevel,
		"SpineNodes": strings.Join(spineNodes, ", "),
	}
	if retryNote != "" {
		vars["RetryNote"] = retryNote
	}
	if req.Purpose == purposeResource && req.SourceText != "" {
		vars["SourceText"] = req.SourceText
	}
	if req.Kind == kindSpeakingTask {
		vars["TaskType"] = subTypeRespond
	}
	if format := examFormat(req.ExamConstraints); format != "" {
		vars["ExamFormat"] = format
	}
	return vars
}

// examFormat renders an exam part's published format for the generation prompt,
// so "TOEIC Part 3" asks for the right shape rather than a generic item
// (WO 22 Stage I.3.1).
func examFormat(c *learningcontract.ExamPartConstraints) string {
	if c == nil {
		return ""
	}
	var lines []string
	if len(c.AllowedTypes) > 0 {
		lines = append(lines, "Allowed question types: "+strings.Join(c.AllowedTypes, ", ")+".")
	}
	if c.QuestionsPerGroup > 0 {
		lines = append(lines, fmt.Sprintf("Questions in this group: exactly %d.", c.QuestionsPerGroup))
	}
	switch {
	case c.OptionCount > 0:
		lines = append(lines, fmt.Sprintf("Options per question: exactly %d.", c.OptionCount))
	case isTaskOnlyPart(c.AllowedTypes):
		// A writing or speaking task has no questions to type short answers to.
	case len(c.AllowedTypes) > 1:
		lines = append(lines,
			`Completion questions have no options: their answer is typed and set in "key". `+
				`True/false/not given questions use the key "True", "False" or "Not Given". `+
				"Other question types carry their own options.")
	default:
		lines = append(lines, `Questions are typed (no options); each answer is a short text set in "key".`)
	}
	if c.MaxWords > 0 {
		lines = append(lines, fmt.Sprintf("No typed answer may exceed %d words.", c.MaxWords))
	}
	if c.MinWords > 0 {
		lines = append(lines, fmt.Sprintf("The response must be at least %d words.", c.MinWords))
	}
	if c.Plays > 0 {
		lines = append(lines, fmt.Sprintf("The recording is played %d time(s).", c.Plays))
	}
	if mix := formatTypeMix(c.TypeMix); mix != "" {
		lines = append(lines, "Question-type mix: "+mix+".")
	}
	return strings.Join(lines, "\n")
}

// isTaskOnlyPart reports a writing or speaking part: its item is one task, not
// questions with answers.
func isTaskOnlyPart(types []string) bool {
	if len(types) == 0 {
		return false
	}
	for _, typ := range types {
		if typ != kindWritingPrompt && typ != kindSpeakingTask {
			return false
		}
	}
	return true
}

// withGroupWordLimit writes an exam part's typed-answer word limit onto a
// passage or recording that has typed questions, so the grader enforces it and
// the sitting shows it ("NO MORE THAN TWO WORDS", D22-25). A body that does
// not parse, or a part with no limit, is left as it is.
func withGroupWordLimit(body json.RawMessage, c *learningcontract.ExamPartConstraints) json.RawMessage {
	if c == nil || c.MaxWords <= 0 {
		return body
	}
	var decoded map[string]any
	if err := json.Unmarshal(body, &decoded); err != nil {
		return body
	}
	if _, ok := decoded["questions"]; !ok {
		return body
	}
	decoded["max_words"] = c.MaxWords
	limited, err := json.Marshal(decoded)
	if err != nil {
		return body
	}
	return limited
}

// formatTypeMix renders a part's question-type mix in a stable order, so the
// generation prompt asks for the published proportion rather than a group of
// whatever type the model prefers (WO 22 Stage I.3.4).
func formatTypeMix(mix map[string]float64) string {
	if len(mix) == 0 {
		return ""
	}
	types := make([]string, 0, len(mix))
	for typ := range mix {
		types = append(types, typ)
	}
	sort.Strings(types)
	parts := make([]string, 0, len(types))
	for _, typ := range types {
		parts = append(parts, fmt.Sprintf("%s %d%%", typ, int(mix[typ]*100+0.5)))
	}
	return strings.Join(parts, ", ")
}

func (s *Service) attachProvenanceAndVerify(
	ctx context.Context,
	req learningcontract.GenerateRequest,
	preparedBody json.RawMessage,
	model string,
	blindSolve bool,
) (json.RawMessage, string, uuid.UUID, error) {
	aiRequestID := uuid.New()
	promptVersion := "item_generate.v1"
	isTopic := (req.Kind == kindFoundationTopic)
	if isTopic {
		promptVersion = "foundation_topic_generate.v1"
	}

	// A prompt or a task has no answer key to blind-solve against: asking the
	// model to solve it and parsing the reply fails, and it failed every
	// generated writing and speaking item (found in the live run).
	hasKey := kindHasAnswerKey(req.Kind)
	blindSolvePayload, err := s.maybeBlindSolve(ctx, req, preparedBody, isTopic, blindSolve && hasKey)
	if err != nil {
		return nil, "", uuid.Nil, fmt.Errorf("blind solve item: %w", err)
	}

	checkCEFR := (req.Purpose == purposeBank || req.Purpose == purposeFoundation) && !isTopic
	judgedCEFR, cefrReasoning, err := s.maybeEvaluateCEFR(ctx, req, preparedBody, checkCEFR)
	if err != nil {
		return nil, "", uuid.Nil, fmt.Errorf("cefr evaluation failed: %w", err)
	}

	bodyWithProv, err := injectProvenance(
		preparedBody, promptVersion, model, aiRequestID,
		req.Purpose, req.Batch, blindSolvePayload, judgedCEFR, cefrReasoning,
	)
	if err != nil {
		bodyWithProv = preparedBody
	}

	vReq := learningcontract.VerifyItemRequest{
		Kind:            req.Kind,
		CEFRLevel:       req.CEFRLevel,
		Body:            bodyWithProv,
		BlindSolve:      blindSolve && !isTopic && hasKey,
		CheckCEFR:       checkCEFR,
		CheckProvenance: true,
		// The exam part's published format, when the caller supplied it: the
		// structural check finally runs (WO 22 Stage I).
		ExamConstraints: req.ExamConstraints,
	}
	if req.Kind == kindSpeakingTask {
		vReq.TaskType = subTypeRespond
	}
	if err := s.VerifyItem(ctx, vReq); err != nil {
		return nil, "", uuid.Nil, fmt.Errorf("verify item: %w", err)
	}

	return bodyWithProv, promptVersion, aiRequestID, nil
}

// maybeBlindSolve solves an item with the key hidden, when the caller asked and
// the kind has a key at all.
func (s *Service) maybeBlindSolve(
	ctx context.Context,
	req learningcontract.GenerateRequest,
	preparedBody json.RawMessage,
	isTopic, shouldSolve bool,
) (json.RawMessage, error) {
	if !shouldSolve || isTopic {
		return nil, nil
	}
	return s.blindSolveItem(ctx, req.Kind, preparedBody)
}

// maybeEvaluateCEFR judges an item's level, when the caller asked and a model is
// configured to ask.
func (s *Service) maybeEvaluateCEFR(
	ctx context.Context,
	req learningcontract.GenerateRequest,
	preparedBody json.RawMessage,
	check bool,
) (string, string, error) {
	if !check || s.ai == nil {
		return "", "", nil
	}
	return s.evaluateCEFR(ctx, req.Kind, req.CEFRLevel, preparedBody)
}

func (s *Service) generateSingleItem(
	ctx context.Context,
	req learningcontract.GenerateRequest,
	spineNodeStrings []string,
	tagRefs []contentcontract.TagRef,
	authorID uuid.UUID,
	blindSolve bool,
	itemIndex int,
	retryNote string,
) (*learningcontract.GeneratedItem, error) {
	vars := buildGenerateVars(req, spineNodeStrings, retryNote)

	var candidateBody json.RawMessage
	task := ai.TaskItemGenerate
	if req.Kind == kindFoundationTopic {
		task = ai.TaskFoundationTopicGenerate
	}
	resp, err := ai.CompleteJSONWithResponse(ctx, s.ai, ai.Request{
		Task: task,
		Vars: vars,
	}, &candidateBody)
	if err != nil {
		return nil, fmt.Errorf("ai generate call failed: %w", err)
	}

	preparedBody, err := s.prepareCandidateBody(ctx, req, candidateBody)
	if err != nil {
		return nil, fmt.Errorf("prepare candidate body: %w", err)
	}
	preparedBody = withGroupWordLimit(preparedBody, req.ExamConstraints)

	model := resp.Model
	if model == "" {
		model = "mock"
	}

	bodyWithProv, promptVersion, aiRequestID, err := s.attachProvenanceAndVerify(
		ctx, req, preparedBody, model, blindSolve,
	)
	if err != nil {
		return nil, err
	}

	slug := req.SlugPrefix
	if slug == "" {
		slugPrefix := fmt.Sprintf("%s-%s-%s",
			strings.ToLower(req.Purpose),
			strings.ToLower(req.CEFRLevel),
			strings.ToLower(strings.ReplaceAll(req.Kind, "_", "-")),
		)
		slug = fmt.Sprintf("%s-%s", slugPrefix, uuid.New().String()[:8])
	} else if itemIndex > 0 {
		slug = fmt.Sprintf("%s-%d", req.SlugPrefix, itemIndex+1)
	}
	slug = strings.ToLower(strings.ReplaceAll(slug, "_", "-"))

	spec := contentcontract.AuthorSpec{
		Slug:      slug,
		Kind:      req.Kind,
		CEFRLevel: req.CEFRLevel,
		Body:      bodyWithProv,
		AuthorID:  authorID,
		Tags:      tagRefs,
	}

	versionID, err := s.authorGeneratedItem(ctx, req, spec, bodyWithProv)
	if err != nil {
		return nil, err
	}

	return &learningcontract.GeneratedItem{
		ContentVersionID: versionID,
		Body:             bodyWithProv,
		PromptVersion:    promptVersion,
		Model:            model,
		AIRequestID:      aiRequestID,
	}, nil
}

// authorGeneratedItem stores a generated item: practice content publishes
// directly, everything else enters as a draft, and foundation and bank drafts
// an independent verifier confirms publish without a person (WO 22 Stage A).
func (s *Service) authorGeneratedItem(
	ctx context.Context,
	req learningcontract.GenerateRequest,
	spec contentcontract.AuthorSpec,
	bodyWithProv json.RawMessage,
) (uuid.UUID, error) {
	var versionID uuid.UUID
	var err error

	if req.Purpose == purposePractice {
		versionID, err = s.contentAuthor.EnsurePublished(ctx, spec)
		if err == nil && s.lessonAuthor != nil {
			if layout, errLayout := s.practicePool(ctx); errLayout == nil {
				if lessonID, ok := layout.lessons[slotKey{level: req.CEFRLevel, slotName: req.Kind}]; ok {
					_, _ = s.lessonAuthor.AppendActivity(ctx, lessonID, lessoncontract.ActivitySpec{
						Kind:             req.Kind,
						ContentVersionID: versionID,
						Config:           bodyWithProv,
						Weight:           1,
					})
				}
			}
		}
	} else {
		versionID, err = s.contentAuthor.EnsureDraft(ctx, spec)
	}
	if err != nil {
		return uuid.Nil, fmt.Errorf("author content: %w", err)
	}

	if s.autoPublish && (req.Purpose == purposeFoundation || req.Purpose == purposeBank) {
		if err := s.publishVerifiedDraft(ctx, versionID, req, bodyWithProv); err != nil {
			return uuid.Nil, fmt.Errorf("auto-publish verified item: %w", err)
		}
	}
	return versionID, nil
}

func (s *Service) prepareCandidateBody(
	ctx context.Context, req learningcontract.GenerateRequest, body json.RawMessage,
) (json.RawMessage, error) {
	switch req.Kind {
	case kindListeningComprehension:
		// An exam part states its questions per recording (VSTEP Part 1 has
		// one per announcement); the default floor of four is for practice.
		minQuestions := 4
		if req.ExamConstraints != nil && req.ExamConstraints.QuestionsPerGroup > 0 {
			minQuestions = req.ExamConstraints.QuestionsPerGroup
		}
		cand, err := parseListeningCandidateWithMin(body, minQuestions)
		if err != nil {
			return nil, err
		}
		if s.synthesiser != nil {
			audioKey, err := s.synthesiser.Synthesise(ctx, cand.Script, cand.Voice)
			if err != nil {
				slog.WarnContext(ctx, "listening item prepared without audio; cmd/tts renders it", "error", err)
			} else {
				cand.AudioObjectKey = audioKey
			}
		}
		return json.Marshal(cand)
	case kindSpeakingTask:
		var cand speakingTaskCand
		if err := json.Unmarshal(body, &cand); err != nil {
			return nil, err
		}
		if cand.SpeakingTimeSeconds <= 0 {
			cand.SpeakingTimeSeconds = defaultSpeakingSeconds
		}
		if cand.TaskType == "" {
			cand.TaskType = subTypeRespond
		}
		return json.Marshal(cand)
	default:
		return body, nil
	}
}

func injectProvenance(
	body json.RawMessage,
	promptVersion, model string,
	aiRequestID uuid.UUID,
	purpose, batch string,
	blindSolveAnswer json.RawMessage,
	cefrEstimate, cefrReasoning string,
) (json.RawMessage, error) {
	var decoded map[string]any
	if err := json.Unmarshal(body, &decoded); err != nil {
		return nil, err
	}
	prov := map[string]any{
		"prompt_version": promptVersion,
		"model":          model,
		"ai_request_id":  aiRequestID.String(),
	}
	if purpose != "" {
		prov["purpose"] = purpose
	}
	if strings.TrimSpace(batch) != "" {
		prov["batch"] = strings.TrimSpace(batch)
	}
	if len(blindSolveAnswer) > 0 {
		var ans any
		if err := json.Unmarshal(blindSolveAnswer, &ans); err == nil {
			prov["blind_solve_answer"] = ans
		} else {
			prov["blind_solve_answer"] = string(blindSolveAnswer)
		}
	}
	if cefrEstimate != "" {
		prov["cefr_estimate"] = cefrEstimate
	}
	if cefrReasoning != "" {
		prov["cefr_reasoning"] = cefrReasoning
	}
	decoded["_provenance"] = prov
	return json.Marshal(decoded)
}

func (s *Service) blindSolveItem(ctx context.Context, kind string, body json.RawMessage) (json.RawMessage, error) {
	if s.ai == nil {
		return nil, nil
	}
	redacted := contentcontract.RedactForLearner(body)
	var reply json.RawMessage
	if err := ai.CompleteJSON(ctx, s.ai, ai.Request{
		Task: ai.TaskItemSolve,
		Vars: map[string]any{varKind: kind, varRedactedBody: string(redacted)},
	}, &reply); err != nil {
		return nil, fmt.Errorf("ai blind solve call failed: %w", err)
	}
	payload, err := parseBlindSolvePayload(kind, reply)
	if err != nil {
		return nil, fmt.Errorf("parse blind solve response: %w", err)
	}
	return payload, nil
}

var _ learningcontract.Generator = (*Service)(nil)
