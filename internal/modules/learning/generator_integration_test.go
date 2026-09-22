//go:build integration

package learning_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fluentra/fluentra/internal/modules/content"
	contentcontract "github.com/fluentra/fluentra/internal/modules/content/contract"
	learningcontract "github.com/fluentra/fluentra/internal/modules/learning/contract"
	"github.com/fluentra/fluentra/internal/modules/learning/domain"
	"github.com/fluentra/fluentra/internal/modules/learning/repository"
	"github.com/fluentra/fluentra/internal/modules/learning/service"
	"github.com/fluentra/fluentra/internal/modules/lesson"
	"github.com/fluentra/fluentra/internal/platform/ai"
	"github.com/fluentra/fluentra/internal/shared/clock"
)

type integrationTenseChoiceModel struct{}

func (m *integrationTenseChoiceModel) Complete(_ context.Context, req ai.Request) (ai.Response, error) {
	if req.Task == ai.TaskItemSolve || req.Task == ai.TaskPracticeSolve {
		return ai.Response{Text: `{"selected_option_id": "A"}`, Model: "mock-solver"}, nil
	}
	item := `{
		"prompt": "She ___ lived here for three years.",
		"options": [
			{"id": "A", "text": "has"},
			{"id": "B", "text": "have"},
			{"id": "C", "text": "had"},
			{"id": "D", "text": "having"}
		],
		"correct_option_id": "A",
		"explanation": {
			"explanation_en": "Present perfect with she uses has.",
			"explanation_vi": "Hiện tại hoàn thành với she dùng has."
		}
	}`
	return ai.Response{Text: item, Model: "mock-generator"}, nil
}

type integrationGrader struct{}

func (g *integrationGrader) Grade(
	_ context.Context, req learningcontract.GradeRequest,
) (learningcontract.GradeResult, error) {
	var resp struct {
		SelectedOptionID string `json:"selected_option_id"`
	}
	_ = json.Unmarshal(req.Response, &resp)
	if resp.SelectedOptionID == "A" {
		return learningcontract.GradeResult{Score: 100, Correct: true}, nil
	}
	return learningcontract.GradeResult{Score: 0, Correct: false}, nil
}

// TestModule_WorkOrder19StageCGate verifies WO-19 Stage C Gate against real PostgreSQL:
// "A Generate call for grammar_tense_choice, PRESENT_PERFECT, B1, count 3, purpose foundation,
// produces three draft versions tagged grammar.PRESENT_PERFECT, each with provenance, none visible to a learner."
func TestModule_WorkOrder19StageCGate(t *testing.T) {
	if attemptPool == nil {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	ctx := context.Background()

	adminID := uuid.New()
	if _, err := attemptPool.Exec(ctx,
		`INSERT INTO core.users (id, email, status) VALUES ($1, $2, 'active')`,
		adminID, "gen-stage-c-"+adminID.String()+"@example.test"); err != nil {
		t.Fatalf("seed owner: %v", err)
	}

	contentModule := content.NewAuthoring(content.Deps{Pool: attemptPool})
	lessonModule := lesson.New(lesson.Deps{Pool: attemptPool, Guard: allowAll{}, Env: envTest})

	graders := domain.NewGraderRegistry()
	_ = graders.Register(kindTenseChoice, &integrationGrader{})

	svc := service.New(service.Deps{
		Pool:              attemptPool,
		Repo:              repositoryAdapter{Repository: repository.New(attemptPool)},
		Lesson:            lessonModule.Reader(),
		LessonAuthor:      lessonModule.Author(),
		Content:           contentModule.Reader(),
		ContentAuthor:     contentModule.Author(),
		Taxonomies:        contentModule.TaxonomyResolver(),
		Graders:           graders,
		AI:                &integrationTenseChoiceModel{},
		Clock:             clock.Real{},
		GeneratorAuthorID: adminID,
	})

	req := learningcontract.GenerateRequest{
		Kind:      kindTenseChoice,
		CEFRLevel: "B1",
		NodeCodes: []string{nodePresentPerf},
		Count:     3,
		Purpose:   purposeFound,
	}

	items, err := svc.Generate(ctx, req)
	require.NoError(t, err)
	require.Len(t, items, 3, "must generate exactly 3 items")

	for _, item := range items {
		// 1. Verify version exists in content.content_versions with status = 'draft'
		var status string
		var rawBody []byte
		err := attemptPool.QueryRow(ctx,
			`SELECT status, body FROM content.content_versions WHERE id = $1`,
			item.ContentVersionID,
		).Scan(&status, &rawBody)
		require.NoError(t, err)
		assert.Equal(t, "draft", status, "foundation generated item must be authored as draft")

		// 2. Verify tagging in content.content_tags
		var tagCount int
		err = attemptPool.QueryRow(ctx,
			`SELECT count(*) FROM content.content_tags ct
			 JOIN content.taxonomies t ON t.id = ct.taxonomy_id
			 JOIN content.content_versions cv ON cv.item_id = ct.item_id
			 WHERE cv.id = $1 AND t.namespace = 'grammar' AND t.code = 'PRESENT_PERFECT'`,
			item.ContentVersionID,
		).Scan(&tagCount)
		require.NoError(t, err)
		assert.Equal(t, 1, tagCount, "item must be tagged grammar.PRESENT_PERFECT in content_tags")

		// 3. Verify provenance in body
		var bodyMap map[string]any
		err = json.Unmarshal(rawBody, &bodyMap)
		require.NoError(t, err)
		provRaw, exists := bodyMap["_provenance"]
		require.True(t, exists, "body in DB must contain _provenance")
		prov, ok := provRaw.(map[string]any)
		require.True(t, ok)
		assert.NotEmpty(t, prov["prompt_version"])
		assert.NotEmpty(t, prov["model"])
		assert.NotEmpty(t, prov["ai_request_id"])

		// 4. Verify RedactForLearner strips provenance
		redacted := contentcontract.RedactForLearner(rawBody)
		var redactedMap map[string]any
		err = json.Unmarshal(redacted, &redactedMap)
		require.NoError(t, err)
		_, provInRedacted := redactedMap["_provenance"]
		assert.False(t, provInRedacted, "_provenance must be stripped by RedactForLearner")

		kindFilter := kindTenseChoice
		publishedList, _, err := contentModule.Reader().Browse(ctx, contentcontract.BrowseFilter{
			Kind:  &kindFilter,
			Limit: 100,
		})
		require.NoError(t, err)
		for _, pub := range publishedList {
			assert.NotEqual(t, item.ContentVersionID, pub.ID, "draft version must not appear in published browse")
		}
	}
}
