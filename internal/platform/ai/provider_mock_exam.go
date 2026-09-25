package ai

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// The mock's exam items (WO 22 Stage N).
//
// A generation request for an exam part carries the part's published format
// (Vars["ExamFormat"], built by the learning generator from the part's spec).
// The mock answers it with an item of that shape — the question count, the
// options per question, the question-type mix, the word limit — so the whole
// pipeline, generation to verification to a composed fixed test, runs offline
// on a throwaway database. Every answer key is "A", which the mock's solver
// answers, so the blind solve and the independent verifier confirm it.

var (
	formatQuestions = regexp.MustCompile(`Questions in this group: exactly (\d+)`)
	formatOptions   = regexp.MustCompile(`Options per question: exactly (\d+)`)
	formatMinWords  = regexp.MustCompile(`at least (\d+) words`)
	formatMix       = regexp.MustCompile(`Question-type mix: ([^\n]+)\.`)
	formatMixPart   = regexp.MustCompile(`([a-z_]+) (\d+)%`)
)

// mockExamKinds are the kinds the mock writes in an exam part's shape; any
// other kind is answered by the practice mock as before.
var mockExamKinds = map[string]bool{
	"listening_comprehension": true, "reading_comprehension": true, "text_completion": true,
	"photo_description": true, "question_response": true, "mcq_gap": true,
	"writing_prompt": true, "speaking_task": true,
}

// mockExamFormat is what the mock reads from a part's format.
type mockExamFormat struct {
	questions int
	options   int
	typed     bool
	minWords  int
	types     []string
}

func parseMockExamFormat(format string) mockExamFormat {
	f := mockExamFormat{questions: 1, options: 4}
	if m := formatQuestions.FindStringSubmatch(format); m != nil {
		f.questions, _ = strconv.Atoi(m[1])
	}
	if m := formatOptions.FindStringSubmatch(format); m != nil {
		f.options, _ = strconv.Atoi(m[1])
	}
	f.typed = strings.Contains(format, "have no options") || strings.Contains(format, "Questions are typed")
	if m := formatMinWords.FindStringSubmatch(format); m != nil {
		f.minWords, _ = strconv.Atoi(m[1])
	}
	f.types = mockTypeMix(format, f.questions)
	return f
}

// mockTypeMix assigns each question a type in the part's published mix: every
// type in it at least once, the rest by share.
func mockTypeMix(format string, n int) []string {
	m := formatMix.FindStringSubmatch(format)
	if m == nil || n <= 0 {
		return nil
	}
	type share struct {
		name    string
		percent int
	}
	var shares []share
	for _, part := range formatMixPart.FindAllStringSubmatch(m[1], -1) {
		percent, _ := strconv.Atoi(part[2])
		shares = append(shares, share{part[1], percent})
	}
	sort.SliceStable(shares, func(i, j int) bool { return shares[i].percent > shares[j].percent })
	types := make([]string, 0, n)
	for _, s := range shares {
		count := max(1, s.percent*n/100)
		for i := 0; i < count && len(types) < n; i++ {
			types = append(types, s.name)
		}
	}
	for len(types) < n && len(shares) > 0 {
		types = append(types, shares[0].name)
	}
	return types
}

// examGenerate is the mock's answer to a generation request for an exam part.
func (p *MockProvider) examGenerate(kind, format string) (Response, error) {
	f := parseMockExamFormat(format)
	var body map[string]any
	switch kind {
	case "listening_comprehension":
		body = map[string]any{
			"title":     "Exam recording",
			"script":    mockText("A speaker explains the plan for the new library and when it opens.", 80),
			"questions": mockExamQuestions(f),
		}
	case "reading_comprehension", "text_completion":
		body = map[string]any{
			"passage_title": "Exam passage",
			"passage":       mockText("The city library opened a new reading room last spring.", 180),
			"questions":     mockExamQuestions(f),
		}
	case "photo_description":
		body = mockExamChoice("Look at the photograph.", "statements", 4)
		body["image_url"] = "https://upload.wikimedia.org/mock/photo.jpg"
	case "question_response":
		body = mockExamChoice("When does the meeting start?", "responses", 3)
	case "mcq_gap":
		body = mockExamChoice("The report must be finished ___ Friday.", "options", 4)
	case "writing_prompt":
		minWords := max(f.minWords, 120)
		body = map[string]any{
			"prompt":             "Write to a friend about a place you visited and why you would go back.",
			"model_answer":       mockText("Dear Minh, last month I visited Da Lat with my family.", minWords+20),
			"min_words":          minWords,
			"time_limit_minutes": 20,
			"explanation":        mockExplanation(),
		}
	case "speaking_task":
		body = map[string]any{
			"task_type":             "respond",
			"prompt":                "Describe a place in your town you like to visit, what you do there and why you like it.",
			"speaking_time_seconds": 120,
			"explanation":           mockExplanation(),
		}
	default:
		return Response{}, fmt.Errorf("ai: mock has no exam item for kind %q", kind)
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return Response{}, fmt.Errorf("ai: encode mock exam item: %w", err)
	}
	return Response{Text: string(payload), Model: MockModelName}, nil
}

// mockExamChoice is a single choice item with count options under field.
func mockExamChoice(prompt, field string, count int) map[string]any {
	return map[string]any{
		"prompt":            prompt,
		field:               mockLetterOptions(count),
		"correct_option_id": "A",
		"explanation":       mockExplanation(),
	}
}

func mockLetterOptions(count int) []map[string]string {
	options := make([]map[string]string, 0, count)
	for i := 0; i < count; i++ {
		letter := string(rune('A' + i))
		options = append(options, map[string]string{"id": letter, "text": "Choice " + letter})
	}
	return options
}

// mockExamQuestions is a group's questions in the part's shape: a typed
// completion keyed "A" where the part is typed, a choice of the part's option
// count otherwise.
func mockExamQuestions(f mockExamFormat) []map[string]any {
	questions := make([]map[string]any, 0, f.questions)
	for i := 1; i <= f.questions; i++ {
		kind := "multiple_choice"
		if i-1 < len(f.types) {
			kind = f.types[i-1]
		}
		question := map[string]any{
			"id":          fmt.Sprintf("q%d", i),
			"type":        kind,
			"prompt":      fmt.Sprintf("Question %d about the text?", i),
			"explanation": mockExplanation(),
		}
		options := f.options
		if options == 0 {
			options = 4
		}
		if kind == "completion" && f.typed {
			question["key"] = "A"
		} else {
			question["options"] = mockLetterOptions(options)
			question["correct_option_id"] = "A"
		}
		questions = append(questions, question)
	}
	return questions
}
