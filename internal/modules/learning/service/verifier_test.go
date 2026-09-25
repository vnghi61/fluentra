package service_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	learningcontract "github.com/fluentra/fluentra/internal/modules/learning/contract"
	"github.com/fluentra/fluentra/internal/modules/learning/service"
	"github.com/fluentra/fluentra/internal/platform/ai"
	"github.com/fluentra/fluentra/internal/shared/clock"
)

// Question-type names the exam structure tests use.
const (
	typeCompletion     = "completion"
	typeMultipleChoice = "multiple_choice"
	typeTrueFalse      = "true_false_not_given"
)

type customCEFRAI struct {
	judgedLevel string
	reasoning   string
}

func (c *customCEFRAI) Complete(_ context.Context, req ai.Request) (ai.Response, error) {
	if req.Task == ai.TaskItemLevel {
		res := map[string]any{
			"cefr_level": c.judgedLevel,
			"reasoning":  c.reasoning,
			"confidence": 0.95,
		}
		raw, _ := json.Marshal(res)
		return ai.Response{Text: string(raw), Model: "mock-cefr-judge"}, nil
	}
	if req.Task == ai.TaskPracticeSolve || req.Task == ai.TaskItemSolve {
		return ai.Response{Text: `{"selected_option_id": "A", "answers": {"q1": "A", "q2": "A", "q3": "A"}}`}, nil
	}
	return ai.Response{}, nil
}

func TestVerifyItem_WorkOrder19StageEGate_CEFRCheck(t *testing.T) {
	ctx := context.Background()

	// Gate: "A deliberately mislevelled item (a C1 passage requested as A2) is refused by the CEFR check."
	aiClient := &customCEFRAI{
		judgedLevel: "C1",
		reasoning:   "Advanced syntactic structures and sophisticated lexical choices.",
	}

	svc := service.New(service.Deps{
		Lesson:       newFakePoolLessons(),
		LessonAuthor: newFakePoolLessons(),
		Graders:      passingGraders(),
		AI:           aiClient,
		Clock:        clock.NewFake(time.Now()),
	})

	body := json.RawMessage(`{
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
			"explanation_vi": "Hiện tại hoàn thành với she dùng has."
		}
	}`)

	// 1. Deliberately mislevelled: requested A2, judged C1 (>1 band diff) -> MUST FAIL
	err := svc.VerifyItem(ctx, learningcontract.VerifyItemRequest{
		Kind:       kindChoice,
		CEFRLevel:  "A2",
		Body:       body,
		CheckCEFR:  true,
		BlindSolve: false,
	})
	require.Error(t, err, "mislevelled item must be refused")
	assert.Contains(t, err.Error(), "check 7 (cefr) failed")
	assert.Contains(t, err.Error(), "item judged C1, requested A2")

	// 2. Compatible level: requested B2, judged C1 (diff = 1 band) -> PASS
	err = svc.VerifyItem(ctx, learningcontract.VerifyItemRequest{
		Kind:       kindChoice,
		CEFRLevel:  "B2",
		Body:       body,
		CheckCEFR:  true,
		BlindSolve: false,
	})
	require.NoError(t, err, "item within 1 band must pass")

	// 3. Exact level: requested C1, judged C1 -> PASS
	err = svc.VerifyItem(ctx, learningcontract.VerifyItemRequest{
		Kind:       kindChoice,
		CEFRLevel:  "C1",
		Body:       body,
		CheckCEFR:  true,
		BlindSolve: false,
	})
	require.NoError(t, err, "matching level item must pass")
}

