package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/google/uuid"

	contentcontract "github.com/fluentra/fluentra/internal/modules/content/contract"
	learningcontract "github.com/fluentra/fluentra/internal/modules/learning/contract"
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
	results := make([]learningcontract.GeneratedItem, 0, req.Count)

	for i := 0; i < req.Count; i++ {
		authoredItem, err := s.retryGenerateSingleItem(ctx, req, spineNodeStrings, tagRefs, authorID, blindSolve)
		if err != nil {
			return nil, fmt.Errorf("generate item %d of %d failed after %d attempts: %w",
				i+1, req.Count, maxRetriesPerItem+1, err)
		}
		results = append(results, *authoredItem)
	}

	return results, nil
}

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
) (*learningcontract.GeneratedItem, error) {
	var lastErr error
	for attempt := 0; attempt <= maxRetriesPerItem; attempt++ {
		item, err := s.generateSingleItem(ctx, req, spineNodeStrings, tagRefs, authorID, blindSolve)
		if err == nil {
			return item, nil
		}
		lastErr = err
		slog.WarnContext(ctx, "generator candidate rejected",
			"purpose", req.Purpose, "kind", req.Kind, "level", req.CEFRLevel,
			"attempt", attempt+1, "reason", err)
	}
	return nil, lastErr
}

func (s *Service) generateSingleItem(
	ctx context.Context,
	req learningcontract.GenerateRequest,
	spineNodeStrings []string,
	tagRefs []contentcontract.TagRef,
	authorID uuid.UUID,
	blindSolve bool,
) (*learningcontract.GeneratedItem, error) {
	vars := map[string]any{
		"Kind":       req.Kind,
		"CEFRLevel":  req.CEFRLevel,
		"SpineNodes": strings.Join(spineNodeStrings, ", "),
	}
	if req.Purpose == purposeResource && req.SourceText != "" {
		vars["SourceText"] = req.SourceText
	}
	if req.Kind == kindSpeakingTask {
		vars["TaskType"] = subTypeRespond
	}

	var candidateBody json.RawMessage
	resp, err := ai.CompleteJSONWithResponse(ctx, s.ai, ai.Request{
		Task: ai.TaskItemGenerate,
		Vars: vars,
	}, &candidateBody)
	if err != nil {
		return nil, fmt.Errorf("ai generate call failed: %w", err)
	}

	preparedBody, err := s.prepareCandidateBody(ctx, req.Kind, candidateBody)
	if err != nil {
		return nil, fmt.Errorf("prepare candidate body: %w", err)
	}

	vReq := learningcontract.VerifyItemRequest{
		Kind:       req.Kind,
		CEFRLevel:  req.CEFRLevel,
		Body:       preparedBody,
		BlindSolve: blindSolve,
	}
	if req.Kind == kindSpeakingTask {
		vReq.TaskType = subTypeRespond
	}
	if err := s.VerifyItem(ctx, vReq); err != nil {
		return nil, fmt.Errorf("verify item: %w", err)
	}

	aiRequestID := uuid.New()
	promptVersion := "item_generate.v1"
	model := resp.Model
	if model == "" {
		model = "mock"
	}

	bodyWithProv, err := injectProvenance(preparedBody, promptVersion, model, aiRequestID)
	if err != nil {
		bodyWithProv = preparedBody
	}

	slugPrefix := fmt.Sprintf("%s-%s-%s",
		strings.ToLower(req.Purpose),
		strings.ToLower(req.CEFRLevel),
		strings.ToLower(strings.ReplaceAll(req.Kind, "_", "-")),
	)
	slug := fmt.Sprintf("%s-%s", slugPrefix, uuid.New().String()[:8])

	spec := contentcontract.AuthorSpec{
		Slug:      slug,
		Kind:      req.Kind,
		CEFRLevel: req.CEFRLevel,
		Body:      bodyWithProv,
		AuthorID:  authorID,
		Tags:      tagRefs,
	}

	var versionID uuid.UUID
	if req.Purpose == purposePractice {
		versionID, err = s.contentAuthor.EnsurePublished(ctx, spec)
	} else {
		versionID, err = s.contentAuthor.EnsureDraft(ctx, spec)
	}
	if err != nil {
		return nil, fmt.Errorf("author content: %w", err)
	}

	return &learningcontract.GeneratedItem{
		ContentVersionID: versionID,
		Body:             bodyWithProv,
		PromptVersion:    promptVersion,
		Model:            model,
		AIRequestID:      aiRequestID,
	}, nil
}

func (s *Service) prepareCandidateBody(
	ctx context.Context, kind string, body json.RawMessage,
) (json.RawMessage, error) {
	switch kind {
	case kindListeningComprehension:
		cand, err := parseListeningCandidate(body)
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
	body json.RawMessage, promptVersion, model string, aiRequestID uuid.UUID,
) (json.RawMessage, error) {
	var decoded map[string]any
	if err := json.Unmarshal(body, &decoded); err != nil {
		return nil, err
	}
	decoded["_provenance"] = map[string]any{
		"prompt_version": promptVersion,
		"model":          model,
		"ai_request_id":  aiRequestID.String(),
	}
	return json.Marshal(decoded)
}

var _ learningcontract.Generator = (*Service)(nil)
