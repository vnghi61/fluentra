package service_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	learningcontract "github.com/fluentra/fluentra/internal/modules/learning/contract"
	"github.com/fluentra/fluentra/internal/modules/learning/domain"
	"github.com/fluentra/fluentra/internal/modules/learning/service"
	"github.com/fluentra/fluentra/internal/platform/ai"
	"github.com/fluentra/fluentra/internal/shared/clock"
)

// testVerifierModel is the model a fake verifier reports.
const testVerifierModel = "verifier-model"

// independentVerifyConfirmed is the verdict the judge returns when it agrees.
const independentVerifyConfirmed = "confirmed"

// independentVerifyAI answers the two calls an independent verification makes.
type independentVerifyAI struct {
	solveAnswer string
	solveErr    error
	verdict     string
}

func (a *independentVerifyAI) Complete(_ context.Context, req ai.Request) (ai.Response, error) {
	switch req.Task {
	case ai.TaskItemSolve:
		if a.solveErr != nil {
			return ai.Response{}, a.solveErr
		}
		return ai.Response{
			Text:  fmt.Sprintf(`{"selected_option_id": %q}`, a.solveAnswer),
			Model: testVerifierModel,
		}, nil
	case ai.TaskItemVerify:
		return ai.Response{
			Text:  fmt.Sprintf(`{"verdict": %q, "reason": ""}`, a.verdict),
			Model: testVerifierModel,
		}, nil
	default:
		return ai.Response{}, nil
	}
}

func independentVerifyService(model *independentVerifyAI) *service.Service {
	graders := domain.NewGraderRegistry()
	_ = graders.Register(kindChoice, keyGrader{option: "A"})
	return service.New(service.Deps{
		Lesson:       newFakePoolLessons(),
		LessonAuthor: newFakePoolLessons(),
		Graders:      graders,
		AI:           model,
		Clock:        clock.NewFake(time.Now()),
	})
}

const independentVerifyBody = `{
	"prompt": "She ___ lived here for three years.",
	"options": [
		{"id": "A", "text": "has"},
		{"id": "B", "text": "have"},
		{"id": "C", "text": "had"},
		{"id": "D", "text": "having"}
	],
	"correct_option_id": "A",
	"explanation": {
		"explanation_en": "Present perfect with she uses has.",
		"explanation_vi": "Hien tai hoan thanh voi she dung has."
	},
	"_provenance": {
		"prompt_version": "item_generate.v1",
		"model": "writer-model",
		"ai_request_id": "00000000-0000-0000-0000-000000000001"
	}
}`

func TestVerifyIndependently_ConfirmsWhenTheBlindAnswerMatchesAndTheJudgeAgrees(t *testing.T) {
	svc := independentVerifyService(&independentVerifyAI{solveAnswer: "A", verdict: independentVerifyConfirmed})

	verification, err := svc.VerifyIndependently(context.Background(), learningcontract.IndependentVerifyRequest{
		Kind:      kindChoice,
		CEFRLevel: "B1",
		Body:      json.RawMessage(independentVerifyBody),
	})
	require.NoError(t, err)
	assert.True(t, verification.Confirmed)
	assert.Equal(t, "verifier-model", verification.Model)
}

// TestVerifyIndependently_DoubtsWhenTheBlindAnswerDiffersFromTheKey is the WO 22
// Stage A gate: a choice item whose independent blind answer differs from its
// key is never confirmed.
func TestVerifyIndependently_DoubtsWhenTheBlindAnswerDiffersFromTheKey(t *testing.T) {
	svc := independentVerifyService(&independentVerifyAI{solveAnswer: "B", verdict: independentVerifyConfirmed})

	verification, err := svc.VerifyIndependently(context.Background(), learningcontract.IndependentVerifyRequest{
		Kind:      kindChoice,
		CEFRLevel: "B1",
		Body:      json.RawMessage(independentVerifyBody),
	})
	require.NoError(t, err)
	assert.False(t, verification.Confirmed)
	assert.Contains(t, verification.Reason, "does not match the key")
}

func TestVerifyIndependently_DoubtsWhenTheJudgeDoesNotConfirm(t *testing.T) {
	svc := independentVerifyService(&independentVerifyAI{solveAnswer: "A", verdict: "doubt"})

	verification, err := svc.VerifyIndependently(context.Background(), learningcontract.IndependentVerifyRequest{
		Kind:      kindChoice,
		CEFRLevel: "B1",
		Body:      json.RawMessage(independentVerifyBody),
	})
	require.NoError(t, err)
	assert.False(t, verification.Confirmed)
	assert.NotEmpty(t, verification.Reason)
}

// TestVerifyIndependently_EscalatesWithOneProvider. With no independent model to
// ask, the router returns ErrDisabled and the verifier escalates rather than
// confirming (D22-2).
func TestVerifyIndependently_EscalatesWithOneProvider(t *testing.T) {
	svc := independentVerifyService(&independentVerifyAI{solveErr: ai.ErrDisabled})

	verification, err := svc.VerifyIndependently(context.Background(), learningcontract.IndependentVerifyRequest{
		Kind:      kindChoice,
		CEFRLevel: "B1",
		Body:      json.RawMessage(independentVerifyBody),
	})
	require.NoError(t, err)
	assert.False(t, verification.Confirmed)
	assert.NotEmpty(t, verification.Reason)
}

// TestVerifyIndependently_TopicHasNoBlindSolve. A foundation topic has no answer
// key, so only the judgment call runs (D22-13).
func TestVerifyIndependently_TopicHasNoBlindSolve(t *testing.T) {
	svc := independentVerifyService(&independentVerifyAI{verdict: independentVerifyConfirmed})

	body := json.RawMessage(`{
		"schema_version": 1,
		"objective": "Use the present perfect.",
		"explanation": {"en": "Present perfect links past to present.", "vi": "..."},
		"examples": [{"text": "She has lived here.", "note": ""}],
		"_provenance": {
			"prompt_version": "foundation_topic_generate.v1",
			"model": "writer-model",
			"ai_request_id": "00000000-0000-0000-0000-000000000001"
		}
	}`)

	verification, err := svc.VerifyIndependently(context.Background(), learningcontract.IndependentVerifyRequest{
		Kind:      learningcontract.KindFoundationTopic,
		CEFRLevel: "B1",
		Body:      body,
	})
	require.NoError(t, err)
	assert.True(t, verification.Confirmed)
}
