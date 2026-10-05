package service_test

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"

	contentcontract "github.com/fluentra/fluentra/internal/modules/content/contract"
	"github.com/fluentra/fluentra/internal/modules/resource/contract"
	"github.com/fluentra/fluentra/internal/modules/resource/domain"
	"github.com/fluentra/fluentra/internal/modules/resource/service"
	"github.com/fluentra/fluentra/internal/platform/ai"
)

const (
	classificationPrompt    = "resource_classify.v2"
	oldClassificationPrompt = "resource_classify.v1"
	classificationSkill     = "grammar"
	classificationNode      = "PRESENT_PERFECT"
)

type classificationAI struct {
	text  string
	calls int
}

func (c *classificationAI) Complete(_ context.Context, req ai.Request) (ai.Response, error) {
	c.calls++
	if req.Task != ai.TaskResourceClassify {
		return ai.Response{}, errors.New("unexpected AI task")
	}
	return ai.Response{Text: c.text, Model: "mock"}, nil
}

type classificationTaxonomies struct {
	contentcontract.TaxonomyResolver
}

func (classificationTaxonomies) ListTaxonomiesInNamespace(
	_ context.Context, namespace string,
) ([]contentcontract.TaxonomyNode, error) {
	if namespace != classificationSkill {
		return nil, nil
	}
	return []contentcontract.TaxonomyNode{{Code: classificationNode, Namespace: classificationSkill}}, nil
}

func TestClassifyResource_RequiresSuitabilityAndClearsUnsuitableTags(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name     string
		response string
		wantErr  bool
		suitable bool
	}{
		{"suitable grounded material", `{"learning_suitable":true,"cefr_estimate":"B1","skill":"grammar",
			"node_codes":["PRESENT_PERFECT","UNKNOWN_CODE"]}`, false, true},
		{"unsuitable ignores returned tags", `{"learning_suitable":false,"cefr_estimate":"B1","skill":"grammar",
			"node_codes":["PRESENT_PERFECT"]}`, false, false},
		{"missing suitability", `{"cefr_estimate":"B1","skill":"grammar",
			"node_codes":["PRESENT_PERFECT"]}`, true, false},
		{"null suitability", `{"learning_suitable":null,"cefr_estimate":"B1","skill":"grammar",
			"node_codes":["PRESENT_PERFECT"]}`, true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := newMockRepo()
			resourceID := uuid.New()
			text := "The present perfect connects a past action to the present."
			repo.extractions[resourceID] = &contract.Extraction{ResourceID: resourceID, Text: text, CharCount: len(text)}
			client := &classificationAI{text: tc.response}
			svc := service.New(repo, newMockStorage(), nil, nil, nil, nil)
			svc.SetAIClient(client)
			svc.SetTaxonomies(classificationTaxonomies{})

			err := svc.ClassifyResource(context.Background(), resourceID)
			if (err != nil) != tc.wantErr {
				t.Fatalf("ClassifyResource error = %v, wantErr %v", err, tc.wantErr)
			}
			classification := repo.classifications[resourceID]
			if tc.wantErr {
				if classification != nil {
					t.Fatal("missing suitability was persisted as a classification")
				}
				return
			}
			if classification == nil || classification.PromptVersion != classificationPrompt {
				t.Fatalf("classification = %+v, want current v2 metadata", classification)
			}
			if suitable := classification.LearningSuitable(); suitable == nil || *suitable != tc.suitable {
				t.Fatalf("learning suitability = %v, want %v", suitable, tc.suitable)
			}
			assertClassificationTags(t, classification, tc.suitable)
		})
	}
}

func assertClassificationTags(t *testing.T, classification *contract.Classification, suitable bool) {
	t.Helper()
	if suitable {
		if classification.CEFREstimate == nil || *classification.CEFREstimate != "B1" ||
			classification.Skill == nil || *classification.Skill != classificationSkill ||
			len(classification.NodeCodes) != 1 || classification.NodeCodes[0] != classificationNode {
			t.Fatalf("classification did not retain only grounded learning tags: %+v", classification)
		}
	} else if classification.CEFREstimate != nil || classification.Skill != nil || len(classification.NodeCodes) != 0 {
		t.Fatalf("unsuitable material retained learning tags: %+v", classification)
	}
}