func TestVerifyItem_WorkOrder19StageEGate_ExamStructureCheck(t *testing.T) {
	ctx := context.Background()

	svc := service.New(service.Deps{
		Lesson:       newFakePoolLessons(),
		LessonAuthor: newFakePoolLessons(),
		Graders:      passingGraders(),
		Clock:        clock.NewFake(time.Now()),
	})

	// Gate: "A TOEIC Part 3 item with two questions is refused by the structure check."
	twoQuestionsListening := json.RawMessage(`{
		"title": "Office Conversation",
		"script": "Hello John, did you receive the report from the marketing department yesterday?",
		"questions": [
			{
				"id": "q1",
				"type": "multiple_choice",
				"prompt": "What department sent the report?",
				"options": [
					{"id": "A", "text": "Marketing"},
					{"id": "B", "text": "Sales"},
					{"id": "C", "text": "Finance"},
					{"id": "D", "text": "HR"}
				],
				"correct_option_id": "A",
				"explanation": {"explanation_en": "Marketing department.", "explanation_vi": "Phòng marketing."}
			},
			{
				"id": "q2",
				"type": "multiple_choice",
				"prompt": "When was it sent?",
				"options": [
					{"id": "A", "text": "Yesterday"},
					{"id": "B", "text": "Today"},
					{"id": "C", "text": "Last week"},
					{"id": "D", "text": "This morning"}
				],
				"correct_option_id": "A",
				"explanation": {"explanation_en": "Yesterday.", "explanation_vi": "Hôm qua."}
			}
		]
	}`)

	// 1. TOEIC Part 3 expects 3 questions per conversation. With 2 questions -> MUST FAIL
	err := svc.VerifyItem(ctx, learningcontract.VerifyItemRequest{
		Kind:      kindListening,
		CEFRLevel: "B1",
		Body:      twoQuestionsListening,
		ExamConstraints: &learningcontract.ExamPartConstraints{
			QuestionsPerGroup: 3,
			OptionCount:       4,
			AudioRequired:     true,
		},
		BlindSolve: false,
	})
	require.Error(t, err, "TOEIC Part 3 with 2 questions must be refused")
	assert.Contains(t, err.Error(), "check (exam structure) failed")
	assert.Contains(t, err.Error(), "questions per group is 2, want 3")

	// 2. Three questions -> PASS
	threeQuestionsListening := json.RawMessage(`{
		"title": "Office Conversation",
		"script": "Hello John, did you receive the report from the marketing department yesterday? Yes I did.",
		"questions": [
			{
				"id": "q1",
				"type": "multiple_choice",
				"prompt": "What department sent the report?",
				"options": [
					{"id": "A", "text": "Marketing"},
					{"id": "B", "text": "Sales"},
					{"id": "C", "text": "Finance"},
					{"id": "D", "text": "HR"}
				],
				"correct_option_id": "A",
				"explanation": {"explanation_en": "Marketing department.", "explanation_vi": "Phòng marketing."}
			},
			{
				"id": "q2",
				"type": "multiple_choice",
				"prompt": "When was it sent?",
				"options": [
					{"id": "A", "text": "Yesterday"},
					{"id": "B", "text": "Today"},
					{"id": "C", "text": "Last week"},
					{"id": "D", "text": "This morning"}
				],
				"correct_option_id": "A",
				"explanation": {"explanation_en": "Yesterday.", "explanation_vi": "Hôm qua."}
			},
			{
				"id": "q3",
				"type": "multiple_choice",
				"prompt": "Did John receive it?",
				"options": [
					{"id": "A", "text": "Yes"},
					{"id": "B", "text": "No"},
					{"id": "C", "text": "Maybe"},
					{"id": "D", "text": "Not sure"}
				],
				"correct_option_id": "A",
				"explanation": {"explanation_en": "Yes he did.", "explanation_vi": "Vâng anh ấy đã nhận."}
			}
		]
	}`)

	err = svc.VerifyItem(ctx, learningcontract.VerifyItemRequest{
		Kind:      kindListening,
		CEFRLevel: "B1",
		Body:      threeQuestionsListening,
		ExamConstraints: &learningcontract.ExamPartConstraints{
			QuestionsPerGroup: 3,
			OptionCount:       4,
			AudioRequired:     true,
		},
		BlindSolve: false,
	})
	require.NoError(t, err, "TOEIC Part 3 with 3 questions must pass")
}

// TestVerifyItem_ExamStructure_QuestionTypesMustBeAllowed is Stage I: an item
// whose question type the part does not permit is refused.
func TestVerifyItem_ExamStructure_QuestionTypesMustBeAllowed(t *testing.T) {
	ctx := context.Background()

	svc := service.New(service.Deps{
		Lesson:       newFakePoolLessons(),
		LessonAuthor: newFakePoolLessons(),
		Graders:      passingGraders(),
		Clock:        clock.NewFake(time.Now()),
	})

	passage := "The city opened a new library in 2020. It holds ten thousand books and a reading room."
	question := func(id string) string {
		return `{
			"id": "` + id + `",
			"type": "multiple_choice",
			"prompt": "When did the library open?",
			"options": [
				{"id": "A", "text": "2020"},
				{"id": "B", "text": "2019"},
				{"id": "C", "text": "2021"},
				{"id": "D", "text": "2018"}
			],
			"correct_option_id": "A",
			"explanation": {"explanation_en": "It opened in 2020.", "explanation_vi": "Mở năm 2020."}
		}`
	}
	body := json.RawMessage(`{
		"passage": "` + passage + `",
		"questions": [` + question("q1") + `,` + question("q2") + `,` + question("q3") + `,` + question("q4") + `]
	}`)

	// A part that allows only completion and true/false/not given refuses it.
	err := svc.VerifyItem(ctx, learningcontract.VerifyItemRequest{
		Kind:      kindReading,
		CEFRLevel: "B1",
		Body:      body,
		ExamConstraints: &learningcontract.ExamPartConstraints{
			QuestionsPerGroup: 4,
			AllowedTypes:      []string{typeCompletion, typeTrueFalse},
		},
	})
	require.Error(t, err, "a disallowed question type must be refused")
	assert.Contains(t, err.Error(), "is not allowed by this part")

	// Allowing multiple choice lets the same item through.
	err = svc.VerifyItem(ctx, learningcontract.VerifyItemRequest{
		Kind:      kindReading,
		CEFRLevel: "B1",
		Body:      body,
		ExamConstraints: &learningcontract.ExamPartConstraints{
			QuestionsPerGroup: 4,
			AllowedTypes:      []string{typeMultipleChoice},
		},
	})
	require.NoError(t, err, "an allowed question type must pass")
}

