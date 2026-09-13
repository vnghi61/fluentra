// Package service implements the business logic and graders for the writing module.
package service

import (
	"strings"
	"unicode/utf8"

	"github.com/fluentra/fluentra/internal/modules/writing/contract"
)

type rawAnnotation struct {
	QuotedText string `json:"quoted_text"`
	CommentEn  string `json:"comment_en"`
	CommentVi  string `json:"comment_vi"`
}

// locateAnnotations finds each quoted span in the submission, computes rune offsets,
// and drops quotes that cannot be found (guarding against hallucinated quotes).
func locateAnnotations(submission string, quotes []rawAnnotation) []contract.WritingAnnotation {
	if len(quotes) == 0 || submission == "" {
		return []contract.WritingAnnotation{}
	}

	result := make([]contract.WritingAnnotation, 0, len(quotes))
	searchOffset := 0

	for _, q := range quotes {
		text := q.QuotedText
		if text == "" {
			continue
		}

		// Try to find text starting from searchOffset
		byteIdx := strings.Index(submission[searchOffset:], text)
		var matchStart int
		if byteIdx >= 0 {
			matchStart = searchOffset + byteIdx
			searchOffset = matchStart + len(text)
		} else {
			// If not found after searchOffset, try searching from beginning
			byteIdx = strings.Index(submission, text)
			if byteIdx >= 0 {
				matchStart = byteIdx
			} else {
				// Try trimmed text if quote has extra leading/trailing whitespace
				trimmed := strings.TrimSpace(text)
				if trimmed != "" && trimmed != text {
					byteIdx = strings.Index(submission, trimmed)
					if byteIdx >= 0 {
						matchStart = byteIdx
						text = trimmed
					} else {
						// Drop hallucinated quote
						continue
					}
				} else {
					// Drop hallucinated quote
					continue
				}
			}
		}

		// Calculate rune-based offsets (matching JavaScript string indices)
		startRune := utf8.RuneCountInString(submission[:matchStart])
		endRune := startRune + utf8.RuneCountInString(text)

		result = append(result, contract.WritingAnnotation{
			QuotedText:  text,
			StartOffset: startRune,
			EndOffset:   endRune,
			CommentEn:   q.CommentEn,
			CommentVi:   q.CommentVi,
		})
	}

	return result
}
