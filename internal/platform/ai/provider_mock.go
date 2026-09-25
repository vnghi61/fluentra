package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync/atomic"
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

// Model returns the model the mock reports. It is the same constant its replies
// carry, so excluding a stored "mock" provenance also excludes this provider.
func (p *MockProvider) Model() string {
	return MockModelName
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

	if resp, handled, err := p.itemTask(req); handled {
		return resp, err
	}
	return p.reply(req)
}

// reply answers the tasks that are not item generation.
func (p *MockProvider) reply(req Request) (Response, error) {
	switch req.Task {
	case TaskVerifyVocabulary:
		return p.verifyVocabulary(req)
	case TaskEnrichExamples:
		return p.enrichExamples(req)
	case TaskGradeWriting:
		return p.gradeWriting(req)
	case TaskGradeSpeaking:
		return p.gradeSpeaking(req)
	case TaskPracticeGenerate, TaskItemGenerate:
		return distinctMockItem(p.practiceGenerate(req))
	case TaskPracticeSolve, TaskItemSolve:
		return p.practiceSolve(req)
	case TaskListeningGenerate:
		return distinctMockItem(p.listeningGenerate(req))
	case TaskPlacementGenerate:
		return p.placementGenerate(req)
	case TaskPlacementSolve:
		return p.placementSolve(req)
	case TaskChooseVocabularySense:
		return p.chooseSense(req)
	case TaskVocabMeanings:
		return p.vocabMeanings(req)
	default:
		return Response{}, fmt.Errorf("ai: mock provider has no answer for task %q", req.Task)
	}
}

// vocabMeanings answers the word-list build tool with placeholder meanings, so
// the offline stack can exercise the pipeline. The output is deliberately
// marked as the mock provider's and must never be frozen into a fixture.
func (p *MockProvider) vocabMeanings(req Request) (Response, error) {
	type example struct {
		Sentence   string `json:"sentence"`
		SentenceVi string `json:"sentence_vi"`
	}
	type meaning struct {
		Lemma        string    `json:"lemma"`
		POS          string    `json:"pos"`
		CEFRLevel    string    `json:"cefr_level"`
		Definition   string    `json:"definition"`
		DefinitionVI string    `json:"definition_vi"`
		Examples     []example `json:"examples"`
	}

	var out []meaning
	for _, raw := range strings.Split(stringVar(req.Vars, "Lemmas"), "\n") {
		lemma := strings.TrimSpace(raw)
		if lemma == "" {
			continue
		}
		out = append(out, meaning{
			Lemma:        lemma,
			POS:          "noun",
			CEFRLevel:    "A2",
			Definition:   "The mock meaning of " + lemma + ".",
			DefinitionVI: "Nghĩa thử nghiệm của " + lemma,
			Examples: []example{
				{Sentence: "This is an example with " + lemma + ".", SentenceVi: "Đây là ví dụ với " + lemma + "."},
				{Sentence: "We use " + lemma + " every day.", SentenceVi: "Chúng ta dùng " + lemma + " mỗi ngày."},
			},
		})
	}
	encoded, err := json.Marshal(out)
	if err != nil {
		return Response{}, err
	}
	return Response{Text: string(encoded), Model: MockModelName}, nil
}

// chooseSense picks the first stored sense the mock is given, so a development
// stack reuses senses rather than duplicating them. It parses the Senses JSON
// the caller passes; an empty list means the meaning is new.
func (p *MockProvider) chooseSense(req Request) (Response, error) {
	var senses []struct {
		ID string `json:"id"`
	}
	if raw := stringVar(req.Vars, "Senses"); raw != "" {
		if err := json.Unmarshal([]byte(raw), &senses); err != nil {
			return Response{}, fmt.Errorf("ai: mock choose sense: %w", err)
		}
	}
	payload, err := json.Marshal(map[string]any{
		"sense_id": "",
		"is_new":   true,
		"reason":   "",
	})
	if err == nil && len(senses) > 0 {
		payload, err = json.Marshal(map[string]any{
			"sense_id": senses[0].ID,
			"is_new":   false,
			"reason":   "",
		})
	}
	if err != nil {
		return Response{}, fmt.Errorf("ai: encode mock choose sense: %w", err)
	}
	return Response{Text: string(payload), Model: MockModelName}, nil
}