// TestVerifyItem_ExamStructure_QuestionTypeMixIsRequired is Stage I.3.4: a group
// that ignores the part's type mix and returns one type is refused.
func TestVerifyItem_ExamStructure_QuestionTypeMixIsRequired(t *testing.T) {
	ctx := context.Background()

	svc := service.New(service.Deps{
		Lesson:       newFakePoolLessons(),
		LessonAuthor: newFakePoolLessons(),
		Graders:      passingGraders(),
		Clock:        clock.NewFake(time.Now()),
	})

	passage := "The city opened a new library in 2020. It holds ten thousand books and a reading room."
	question := func(id string) string {
		return `{
			"id": "` + id + `",
			"type": "multiple_choice",
			"prompt": "When did the library open?",
			"options": [
				{"id": "A", "text": "2020"},
				{"id": "B", "text": "2019"},
				{"id": "C", "text": "2021"},
				{"id": "D", "text": "2018"}
			],
			"correct_option_id": "A",
			"explanation": {"explanation_en": "It opened in 2020.", "explanation_vi": "Mở năm 2020."}
		}`
	}
	body := json.RawMessage(`{
		"passage": "` + passage + `",
		"questions": [` + question("q1") + `,` + question("q2") + `,` + question("q3") + `,` + question("q4") + `]
	}`)

	err := svc.VerifyItem(ctx, learningcontract.VerifyItemRequest{
		Kind:      kindReading,
		CEFRLevel: "B1",
		Body:      body,
		ExamConstraints: &learningcontract.ExamPartConstraints{
			QuestionsPerGroup: 4,
			AllowedTypes:      []string{typeMultipleChoice, typeCompletion},
			TypeMix:           map[string]float64{typeMultipleChoice: 0.5, typeCompletion: 0.5},
		},
	})
	require.Error(t, err, "a group missing half its types must be refused")
	assert.Contains(t, err.Error(), `type "completion" should be about 50%`)

	err = svc.VerifyItem(ctx, learningcontract.VerifyItemRequest{
		Kind:      kindReading,
		CEFRLevel: "B1",
		Body:      body,
		ExamConstraints: &learningcontract.ExamPartConstraints{
			QuestionsPerGroup: 4,
			AllowedTypes:      []string{typeMultipleChoice},
			TypeMix:           map[string]float64{typeMultipleChoice: 1.0},
		},
	})
	require.NoError(t, err, "a group of the only expected type must pass")
}

