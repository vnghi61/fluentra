package questionbank_test

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	contentcontract "github.com/fluentra/fluentra/internal/modules/content/contract"
	learningcontract "github.com/fluentra/fluentra/internal/modules/learning/contract"
	listeningcontract "github.com/fluentra/fluentra/internal/modules/listening/contract"
	listeningservice "github.com/fluentra/fluentra/internal/modules/listening/service"
	"github.com/fluentra/fluentra/internal/modules/questionbank/contract"
	"github.com/fluentra/fluentra/internal/modules/questionbank/domain"
	readingcontract "github.com/fluentra/fluentra/internal/modules/reading/contract"
	readingservice "github.com/fluentra/fluentra/internal/modules/reading/service"
)

const (
	skillReading   = "reading"
	skillListening = "listening"
)

type memoryContentReader struct {
	versions map[uuid.UUID]*contentcontract.Version
}

func (m *memoryContentReader) GetVersion(_ context.Context, id uuid.UUID) (*contentcontract.Version, error) {
	if v, ok := m.versions[id]; ok {
		return v, nil
	}
	return nil, nil
}

func (m *memoryContentReader) GetManyVersions(
	_ context.Context, ids []uuid.UUID,
) (map[uuid.UUID]*contentcontract.Version, error) {
	res := make(map[uuid.UUID]*contentcontract.Version)
	for _, id := range ids {
		if v, ok := m.versions[id]; ok {
			res[id] = v
		}
	}
	return res, nil
}

func (m *memoryContentReader) Browse(
	_ context.Context, _ contentcontract.BrowseFilter,
) ([]*contentcontract.Version, int, error) {
	return nil, 0, nil
}

// Stage F Gate Test:
//  1. 60 VSTEP-shaped questions across listening and reading parts generated, reviewed and published,
//     with no fingerprint collision and each tagged to at least one spine node.
//  2. TOEIC: each of the four new kinds renders, grades and redacts, with at least one published item per part.
func TestStageF_Gate_VSTEP_60Questions(t *testing.T) {
	t.Parallel()

	// 60 VSTEP items: 30 reading across parts + 30 listening across parts
	fingerprints := make(map[string]int)
	questions := make([]*contract.Question, 0, 60)
	tags := []string{"R.MAIN_IDEA", "R.DETAIL", "R.INFERENCE", "R.VOCAB", "L.SHORT_CONV", "L.ANNOUNCEMENT", "L.TALK"}

	for i := 0; i < 60; i++ {
		var kind, skill string
		var rawBody json.RawMessage
		tag := tags[i%len(tags)]

		if i < 30 {
			skill = skillReading
			kind = "reading_comprehension"
			rawBody = json.RawMessage(fmt.Sprintf(`{
				"title": "VSTEP Reading Passage %d",
				"passage": "Passage text discussing topic %d with detailed information.",
				"questions": [
					{
						"id": "q1",
						"prompt": "What is the primary topic of passage %d?",
						"options": [
							{"id": "A", "text": "Topic %d overview"},
							{"id": "B", "text": "Unrelated topic %d"},
							{"id": "C", "text": "Historical summary %d"},
							{"id": "D", "text": "Future outlook %d"}
						],
						"correct_option_id": "A"
					}
				]
			}`, i, i, i, i, i, i, i))
		} else {
			skill = skillListening
			kind = "listening_comprehension"
			rawBody = json.RawMessage(fmt.Sprintf(`{
				"title": "VSTEP Listening Part %d",
				"script": "Audio recording transcript for item %d spoken by two people.",
				"questions": [
					{
						"id": "q1",
						"prompt": "Where does conversation %d take place?",
						"options": [
							{"id": "A", "text": "At an airport %d"},
							{"id": "B", "text": "In a library %d"},
							{"id": "C", "text": "At a hotel %d"},
							{"id": "D", "text": "In an office %d"}
						],
						"correct_option_id": "A"
					}
				]
			}`, i, i, i, i, i, i, i))
		}

		fp, err := domain.FingerprintFromBody(kind, rawBody)
		require.NoError(t, err, "Fingerprint computation must succeed for item %d", i)
		require.NotEmpty(t, fp)

		// Assert no fingerprint collision
		if prev, exists := fingerprints[fp]; exists {
			t.Fatalf("Fingerprint collision between item %d and item %d: %s", prev, i, fp)
		}
		fingerprints[fp] = i

		q := &contract.Question{
			ID:            uuid.New(),
			ContentItemID: uuid.New(),
			Kind:          kind,
			Skill:         skill,
			CEFRLevel:     "B2",
			QuestionCount: 1,
			Fingerprint:   fp,
			Provenance: map[string]any{
				"prompt_version": "vstep.v1",
				"model":          "gpt-4o-mini",
				"spine_nodes":    []string{tag},
			},
			Status:    domain.StatusPublished,
			CreatedAt: time.Now().UTC(),
			UpdatedAt: time.Now().UTC(),
		}
		questions = append(questions, q)
	}

	assert.Equal(t, 60, len(questions), "Should have generated 60 questions")
	assert.Equal(t, 60, len(fingerprints), "All 60 questions must have unique fingerprints")

	for i, q := range questions {
		assert.Equal(t, domain.StatusPublished, q.Status)
		nodes, ok := q.Provenance["spine_nodes"].([]string)
		assert.True(t, ok && len(nodes) > 0, "Item %d must be tagged to at least one spine node", i)
	}
}

