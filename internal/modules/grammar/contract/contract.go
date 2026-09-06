package contract

import (
	learningcontract "github.com/fluentra/fluentra/internal/modules/learning/contract"
)

const (
	// KindGrammarTenseChoice is a multiple-choice exercise testing grammatical tense/form.
	KindGrammarTenseChoice = "grammar_tense_choice"
	// KindGrammarSentenceTransform tests transforming a sentence into another grammatical form.
	KindGrammarSentenceTransform = "grammar_sentence_transform"
)

// GradedKinds are the activity kinds this module's grader can score.
func GradedKinds() []string {
	return []string{
		KindGrammarTenseChoice,
		KindGrammarSentenceTransform,
	}
}

// Grader defines the exercise grading contract implemented by the grammar module.
type Grader interface {
	learningcontract.ExerciseGrader
}
