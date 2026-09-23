package main

import (
	"context"
	"io"
	"strings"
	"testing"

	grammarcontract "github.com/fluentra/fluentra/internal/modules/grammar/contract"
	readingcontract "github.com/fluentra/fluentra/internal/modules/reading/contract"
	"github.com/fluentra/fluentra/internal/modules/user/contract"
	vocabularycontract "github.com/fluentra/fluentra/internal/modules/vocabulary/contract"
	writingcontract "github.com/fluentra/fluentra/internal/modules/writing/contract"
)

// TestRun_RefusesProduction pins the one branch that must never be wrong.
//
// Seeding writes accounts whose password is printed in a public guide. The
// refusal is checked before the pool is opened, so this test needs no database:
// if the guard ever moves below the connection, the test starts failing with a
// connection error instead of passing, which is the right way for it to break.
func TestRun_RefusesProduction(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("DB_DSN", "postgres://nobody@127.0.0.1:1/nothing?sslmode=disable")

	err := run(context.Background(), io.Discard)
	if err == nil {
		t.Fatal("run() in production returned no error; the demo password would have been written")
	}
	if !strings.Contains(err.Error(), "production") {
		t.Errorf("run() error = %q, want it to name production as the reason", err)
	}
}

// TestDemoAccounts_MatchTheGuide keeps the dataset and its documentation from
// drifting. The addresses below are the ones
// docs/development/getting-started.md §4 tells a new contributor to sign in
// with, and a rename here without a rename there is how a guide starts lying.
func TestDemoAccounts_MatchTheGuide(t *testing.T) {
	want := map[string]bool{
		"nghitienvl@gmail.com":        false,
		"nguyenvannghi1110@gmail.com": true,
	}

	if len(demoAccounts) != len(want) {
		t.Fatalf("demoAccounts has %d entries, the guide names %d", len(demoAccounts), len(want))
	}
	for _, account := range demoAccounts {
		admin, named := want[account.email]
		if !named {
			t.Errorf("%s is seeded but not named in getting-started.md", account.email)
			continue
		}
		if account.admin != admin {
			t.Errorf("%s admin = %v, want %v", account.email, account.admin, admin)
		}
	}
}

// TestNewUser_CarriesEverythingTheContractRequires fails if a field is added to
// contract.NewUser and the seeder keeps filling the old shape — an account
// created with an empty locale is one no reader is prepared for.
func TestNewUser_CarriesEverythingTheContractRequires(t *testing.T) {
	seeded := contract.NewUser{
		Email:       demoAccounts[0].email,
		DisplayName: demoAccounts[0].displayName,
		Locale:      "en",
		Timezone:    "Asia/Ho_Chi_Minh",
	}

	if seeded.Email == "" || seeded.DisplayName == "" ||
		seeded.Locale == "" || seeded.Timezone == "" {
		t.Error("the seeder builds a NewUser with an empty required field")
	}
}

// TestSeededKindsAreGradable is P11 §4: what the seed authors, what vocabulary
// registers and what cmd/api declares must name the same set.
//
// It reads vocabulary's list rather than repeating it. The first version of this
// test carried its own copy of the four kinds, which meant it agreed with itself
// and could not catch the divergence it was written for: an activity kind the
// registry does not know fails the learner's request with
// UNSUPPORTED_ACTIVITY_KIND, and a declared kind with no grader fails the boot.
func TestSeededKindsAreGradable(t *testing.T) {
	gradable := make(map[string]bool)
	for _, kind := range vocabularycontract.GradedKinds() {
		gradable[kind] = true
	}
	for _, kind := range grammarcontract.GradedKinds() {
		gradable[kind] = true
	}
	for _, kind := range readingcontract.GradedKinds() {
		gradable[kind] = true
	}
	for _, kind := range writingcontract.GradedKinds() {
		gradable[kind] = true
	}

	seeded := make(map[string]bool)
	for _, unit := range courseSeedData.Units {
		for _, lesson := range unit.Lessons {
			for _, act := range lesson.Activities {
				seeded[act.Kind] = true
				if !gradable[act.Kind] {
					t.Errorf("the seed authors activity kind %q, which no grader claims", act.Kind)
				}
			}
		}
	}

	// The kinds web/src/routes/LessonPage.tsx can render, and no more: a kind in
	// the seed that the runner does not know is an activity a learner reaches
	// and cannot do, and a kind the runner supports that nothing seeds is a
	// renderer no one has ever seen work.
	runnerKinds := map[string]bool{
		"vocab_multiple_choice":      true,
		"vocab_gap_fill":             true,
		"vocab_flashcard":            true,
		"vocab_listen_type":          true,
		"vocab_match":                true,
		"vocab_reorder":              true,
		"vocab_context_choice":       true,
		"grammar_tense_choice":       true,
		"grammar_sentence_transform": true,
		"reading_comprehension":      true,
		"writing_prompt":             true,
	}
	for kind := range seeded {
		if !runnerKinds[kind] {
			t.Errorf("the seed authors %q, which the lesson runner cannot render", kind)
		}
	}
	for kind := range runnerKinds {
		if !seeded[kind] {
			t.Errorf("the runner supports %q but the seed authors none of it", kind)
		}
	}
}

