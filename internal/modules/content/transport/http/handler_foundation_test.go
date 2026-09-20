package http_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/fluentra/fluentra/internal/modules/content/domain"
	"github.com/fluentra/fluentra/internal/modules/content/service"
	contenthttp "github.com/fluentra/fluentra/internal/modules/content/transport/http"
	"github.com/fluentra/fluentra/internal/shared/apperr"
	"github.com/fluentra/fluentra/internal/shared/httpx"
)

func TestListFoundationTopicsHandler(t *testing.T) {
	t.Parallel()

	topicID := uuid.New()
	levelB1 := "B1"
	svc := &mockContentService{
		listFoundationTopicsFn: func(_ context.Context, filter service.FoundationTopicFilter) (
			[]domain.Taxonomy, int64, error,
		) {
			if filter.Namespace != nil && *filter.Namespace != "grammar" {
				return nil, 0, nil
			}
			return []domain.Taxonomy{
				{
					ID:          topicID,
					Namespace:   domain.NamespaceGrammar,
					Code:        codePresentPerfect,
					Label:       "Present Perfect",
					Description: "Express experience and unfinished actions.",
					CEFRLevel:   &levelB1,
					Position:    10,
					CreatedAt:   time.Now(),
					UpdatedAt:   time.Now(),
				},
			}, 1, nil
		},
	}

	router := setupTestRouter(svc, &mockGuard{})

	req := httptest.NewRequest(http.MethodGet, "/foundation/topics?namespace=grammar", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp contenthttp.FoundationTopicListResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Total != 1 || len(resp.Items) != 1 {
		t.Fatalf("expected 1 item, got %d (total %d)", len(resp.Items), resp.Total)
	}
	if resp.Items[0].Code != codePresentPerfect {
		t.Fatalf("expected code PRESENT_PERFECT, got %s", resp.Items[0].Code)
	}
}

func TestGetFoundationTopicHandler(t *testing.T) {
	t.Parallel()

	topicID := uuid.New()
	levelB1 := "B1"
	svc := &mockContentService{
		getFoundationTopicFn: func(_ context.Context, code string) (service.FoundationTopicDetail, error) {
			if code == codePresentPerfect {
				return service.FoundationTopicDetail{
					Topic: domain.Taxonomy{
						ID:          topicID,
						Namespace:   domain.NamespaceGrammar,
						Code:        codePresentPerfect,
						Label:       "Present Perfect",
						Description: "Express experience and unfinished actions.",
						CEFRLevel:   &levelB1,
						Position:    10,
						CreatedAt:   time.Now(),
						UpdatedAt:   time.Now(),
					},
					Prerequisites: []domain.Taxonomy{},
					Dependants:    []domain.Taxonomy{},
					Related:       []string{"PAST_SIMPLE"},
					ExerciseCount: 2,
					QuizCount:     1,
					ReviewCount:   1,
				}, nil
			}
			return service.FoundationTopicDetail{}, domain.ErrTaxonomyNodeNotFound
		},
	}

	router := setupTestRouter(svc, &mockGuard{})

	t.Run("found", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/foundation/topics/PRESENT_PERFECT", nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}

		var resp contenthttp.FoundationTopicDetailResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if resp.Code != codePresentPerfect {
			t.Fatalf("expected PRESENT_PERFECT, got %s", resp.Code)
		}
		if resp.ExerciseCount != 2 {
			t.Fatalf("expected 2 exercises, got %d", resp.ExerciseCount)
		}
	})

	t.Run("not found", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/foundation/topics/UNKNOWN", nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Fatalf("expected 404, got %d", rec.Code)
		}
	})
}

func TestGetFoundationPathHandler(t *testing.T) {
	t.Parallel()

	svc := &mockContentService{
		getFoundationPathFn: func(_ context.Context, _ *string, _ *string) ([]domain.Taxonomy, error) {
			return []domain.Taxonomy{
				{
					ID:        uuid.New(),
					Namespace: domain.NamespaceGrammar,
					Code:      "SENTENCE_STRUCTURE",
					Label:     "Sentence Structure",
					Position:  1,
				},
				{
					ID:        uuid.New(),
					Namespace: domain.NamespaceGrammar,
					Code:      "PRESENT_SIMPLE",
					Label:     "Present Simple",
					Position:  2,
				},
			}, nil
		},
	}

	router := setupTestRouter(svc, &mockGuard{})

	req := httptest.NewRequest(http.MethodGet, "/foundation/path?target=PRESENT_SIMPLE", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp contenthttp.FoundationPathResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(resp.Items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(resp.Items))
	}
	if *resp.Target != "PRESENT_SIMPLE" {
		t.Fatalf("expected target PRESENT_SIMPLE, got %v", resp.Target)
	}
}

