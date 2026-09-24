package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFoundationCourseMeta_CoversTheThirteenCourses(t *testing.T) {
	assert.Len(t, foundationCourseMetaBySlug, 13,
		"the thirteen Foundation courses each need a card")
	assert.Len(t, phase2CourseSlugs, 5,
		"the five Phase 2 courses are archived, not deleted")

	for slug, meta := range foundationCourseMetaBySlug {
		assert.NotEmpty(t, meta.Title, "course %s needs a title", slug)
		assert.NotEmpty(t, meta.Description, "course %s needs a description", slug)
		assert.NotEmpty(t, meta.CEFRFrom, "course %s needs a starting level", slug)
		assert.NotEmpty(t, meta.CEFRTo, "course %s needs an ending level", slug)
		assert.Positive(t, meta.Hours, "course %s needs an estimated length", slug)
	}
}

func TestGroupByCourse_PreservesTheMapOrder(t *testing.T) {
	nodes := []foundationCourseNode{
		{courseSlug: courseEnglishTenses, code: "TENSES", position: 1},
		{courseSlug: courseGrammarFoundations, code: "NOUNS", position: 2},
		{courseSlug: courseEnglishTenses, code: "PRESENT_SIMPLE", position: 2},
	}
	order, byCourse := groupByCourse(nodes)

	assert.Equal(t, []string{courseEnglishTenses, courseGrammarFoundations}, order)
	require.Len(t, byCourse[courseEnglishTenses], 2)
	assert.Equal(t, "TENSES", byCourse[courseEnglishTenses][0].code)
	assert.Equal(t, "PRESENT_SIMPLE", byCourse[courseEnglishTenses][1].code)
	assert.Len(t, byCourse[courseGrammarFoundations], 1)
}

func TestSkillFocusFor(t *testing.T) {
	tests := []struct {
		name string
		node foundationCourseNode
		want string
	}{
		{"grammar", foundationCourseNode{namespace: skillGrammar, code: "NOUNS"}, skillGrammar},
		{"vocabulary", foundationCourseNode{namespace: skillVocabulary, code: "IDIOMS"}, skillVocabulary},
		{"pattern", foundationCourseNode{namespace: namespacePattern, code: "ASKING_QUESTIONS"}, namespacePattern},
		{"pronunciation",
			foundationCourseNode{namespace: namespacePronunciation, code: "WORD_STRESS"},
			namespacePronunciation},
		{"listening", foundationCourseNode{namespace: namespaceSkill, code: "LISTENING_WORDS"}, skillListening},
		{"speaking", foundationCourseNode{namespace: namespaceSkill, code: "SPEAKING_PARAGRAPHS"}, skillSpeaking},
		{"reading", foundationCourseNode{namespace: namespaceSkill, code: "READING"}, skillReading},
		{"writing", foundationCourseNode{namespace: namespaceSkill, code: "WRITING_LINKING_WORDS"}, skillWriting},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, skillFocusFor(tt.node), tt.name)
	}
}
