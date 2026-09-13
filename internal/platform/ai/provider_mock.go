package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// MockProvider answers without a network call.
//
// It is the default, and that is deliberate. `make dev` has to produce a working
// stack for someone who has not signed up to anything, and a verification job
// that fails on every batch because no model is configured looks like a broken
// feature rather than an unconfigured one. The mock returns an answer of the
// right shape, so the pipeline around it — the job, the deck, the review cards,
// the XP — is exercised end to end on a laptop with no API key and no internet.
//
// It is not a model. It accepts any word it is given, which is exactly what a
// real verification must not do, so nothing that matters may rely on it. The
// summary a learner sees names the model that answered for this reason.
type MockProvider struct {
	registry *Registry
}

// NewMockProvider builds the offline provider.
func NewMockProvider(registry *Registry) *MockProvider {
	return &MockProvider{registry: registry}
}

// Name returns the provider identifier.
func (p *MockProvider) Name() string {
	return ProviderMock
}

// MockModelName is what a mocked answer reports as its model, so a stored
// verification can be told apart from a real one later.
const MockModelName = "mock"

// Complete implements Client interface.
func (p *MockProvider) Complete(_ context.Context, req Request) (Response, error) {
	// The template is still rendered and its errors still surface: a broken
	// prompt should fail in development, where the mock is what runs, rather
	// than first in production against a real provider.
	if p.registry != nil {
		tmpl, err := p.registry.Get(req.Task)
		if err != nil {
			return Response{}, err
		}
		if _, err := tmpl.Render(req.Vars); err != nil {
			return Response{}, err
		}
	}

	switch req.Task {
	case TaskVerifyVocabulary:
		return p.verifyVocabulary(req)
	case TaskEnrichExamples:
		return p.enrichExamples(req)
	case TaskGradeWriting:
		return p.gradeWriting(req)
	case TaskGradeSpeaking:
		return p.gradeSpeaking(req)
	case TaskPracticeGenerate:
		return p.practiceGenerate(req)
	case TaskPracticeSolve:
		return p.practiceSolve(req)
	case TaskListeningGenerate:
		return p.listeningGenerate(req)
	default:
		return Response{}, fmt.Errorf("ai: mock provider has no answer for task %q", req.Task)
	}
}

func (p *MockProvider) gradeWriting(req Request) (Response, error) {
	submission := strings.TrimSpace(stringVar(req.Vars, "Submission"))
	score := 85
	correct := true
	feedback := "Good writing response with clear vocabulary."
	feedbackVi := "Bài viết tốt với vốn từ vựng rõ ràng."
	if len(strings.Fields(submission)) < 3 {
		score = 30
		correct = false
		feedback = "The submission is too short to evaluate properly."
		feedbackVi = "Bài viết quá ngắn để đánh giá chi tiết."
	}
	payload, err := json.Marshal(map[string]any{
		"overall_band": 6.5,
		"score":        score,
		"correct":      correct,
		"feedback":     feedback,
		"feedback_en":  feedback,
		"feedback_vi":  feedbackVi,
		"criteria": []map[string]any{
			mockCriterion("task_response", 7.0, "Good response to the prompt.", "Phản hồi tốt yêu cầu đề bài."),
			mockCriterion("coherence_cohesion", 6.5, "Clear progression of ideas.", "Ý tứ phát triển rõ ràng."),
			mockCriterion("lexical_resource", 6.5, "Varied vocabulary used appropriately.", "Sử dụng từ vựng đa dạng, phù hợp."),
			mockCriterion("grammatical_range", 6.0, "Good range of grammatical structures.", "Cấu trúc ngữ pháp tương đối tốt."),
		},
		"annotations": []map[string]any{},
	})
	if err != nil {
		return Response{}, fmt.Errorf("ai: encode mock writing grade: %w", err)
	}
	return Response{Text: string(payload), Model: MockModelName}, nil
}