func TestCreateFoundationTopicHandler(t *testing.T) {
	t.Parallel()

	adminID := uuid.New()
	levelB1 := "B1"
	topicID := uuid.New()

	svc := &mockContentService{
		createFoundationTopicFn: func(_ context.Context, actorID uuid.UUID, req service.CreateFoundationTopicRequest) (
			domain.Taxonomy, error,
		) {
			if actorID != adminID {
				t.Fatalf("expected actor %s, got %s", adminID, actorID)
			}
			return domain.Taxonomy{
				ID:          topicID,
				Namespace:   req.Namespace,
				Code:        req.Code,
				Label:       req.Label,
				Description: req.Description,
				CEFRLevel:   &levelB1,
				Position:    req.Position,
				CreatedAt:   time.Now(),
				UpdatedAt:   time.Now(),
			}, nil
		},
	}

	payload := `{"namespace":"grammar","code":"PRESENT_PERFECT","label":"Present Perfect",` +
		`"description":"desc","cefr_level":"B1","position":10}`

	t.Run("unauthenticated", func(t *testing.T) {
		router := setupTestRouter(svc, &mockGuard{})
		req := httptest.NewRequest(http.MethodPost, "/admin/foundation/topics", bytes.NewBufferString(payload))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d", rec.Code)
		}
	})

	t.Run("forbidden", func(t *testing.T) {
		router := setupTestRouter(svc, &mockGuard{deniedPermission: contenthttp.PermContentCreate})
		req := httptest.NewRequest(http.MethodPost, "/admin/foundation/topics", bytes.NewBufferString(payload))
		req.Header.Set("Content-Type", "application/json")
		ctx := httpx.WithActor(req.Context(), httpx.Actor{UserID: adminID, Role: roleAdmin})
		req = req.WithContext(ctx)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusForbidden {
			t.Fatalf("expected 403, got %d", rec.Code)
		}
	})

	t.Run("success", func(t *testing.T) {
		router := setupTestRouter(svc, &mockGuard{})
		req := httptest.NewRequest(http.MethodPost, "/admin/foundation/topics", bytes.NewBufferString(payload))
		req.Header.Set("Content-Type", "application/json")
		ctx := httpx.WithActor(req.Context(), httpx.Actor{UserID: adminID, Role: roleAdmin})
		req = req.WithContext(ctx)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusCreated {
			t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
		}

		var resp contenthttp.FoundationTopicResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if resp.Code != codePresentPerfect {
			t.Fatalf("expected PRESENT_PERFECT, got %s", resp.Code)
		}
	})
}

func TestReplacePrerequisitesHandler(t *testing.T) {
	t.Parallel()

	adminID := uuid.New()
	topicID := uuid.New()

	svc := &mockContentService{
		replacePrerequisitesFn: func(_ context.Context, _ uuid.UUID, _ string, requiresCodes []string) error {
			if len(requiresCodes) > 0 && requiresCodes[0] == "CYCLE" {
				return apperr.New(apperr.Validation, "TAXONOMY_CYCLE", "Cycle detected")
			}
			return nil
		},
		getFoundationTopicFn: func(_ context.Context, code string) (service.FoundationTopicDetail, error) {
			return service.FoundationTopicDetail{
				Topic: domain.Taxonomy{
					ID:        topicID,
					Namespace: domain.NamespaceGrammar,
					Code:      code,
					Label:     "Topic",
				},
				Prerequisites: []domain.Taxonomy{},
				Dependants:    []domain.Taxonomy{},
				Related:       []string{},
			}, nil
		},
	}

	router := setupTestRouter(svc, &mockGuard{})

	t.Run("cycle returns 422", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPut, "/admin/foundation/topics/PRESENT_PERFECT/prerequisites",
			bytes.NewBufferString(`{"requires_codes":["CYCLE"]}`))
		req.Header.Set("Content-Type", "application/json")
		ctx := httpx.WithActor(req.Context(), httpx.Actor{UserID: adminID, Role: roleAdmin})
		req = req.WithContext(ctx)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnprocessableEntity {
			t.Fatalf("expected 422, got %d: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("valid replaces prerequisites and returns detail", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPut, "/admin/foundation/topics/PRESENT_PERFECT/prerequisites",
			bytes.NewBufferString(`{"requires_codes":["PAST_SIMPLE"]}`))
		req.Header.Set("Content-Type", "application/json")
		ctx := httpx.WithActor(req.Context(), httpx.Actor{UserID: adminID, Role: roleAdmin})
		req = req.WithContext(ctx)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}
	})
}
