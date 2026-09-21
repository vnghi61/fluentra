//go:build integration

package learning_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fluentra/fluentra/internal/modules/content"
	contentdomain "github.com/fluentra/fluentra/internal/modules/content/domain"
	contentsvc "github.com/fluentra/fluentra/internal/modules/content/service"
	"github.com/fluentra/fluentra/internal/modules/grammar"
	grammarcontract "github.com/fluentra/fluentra/internal/modules/grammar/contract"
	"github.com/fluentra/fluentra/internal/modules/learning"
	learningcontract "github.com/fluentra/fluentra/internal/modules/learning/contract"
	"github.com/fluentra/fluentra/internal/modules/lesson"
	"github.com/fluentra/fluentra/internal/platform/ai"
	"github.com/fluentra/fluentra/internal/shared/apperr"
	"github.com/fluentra/fluentra/internal/shared/clock"
)

// TestModule_WorkOrder19StageDGate verifies WO-19 Stage D Gate against real PostgreSQL:
//  1. cmd/foundation generation produces draft sets for nodes.
//  2. Publishing a topic with no approved exercises fails (ErrFoundationIncomplete).
//  3. At least the two acceptance chains of WO 16 (SENTENCE_STRUCTURE -> PRESENT_PERFECT,
//     SENTENCE_STRUCTURE -> ADVERBIAL_CLAUSES) are reviewed and published end to end.
//  4. Approving each foundation topic sets the node's cefr_level in content.taxonomies.
//  5. GET /foundation/topics/PRESENT_PERFECT returns body, counts, and prerequisites.
func TestModule_WorkOrder19StageDGate(t *testing.T) {
	if attemptPool == nil {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	ctx := context.Background()

	// Seed author and reviewer accounts to enforce BR-CONTENT-03 (no self-approval)
	authorID := uuid.New()
	reviewerID := uuid.New()
	for _, u := range []struct {
		id    uuid.UUID
		email string
	}{
		{authorID, "foundation-author-" + authorID.String() + "@example.test"},
		{reviewerID, "foundation-reviewer-" + reviewerID.String() + "@example.test"},
	} {
		_, err := attemptPool.Exec(ctx,
			`INSERT INTO core.users (id, email, status) VALUES ($1, $2, 'active')`,
			u.id, u.email)
		require.NoError(t, err)
	}

	contentMod := content.New(content.Deps{
		Pool:  attemptPool,
		Guard: allowAll{},
	})
	lessonMod := lesson.New(lesson.Deps{
		Pool:  attemptPool,
		Guard: allowAll{},
		Env:   "test",
	})
	grammarMod := grammar.New(grammar.Deps{
		Content: contentMod.Reader(),
	})

	graders := make(map[string]learningcontract.ExerciseGrader)
	for _, kind := range grammarcontract.GradedKinds() {
		graders[kind] = grammarMod.Grader()
	}
	graders[learningcontract.KindFoundationQuiz] = grammarMod.Grader()
	graders[learningcontract.KindFoundationReview] = grammarMod.Grader()

	aiClient := ai.NewMockProvider(nil)

	learningMod := learning.New(learning.Deps{
		Pool:              attemptPool,
		Lesson:            lessonMod.Reader(),
		LessonAuthor:      lessonMod.Author(),
		Content:           contentMod.Reader(),
		ContentAuthor:     contentMod.Author(),
		Taxonomies:        contentMod.TaxonomyResolver(),
		Graders:           graders,
		AI:                aiClient,
		Clock:             clock.Real{},
		GeneratorAuthorID: authorID,
	})

	gen := learningMod.Generator()

	// Acceptance chain nodes from WO 16:
	// Chain 1: SENTENCE_STRUCTURE -> PRESENT_SIMPLE -> PRESENT_CONTINUOUS -> PAST_SIMPLE -> PRESENT_PERFECT
	// Chain 2: SENTENCE_STRUCTURE -> RELATIVE_CLAUSES -> NOUN_CLAUSES -> ADVERBIAL_CLAUSES
	acceptanceNodes := []struct {
		code      string
		namespace string
		cefr      string
	}{
		{"SENTENCE_STRUCTURE", "grammar", "A1"},
		{"PRESENT_SIMPLE", "grammar", "A1"},
		{"PRESENT_CONTINUOUS", "grammar", "A1"},
		{"PAST_SIMPLE", "grammar", "A2"},
		{"PRESENT_PERFECT", "grammar", "B1"},
		{"RELATIVE_CLAUSES", "grammar", "B1"},
		{"NOUN_CLAUSES", "grammar", "B2"},
		{"ADVERBIAL_CLAUSES", "grammar", "B2"},
	}

	type nodeDrafts struct {
		topicItemID uuid.UUID
		exercises   []uuid.UUID
		quiz        uuid.UUID
		review      uuid.UUID
	}
	generatedDrafts := make(map[string]nodeDrafts)

	resolveItemID := func(verID uuid.UUID) uuid.UUID {
		var itemID uuid.UUID
		err := attemptPool.QueryRow(ctx,
			`SELECT item_id FROM content.content_versions WHERE id = $1`, verID).Scan(&itemID)
		require.NoError(t, err)
		return itemID
	}

	// 1. Generate drafts for all acceptance nodes
	for _, n := range acceptanceNodes {
		// A. 1x foundation_topic
		topicItems, err := gen.Generate(ctx, learningcontract.GenerateRequest{
			Kind:       learningcontract.KindFoundationTopic,
			CEFRLevel:  n.cefr,
			NodeCodes:  []string{n.code},
			Purpose:    "foundation",
			Count:      1,
			OwnerID:    &authorID,
			SlugPrefix: fmt.Sprintf("test-fd-topic-%s", strings.ToLower(strings.ReplaceAll(n.code, "_", "-"))),
		})
		require.NoError(t, err, "failed to generate topic for %s", n.code)
		require.Len(t, topicItems, 1)

		// B. 3x exercises
		exKinds := []string{
			"grammar_tense_choice",
			"grammar_sentence_transform",
			"grammar_tense_choice",
		}
		var exItemIDs []uuid.UUID
		for idx, ek := range exKinds {
			items, err := gen.Generate(ctx, learningcontract.GenerateRequest{
				Kind:       ek,
				CEFRLevel:  n.cefr,
				NodeCodes:  []string{n.code},
				Purpose:    "foundation",
				Count:      1,
				OwnerID:    &authorID,
				SlugPrefix: fmt.Sprintf("test-fd-ex-%s-%d", strings.ToLower(strings.ReplaceAll(n.code, "_", "-")), idx+1),
			})
			require.NoError(t, err, "failed to generate exercise %d for %s", idx+1, n.code)
			require.Len(t, items, 1)
			exItemIDs = append(exItemIDs, resolveItemID(items[0].ContentVersionID))
		}

		// C. 1x foundation_quiz
		quizItems, err := gen.Generate(ctx, learningcontract.GenerateRequest{
			Kind:       learningcontract.KindFoundationQuiz,
			CEFRLevel:  n.cefr,
			NodeCodes:  []string{n.code},
			Purpose:    "foundation",
			Count:      1,
			OwnerID:    &authorID,
			SlugPrefix: fmt.Sprintf("test-fd-quiz-%s", strings.ToLower(strings.ReplaceAll(n.code, "_", "-"))),
		})
		require.NoError(t, err, "failed to generate quiz for %s", n.code)
		require.Len(t, quizItems, 1)

		// D. 1x foundation_review
		reviewItems, err := gen.Generate(ctx, learningcontract.GenerateRequest{
			Kind:       learningcontract.KindFoundationReview,
			CEFRLevel:  n.cefr,
			NodeCodes:  []string{n.code},
			Purpose:    "foundation",
			Count:      1,
			OwnerID:    &authorID,
			SlugPrefix: fmt.Sprintf("test-fd-rev-%s", strings.ToLower(strings.ReplaceAll(n.code, "_", "-"))),
		})
		require.NoError(t, err, "failed to generate review for %s", n.code)
		require.Len(t, reviewItems, 1)

		generatedDrafts[n.code] = nodeDrafts{
			topicItemID: resolveItemID(topicItems[0].ContentVersionID),
			exercises:   exItemIDs,
			quiz:        resolveItemID(quizItems[0].ContentVersionID),
			review:      resolveItemID(reviewItems[0].ContentVersionID),
		}
	}

	contentSvc := contentMod.Service()

	// 2. Negative Gate: Publishing PRESENT_PERFECT topic without published exercises must fail
	ppDrafts := generatedDrafts["PRESENT_PERFECT"]
	_, err := contentSvc.SubmitForReview(ctx, authorID, ppDrafts.topicItemID)
	require.NoError(t, err)

	_, err = contentSvc.Review(ctx, reviewerID, ppDrafts.topicItemID, contentsvc.ReviewDecisionRequest{
		Decision: contentdomain.ReviewDecisionApproved,
	})
	require.NoError(t, err)

	_, err = contentSvc.Publish(ctx, reviewerID, ppDrafts.topicItemID)
	require.Error(t, err, "publishing topic without published exercises must fail")
	var ae *apperr.Error
	require.True(t, errors.As(err, &ae) && ae.Code == "FOUNDATION_INCOMPLETE",
		"expected FOUNDATION_INCOMPLETE, got: %v", err)

	// 3. Editorial Lifecycle: Approve and publish all exercises, quizzes, reviews, and topics
	// for both acceptance chains
	publishItem := func(itemID uuid.UUID) {
		_, err := contentSvc.SubmitForReview(ctx, authorID, itemID)
		require.NoError(t, err)
		_, err = contentSvc.Review(ctx, reviewerID, itemID, contentsvc.ReviewDecisionRequest{
			Decision: contentdomain.ReviewDecisionApproved,
		})
		require.NoError(t, err)
		ver, err := contentSvc.Publish(ctx, reviewerID, itemID)
		require.NoError(t, err)
		assert.Equal(t, contentdomain.StatusPublished, ver.Status)
	}

	for _, n := range acceptanceNodes {
		drafts := generatedDrafts[n.code]
		// Publish exercises
		for _, exID := range drafts.exercises {
			publishItem(exID)
		}
		// Publish quiz
		publishItem(drafts.quiz)
		// Publish review
		publishItem(drafts.review)

		// Now publish topic
		if n.code == "PRESENT_PERFECT" {
			// Already submitted and reviewed above, just publish
			ver, err := contentSvc.Publish(ctx, reviewerID, drafts.topicItemID)
			require.NoError(t, err)
			assert.Equal(t, contentdomain.StatusPublished, ver.Status)
		} else {
			publishItem(drafts.topicItemID)
		}

		// 4. Assert each node's cefr_level in content.taxonomies was updated
		var dbCEFR string
		err := attemptPool.QueryRow(ctx,
			`SELECT cefr_level FROM content.taxonomies WHERE namespace = $1 AND code = $2`,
			n.namespace, n.code).Scan(&dbCEFR)
		require.NoError(t, err)
		assert.Equal(t, n.cefr, dbCEFR, "node %s must have cefr_level set to %s", n.code, n.cefr)
	}

	// 5. Verify GetFoundationTopicByCode for PRESENT_PERFECT
	topicDetail, err := contentSvc.GetFoundationTopicByCode(ctx, "PRESENT_PERFECT")
	require.NoError(t, err)
	assert.NotEmpty(t, topicDetail.Body)
	assert.GreaterOrEqual(t, topicDetail.ExerciseCount, 3)
	assert.GreaterOrEqual(t, topicDetail.QuizCount, 1)
	assert.GreaterOrEqual(t, topicDetail.ReviewCount, 1)

	// Prerequisites must contain PAST_SIMPLE
	foundPrereq := false
	for _, pr := range topicDetail.Prerequisites {
		if pr.Code == "PAST_SIMPLE" {
			foundPrereq = true
			break
		}
	}
	assert.True(t, foundPrereq, "PRESENT_PERFECT must have PAST_SIMPLE in prerequisites")

	// 6. HTTP API Verification: GET /foundation/topics/PRESENT_PERFECT
	router := chi.NewRouter()
	contentMod.Routes(router)

	rec := httptest.NewRecorder()
	httpReq := httptest.NewRequest(http.MethodGet, "/foundation/topics/PRESENT_PERFECT", nil)
	router.ServeHTTP(rec, httpReq)

	require.Equal(t, http.StatusOK, rec.Code)

	var httpResp struct {
		Code          string `json:"code"`
		Namespace     string `json:"namespace"`
		Label         string `json:"label"`
		CEFRLevel     string `json:"cefr_level"`
		ExerciseCount int    `json:"exercise_count"`
		QuizCount     int    `json:"quiz_count"`
		ReviewCount   int    `json:"review_count"`
		Prerequisites []struct {
			Code string `json:"code"`
		} `json:"prerequisites"`
		Body struct {
			SchemaVersion int               `json:"schema_version"`
			Objective     string            `json:"objective"`
			Explanation   map[string]string `json:"explanation"`
			Examples      []any             `json:"examples"`
		} `json:"body"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &httpResp))
	assert.Equal(t, "PRESENT_PERFECT", httpResp.Code)
	assert.Equal(t, "B1", httpResp.CEFRLevel)
	assert.GreaterOrEqual(t, httpResp.ExerciseCount, 3)
	assert.GreaterOrEqual(t, httpResp.QuizCount, 1)
	assert.GreaterOrEqual(t, httpResp.ReviewCount, 1)

	foundHTTPPrereq := false
	for _, pr := range httpResp.Prerequisites {
		if pr.Code == "PAST_SIMPLE" {
			foundHTTPPrereq = true
			break
		}
	}
	assert.True(t, foundHTTPPrereq, "HTTP response prerequisites must contain PAST_SIMPLE")
	assert.Equal(t, 1, httpResp.Body.SchemaVersion)
	assert.NotEmpty(t, httpResp.Body.Objective)
	assert.NotEmpty(t, httpResp.Body.Explanation["en"])
	assert.NotEmpty(t, httpResp.Body.Explanation["vi"])
	assert.NotEmpty(t, httpResp.Body.Examples)
}