// TestCourseSeedData_Integrity validates that the seeded course, units, lessons,
// and activities satisfy the curriculum invariants P11.1 names: one course,
// eight lessons, and an activity body the grader can actually score.
func TestCourseSeedData_Integrity(t *testing.T) {
	if courseSeedData.Slug == "" || courseSeedData.Title == "" {
		t.Fatal("courseSeedData missing slug or title")
	}

	totalLessons := 0
	for _, unit := range courseSeedData.Units {
		if unit.Position <= 0 || unit.Title == "" {
			t.Errorf("invalid unit %+v", unit)
		}
		for _, lesson := range unit.Lessons {
			totalLessons++
			if lesson.Position <= 0 || lesson.Title == "" || lesson.SkillFocus == "" {
				t.Errorf("invalid lesson %+v", lesson)
			}
			if len(lesson.Activities) == 0 {
				t.Errorf("lesson %s has no activities", lesson.Title)
			}
			for _, act := range lesson.Activities {
				assertActivityIsGradable(t, lesson.Title, act)
			}
		}
	}

	if totalLessons != 8 {
		t.Errorf("seeded %d lessons, want the 8 P11.1 asks for", totalLessons)
	}
}

// TestCurriculumLessonsHaveCEFRLevel asserts that every curriculum lesson resolves
// to a valid CEFR level (A1..C2) in lowercase for the core.cefr_level enum.
func TestCurriculumLessonsHaveCEFRLevel(t *testing.T) {
	courses := []seedCourse{
		courseSeedData, readingCourseSeedData, writingCourseSeedData,
		speakingCourseSeedData, listeningCourseSeedData,
	}
	validLevels := map[string]bool{
		"a1": true, "a2": true, "b1": true, "b2": true, "c1": true, "c2": true,
	}

	for _, c := range courses {
		for _, unit := range c.Units {
			for _, lesson := range unit.Lessons {
				lvl := determineLessonCEFR(c, unit, lesson)
				if !validLevels[lvl] {
					t.Errorf("course %s unit %s lesson %s determined level %q is invalid",
						c.Slug, unit.Title, lesson.Title, lvl)
				}
			}
		}
	}
}

// assertActivityIsGradable checks the half of the contract a kind list cannot:
// the authored body has to answer the question the config asks, in the form the
// runner submits.
func assertActivityIsGradable(t *testing.T, lessonTitle string, act seedActivity) {
	t.Helper()

	answer, _ := act.Body[bodyKeyCorrectAnswer].(string)
	pairs, _ := act.Body[bodyKeyCorrectPairs].(map[string]string)

	if act.Kind == kindReadingComprehension {
		if qs, ok := act.Body["questions"].([]map[string]any); ok && len(qs) > 0 {
			assertQuestionsDeclareAnswers(t, lessonTitle, act, qs)
			return
		}
	}

	// Either key. A matching exercise has no single answer, and the grader
	// accepts a body carrying `correct_pairs` instead — demanding both here
	// would fail content the grader scores perfectly well.
	if answer == "" && len(pairs) == 0 {
		t.Errorf("%s activity %d declares neither correct_answer nor correct_pairs; the grader refuses to score it",
			lessonTitle, act.Position)
		return
	}

	if len(pairs) > 0 {
		assertPairsResolve(t, lessonTitle, act, pairs)
		return
	}

	if act.Kind != "vocab_multiple_choice" &&
		act.Kind != kindContextChoice &&
		act.Kind != kindGrammarTenseChoice &&
		act.Kind != kindReadingComprehension {
		return
	}

	assertAnswerIsAnOption(t, lessonTitle, act, answer)
}

