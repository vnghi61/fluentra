package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/uuid"

	contentcontract "github.com/fluentra/fluentra/internal/modules/content/contract"
	learningcontract "github.com/fluentra/fluentra/internal/modules/learning/contract"
	"github.com/fluentra/fluentra/internal/platform/ai"
)

// VerifyIndependently implements learning.IndependentVerifier (WO 22 Stage A).
//
// Two calls, both excluded from the writer's model:
//
//  1. a blind solve of the redacted item, graded by the item's own grader, which
//     catches a wrong answer key; and
//  2. a judgment of the key and explanation against the checklist.
//
// Confirmed only when the blind answer matches the key and the judge confirms.
// Every other outcome — no independent provider, a parse failure, a timeout, an
// "unsure" — is a doubt with a reason, never an error.
func (s *Service) VerifyIndependently(
	ctx context.Context, req learningcontract.IndependentVerifyRequest,
) (contentcontract.Verification, error) {
	now := s.clock.Now().UTC()
	doubt := func(reason string) contentcontract.Verification {
		return contentcontract.Verification{Confirmed: false, Reason: reason, CheckedAt: now}
	}

	if s.ai == nil {
		return doubt("AI is not configured, so no independent model can check this item"), nil
	}
	if strings.TrimSpace(req.Kind) == "" {
		return doubt("the item has no kind"), nil
	}
	writerModel := provenanceModel(req.Body)
	if writerModel == "" {
		return doubt("the item has no _provenance.model, so no independent model can be chosen"), nil
	}

	if err := s.independentBlindSolve(ctx, req.Kind, req.Body, writerModel); err != nil {
		return doubt(err.Error()), nil
	}

	verdict, model, err := s.judgeItem(ctx, req, writerModel)
	if err != nil {
		return doubt(fmt.Sprintf("independent verification call failed: %v", err)), nil
	}
	if verdict != "confirmed" {
		reason := "the independent verifier did not confirm the item"
		if verdict != "" {
			reason = fmt.Sprintf("the independent verifier answered %q", verdict)
		}
		return doubt(reason), nil
	}
	return contentcontract.Verification{Confirmed: true, Model: model, CheckedAt: now}, nil
}

// independentBlindSolve solves the redacted item with a model other than the
// writer's and grades that answer against the stored key.
func (s *Service) independentBlindSolve(
	ctx context.Context, kind string, body json.RawMessage, excludeModel string,
) error {
	// A topic or a prompt has no key to solve against; its verification is the
	// judgment call alone (D22-13).
	if !kindHasAnswerKey(kind) {
		return nil
	}
	grader, ok := s.graders.Get(kind)
	if !ok || grader == nil {
		return fmt.Errorf("no grader is registered for kind %s", kind)
	}

	redacted := contentcontract.RedactForLearner(body)
	var reply json.RawMessage
	if err := ai.CompleteJSON(ctx, s.ai, ai.Request{
		Task:         ai.TaskItemSolve,
		Vars:         map[string]any{varKind: kind, varRedactedBody: string(redacted)},
		ExcludeModel: excludeModel,
	}, &reply); err != nil {
		return fmt.Errorf("independent blind solve failed: %w", err)
	}
	payload, err := parseBlindSolvePayload(kind, reply)
	if err != nil {
		return fmt.Errorf("parse independent blind solve: %w", err)
	}

	versionID := uuid.New()
	gradeCtx := contentcontract.ContextWithTempVersion(ctx, &contentcontract.Version{
		ID: versionID, Kind: kind, Body: body, Status: statusPublished,
	})
	if err := gradesFullMarks(gradeCtx, grader, versionID, payload); err != nil {
		return fmt.Errorf("the independent blind answer does not match the key: %w", err)
	}
	return nil
}

// judgeItem asks an independent model to judge the key and explanation.
func (s *Service) judgeItem(
	ctx context.Context, req learningcontract.IndependentVerifyRequest, excludeModel string,
) (verdict, model string, err error) {
	redacted := contentcontract.RedactForLearner(req.Body)
	var reply struct {
		Verdict string `json:"verdict"`
		Reason  string `json:"reason"`
	}
	resp, err := ai.CompleteJSONWithResponse(ctx, s.ai, ai.Request{
		Task: ai.TaskItemVerify,
		Vars: map[string]any{
			varKind:        req.Kind,
			"TargetLevel":  req.CEFRLevel,
			"RedactedBody": string(redacted),
			"AnswerKey":    string(req.Body),
			"OfficialSpec": req.OfficialSpec,
		},
		ExcludeModel: excludeModel,
	}, &reply)
	if err != nil {
		return "", "", err
	}
	model = strings.TrimSpace(resp.Model)
	if model == "" {
		model = "unknown"
	}
	return strings.ToLower(strings.TrimSpace(reply.Verdict)), model, nil
}

// kindHasAnswerKey reports whether an item kind carries a key the blind solve
// can be graded against.
func kindHasAnswerKey(kind string) bool {
	switch kind {
	case kindFoundationTopic, learningcontract.KindLessonMaterial, kindWritingPrompt, kindSpeakingTask:
		return false
	default:
		return true
	}
}

// provenanceModel reads the model that wrote the item from its provenance.
func provenanceModel(body json.RawMessage) string {
	var decoded struct {
		Provenance struct {
			Model string `json:"model"`
		} `json:"_provenance"`
	}
	if err := json.Unmarshal(body, &decoded); err != nil {
		return ""
	}
	return strings.TrimSpace(decoded.Provenance.Model)
}

var _ learningcontract.IndependentVerifier = (*Service)(nil)
