package service_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/fluentra/fluentra/internal/modules/learning/domain"
	learningcontract "github.com/fluentra/fluentra/internal/modules/learning/contract"
	"github.com/fluentra/fluentra/internal/modules/learning/service"
	"github.com/fluentra/fluentra/internal/shared/clock"
)

type mockGrader struct {
	pass bool
}

func (g *mockGrader) Grade(_ context.Context, _ learningcontract.GradeRequest) (learningcontract.GradeResult, error) {
	if g.pass {
		return learningcontract.GradeResult{Score: 100, Correct: true}, nil
	}
	return learningcontract.GradeResult{Score: 0, Correct: false}, nil
}

func newVerifierTestService(t *testing.T, graderPass bool) *service.Service {
	t.Helper()
	graders := domain.NewGraderRegistry()
	_ = graders.Register("grammar_tense_choice", &mockGrader{pass: graderPass})
	_ = graders.Register("vocab_multiple_choice", &mockGrader{pass: graderPass})
	_ = graders.Register("reading_comprehension", &mockGrader{pass: graderPass})

	clk := clock.NewFake(time.Date(2026, 9, 19, 12, 0, 0, 0, time.UTC))
	return service.New(service.Deps{
		Graders:           graders,
		Clock:             clk,
		GeneratorAuthorID: uuid.New(),
	})
}

func TestVerifyItem_EmptyKindOrBody(t *testing.T) {
	t.Parallel()
	svc := newVerifierTestService(t, true)

	err := svc.VerifyItem(context.Background(), learningcontract.VerifyItemRequest{
		Kind: "",
		Body: json.RawMessage(`{}`),
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "check 1")

	err = svc.VerifyItem(context.Background(), learningcontract.VerifyItemRequest{
		Kind: "grammar_tense_choice",
		Body: nil,
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "check 1")
}

func TestVerifyItem_GrammarTenseChoice_Pass(t *testing.T) {
	t.Parallel()
	svc := newVerifierTestService(t, true)

	body := json.RawMessage(`{
		"prompt": "She ___ to school every day.",
		"options": [
			{"id": "opt1", "text": "goes"},
			{"id": "opt2", "text": "go"},
			{"id": "opt3", "text": "went"},
			{"id": "opt4", "text": "gone"}
		],
		"correct_option_id": "opt1",
		"explanation": {"explanation_vi": "Hiện tại đơn diễn tả thói quen lặp đi lặp lại"}
	}`)

	err := svc.VerifyItem(context.Background(), learningcontract.VerifyItemRequest{
		Kind:       "grammar_tense_choice",
		CEFRLevel:  "A2",
		Body:       body,
		BlindSolve: false,
	})
	require.NoError(t, err)
}

func TestVerifyItem_GrammarTenseChoice_Check2_GraderFails(t *testing.T) {
	t.Parallel()
	svc := newVerifierTestService(t, false) // Grader fails

	body := json.RawMessage(`{
		"prompt": "She ___ to school every day.",
		"options": [
			{"id": "opt1", "text": "goes"},
			{"id": "opt2", "text": "go"},
			{"id": "opt3", "text": "went"},
			{"id": "opt4", "text": "gone"}
		],
		"correct_option_id": "opt1",
		"explanation": {"explanation_vi": "Hiện tại đơn diễn tả thói quen"}
	}`)

	err := svc.VerifyItem(context.Background(), learningcontract.VerifyItemRequest{
		Kind:       "grammar_tense_choice",
		CEFRLevel:  "A2",
		Body:       body,
		BlindSolve: false,
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "check 2 (own answer scores full marks) failed")
}

func TestVerifyItem_GrammarTenseChoice_Check5_Duplicate(t *testing.T) {
	t.Parallel()
	svc := newVerifierTestService(t, true)

	body := json.RawMessage(`{
		"prompt": "She ___ to school every day.",
		"options": [
			{"id": "opt1", "text": "goes"},
			{"id": "opt2", "text": "go"},
			{"id": "opt3", "text": "went"},
			{"id": "opt4", "text": "gone"}
		],
		"correct_option_id": "opt1",
		"explanation": {"explanation_vi": "Hiện tại đơn diễn tả thói quen"}
	}`)

	existing := []json.RawMessage{body}

	err := svc.VerifyItem(context.Background(), learningcontract.VerifyItemRequest{
		Kind:       "grammar_tense_choice",
		CEFRLevel:  "A2",
		Body:       body,
		Existing:   existing,
		BlindSolve: false,
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "check 5 (deduplication) failed")
}

func TestVerifyItem_WritingPrompt_Pass(t *testing.T) {
	t.Parallel()
	svc := newVerifierTestService(t, true)

	body := json.RawMessage(`{
		"prompt": "Describe your favourite hobby and explain why you enjoy doing it in your free time with friends.",
		"model_answer": "My favourite hobby is playing chess with my close friends on weekends. Chess requires deep concentration, creativity, and strategic thinking which helps me develop my problem solving skills in daily situations. Whenever I sit down to play a match, I feel completely engaged in the tactical intricacies of the game. It is a wonderful way to challenge my intellect while having meaningful conversations with companions. Furthermore, chess teaches me patience, resilience, and the humility to accept defeats gracefully. Overall, chess is not only an entertaining game but also a wonderful mental exercise that significantly enriches my personal life and friendships."
	}`)

	err := svc.VerifyItem(context.Background(), learningcontract.VerifyItemRequest{
		Kind:       "writing_prompt",
		CEFRLevel:  "B1",
		Body:       body,
		BlindSolve: false,
	})
	require.NoError(t, err)
}

func TestVerifyItem_SpeakingTask_Pass(t *testing.T) {
	t.Parallel()
	svc := newVerifierTestService(t, true)

	body := json.RawMessage(`{
		"task_type": "read_aloud",
		"prompt": "Please read the following sentence clearly and naturally.",
		"reference_text": "Learning a new language opens up exciting opportunities to travel and connect with people worldwide.",
		"speaking_time_seconds": 30
	}`)

	err := svc.VerifyItem(context.Background(), learningcontract.VerifyItemRequest{
		Kind:       "speaking_task",
		TaskType:   "read_aloud",
		CEFRLevel:  "A2",
		Body:       body,
		BlindSolve: false,
	})
	require.NoError(t, err)
}

func TestVerifyItem_Vocabulary_Pass(t *testing.T) {
	t.Parallel()
	svc := newVerifierTestService(t, true)

	body := json.RawMessage(`{
		"prompt": "Choose the word that means 'to make better':",
		"options": [
			{"id": "1", "text": "improve"},
			{"id": "2", "text": "damage"},
			{"id": "3", "text": "reduce"},
			{"id": "4", "text": "ignore"}
		],
		"correct_option_id": "1"
	}`)

	err := svc.VerifyItem(context.Background(), learningcontract.VerifyItemRequest{
		Kind:       "vocab_multiple_choice",
		CEFRLevel:  "A2",
		Body:       body,
		BlindSolve: false,
	})
	require.NoError(t, err)
}