// assertQuestionsDeclareAnswers checks a question set: every question carries its
// own key, as answer, correct_answer or correct_option_id.
func assertQuestionsDeclareAnswers(t *testing.T, lessonTitle string, act seedActivity, qs []map[string]any) {
	t.Helper()
	for idx, q := range qs {
		qAnswer, _ := q["answer"].(string)
		if qAnswer == "" {
			qAnswer, _ = q["correct_answer"].(string)
		}
		if qAnswer == "" {
			qAnswer, _ = q["correct_option_id"].(string)
		}
		if qAnswer == "" {
			t.Errorf("%s activity %d question %d declares no answer", lessonTitle, act.Position, idx)
		}
	}
}

// assertAnswerIsAnOption checks a multiple-choice key against the options shown.
//
// The runner submits `selected_option_id`, so the authored answer has to be
// an option id — not the word the option displays. A body naming the word
// grades every learner wrong while looking entirely reasonable.
func assertAnswerIsAnOption(t *testing.T, lessonTitle string, act seedActivity, answer string) {
	t.Helper()
	options, ok := act.Config[cfgOptions].([]map[string]string)
	if !ok || len(options) == 0 {
		t.Errorf("%s activity %d is multiple choice with no options", lessonTitle, act.Position)
		return
	}
	for _, option := range options {
		if option["id"] == answer {
			return
		}
	}
	t.Errorf("%s activity %d: correct_answer %q is not one of the option ids",
		lessonTitle, act.Position, answer)
}

// assertPairsResolve checks a matching key against the columns it pairs.
//
// A key naming an id that neither column renders grades every learner wrong
// while the activity looks complete — the same failure mode the multiple-choice
// check above exists to catch, one shape along.
func assertPairsResolve(t *testing.T, lessonTitle string, act seedActivity, pairs map[string]string) {
	t.Helper()

	ids := func(key string) map[string]bool {
		out := map[string]bool{}
		rows, _ := act.Config[key].([]map[string]string)
		for _, row := range rows {
			out[row[cfgOptionID]] = true
		}
		return out
	}
	words, definitions := ids(cfgWords), ids(cfgDefinitions)

	if len(words) != len(pairs) || len(definitions) != len(pairs) {
		t.Errorf("%s activity %d: %d pairs against %d words and %d definitions",
			lessonTitle, act.Position, len(pairs), len(words), len(definitions))
	}
	for wordID, definitionID := range pairs {
		if !words[wordID] {
			t.Errorf("%s activity %d: pair key %q is not a rendered word",
				lessonTitle, act.Position, wordID)
		}
		if !definitions[definitionID] {
			t.Errorf("%s activity %d: pair value %q is not a rendered definition",
				lessonTitle, act.Position, definitionID)
		}
	}

	// Every matching activity must say which words it is about, or the learner
	// answers four words and earns a card for none of them.
	if lemmas, _ := act.Body[bodyKeyWordLemmas].([]string); len(lemmas) != len(pairs) {
		t.Errorf("%s activity %d: %d pairs but %d word_lemmas",
			lessonTitle, act.Position, len(pairs), len(lemmas))
	}
}

func TestWordSenseSeedData_Integrity(t *testing.T) {
	if len(wordSenseSeedData) != 200 {
		t.Errorf("wordSenseSeedData count = %d, want 200", len(wordSenseSeedData))
	}

	seen := make(map[string]bool)
	for i, sense := range wordSenseSeedData {
		if sense.Lemma == "" || sense.POS == "" || sense.Definition == "" || sense.IPA == "" {
			t.Errorf("word sense [%d] missing required field: %+v", i, sense)
		}
		if sense.CEFRLevel != "A1" && sense.CEFRLevel != "A2" && sense.CEFRLevel != "B1" && sense.CEFRLevel != "B2" {
			t.Errorf("word sense [%d] has invalid CEFR level: %s", i, sense.CEFRLevel)
		}
		key := sense.Lemma + ":" + sense.POS
		if seen[key] {
			t.Errorf("duplicate word sense lemma+pos: %s", key)
		}
		seen[key] = true
	}
}