func TestClassifyResource_RechecksOldPromptAndReusesCurrentPrompt(t *testing.T) {
	t.Parallel()
	for _, version := range []string{oldClassificationPrompt, classificationPrompt} {
		t.Run(version, func(t *testing.T) {
			repo := newMockRepo()
			resourceID := uuid.New()
			previous := &contract.Classification{ResourceID: resourceID, PromptVersion: version}
			repo.classifications[resourceID] = previous
			text := "Invoice total: 120 euros."
			repo.extractions[resourceID] = &contract.Extraction{ResourceID: resourceID, Text: text, CharCount: len(text)}
			client := &classificationAI{text: `{"learning_suitable":false,"cefr_estimate":"","skill":"","node_codes":[]}`}
			svc := service.New(repo, newMockStorage(), nil, nil, nil, nil)
			svc.SetAIClient(client)

			if err := svc.ClassifyResource(context.Background(), resourceID); err != nil {
				t.Fatal(err)
			}
			if version == oldClassificationPrompt {
				if client.calls != 1 || repo.classifications[resourceID].PromptVersion != classificationPrompt {
					t.Fatal("old prompt metadata was not reclassified")
				}
			} else if client.calls != 0 || repo.classifications[resourceID] != previous {
				t.Fatal("current classification was not reused")
			}
		})
	}
}

type revalidationEnqueuer struct {
	calls int
}

func (e *revalidationEnqueuer) EnqueueTx(
	context.Context, pgx.Tx, river.JobArgs, *river.InsertOpts,
) (*rivertype.JobInsertResult, error) {
	e.calls++
	return nil, errors.New("unexpected enqueue")
}

func TestRevalidateResource_ForeignOwnerCannotClearClassificationOrEnqueue(t *testing.T) {
	t.Parallel()
	repo := newMockRepo()
	resourceID, ownerID := uuid.New(), uuid.New()
	repo.resources[resourceID] = &contract.Resource{ID: resourceID, UserID: ownerID, Status: domain.StatusValidated}
	repo.extractions[resourceID] = &contract.Extraction{Text: "Present perfect grammar practice.", CharCount: 33}
	classification := &contract.Classification{ResourceID: resourceID, PromptVersion: oldClassificationPrompt}
	repo.classifications[resourceID] = classification
	enqueuer := &revalidationEnqueuer{}
	svc := service.New(repo, newMockStorage(), nil, enqueuer, nil, nil)

	err := svc.RevalidateResource(context.Background(), uuid.New(), resourceID)
	if !errors.Is(err, domain.ErrResourceNotFound) {
		t.Fatalf("error = %v, want resource not found", err)
	}
	if enqueuer.calls != 0 || repo.classificationDeletes != 0 || repo.classifications[resourceID] != classification {
		t.Fatal("foreign revalidation changed classification or enqueued work")
	}
}

func TestListResources_BatchesAllRenditionStatesForOwnedResources(t *testing.T) {
	t.Parallel()
	repo := newMockRepo()
	userID := uuid.New()
	owned := []uuid.UUID{uuid.New(), uuid.New()}
	for i, id := range owned {
		key := "uploads/" + id.String() + ".pdf"
		repo.resources[id] = &contract.Resource{
			ID: id, UserID: userID, Kind: domain.KindFile, Status: domain.StatusValidated, ObjectKey: &key,
		}
		statuses := []string{domain.RenditionStatusReady, domain.RenditionStatusPending}
		if i == 1 {
			statuses = []string{domain.RenditionStatusFailed, domain.RenditionStatusSkipped}
		}
		for _, status := range statuses {
			renditionID := uuid.New()
			repo.renditions[renditionID] = &contract.Rendition{ID: renditionID, ResourceID: id, Status: status}
		}
	}
	foreignID := uuid.New()
	repo.resources[foreignID] = &contract.Resource{
		ID: foreignID, UserID: uuid.New(), Kind: domain.KindFile, Status: domain.StatusValidated,
	}
	foreignRendition := uuid.New()
	repo.renditions[foreignRendition] = &contract.Rendition{ResourceID: foreignID, Status: domain.RenditionStatusReady}
	svc := service.New(repo, newMockStorage(), nil, nil, nil, nil)

	list, err := svc.ListResources(context.Background(), userID, nil, nil, 1, 20)
	if err != nil {
		t.Fatal(err)
	}
	if list.Total != 2 || len(list.Items) != 2 || repo.renditionBatchReads != 1 || repo.renditionSingleReads != 0 {
		t.Fatalf("list should use one batched read: total=%d, items=%d, batches=%d, singles=%d",
			list.Total, len(list.Items), repo.renditionBatchReads, repo.renditionSingleReads)
	}
	for _, item := range list.Items {
		if item.ID == foreignID || len(item.Renditions) != 2 {
			t.Fatalf("list omitted states or included a foreign resource: %+v", item)
		}
		detail, err := svc.GetResource(context.Background(), item.ID, userID)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(renditionStates(item.Renditions), renditionStates(detail.Renditions)) {
			t.Fatalf("list/detail rendition states differ for %s: list=%+v, detail=%+v",
				item.ID, item.Renditions, detail.Renditions)
		}
	}
}

func renditionStates(renditions []contract.Rendition) map[uuid.UUID]map[string]int {
	states := map[uuid.UUID]map[string]int{}
	for _, rendition := range renditions {
		if states[rendition.ResourceID] == nil {
			states[rendition.ResourceID] = map[string]int{}
		}
		states[rendition.ResourceID][rendition.Status]++
	}
	return states
}
