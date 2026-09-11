package service

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLocateAnnotations_MatchesPhrasesWithRuneOffsets(t *testing.T) {
	submission := "In recent years, the rapid advancement of artificial intelligence has transformed education."
	quotes := []rawAnnotation{
		{
			QuotedText: "rapid advancement",
			CommentEn:  "Strong adjective-noun collocation.",
			CommentVi:  "Cụm tính từ - danh từ rất tốt.",
		},
		{
			QuotedText: "transformed education",
			CommentEn:  "Clear verb choice.",
			CommentVi:  "Chọn động từ phù hợp.",
		},
	}

	located := locateAnnotations(submission, quotes)
	require.Len(t, located, 2)

	runes := []rune(submission)

	assert.Equal(t, "rapid advancement", located[0].QuotedText)
	assert.Equal(t, "rapid advancement", string(runes[located[0].StartOffset:located[0].EndOffset]))
	assert.Equal(t, "Strong adjective-noun collocation.", located[0].CommentEn)
	assert.Equal(t, "Cụm tính từ - danh từ rất tốt.", located[0].CommentVi)

	assert.Equal(t, "transformed education", located[1].QuotedText)
	assert.Equal(t, "transformed education", string(runes[located[1].StartOffset:located[1].EndOffset]))
}

func TestLocateAnnotations_HandlesVietnameseAndMultiByteRunes(t *testing.T) {
	submission := "Học tiếng Anh giúp tôi có thêm cơ hội việc làm tốt hơn."
	quotes := []rawAnnotation{
		{
			QuotedText: "tiếng Anh",
			CommentEn:  "Language topic.",
			CommentVi:  "Chủ đề ngôn ngữ.",
		},
		{
			QuotedText: "việc làm tốt hơn",
			CommentEn:  "Clear phrasing.",
			CommentVi:  "Diễn đạt rõ ràng.",
		},
	}

	located := locateAnnotations(submission, quotes)
	require.Len(t, located, 2)

	runes := []rune(submission)
	assert.Equal(t, "tiếng Anh", string(runes[located[0].StartOffset:located[0].EndOffset]))
	assert.Equal(t, "việc làm tốt hơn", string(runes[located[1].StartOffset:located[1].EndOffset]))
}

func TestLocateAnnotations_DropsHallucinatedQuotes(t *testing.T) {
	submission := "Although technology brings many benefits, it also has certain drawbacks."
	quotes := []rawAnnotation{
		{
			QuotedText: "many benefits",
			CommentEn:  "Good point.",
			CommentVi:  "Ý kiến hay.",
		},
		{
			QuotedText: "this text does not exist anywhere in the essay",
			CommentEn:  "Hallucinated comment.",
			CommentVi:  "Nhận xét ảo.",
		},
		{
			QuotedText: "certain drawbacks",
			CommentEn:  "Good counter-argument.",
			CommentVi:  "Phản đề tốt.",
		},
	}

	located := locateAnnotations(submission, quotes)
	require.Len(t, located, 2)
	assert.Equal(t, "many benefits", located[0].QuotedText)
	assert.Equal(t, "certain drawbacks", located[1].QuotedText)
}

func TestLocateAnnotations_TrimmedQuoteFallback(t *testing.T) {
	submission := "Global warming is a serious problem for the whole world."
	quotes := []rawAnnotation{
		{
			QuotedText: "  serious problem  ",
			CommentEn:  "Accurate vocabulary.",
			CommentVi:  "Từ vựng chính xác.",
		},
	}

	located := locateAnnotations(submission, quotes)
	require.Len(t, located, 1)
	assert.Equal(t, "serious problem", located[0].QuotedText)
	runes := []rune(submission)
	assert.Equal(t, "serious problem", string(runes[located[0].StartOffset:located[0].EndOffset]))
}

func TestLocateAnnotations_EmptyCases(t *testing.T) {
	assert.Empty(t, locateAnnotations("", []rawAnnotation{{QuotedText: "test"}}))
	assert.Empty(t, locateAnnotations("Hello world", nil))
	assert.Empty(t, locateAnnotations("Hello world", []rawAnnotation{}))
	assert.Empty(t, locateAnnotations("Hello world", []rawAnnotation{{QuotedText: ""}}))
}