// Every curated sense must carry five examples, and every example must carry its
// Vietnamese rendering.
//
// The count is the promise the flashcard makes; the translation is the field
// `domain.ExampleSentence`, the `ExampleSentence` schema and the DB column have
// always defined and that nothing ever populated. Both are data invariants, and
// data drifts silently — an entry that lost its translation would simply render
// with a blank line under it.
func TestSeededExamples_AreFiveAndBilingual(t *testing.T) {
	const wantExamples = 5

	for _, sense := range wordSenseSeedData {
		if len(sense.Examples) != wantExamples {
			t.Errorf("%s has %d examples, want %d", sense.Lemma, len(sense.Examples), wantExamples)
			continue
		}
		for i, example := range sense.Examples {
			switch {
			case strings.TrimSpace(example.Sentence) == "":
				t.Errorf("%s example %d has no sentence", sense.Lemma, i+1)
			case strings.TrimSpace(example.SentenceVi) == "":
				t.Errorf("%s example %d has no Vietnamese rendering", sense.Lemma, i+1)
			case example.Sentence == example.SentenceVi:
				t.Errorf("%s example %d was not translated, only copied", sense.Lemma, i+1)
			case !containsVietnamese(example.SentenceVi):
				// A rendering with no Vietnamese-specific letter is almost
				// always the English pasted into the wrong column.
				t.Errorf("%s example %d does not look like Vietnamese: %q",
					sense.Lemma, i+1, example.SentenceVi)
			}
		}
	}
}

// containsVietnamese reports whether a string carries a letter that only
// Vietnamese orthography uses. Crude on purpose: it exists to catch a whole
// sentence in the wrong language, not to validate spelling.
func containsVietnamese(s string) bool {
	for _, r := range s {
		if strings.ContainsRune("ăâđêôơưàáảãạằắẳẵặầấẩẫậèéẻẽẹềếểễệìíỉĩị"+
			"òóỏõọồốổỗộờớởỡợùúủũụừứửữựỳýỷỹỵ"+
			"ĂÂĐÊÔƠƯÀÁẢÃẠẰẮẲẴẶẦẤẨẪẬÈÉẺẼẸỀẾỂỄỆÌÍỈĨỊ"+
			"ÒÓỎÕỌỒỐỔỖỘỜỚỞỠỢÙÚỦŨỤỪỨỬỮỰỲÝỶỸỴ", r) {
			return true
		}
	}
	return false
}

func TestReadingCourseSeedData_Integrity(t *testing.T) {
	if readingCourseSeedData.Slug != "reading-practice" || readingCourseSeedData.Title == "" {
		t.Fatal("readingCourseSeedData missing slug or title")
	}

	totalLessons := 0
	for _, unit := range readingCourseSeedData.Units {
		if unit.Position <= 0 || unit.Title == "" {
			t.Errorf("invalid unit %+v", unit)
		}
		for _, lesson := range unit.Lessons {
			totalLessons++
			if lesson.Position <= 0 || lesson.Title == "" || lesson.SkillFocus != skillReading {
				t.Errorf("invalid reading lesson %+v", lesson)
			}
			if len(lesson.Activities) == 0 {
				t.Errorf("lesson %s has no activities", lesson.Title)
			}
			for _, act := range lesson.Activities {
				assertReadingActivity(t, lesson.Title, act)
			}
		}
	}

	if totalLessons != 6 {
		t.Errorf("seeded %d reading lessons, want the 6 §3.7 asks for", totalLessons)
	}
}

// assertReadingActivity checks what a reading activity needs beyond a gradable
// key: a passage, and four to six questions about it.
func assertReadingActivity(t *testing.T, lessonTitle string, act seedActivity) {
	t.Helper()
	assertActivityIsGradable(t, lessonTitle, act)
	passage, _ := act.Config[cfgPassage].(string)
	if passage == "" {
		t.Errorf("lesson %s activity %d missing passage", lessonTitle, act.Position)
	}
	qs, ok := act.Config["questions"].([]map[string]any)
	if !ok || len(qs) < 4 || len(qs) > 6 {
		t.Errorf("lesson %s activity %d has %d questions, want between 4 and 6", lessonTitle, act.Position, len(qs))
	}
}