func (p *MockProvider) gradeSpeaking(req Request) (Response, error) {
	transcript := strings.TrimSpace(stringVar(req.Vars, "Transcript"))
	score := 80
	correct := true
	feedback := "Good spoken response with clear communication."
	feedbackVi := "Bài nói tốt với khả năng giao tiếp rõ ràng."
	if len(strings.Fields(transcript)) < 2 {
		score = 25
		correct = false
		feedback = "The spoken response is too brief."
		feedbackVi = "Bài nói quá ngắn."
	}
	payload, err := json.Marshal(map[string]any{
		"overall_band": 6.5,
		"score":        score,
		"correct":      correct,
		"feedback":     feedback,
		"feedback_en":  feedback,
		"feedback_vi":  feedbackVi,
		"criteria": []map[string]any{
			mockCriterion("task_response", 6.5, "Addressed the task reasonably well.", "Đáp ứng khá tốt yêu cầu bài nói."),
			mockCriterion("fluency_coherence", 6.5, "Good flow of speech.", "Độ trôi chảy tốt."),
			mockCriterion("lexical_resource", 6.5, "Appropriate vocabulary.", "Từ vựng phù hợp."),
			mockCriterion("grammatical_range", 6.0, "Generally accurate grammar.", "Ngữ pháp nhìn chung chính xác."),
		},
	})
	if err != nil {
		return Response{}, fmt.Errorf("ai: encode mock speaking grade: %w", err)
	}
	return Response{Text: string(payload), Model: MockModelName}, nil
}

func (p *MockProvider) enrichExamples(req Request) (Response, error) {
	term := strings.TrimSpace(stringVar(req.Vars, "Term"))
	if term == "" {
		return Response{}, fmt.Errorf("ai: mock enrichment needs a term")
	}
	count := intVar(req.Vars, "Count", 5, maxMockExamples)
	examples := make([]map[string]string, 0, count)
	for i := 1; i <= count; i++ {
		examples = append(examples, map[string]string{
			"sentence":    fmt.Sprintf("Enriched example %d for %q.", i, term),
			"sentence_vi": fmt.Sprintf("Ví dụ mở rộng %d cho %q.", i, term),
		})
	}
	payload, err := json.Marshal(map[string]any{
		"examples": examples,
	})
	if err != nil {
		return Response{}, fmt.Errorf("ai: encode mock answer: %w", err)
	}
	return Response{Text: string(payload), Model: MockModelName}, nil
}

func (p *MockProvider) verifyVocabulary(req Request) (Response, error) {
	term := strings.TrimSpace(stringVar(req.Vars, "Term"))
	if term == "" {
		return Response{}, fmt.Errorf("ai: mock verification needs a term")
	}

	definition := stringVar(req.Vars, "DictionaryDefinition")
	if definition == "" {
		definition = fmt.Sprintf("A placeholder definition of %q, written by the mock provider.", term)
	}
	partOfSpeech := stringVar(req.Vars, "PartOfSpeech")
	if partOfSpeech == "" {
		partOfSpeech = "noun"
	}

	count := intVar(req.Vars, "ExampleCount", 5, maxMockExamples)
	// Deliberately flat and repetitive. A mocked sentence that reads like a
	// real one is a mocked sentence that ships.
	examples := make([]string, 0, count)
	for i := 1; i <= count; i++ {
		examples = append(examples,
			fmt.Sprintf("Example %d for %q, generated offline by the mock provider.", i, term))
	}

	payload, err := json.Marshal(map[string]any{
		"valid":           true,
		"reason":          "",
		"lemma":           strings.ToLower(term),
		"part_of_speech":  partOfSpeech,
		"cefr_level":      "B1",
		"definition":      definition,
		"meaning_matches": true,
		"examples":        examples,
	})
	if err != nil {
		return Response{}, fmt.Errorf("ai: encode mock answer: %w", err)
	}
	return Response{Text: string(payload), Model: MockModelName}, nil
}

func stringVar(vars map[string]any, key string) string {
	if value, ok := vars[key].(string); ok {
		return value
	}
	return ""
}

// maxMockExamples bounds every count the mock will honour.
//
// The real caller asks for at most `examplesPerSweep`, and the vocabulary
// domain caps a sense at fifteen sentences. This ceiling is deliberately well
// above both: it is not a business rule, it is the point past which a number
// arriving in `Vars` stops being a request and starts being an allocation.
const maxMockExamples = 50