func TestVerifyItem_ProvenanceCheck(t *testing.T) {
	ctx := context.Background()

	svc := service.New(service.Deps{
		Lesson:       newFakePoolLessons(),
		LessonAuthor: newFakePoolLessons(),
		Graders:      passingGraders(),
		Clock:        clock.NewFake(time.Now()),
	})

	bodyWithoutProv := json.RawMessage(`{
		"prompt": "She ___ lived here for three years.",
		"options": [
			{"id": "A", "text": "has"},
			{"id": "B", "text": "have"},
			{"id": "C", "text": "had"},
			{"id": "D", "text": "having"}
		],
		"correct_option_id": "A",
		"explanation": {"explanation_en": "Present perfect with she uses has.", "explanation_vi": "Hiện tại hoàn thành."}
	}`)

	// CheckProvenance true without _provenance -> MUST FAIL
	err := svc.VerifyItem(ctx, learningcontract.VerifyItemRequest{
		Kind:            "grammar_tense_choice",
		CEFRLevel:       "B1",
		Body:            bodyWithoutProv,
		CheckProvenance: true,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "check 8 (provenance) failed")

	bodyWithProv := json.RawMessage(`{
		"prompt": "She ___ lived here for three years.",
		"options": [
			{"id": "A", "text": "has"},
			{"id": "B", "text": "have"},
			{"id": "C", "text": "had"},
			{"id": "D", "text": "having"}
		],
		"correct_option_id": "A",
		"explanation": {"explanation_en": "Present perfect with she uses has.", "explanation_vi": "Hiện tại hoàn thành."},
		"_provenance": {
			"prompt_version": "item_generate.v1",
			"model": "mock-model",
			"ai_request_id": "11111111-2222-3333-4444-555555555555"
		}
	}`)

	err = svc.VerifyItem(ctx, learningcontract.VerifyItemRequest{
		Kind:            "grammar_tense_choice",
		CEFRLevel:       "B1",
		Body:            bodyWithProv,
		CheckProvenance: true,
	})
	require.NoError(t, err)
}

// TestVerifyItem_ExamStructure_IELTSGroupWithTypedAnswers is D22-25 end to end
// at the verifier: a passage whose questions mix completion, true/false/not
// given and multiple choice passes an IELTS part whose word limit is its
// answers', and a completion key over that limit is refused.
func TestVerifyItem_ExamStructure_IELTSGroupWithTypedAnswers(t *testing.T) {
	ctx := context.Background()

	svc := service.New(service.Deps{
		Lesson:       newFakePoolLessons(),
		LessonAuthor: newFakePoolLessons(),
		Graders:      passingGraders(),
		Clock:        clock.NewFake(time.Now()),
	})

	explanation := `"explanation": {"explanation_en": "See the passage.", "explanation_vi": "Xem đoạn văn."}`
	body := func(completionKey string) json.RawMessage {
		return json.RawMessage(`{
			"passage": "The city opened a new library in 2020. It holds ten thousand books and a reading room.",
			"questions": [
				{"id": "q1", "type": "completion", "prompt": "The library holds ten thousand ___.",
				 "key": "` + completionKey + `", ` + explanation + `},
				{"id": "q2", "type": "completion", "prompt": "It also has a ___ room.",
				 "key": "reading", ` + explanation + `},
				{"id": "q3", "type": "true_false_not_given", "prompt": "The library opened in 2020.",
				 "key": "True", ` + explanation + `},
				{"id": "q4", "type": "multiple_choice", "prompt": "When did the library open?",
				 "options": [{"id": "A", "text": "2020"}, {"id": "B", "text": "2019"},
				             {"id": "C", "text": "2021"}, {"id": "D", "text": "2018"}],
				 "correct_option_id": "A", ` + explanation + `}
			]
		}`)
	}
	constraints := &learningcontract.ExamPartConstraints{
		QuestionsPerGroup: 4,
		MaxWords:          2,
		AllowedTypes:      []string{typeCompletion, typeTrueFalse, typeMultipleChoice},
	}

	err := svc.VerifyItem(ctx, learningcontract.VerifyItemRequest{
		Kind: kindReading, CEFRLevel: "B2", Body: body("books"), ExamConstraints: constraints,
	})
	require.NoError(t, err, "typed and true/false/not given questions are valid in an IELTS group")

	err = svc.VerifyItem(ctx, learningcontract.VerifyItemRequest{
		Kind: kindReading, CEFRLevel: "B2", Body: body("very many printed books"), ExamConstraints: constraints,
	})
	require.Error(t, err, "a completion key over the word limit must be refused")
	assert.Contains(t, err.Error(), "the limit is 2")
}

// TestVerifyItem_TOEICPart2HasThreeResponses: a question-response item is a
// question and three spoken responses, as the published format has it; the
// structure check used to demand four and refused every Part 2 item.
func TestVerifyItem_TOEICPart2HasThreeResponses(t *testing.T) {
	graders := passingGraders()
	require.NoError(t, graders.Register("question_response", &testPracticeGrader{shouldPass: true}))
	svc := service.New(service.Deps{
		Lesson:       newFakePoolLessons(),
		LessonAuthor: newFakePoolLessons(),
		Graders:      graders,
		Clock:        clock.NewFake(time.Now()),
	})
	body := json.RawMessage(`{
		"prompt": "When does the meeting start?",
		"responses": [{"id": "A", "text": "At ten o'clock."}, {"id": "B", "text": "In room four."},
		              {"id": "C", "text": "Yes, I did."}],
		"correct_option_id": "A",
		"explanation": {"explanation_en": "It asks for a time.", "explanation_vi": "Câu hỏi về thời gian."}
	}`)
	err := svc.VerifyItem(context.Background(), learningcontract.VerifyItemRequest{
		Kind: "question_response", CEFRLevel: "B1", Body: body,
		ExamConstraints: &learningcontract.ExamPartConstraints{OptionCount: 3, QuestionsPerGroup: 1},
	})
	require.NoError(t, err, "three responses is the published Part 2 shape")
}