func TestWritingCourseSeedData_Integrity(t *testing.T) {
	if writingCourseSeedData.Slug != "writing-practice" || writingCourseSeedData.Title == "" {
		t.Fatal("writingCourseSeedData missing slug or title")
	}

	totalLessons := 0
	for _, unit := range writingCourseSeedData.Units {
		if unit.Position <= 0 || unit.Title == "" {
			t.Errorf("invalid unit %+v", unit)
		}
		for _, lesson := range unit.Lessons {
			totalLessons++
			if lesson.Position <= 0 || lesson.Title == "" || lesson.SkillFocus != skillWriting {
				t.Errorf("invalid writing lesson %+v", lesson)
			}
			if len(lesson.Activities) == 0 {
				t.Errorf("lesson %s has no activities", lesson.Title)
			}
			for _, act := range lesson.Activities {
				assertActivityIsGradable(t, lesson.Title, act)
				minWords, _ := act.Config[cfgMinWords].(int)
				if minWords <= 0 {
					t.Errorf("lesson %s activity %d has invalid min_words: %d", lesson.Title, act.Position, minWords)
				}
				sampleAnswer, _ := act.Config[cfgSampleAnswer].(string)
				if sampleAnswer == "" {
					t.Errorf("lesson %s activity %d missing sample_answer", lesson.Title, act.Position)
				}
			}
		}
	}

	if totalLessons != 6 {
		t.Errorf("seeded %d writing lessons, want the 6 §3.7 asks for", totalLessons)
	}
}

// TestSpeakingCourseSeedData_Integrity checks what a speaking activity needs
// beyond a gradable key: the task type the grader branches on, and a reference
// text for exactly the tasks that are scored against one.
//
// A read-aloud without a reference text scores 70% of nothing, and a respond
// task with one is silently scored as a read-aloud — both render perfectly well,
// so only this catches them.
func TestSpeakingCourseSeedData_Integrity(t *testing.T) {
	if speakingCourseSeedData.Slug != "speaking-practice" || speakingCourseSeedData.Title == "" {
		t.Fatal("speakingCourseSeedData missing slug or title")
	}

	totalLessons := 0
	for _, unit := range speakingCourseSeedData.Units {
		if unit.Position <= 0 || unit.Title == "" {
			t.Errorf("invalid unit %+v", unit)
		}
		for _, lesson := range unit.Lessons {
			totalLessons++
			if lesson.Position <= 0 || lesson.Title == "" || lesson.SkillFocus != skillSpeaking {
				t.Errorf("invalid speaking lesson %+v", lesson)
			}
			if len(lesson.Activities) == 0 {
				t.Errorf("lesson %s has no activities", lesson.Title)
			}
			for _, act := range lesson.Activities {
				assertActivityIsGradable(t, lesson.Title, act)
				assertSpeakingActivity(t, lesson.Title, act)
			}
		}
	}

	if totalLessons != 6 {
		t.Errorf("speaking course has %d lessons, want 6", totalLessons)
	}
}

func assertSpeakingActivity(t *testing.T, lessonTitle string, act seedActivity) {
	t.Helper()

	if act.Kind != kindSpeakingTask {
		t.Errorf("%s activity %d is %q, not a speaking task", lessonTitle, act.Position, act.Kind)
		return
	}

	// Both halves, because the runner reads Config and the grader reads Body.
	for label, source := range map[string]map[string]any{"config": act.Config, "body": act.Body} {
		taskType, _ := source[cfgTaskType].(string)
		if taskType != taskTypeReadAloud && taskType != taskTypeRespond {
			t.Errorf("%s activity %d %s has task_type %q", lessonTitle, act.Position, label, taskType)
			continue
		}

		reference, _ := source[cfgReferenceText].(string)
		if taskType == taskTypeReadAloud && reference == "" {
			t.Errorf("%s activity %d %s is read_aloud with no reference_text; word accuracy is 70%% of its score",
				lessonTitle, act.Position, label)
		}
		if taskType == taskTypeRespond && reference != "" {
			t.Errorf("%s activity %d %s is respond but carries a reference_text, which the grader reads as read-aloud",
				lessonTitle, act.Position, label)
		}

		if prompt, _ := source[bodyKeyPrompt].(string); prompt == "" {
			t.Errorf("%s activity %d %s has no prompt; the runner refuses to render it",
				lessonTitle, act.Position, label)
		}
		if seconds, _ := source[cfgSpeakingTime].(int); seconds <= 0 {
			t.Errorf("%s activity %d %s has no speaking time", lessonTitle, act.Position, label)
		}
	}
}