// itemTask answers the item-generation, item-level and item-verify tasks.
//
// Split out of Complete so the one switch there stays under the complexity
// ceiling as tasks accumulate; each of these is one call and no branching.
func (p *MockProvider) itemTask(req Request) (Response, bool, error) {
	switch req.Task {
	case TaskItemLevel:
		resp, err := p.itemLevel(req)
		return resp, true, err
	case TaskItemVerify:
		resp, err := p.itemVerify(req)
		return resp, true, err
	case TaskFoundationTopicGenerate:
		resp, err := p.foundationTopicGenerate(req)
		return resp, true, err
	default:
		return Response{}, false, nil
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
	if format := stringVar(req.Vars, "ExamFormat"); format != "" && mockExamKinds[kind] {
		return p.examGenerate(kind, format)
	}
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
	case "typed_completion":
		payload, err = json.Marshal(map[string]any{
			"prompt":         "Complete the note: the museum opens at ___ every day.",
			"sentence":       "The museum opens at ___ every day.",
			"correct_answer": "9 a.m.",
			"acceptable":     []string{"9am", "nine a.m."},
			"max_words":      2,
			"explanation": map[string]string{
				"explanation_en": "The note states the opening time.",
				"explanation_vi": "Ghi chú nêu giờ mở cửa.",
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
	case "foundation_quiz", "foundation_review", "vocab_multiple_choice":
		payload, err = json.Marshal(map[string]any{
			"prompt": "She has worked at this hospital ___ 2018.",
			"options": []map[string]any{
				{"id": "A", "text": "for"},
				{"id": "B", "text": "since"},
				{"id": "C", "text": "in"},
				{"id": "D", "text": "during"},
			},
			"correct_option_id": "B",
			"explanation": map[string]string{
				"explanation_en": "Use 'since' with a specific point in past time.",
				"explanation_vi": "Dùng 'since' với mốc thời gian trong quá khứ.",
			},
		})
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
	case "reading_comprehension", "listening_comprehension", "text_completion":
		// Every question of a group up to an IELTS passage's fourteen: the
		// mock keys every answer "A".
		answers := make(map[string]string, 20)
		for i := 1; i <= 20; i++ {
			answers[fmt.Sprintf("q%d", i)] = "A"
		}
		payload, err = json.Marshal(map[string]any{"answers": answers})
	case "photo_description", "question_response", "mcq_gap":
		payload, err = json.Marshal(map[string]any{"selected_option_id": "A"})
	case "grammar_tense_choice":
		payload, err = json.Marshal(map[string]any{
			"selected_option_id": "A",
		})
	case "foundation_quiz", "foundation_review", "vocab_multiple_choice":
		payload, err = json.Marshal(map[string]any{
			"selected_option_id": "B",
		})
	case "grammar_sentence_transform":
		payload, err = json.Marshal(map[string]any{
			"answer": "Although he was exhausted, he completed the project.",
		})
	case "typed_completion":
		payload, err = json.Marshal(map[string]any{
			"answer": "9 a.m.",
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

func (p *MockProvider) foundationTopicGenerate(req Request) (Response, error) {
	spineNode := stringVar(req.Vars, "SpineNodes")
	if spineNode == "" {
		spineNode = "Sentence Structure"
	}
	payload, err := json.Marshal(map[string]any{
		"schema_version": 1,
		"objective":      fmt.Sprintf("Understand and master %s", spineNode),
		"explanation": map[string]string{
			"en": fmt.Sprintf("Detailed explanation of %s and its usage rules.", spineNode),
			"vi": fmt.Sprintf("Giải thích chi tiết về %s và các quy tắc sử dụng trong tiếng Anh.", spineNode),
		},
		"examples": []map[string]string{
			{
				"text": "She has lived here for three years.",
				"note": "Indicates an action continuing into the present.",
			},
			{
				"text": "They have finished their assignment.",
				"note": "Indicates completion before the current moment.",
			},
		},
		"related": []string{},
		"common_mistakes": []map[string]string{
			{
				"wrong": "I lived here since 2020.",
				"right": "I have lived here since 2020.",
				"why":   "Use present perfect with 'since' to connect the past to the present.",
			},
		},
	})
	if err != nil {
		return Response{}, fmt.Errorf("ai: encode mock foundation topic: %w", err)
	}
	return Response{Text: string(payload), Model: MockModelName}, nil
}

func (p *MockProvider) listeningGenerate(_ Request) (Response, error) {
	payload, err := json.Marshal(map[string]any{
		"title":  "Flight Announcement at International Airport",
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

// placementMockWords are the word counts the mock writes for a passage and a
// clip's script at each level, inside what the placement pool accepts.
var placementMockWords = map[string][2]int{
	"A1": {70, 55},
	"A2": {90, 65},
	"B1": {115, 85},
	"B2": {135, 100},
	"C1": {160, 115},
}

// placementGenerate writes a distinct item of the shape the placement pool
// checks, at the requested level, so a development top-up fills every slot.
func (p *MockProvider) placementGenerate(req Request) (Response, error) {
	kind := stringVar(req.Vars, "Kind")
	level := stringVar(req.Vars, "CEFRLevel")
	n := placementMockSequence.Add(1)
	words := placementMockWords[level]
	if words[0] == 0 {
		words = placementMockWords["B1"]
	}

	var body map[string]any
	switch kind {
	case "vocabulary", "grammar_tense_choice":
		body = map[string]any{
			"prompt":            fmt.Sprintf("Item %d (%s): choose the word that completes the sentence.", n, level),
			"options":           mockOptions(),
			"correct_option_id": "A",
			"explanation":       mockExplanation(),
		}
	case "reading_comprehension":
		body = map[string]any{
			"passage_title": fmt.Sprintf("Passage %d", n),
			"passage":       mockText(fmt.Sprintf("Passage %d at %s.", n, level), words[0]),
			"questions":     mockQuestions(),
		}
	case "listening_comprehension":
		body = map[string]any{
			"title":     fmt.Sprintf("Clip %d", n),
			"script":    mockText(fmt.Sprintf("Clip %d at %s.", n, level), words[1]),
			"voice":     "en-US-Standard-C",
			"questions": mockQuestions(),
		}
	case "writing_prompt":
		body = map[string]any{
			"prompt": fmt.Sprintf("Task %d (%s): write a short email to a friend about a place you visited "+
				"recently, what you did there and why you would or would not go back.", n, level),
			"model_answer":       mockText("Dear Minh, last month I visited Da Lat with my family.", 90),
			"min_words":          60,
			"time_limit_minutes": 15,
			"explanation":        mockExplanation(),
		}
	case "speaking_task":
		body = map[string]any{
			"task_type": "respond",
			"prompt": fmt.Sprintf("Talk %d (%s): describe a hobby you enjoy, when you started it "+
				"and why you like it.", n, level),
			"speaking_time_seconds": 45,
			"explanation":           mockExplanation(),
		}
	default:
		return Response{}, fmt.Errorf("ai: mock provider placementGenerate unsupported kind %q", kind)
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return Response{}, fmt.Errorf("ai: encode mock placement generate: %w", err)
	}
	return Response{Text: string(payload), Model: MockModelName}, nil
}

// placementMockSequence makes every generated placement item distinct.
var placementMockSequence atomic.Int64

func mockOptions() []map[string]string {
	return []map[string]string{
		{"id": "A", "text": "borrow"},
		{"id": "B", "text": "lend"},
		{"id": "C", "text": "owe"},
		{"id": "D", "text": "spend"},
	}
}

func mockExplanation() map[string]string {
	return map[string]string{
		"explanation_en": "Option A is the only one that fits the sentence.",
		"explanation_vi": "Chỉ phương án A phù hợp với câu.",
	}
}

func mockQuestions() []map[string]any {
	questions := make([]map[string]any, 0, 3)
	for i := 1; i <= 3; i++ {
		questions = append(questions, map[string]any{
			"id":                fmt.Sprintf("q%d", i),
			"type":              "multiple_choice",
			"prompt":            fmt.Sprintf("Question %d about the text?", i),
			"options":           mockOptions(),
			"correct_option_id": "A",
			"explanation":       mockExplanation(),
		})
	}
	return questions
}

// mockText is a lead sentence padded to exactly the given number of words.
func mockText(lead string, words int) string {
	fields := strings.Fields(lead)
	for len(fields) < words {
		fields = append(fields, "word")
	}
	return strings.Join(fields[:words], " ")
}

func (p *MockProvider) placementSolve(req Request) (Response, error) {
	var body map[string]any
	if stringVar(req.Vars, "Kind") == "grammar_tense_choice" {
		body = map[string]any{"selected_option_id": "A"}
	} else {
		body = map[string]any{"answers": map[string]string{"q1": "A", "q2": "A", "q3": "A"}}
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return Response{}, fmt.Errorf("ai: encode mock placement solve: %w", err)
	}
	return Response{Text: string(payload), Model: MockModelName}, nil
}

// itemVerify is the mock's independent-verification answer.
//
// It confirms by default, because the mock is what runs when no real provider
// is configured and a verifier that doubted everything would send the whole
// catalogue to a person on a development stack. Tests force a doubt by putting
// mockVerifyDoubtMarker in the item, the same trick itemLevel uses.
func (p *MockProvider) itemVerify(req Request) (Response, error) {
	body := stringVar(req.Vars, "RedactedBody")
	verdict := "confirmed"
	reason := ""
	if strings.Contains(body, mockVerifyDoubtMarker) {
		verdict = "doubt"
		reason = "mock doubt: second option defensible"
	}
	payload, err := json.Marshal(map[string]any{
		"verdict": verdict,
		"reason":  reason,
	})
	if err != nil {
		return Response{}, fmt.Errorf("ai: encode mock item verify: %w", err)
	}
	return Response{Text: string(payload), Model: "mock-verifier"}, nil
}

// mockVerifyDoubtMarker forces the mock verifier to doubt an item.
const mockVerifyDoubtMarker = "FORCE_VERIFY_DOUBT"

func (p *MockProvider) itemLevel(req Request) (Response, error) {
	requested := stringVar(req.Vars, "RequestedLevel")
	body := stringVar(req.Vars, "RedactedBody")
	level := requested
	if level == "" {
		level = "B1"
	}
	// Support deliberate mislevelling in tests
	if strings.Contains(body, "FORCE_LEVEL_C1") || strings.Contains(body, "C1 passage") {
		level = "C1"
	} else if strings.Contains(body, "FORCE_LEVEL_A1") {
		level = "A1"
	} else if strings.Contains(body, "FORCE_LEVEL_A2") {
		level = "A2"
	}
	payload, err := json.Marshal(map[string]any{
		"cefr_level": level,
		"reasoning":  fmt.Sprintf("Evaluated item grammar and lexical density corresponding to %s descriptors.", level),
		"confidence": 0.95,
	})
	if err != nil {
		return Response{}, fmt.Errorf("ai: encode mock item level: %w", err)
	}
	return Response{Text: string(payload), Model: "mock-level-judge"}, nil
}