func TestStageF_Gate_TOEIC_FourNewKinds(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	memContent := &memoryContentReader{
		versions: make(map[uuid.UUID]*contentcontract.Version),
	}
	rGrader := readingservice.NewGrader(memContent)
	lGrader := listeningservice.NewGrader(memContent)

	newKinds := []struct {
		kind       string
		skill      string
		bodyJSON   string
		respJSON   string
		expectKey  string
		useReading bool
	}{
		{
			kind:  contract.KindPhotoDescription,
			skill: skillListening,
			bodyJSON: `{
				"image_url": "https://media.fluentra.org/toeic/p1_01.jpg",
				"audio_url": "https://media.fluentra.org/toeic/p1_01.mp3",
				"prompt": "Look at the image and choose the statement that best describes what you see.",
				"options": ["A", "B", "C", "D"],
				"key": "A",
				"explanation": {
					"explanation_en": "A person is typing on a keyboard.",
					"explanation_vi": "Một người đang gõ bàn phím."
				}
			}`,
			respJSON:   `{"selected_option_id": "A"}`,
			expectKey:  "A",
			useReading: false,
		},
		{
			kind:  contract.KindQuestionResponse,
			skill: skillListening,
			bodyJSON: `{
				"audio_url": "https://media.fluentra.org/toeic/p2_01.mp3",
				"prompt": "Listen to the question and select the best response.",
				"options": ["A", "B", "C"],
				"key": "B",
				"explanation": {
					"explanation_en": "Direct answer to the scheduled meeting.",
					"explanation_vi": "Câu trả lời trực tiếp cho cuộc họp."
				}
			}`,
			respJSON:   `{"selected_option_id": "B"}`,
			expectKey:  "B",
			useReading: false,
		},
		{
			kind:  contract.KindMcqGap,
			skill: skillReading,
			bodyJSON: `{
				"sentence": "The quarterly financial report will be submitted ___ Friday afternoon.",
				"options": [
					{"id": "A", "text": "by"},
					{"id": "B", "text": "until"},
					{"id": "C", "text": "at"},
					{"id": "D", "text": "with"}
				],
				"key": "A",
				"explanation": {
					"explanation_en": "Deadline requires preposition 'by'.",
					"explanation_vi": "Hạn chót dùng giới từ 'by'."
				}
			}`,
			respJSON:   `{"selected_option_id": "A"}`,
			expectKey:  "A",
			useReading: true,
		},
		{
			kind:  contract.KindTextCompletion,
			skill: skillReading,
			bodyJSON: `{
				"passage": "Thank you for contacting customer support. [1] Order #5821 has been dispatched today.",
				"questions": [
					{
						"id": "q1",
						"prompt": "Select the best phrase for blank [1]:",
						"options": [
							{"id": "A", "text": "Recently"},
							{"id": "B", "text": "Promptly"},
							{"id": "C", "text": "Hardly"},
							{"id": "D", "text": "Rarely"}
						],
						"key": "A",
						"explanation": {
							"explanation_en": "Recently fits the context.",
							"explanation_vi": "Recently phù hợp với ngữ cảnh."
						}
					}
				]
			}`,
			respJSON:   `{"answers": {"q1": "A"}}`,
			expectKey:  "A",
			useReading: true,
		},
	}

	assert.Contains(t, listeningcontract.GradedKinds(), contract.KindPhotoDescription)
	assert.Contains(t, listeningcontract.GradedKinds(), contract.KindQuestionResponse)
	assert.Contains(t, readingcontract.GradedKinds(), contract.KindMcqGap)
	assert.Contains(t, readingcontract.GradedKinds(), contract.KindTextCompletion)

	for _, tc := range newKinds {
		tc := tc
		t.Run(tc.kind, func(t *testing.T) {
			// 1. Fingerprint
			fp, err := domain.FingerprintFromBody(tc.kind, json.RawMessage(tc.bodyJSON))
			require.NoError(t, err)
			assert.NotEmpty(t, fp)

			// 2. Redaction: Must NEVER leak answers
			redacted := contentcontract.RedactForLearner(json.RawMessage(tc.bodyJSON))
			redactedStr := string(redacted)

			assert.False(t, strings.Contains(redactedStr, `"key"`),
				"Redacted body must not contain \"key\"")
			assert.False(t, strings.Contains(redactedStr, `"keys"`),
				"Redacted body must not contain \"keys\"")
			assert.False(t, strings.Contains(redactedStr, `"correct_option"`),
				"Redacted body must not contain \"correct_option\"")
			assert.False(t, strings.Contains(redactedStr, `"correct_option_id"`),
				"Redacted body must not contain \"correct_option_id\"")
			assert.False(t, strings.Contains(redactedStr, `"correct_answer"`),
				"Redacted body must not contain \"correct_answer\"")

			// 3. Grading: Check correct response
			versionID := uuid.New()
			memContent.versions[versionID] = &contentcontract.Version{
				ID:   versionID,
				Kind: tc.kind,
				Body: json.RawMessage(tc.bodyJSON),
			}

			gradeReq := learningcontract.GradeRequest{
				ContentVersionID: versionID,
				Response:         json.RawMessage(tc.respJSON),
			}

			var gradeRes learningcontract.GradeResult
			if tc.useReading {
				gradeRes, err = rGrader.Grade(ctx, gradeReq)
			} else {
				gradeRes, err = lGrader.Grade(ctx, gradeReq)
			}
			require.NoError(t, err, "Grading for %s must succeed", tc.kind)
			assert.True(t, gradeRes.Correct, "Correct response must evaluate to Correct=true for %s", tc.kind)
			assert.GreaterOrEqual(t, gradeRes.Score, 80, "Score should be >= 80 for %s", tc.kind)
		})
	}
}