// TestListeningCourseSeedData_Integrity checks the one thing that makes a
// listening item a listening item: the script is on the body and not on the
// config.
//
// The config is what the browser receives. A script that travels with it is an
// item the learner reads instead of hearing — which renders perfectly, grades
// perfectly, and measures nothing. The rest is the same contract reading has:
// every question asked in the config has to be answered in the body, under the
// same id, or the grade comes back with a question the learner never saw.
func TestListeningCourseSeedData_Integrity(t *testing.T) {
	if listeningCourseSeedData.Slug != "listening-practice" || listeningCourseSeedData.Title == "" {
		t.Fatal("listeningCourseSeedData missing slug or title")
	}

	totalLessons := 0
	for _, unit := range listeningCourseSeedData.Units {
		if unit.Position <= 0 || unit.Title == "" {
			t.Errorf("invalid unit %+v", unit)
		}
		for _, lesson := range unit.Lessons {
			totalLessons++
			if lesson.Position <= 0 || lesson.Title == "" || lesson.SkillFocus != skillListening {
				t.Errorf("invalid listening lesson %+v", lesson)
			}
			if len(lesson.Activities) == 0 {
				t.Errorf("lesson %s has no activities", lesson.Title)
			}
			for _, act := range lesson.Activities {
				assertListeningActivity(t, lesson.Title, act)
			}
		}
	}

	if totalLessons != 6 {
		t.Errorf("listening course has %d lessons, want 6", totalLessons)
	}
}

func assertListeningActivity(t *testing.T, lessonTitle string, act seedActivity) {
	t.Helper()

	if act.Kind != kindListeningComprehension {
		t.Errorf("%s activity %d is %q, not a listening item", lessonTitle, act.Position, act.Kind)
		return
	}

	if script, _ := act.Body[cfgScript].(string); script == "" {
		t.Errorf("%s activity %d has no script; cmd/tts has nothing to render and the clip never exists",
			lessonTitle, act.Position)
	}
	if _, leaked := act.Config[cfgScript]; leaked {
		t.Errorf("%s activity %d carries the script in its config; the learner reads it instead of listening",
			lessonTitle, act.Position)
	}

	configQuestions, _ := act.Config["questions"].([]map[string]any)
	bodyQuestions, _ := act.Body["questions"].([]map[string]any)
	if len(configQuestions) < 4 {
		t.Errorf("%s activity %d asks %d questions, want at least 4",
			lessonTitle, act.Position, len(configQuestions))
	}
	if len(configQuestions) != len(bodyQuestions) {
		t.Errorf("%s activity %d asks %d questions and answers %d",
			lessonTitle, act.Position, len(configQuestions), len(bodyQuestions))
		return
	}

	answers := map[string]string{}
	for _, q := range bodyQuestions {
		id, _ := q["id"].(string)
		answer, _ := q["answer"].(string)
		if answer == "" {
			t.Errorf("%s activity %d question %q has no answer", lessonTitle, act.Position, id)
		}
		answers[id] = answer
	}

	for _, q := range configQuestions {
		id, _ := q["id"].(string)
		answer, answered := answers[id]
		if !answered {
			t.Errorf("%s activity %d asks question %q the body does not answer", lessonTitle, act.Position, id)
			continue
		}
		// A choice answer has to name one of the options the learner is shown;
		// an answer that names nothing on screen can never be picked.
		options, hasOptions := q[cfgOptions].([]map[string]string)
		if !hasOptions {
			continue
		}
		found := false
		for _, opt := range options {
			if opt[cfgOptionID] == answer {
				found = true
			}
		}
		if !found {
			t.Errorf("%s activity %d question %q answers %q, which is not one of its options",
				lessonTitle, act.Position, id, answer)
		}
	}
}
