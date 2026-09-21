// Package domain contains question bank domain entities, invariants, and fingerprinting.
package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"unicode"
)

// NormaliseText produces the canonical form of a text string:
// - lower-cased
// - punctuation and symbols removed
// - whitespace collapsed (all consecutive whitespace turned into a single space, trimmed)
func NormaliseText(s string) string {
	s = strings.ToLower(s)
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if unicode.IsPunct(r) || unicode.IsSymbol(r) {
			continue
		}
		b.WriteRune(r)
	}
	return strings.Join(strings.Fields(b.String()), " ")
}

// NormaliseOptions normalises each option string and returns a sorted slice.
func NormaliseOptions(options []string) []string {
	res := make([]string, 0, len(options))
	for _, opt := range options {
		norm := NormaliseText(opt)
		if norm != "" {
			res = append(res, norm)
		}
	}
	sort.Strings(res)
	return res
}

// ComputeFingerprint computes the SHA-256 hex digest of a canonical text string
// formed by the normalised stem/prompt and sorted normalised options.
// Two items that differ only in option order, punctuation, casing, or whitespace
// produce the identical fingerprint.
func ComputeFingerprint(stem string, options []string) string {
	normStem := NormaliseText(stem)
	normOpts := NormaliseOptions(options)

	canonical := normStem + "|" + strings.Join(normOpts, "|")
	hash := sha256.Sum256([]byte(canonical))
	return hex.EncodeToString(hash[:])
}

// FingerprintFromBody extracts text elements from an activity JSON body and
// computes its canonical fingerprint.
func FingerprintFromBody(_ string, raw json.RawMessage) (string, error) {
	if len(raw) == 0 {
		return "", fmt.Errorf("body is empty")
	}

	var data map[string]any
	if err := json.Unmarshal(raw, &data); err != nil {
		return "", fmt.Errorf("unmarshal body: %w", err)
	}

	stem := extractStem(data)
	if sub := extractSubQuestions(data); sub != "" {
		stem = stem + "||" + sub
	}

	options := extractOptions(data)
	options = extractFallbackOptions(data, options)

	return ComputeFingerprint(stem, options), nil
}

func extractSubQuestions(data map[string]any) string {
	questionsRaw, ok := data["questions"].([]any)
	if !ok || len(questionsRaw) == 0 {
		return ""
	}

	qParts := make([]string, 0, len(questionsRaw))
	for _, qItem := range questionsRaw {
		if qMap, ok := qItem.(map[string]any); ok {
			qPrompt := extractStem(qMap)
			qOpts := extractOptions(qMap)
			qParts = append(qParts, NormaliseText(qPrompt)+":"+strings.Join(NormaliseOptions(qOpts), ","))
		}
	}
	sort.Strings(qParts)
	return strings.Join(qParts, "||")
}

func extractFallbackOptions(data map[string]any, options []string) []string {
	if len(options) > 0 {
		return options
	}
	for _, key := range []string{"statements", "responses"} {
		if arr, ok := data[key].([]any); ok {
			for _, item := range arr {
				if m, ok := item.(map[string]any); ok {
					if txt, ok := m["text"].(string); ok {
						options = append(options, txt)
					}
				}
			}
			if len(options) > 0 {
				break
			}
		}
	}
	return options
}

func extractStem(data map[string]any) string {
	// Check common stem fields
	for _, key := range []string{"passage", "script", "prompt", "sentence", "text", "stem"} {
		if val, ok := data[key].(string); ok && strings.TrimSpace(val) != "" {
			return val
		}
	}
	return ""
}

func extractOptions(data map[string]any) []string {
	var options []string
	optsRaw, ok := data["options"]
	if !ok {
		return options
	}

	switch opts := optsRaw.(type) {
	case []any:
		for _, opt := range opts {
			switch o := opt.(type) {
			case string:
				options = append(options, o)
			case map[string]any:
				if txt, ok := o["text"].(string); ok {
					options = append(options, txt)
				} else if val, ok := o["label"].(string); ok {
					options = append(options, val)
				}
			}
		}
	case []string:
		options = append(options, opts...)
	}

	return options
}
