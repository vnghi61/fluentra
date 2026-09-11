package service_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	contentcontract "github.com/fluentra/fluentra/internal/modules/content/contract"
	"github.com/fluentra/fluentra/internal/modules/grammar/contract"
	"github.com/fluentra/fluentra/internal/modules/grammar/service"
	learningcontract "github.com/fluentra/fluentra/internal/modules/learning/contract"
)

const (
	optHadFinished = "opt_had_finished"
	keyTextAnswer  = "text_answer"
)

type fakeContentReader struct {
	versions map[uuid.UUID]*contentcontract.Version
}

func (f *fakeContentReader) GetVersion(_ context.Context, id uuid.UUID) (*contentcontract.Version, error) {
	return f.versions[id], nil
}

func TestGradedKinds_IncludesBothGrammarKinds(t *testing.T) {
	kinds := contract.GradedKinds()
	assert.Contains(t, kinds, contract.KindGrammarTenseChoice)
	assert.Contains(t, kinds, contract.KindGrammarSentenceTransform)
}

func TestGrammarGrader_TenseChoice(t *testing.T) {
	versionID := uuid.New()
	body := map[string]any{
		"prompt":            "Choose the correct verb tense:",
		"correct_option_id": optHadFinished,
		"correct_answer":    optHadFinished,
		"acceptable":        []string{"had finished", optHadFinished},
	}
	bodyBytes, _ := json.Marshal(body)

	reader := &fakeContentReader{
		versions: map[uuid.UUID]*contentcontract.Version{
			versionID: {ID: versionID, Body: bodyBytes},
		},
	}
	grader := service.NewGrader(reader)

	// Correct selection
	respBytes, _ := json.Marshal(map[string]string{
		"selected_option_id": optHadFinished,
	})
	res, err := grader.Grade(context.Background(), learningcontract.GradeRequest{
		ContentVersionID: versionID,
		Response:         respBytes,
	})
	require.NoError(t, err)
	assert.True(t, res.Correct)
	assert.Equal(t, 100, res.Score)
	assert.Equal(t, optHadFinished, res.CorrectAnswer)
	require.Len(t, res.ReviewItems, 1)
	assert.Equal(t, "grammar", res.ReviewItems[0].Skill)
	assert.Equal(t, "good", res.ReviewItems[0].InitialGrade)

	// Incorrect selection
	wrongResp, _ := json.Marshal(map[string]string{
		"selected_option_id": "opt_finish",
	})
	resWrong, err := grader.Grade(context.Background(), learningcontract.GradeRequest{
		ContentVersionID: versionID,
		Response:         wrongResp,
	})
	require.NoError(t, err)
	assert.False(t, resWrong.Correct)
	assert.Equal(t, 0, resWrong.Score)
	require.Len(t, resWrong.ReviewItems, 1)
	assert.Equal(t, "again", resWrong.ReviewItems[0].InitialGrade)
}

func TestGrammarGrader_SentenceTransform(t *testing.T) {
	versionID := uuid.New()
	body := map[string]any{
		"prompt":         "Rewrite the sentence using present perfect continuous:",
		"correct_answer": "I have been living here for five years.",
		"acceptable":     []string{"I've been living here for five years."},
	}
	bodyBytes, _ := json.Marshal(body)

	reader := &fakeContentReader{
		versions: map[uuid.UUID]*contentcontract.Version{
			versionID: {ID: versionID, Body: bodyBytes},
		},
	}
	grader := service.NewGrader(reader)

	// Exact match ignoring case & punctuation
	respBytes, _ := json.Marshal(map[string]string{
		keyTextAnswer: "i have been living here for five years",
	})
	res, err := grader.Grade(context.Background(), learningcontract.GradeRequest{
		ContentVersionID: versionID,
		Response:         respBytes,
	})
	require.NoError(t, err)
	assert.True(t, res.Correct)
	assert.Equal(t, 100, res.Score)

	// Acceptable alternative
	altRespBytes, _ := json.Marshal(map[string]string{
		keyTextAnswer: "I've been living here for five years.",
	})
	resAlt, err := grader.Grade(context.Background(), learningcontract.GradeRequest{
		ContentVersionID: versionID,
		Response:         altRespBytes,
	})
	require.NoError(t, err)
	assert.True(t, resAlt.Correct)
	assert.Equal(t, 100, resAlt.Score)

	// Incorrect
	wrongResp, _ := json.Marshal(map[string]string{
		keyTextAnswer: "I lived here for five years.",
	})
	resWrong, err := grader.Grade(context.Background(), learningcontract.GradeRequest{
		ContentVersionID: versionID,
		Response:         wrongResp,
	})
	require.NoError(t, err)
	assert.False(t, resWrong.Correct)
	assert.Equal(t, 0, resWrong.Score)
}

func TestGrammarGrader_WritingPracticeWithAnswer(t *testing.T) {
	// §3.8: Rewrite using "although" with multiple valid acceptable answers
	versionID := uuid.New()
	body := map[string]any{
		"prompt":         "Combine the sentences using 'although': It was raining. We went for a walk.",
		"correct_answer": "Although it was raining, we went for a walk.",
		"acceptable": []string{
			"Although it was raining we went for a walk.",
			"We went for a walk although it was raining.",
			"We went for a walk, although it was raining.",
		},
	}
	bodyBytes, _ := json.Marshal(body)

	reader := &fakeContentReader{
		versions: map[uuid.UUID]*contentcontract.Version{
			versionID: {ID: versionID, Body: bodyBytes},
		},
	}
	grader := service.NewGrader(reader)

	// Primary answer with punctuation
	resp1, err := grader.Grade(context.Background(), learningcontract.GradeRequest{
		ContentVersionID: versionID,
		Response:         []byte(`{"text_answer":"Although it was raining, we went for a walk."}`),
	})
	require.NoError(t, err)
	assert.True(t, resp1.Correct)

	// Acceptable alternative clause order
	resp2, err := grader.Grade(context.Background(), learningcontract.GradeRequest{
		ContentVersionID: versionID,
		Response:         []byte(`{"text_answer":"We went for a walk although it was raining"}`),
	})
	require.NoError(t, err)
	assert.True(t, resp2.Correct)

	// Error correction item
	errVersionID := uuid.New()
	errBody := map[string]any{
		"prompt":         "Correct the one error in the sentence: She don't like drinking cold coffee.",
		"correct_answer": "She doesn't like drinking cold coffee.",
		"acceptable": []string{
			"She does not like drinking cold coffee.",
			"She doesn't like drinking cold coffee",
			"She does not like drinking cold coffee",
		},
	}
	errBodyBytes, _ := json.Marshal(errBody)
	reader.versions[errVersionID] = &contentcontract.Version{ID: errVersionID, Body: errBodyBytes}

	resp3, err := grader.Grade(context.Background(), learningcontract.GradeRequest{
		ContentVersionID: errVersionID,
		Response:         []byte(`{"text_answer":"She does not like drinking cold coffee."}`),
	})
	require.NoError(t, err)
	assert.True(t, resp3.Correct)
}