// intVar reads a positive int out of the prompt variables.
//
// `ceiling` is a required argument rather than a default because the value it
// bounds reaches `make(..., count)` directly. `Vars` is an untyped map filled
// by callers, so a count of a billion is a memory-exhaustion vector rather than
// a typo — and this is the mock, which means it runs wherever no real provider
// is configured, including production before the AI slots are filled in.
func intVar(vars map[string]any, key string, fallback, ceiling int) int {
	value, ok := vars[key].(int)
	if !ok || value <= 0 {
		return fallback
	}
	if value > ceiling {
		return ceiling
	}
	return value
}

var _ Client = (*MockProvider)(nil)

// mockCriterion is one rubric criterion in the mock's writing grade.
func mockCriterion(name string, band float64, commentEn, commentVi string) map[string]any {
	return map[string]any{"name": name, "band": band, "comment_en": commentEn, "comment_vi": commentVi}
}

func (p *MockProvider) practiceGenerate(req Request) (Response, error) {
	kind := stringVar(req.Vars, "Kind")
	taskType := stringVar(req.Vars, "TaskType")
	var payload []byte
	var err error

	switch kind {
	case "reading_comprehension":
		payload, err = json.Marshal(map[string]any{
			"passage_title": "The Community Garden Project",
			"passage":       "Last year, the neighborhood association established a community garden in the city center. Residents of all ages volunteered their weekends to plant vegetables, build flower beds, and install irrigation systems. The initiative not only increased local green space but also strengthened interpersonal bonds among neighbors.",
			"questions": []map[string]any{
				{
					"id":     "q1",
					"type":   "multiple_choice",
					"prompt": "What was established in the city center?",
					"options": []map[string]any{
						{"id": "A", "text": "A community garden"},
						{"id": "B", "text": "A shopping center"},
						{"id": "C", "text": "A sports stadium"},
						{"id": "D", "text": "A bus terminal"},
					},
					"correct_option_id": "A",
					"explanation": map[string]string{
						"explanation_en": "The passage states that a community garden was established in the city center.",
						"explanation_vi": "Đoạn văn nêu rõ khu vườn cộng đồng được xây dựng tại trung tâm thành phố.",
					},
				},
				{
					"id":     "q2",
					"type":   "multiple_choice",
					"prompt": "Who volunteered on weekends?",
					"options": []map[string]any{
						{"id": "A", "text": "Residents of all ages"},
						{"id": "B", "text": "Only university students"},
						{"id": "C", "text": "City officials only"},
						{"id": "D", "text": "Hired contractors"},
					},
					"correct_option_id": "A",
					"explanation": map[string]string{
						"explanation_en": "The passage mentions that residents of all ages volunteered.",
						"explanation_vi": "Đoạn văn đề cập người dân ở mọi lứa tuổi đã tình nguyện tham gia.",
					},
				},
				{
					"id":     "q3",
					"type":   "multiple_choice",
					"prompt": "What was one outcome of the initiative?",
					"options": []map[string]any{
						{"id": "A", "text": "It strengthened bonds among neighbors"},
						{"id": "B", "text": "It caused high city taxes"},
						{"id": "C", "text": "It created severe traffic congestion"},
						{"id": "D", "text": "It reduced nearby green areas"},
					},
					"correct_option_id": "A",
					"explanation": map[string]string{
						"explanation_en": "The initiative strengthened interpersonal bonds among neighbors.",
						"explanation_vi": "Sáng kiến này đã thắt chặt tình cảm giữa các cư dân láng giềng.",
					},
				},
				{
					"id":     "q4",
					"type":   "multiple_choice",
					"prompt": "When was the project started?",
					"options": []map[string]any{
						{"id": "A", "text": "Last year"},
						{"id": "B", "text": "Five years ago"},
						{"id": "C", "text": "Last month"},
						{"id": "D", "text": "Yesterday"},
					},
					"correct_option_id": "A",
					"explanation": map[string]string{
						"explanation_en": "The text begins with 'Last year, the neighborhood association...'",
						"explanation_vi": "Bài viết bắt đầu với 'Vào năm ngoái, hiệp hội khu phố...'",
					},
				},
			},
		})
	case "grammar_tense_choice":
		payload, err = json.Marshal(map[string]any{
			"prompt": "By the time the train arrived, they ___ for over an hour.",
			"options": []map[string]any{
				{"id": "A", "text": "had been waiting"},
				{"id": "B", "text": "are waiting"},
				{"id": "C", "text": "will wait"},
				{"id": "D", "text": "waits"},
			},
			"correct_option_id": "A",
			"explanation": map[string]string{
				"explanation_en": "Past perfect continuous is used for an action continuing up to another past event.",
				"explanation_vi": "Quá khứ hoàn thành tiếp diễn dùng để chỉ hành động kéo dài đến một thời điểm trong quá khứ.",
			},
		})
	case "grammar_sentence_transform":
		payload, err = json.Marshal(map[string]any{
			"prompt":         "Rewrite the sentence using 'Although': He was exhausted, but he completed the project.",
			"correct_answer": "Although he was exhausted, he completed the project.",
			"acceptable":     []string{"Although he was exhausted, he completed his project."},
			"explanation": map[string]string{
				"explanation_en": "Begin with 'Although' and remove 'but'.",
				"explanation_vi": "Bắt đầu với 'Although' và bỏ liên từ 'but'.",
			},
		})
	case "writing_prompt":
		payload, err = json.Marshal(map[string]any{
			"prompt":             "Some people believe that public transportation should be completely free for all citizens. Do you agree or disagree? Give reasons and examples.",
			"model_answer":       "The proposition that public transit should be provided at no cost has become a focal point of urban development debates. In my perspective, making public transportation free yields considerable societal advantages. Primarily, zero-fare systems incentivize commuters to leave private automobiles at home, directly curbing traffic gridlock and harmful vehicle emissions. Furthermore, free transit functions as an equalizer, ensuring low-income households maintain reliable access to educational and employment hubs. In conclusion, offering free public transit represents an effective investment in environmental health and social equity.",
			"min_words":          150,
			"time_limit_minutes": 20,
			"topic":              "Urban Planning & Environment",
			"explanation": map[string]string{
				"explanation_en": "A clear position supported by environmental and socioeconomic benefits.",
				"explanation_vi": "Bài viết có quan điểm rõ ràng được hỗ trợ bởi các lý lẽ về môi trường và kinh tế xã hội.",
			},
		})
	case "speaking_task":
		if taskType == "read_aloud" {
			payload, err = json.Marshal(map[string]any{
				"task_type":             "read_aloud",
				"prompt":                "Read the following text aloud clearly and naturally.",
				"reference_text":        "Good afternoon and welcome to the annual technology conference. Please ensure all portable electronic devices are set to silent mode during keynotes. The schedule is available at the registration desk.",
				"speaking_time_seconds": 45,
				"explanation": map[string]string{
					"explanation_en": "Maintain consistent pace and clear articulation of consonant clusters.",
					"explanation_vi": "Giữ tốc độ ổn định và phát âm rõ ràng các cụm phụ âm.",
				},
			})
		} else {
			payload, err = json.Marshal(map[string]any{
				"task_type":             "respond",
				"prompt":                "Describe an accomplishment you are proud of. Explain why it was meaningful and what you learned.",
				"speaking_time_seconds": 45,
				"explanation": map[string]string{
					"explanation_en": "Provide context, detail the action taken, and reflect on the outcome.",
					"explanation_vi": "Cung cấp bối cảnh, chi tiết hành động và bài học rút ra.",
				},
			})
		}
	default:
		return Response{}, fmt.Errorf("ai: mock practiceGenerate has no mock for kind %q", kind)
	}

	if err != nil {
		return Response{}, fmt.Errorf("ai: encode mock practice item: %w", err)
	}
	return Response{Text: string(payload), Model: MockModelName}, nil
}

