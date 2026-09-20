package domain

import (
	"encoding/json"
	"strings"

	"github.com/fluentra/fluentra/internal/shared/apperr"
)

// SupportedFoundationTopicSchemaVersion is the current version supported by this system.
const SupportedFoundationTopicSchemaVersion = 1

// FoundationTopicBody represents the versioned payload of a foundation_topic content version.
type FoundationTopicBody struct {
	SchemaVersion  int                `json:"schema_version"`
	Objective      string             `json:"objective"`
	Explanation    ExplanationLocales `json:"explanation"`
	Examples       []TopicExample     `json:"examples"`
	Related        []string           `json:"related"`
	CommonMistakes []CommonMistake    `json:"common_mistakes"`
}

// ExplanationLocales maps language codes (e.g. en, vi) to explanation texts.
type ExplanationLocales struct {
	EN string `json:"en"`
	VI string `json:"vi"`
}

// TopicExample is an example sentence demonstrating the grammar point or pattern.
type TopicExample struct {
	Text string `json:"text"`
	Note string `json:"note"`
}

// CommonMistake illustrates a frequent learner error, the correction, and explanation.
type CommonMistake struct {
	Wrong string `json:"wrong"`
	Right string `json:"right"`
	Why   string `json:"why"`
}

// ValidateFoundationTopicBody validates the raw JSON body against schema_version 1 requirements.
func ValidateFoundationTopicBody(raw []byte) error {
	if len(raw) == 0 {
		return apperr.New(apperr.Validation, "INVALID_BODY_SCHEMA", "Foundation topic body is empty.")
	}

	var header struct {
		SchemaVersion int `json:"schema_version"`
	}
	if err := json.Unmarshal(raw, &header); err != nil {
		return apperr.New(apperr.Validation, "INVALID_BODY_SCHEMA", "Foundation topic body is not valid JSON.")
	}

	if header.SchemaVersion != SupportedFoundationTopicSchemaVersion {
		return apperr.New(apperr.Validation, "INVALID_BODY_SCHEMA", "Unsupported schema_version.")
	}

	var body FoundationTopicBody
	if err := json.Unmarshal(raw, &body); err != nil {
		return apperr.New(apperr.Validation, "INVALID_BODY_SCHEMA", "Foundation topic body could not be parsed.")
	}

	if strings.TrimSpace(body.Objective) == "" {
		return apperr.New(apperr.Validation, "INVALID_BODY_SCHEMA", "Learning objective is required.")
	}

	if strings.TrimSpace(body.Explanation.EN) == "" || strings.TrimSpace(body.Explanation.VI) == "" {
		return apperr.New(apperr.Validation, "INVALID_BODY_SCHEMA",
			"Explanations in both English and Vietnamese are required.")
	}

	if len(body.Examples) == 0 {
		return apperr.New(apperr.Validation, "INVALID_BODY_SCHEMA", "At least one example is required.")
	}

	for _, ex := range body.Examples {
		if strings.TrimSpace(ex.Text) == "" {
			return apperr.New(apperr.Validation, "INVALID_BODY_SCHEMA", "Example text cannot be empty.")
		}
	}

	for _, cm := range body.CommonMistakes {
		if strings.TrimSpace(cm.Wrong) == "" || strings.TrimSpace(cm.Right) == "" || strings.TrimSpace(cm.Why) == "" {
			return apperr.New(apperr.Validation, "INVALID_BODY_SCHEMA", "Common mistakes must specify wrong, right, and why.")
		}
	}

	return nil
}