func (p *MockProvider) practiceSolve(req Request) (Response, error) {
	kind := stringVar(req.Vars, "Kind")
	var payload []byte
	var err error

	switch kind {
	case "reading_comprehension", "listening_comprehension":
		payload, err = json.Marshal(map[string]any{
			"answers": map[string]string{
				"q1": "A",
				"q2": "A",
				"q3": "A",
				"q4": "A",
				"q5": "A",
			},
		})
	case "grammar_tense_choice":
		payload, err = json.Marshal(map[string]any{
			"selected_option_id": "A",
		})
	case "grammar_sentence_transform":
		payload, err = json.Marshal(map[string]any{
			"answer": "Although he was exhausted, he completed the project.",
		})
	default:
		payload, err = json.Marshal(map[string]any{
			"answers": map[string]string{"q1": "A"},
		})
	}

	if err != nil {
		return Response{}, fmt.Errorf("ai: encode mock solve answer: %w", err)
	}
	return Response{Text: string(payload), Model: MockModelName}, nil
}

func (p *MockProvider) listeningGenerate(_ Request) (Response, error) {
	payload, err := json.Marshal(map[string]any{
		"title": "Flight Announcement at International Airport",
		"script": "Attention all passengers on flight 402 to Tokyo. Due to adverse weather conditions along the flight route, boarding will be delayed by approximately forty-five minutes. Please remain near gate 14 for further announcements.",
		"voice":  "en-US-Standard-C",
		"questions": []map[string]any{
			{
				"id":     "q1",
				"type":   "multiple_choice",
				"prompt": "What is the destination of flight 402?",
				"options": []map[string]any{
					{"id": "A", "text": "Tokyo"},
					{"id": "B", "text": "Seoul"},
					{"id": "C", "text": "London"},
					{"id": "D", "text": "Singapore"},
				},
				"correct_option_id": "A",
				"explanation": map[string]string{
					"explanation_en": "The announcement specifies flight 402 to Tokyo.",
					"explanation_vi": "Thông báo nêu rõ chuyến bay 402 tới Tokyo.",
				},
			},
			{
				"id":     "q2",
				"type":   "multiple_choice",
				"prompt": "Why is the flight delayed?",
				"options": []map[string]any{
					{"id": "A", "text": "Adverse weather conditions"},
					{"id": "B", "text": "Mechanical repairs"},
					{"id": "C", "text": "Crew unavailability"},
					{"id": "D", "text": "Air traffic control strike"},
				},
				"correct_option_id": "A",
				"explanation": map[string]string{
					"explanation_en": "The speaker mentions adverse weather conditions along the route.",
					"explanation_vi": "Người nói nhắc đến điều kiện thời tiết xấu dọc đường bay.",
				},
			},
			{
				"id":     "q3",
				"type":   "multiple_choice",
				"prompt": "How long is the expected delay?",
				"options": []map[string]any{
					{"id": "A", "text": "About 45 minutes"},
					{"id": "B", "text": "Two hours"},
					{"id": "C", "text": "Fifteen minutes"},
					{"id": "D", "text": "Overnight"},
				},
				"correct_option_id": "A",
				"explanation": map[string]string{
					"explanation_en": "The announcement states boarding will be delayed by approximately forty-five minutes.",
					"explanation_vi": "Thông báo nêu rõ hoãn khoảng 45 phút.",
				},
			},
			{
				"id":     "q4",
				"type":   "multiple_choice",
				"prompt": "Where should passengers remain?",
				"options": []map[string]any{
					{"id": "A", "text": "Near gate 14"},
					{"id": "B", "text": "At baggage claim"},
					{"id": "C", "text": "In the dining lounge"},
					{"id": "D", "text": "Outside security"},
				},
				"correct_option_id": "A",
				"explanation": map[string]string{
					"explanation_en": "Passengers are asked to remain near gate 14.",
					"explanation_vi": "Hành khách được yêu cầu ở gần cổng 14.",
				},
			},
		},
	})
	if err != nil {
		return Response{}, fmt.Errorf("ai: encode mock listening generation: %w", err)
	}
	return Response{Text: string(payload), Model: MockModelName}, nil
}

