// Package http implements the HTTP transport handlers and DTOs for the content module.
package http

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"

	"github.com/fluentra/fluentra/internal/modules/content/contract"
	"github.com/fluentra/fluentra/internal/modules/content/domain"
	"github.com/fluentra/fluentra/internal/modules/content/service"
)

// TaxonomyTagResponse describes a taxonomy tag attached to content.
type TaxonomyTagResponse struct {
	Namespace string `json:"namespace"`
	Code      string `json:"code"`
	Label     string `json:"label"`
}

// ContentVersionResponse serializes an immutable content version.
type ContentVersionResponse struct {
	ID          uuid.UUID             `json:"id"`
	ItemID      uuid.UUID             `json:"item_id"`
	Version     int                   `json:"version"`
	Kind        string                `json:"kind"`
	Body        json.RawMessage       `json:"body"`
	CEFRLevel   string                `json:"cefr_level"`
	Status      string                `json:"status"`
	MediaRefs   []string              `json:"media_refs"`
	Tags        []TaxonomyTagResponse `json:"tags"`
	PublishedAt *time.Time            `json:"published_at,omitempty"`
}

// ContentVersionListResponse is the paginated response for browsing content.
type ContentVersionListResponse struct {
	Items []ContentVersionResponse `json:"items"`
	Total int                      `json:"total"`
}

// ContentItemResponse serializes a content item record.
type ContentItemResponse struct {
	ID               uuid.UUID  `json:"id"`
	Kind             string     `json:"kind"`
	Slug             string     `json:"slug"`
	CurrentVersionID *uuid.UUID `json:"current_version_id,omitempty"`
	Status           string     `json:"status"`
	OwnerID          uuid.UUID  `json:"owner_id"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
}

// CreateContentItemRequest payload for creating a new material item.
type CreateContentItemRequest struct {
	Kind      string          `json:"kind"`
	Slug      string          `json:"slug"`
	CEFRLevel string          `json:"cefr_level"`
	Body      json.RawMessage `json:"body"`
	Tags      []string        `json:"tags,omitempty"`
}

// UpdateDraftRequest payload for editing an existing draft version.
type UpdateDraftRequest struct {
	CEFRLevel *string         `json:"cefr_level,omitempty"`
	Body      json.RawMessage `json:"body"`
	Tags      []string        `json:"tags,omitempty"`
}

// ReviewDecisionRequest payload for submitting an editorial review decision.
type ReviewDecisionRequest struct {
	Decision string  `json:"decision"`
	Comments *string `json:"comments,omitempty"`
}

func toContentItemResponse(item domain.Item) ContentItemResponse {
	return ContentItemResponse{
		ID:               item.ID,
		Kind:             item.Kind,
		Slug:             item.Slug,
		CurrentVersionID: item.CurrentVersionID,
		Status:           string(item.Status),
		OwnerID:          item.OwnerID,
		CreatedAt:        item.CreatedAt,
		UpdatedAt:        item.UpdatedAt,
	}
}

func toContentVersionResponse(v *contract.Version) ContentVersionResponse {
	if v == nil {
		return ContentVersionResponse{}
	}
	tags := make([]TaxonomyTagResponse, len(v.Tags))
	for i, t := range v.Tags {
		tags[i] = TaxonomyTagResponse{
			Namespace: "topic",
			Code:      t,
			Label:     t,
		}
	}
	refs := v.MediaRefs
	if refs == nil {
		refs = []string{}
	}
	return ContentVersionResponse{
		ID:          v.ID,
		ItemID:      v.ItemID,
		Version:     v.Version,
		Kind:        v.Kind,
		Body:        v.Body,
		CEFRLevel:   v.CEFRLevel,
		Status:      v.Status,
		MediaRefs:   refs,
		Tags:        tags,
		PublishedAt: v.PublishedAt,
	}
}

func toDomainVersionResponse(v domain.Version) ContentVersionResponse {
	refs := v.MediaRefs
	if refs == nil {
		refs = []string{}
	}
	return ContentVersionResponse{
		ID:          v.ID,
		ItemID:      v.ItemID,
		Version:     v.Version,
		Kind:        v.Kind,
		Body:        v.Body,
		CEFRLevel:   v.CEFRLevel,
		Status:      string(v.Status),
		MediaRefs:   refs,
		Tags:        []TaxonomyTagResponse{},
		PublishedAt: v.PublishedAt,
	}
}

// AdminContentItemListResponse is the response for GET /admin/content.
//
// limit and offset are echoed because AdminContentItemList declares them
// required. A paginated list that does not say which page it is has to be
// counted by the caller, and the two sibling admin lists already echo them.
type AdminContentItemListResponse struct {
	Items  []ContentItemResponse `json:"items"`
	Total  int                   `json:"total"`
	Limit  int                   `json:"limit"`
	Offset int                   `json:"offset"`
}

// AdminContentItemDetailResponse is the response for GET /admin/content/{id}.
//
// The item's fields are inline rather than nested under an `item` key, which is
// what AdminContentItemDetail declares: an item plus its version history, not a
// wrapper around one. The first version nested them, and every field the detail
// screen reads — status, kind, and the id it posts every transition to — was
// undefined in the browser while both sides compiled.
type AdminContentItemDetailResponse struct {
	ContentItemResponse
	Versions []ContentVersionResponse `json:"versions"`
}

// ItemReportResponse serializes an item report.
type ItemReportResponse struct {
	ID               uuid.UUID `json:"id"`
	ContentVersionID uuid.UUID `json:"content_version_id"`
	UserID           uuid.UUID `json:"user_id"`
	Reason           string    `json:"reason"`
	Note             *string   `json:"note,omitempty"`
	CreatedAt        time.Time `json:"created_at"`
}

// ReportedContentVersionResponse serializes a reported version summary.
type ReportedContentVersionResponse struct {
	ContentVersionID uuid.UUID `json:"content_version_id"`
	ItemID           uuid.UUID `json:"item_id"`
	Slug             string    `json:"slug"`
	Kind             string    `json:"kind"`
	CEFRLevel        string    `json:"cefr_level"`
	ItemStatus       string    `json:"item_status"`
	ReportCount      int       `json:"report_count"`
	LastReportedAt   time.Time `json:"last_reported_at"`
}

// ReportedContentListResponse is the paginated response for GET /admin/content/reports.
type ReportedContentListResponse struct {
	Items  []ReportedContentVersionResponse `json:"items"`
	Total  int                              `json:"total"`
	Limit  int                              `json:"limit"`
	Offset int                              `json:"offset"`
}

// FoundationTopicResponse serializes a knowledge spine taxonomy entry.
type FoundationTopicResponse struct {
	ID           uuid.UUID  `json:"id"`
	Namespace    string     `json:"namespace"`
	Code         string     `json:"code"`
	Label        string     `json:"label"`
	Description  string     `json:"description"`
	CEFRLevel    *string    `json:"cefr_level"`
	ParentID     *uuid.UUID `json:"parent_id"`
	Position     int        `json:"position"`
	DeprecatedAt *time.Time `json:"deprecated_at"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

// FoundationTopicDetailResponse serializes detailed topic information.
type FoundationTopicDetailResponse struct {
	ID            uuid.UUID                 `json:"id"`
	Namespace     string                    `json:"namespace"`
	Code          string                    `json:"code"`
	Label         string                    `json:"label"`
	Description   string                    `json:"description"`
	CEFRLevel     *string                   `json:"cefr_level"`
	ParentID      *uuid.UUID                `json:"parent_id"`
	Position      int                       `json:"position"`
	DeprecatedAt  *time.Time                `json:"deprecated_at"`
	CreatedAt     time.Time                 `json:"created_at"`
	UpdatedAt     time.Time                 `json:"updated_at"`
	Body          json.RawMessage           `json:"body"`
	Prerequisites []FoundationTopicResponse `json:"prerequisites"`
	Dependants    []FoundationTopicResponse `json:"dependants"`
	Related       []string                  `json:"related"`
	ExerciseCount int                       `json:"exercise_count"`
	QuizCount     int                       `json:"quiz_count"`
	ReviewCount   int                       `json:"review_count"`
}

// FoundationTopicListResponse is the paginated response for GET /foundation/topics.
type FoundationTopicListResponse struct {
	Items  []FoundationTopicResponse `json:"items"`
	Total  int                       `json:"total"`
	Limit  int                       `json:"limit"`
	Offset int                       `json:"offset"`
}

// FoundationPathResponse is the response for GET /foundation/path.
type FoundationPathResponse struct {
	Target    *string                   `json:"target"`
	Namespace string                    `json:"namespace"`
	Items     []FoundationTopicResponse `json:"items"`
}

// CreateFoundationTopicRequest payload for creating a spine node.
type CreateFoundationTopicRequest struct {
	Namespace   string     `json:"namespace"`
	Code        string     `json:"code"`
	Label       string     `json:"label"`
	Description string     `json:"description"`
	CEFRLevel   *string    `json:"cefr_level"`
	ParentID    *uuid.UUID `json:"parent_id"`
	Position    int        `json:"position"`
}

// UpdateFoundationTopicRequest payload for updating topic metadata.
type UpdateFoundationTopicRequest struct {
	Label       *string    `json:"label"`
	Description *string    `json:"description"`
	CEFRLevel   *string    `json:"cefr_level"`
	ParentID    *uuid.UUID `json:"parent_id"`
	Position    *int       `json:"position"`
	Deprecated  *bool      `json:"deprecated"`
}

// ReplacePrerequisitesRequest payload for replacing prerequisite edges.
type ReplacePrerequisitesRequest struct {
	RequiresCodes []string `json:"requires_codes"`
}

func toFoundationTopicResponse(t domain.Taxonomy) FoundationTopicResponse {
	return FoundationTopicResponse{
		ID:           t.ID,
		Namespace:    t.Namespace,
		Code:         t.Code,
		Label:        t.Label,
		Description:  t.Description,
		CEFRLevel:    t.CEFRLevel,
		ParentID:     t.ParentID,
		Position:     t.Position,
		DeprecatedAt: t.DeprecatedAt,
		CreatedAt:    t.CreatedAt,
		UpdatedAt:    t.UpdatedAt,
	}
}

func toFoundationTopicResponses(list []domain.Taxonomy) []FoundationTopicResponse {
	if list == nil {
		return []FoundationTopicResponse{}
	}
	res := make([]FoundationTopicResponse, len(list))
	for i, t := range list {
		res[i] = toFoundationTopicResponse(t)
	}
	return res
}

func toFoundationTopicDetailResponse(detail service.FoundationTopicDetail) FoundationTopicDetailResponse {
	var body json.RawMessage
	if len(detail.Body) > 0 {
		body = detail.Body
	}
	prereqs := toFoundationTopicResponses(detail.Prerequisites)
	dependants := toFoundationTopicResponses(detail.Dependants)
	related := detail.Related
	if related == nil {
		related = []string{}
	}
	return FoundationTopicDetailResponse{
		ID:            detail.Topic.ID,
		Namespace:     detail.Topic.Namespace,
		Code:          detail.Topic.Code,
		Label:         detail.Topic.Label,
		Description:   detail.Topic.Description,
		CEFRLevel:     detail.Topic.CEFRLevel,
		ParentID:      detail.Topic.ParentID,
		Position:      detail.Topic.Position,
		DeprecatedAt:  detail.Topic.DeprecatedAt,
		CreatedAt:     detail.Topic.CreatedAt,
		UpdatedAt:     detail.Topic.UpdatedAt,
		Body:          body,
		Prerequisites: prereqs,
		Dependants:    dependants,
		Related:       related,
		ExerciseCount: detail.ExerciseCount,
		QuizCount:     detail.QuizCount,
		ReviewCount:   detail.ReviewCount,
	}
}

// AdminReviewQueueItemResponse represents a review queue item for backoffice staff.
type AdminReviewQueueItemResponse struct {
	ID               uuid.UUID       `json:"id"`
	ItemID           uuid.UUID       `json:"item_id"`
	Slug             string          `json:"slug"`
	Kind             string          `json:"kind"`
	CEFRLevel        string          `json:"cefr_level"`
	Status           string          `json:"status"`
	Body             json.RawMessage `json:"body"`
	BlindSolveAnswer any             `json:"blind_solve_answer,omitempty"`
	CEFRReasoning    string          `json:"cefr_reasoning,omitempty"`
	Provenance       any             `json:"provenance,omitempty"`
	NodeCodes        []string        `json:"node_codes"`
	CreatedAt        time.Time       `json:"created_at"`
}

// AdminReviewQueueResponse is the paginated response for GET /admin/review-queue.
type AdminReviewQueueResponse struct {
	Items []AdminReviewQueueItemResponse `json:"items"`
	Total int64                          `json:"total"`
}

func toAdminReviewQueueItemResponse(item domain.ReviewQueueItem) AdminReviewQueueItemResponse {
	resp := AdminReviewQueueItemResponse{
		ID:        item.ID,
		ItemID:    item.ItemID,
		Slug:      item.Slug,
		Kind:      item.Kind,
		CEFRLevel: item.CEFRLevel,
		Status:    string(item.Status),
		Body:      item.Body,
		NodeCodes: item.NodeCodes,
		CreatedAt: item.CreatedAt,
	}
	if resp.NodeCodes == nil {
		resp.NodeCodes = []string{}
	}

	var decoded map[string]any
	if err := json.Unmarshal(item.Body, &decoded); err == nil {
		if prov, ok := decoded["_provenance"].(map[string]any); ok {
			resp.Provenance = prov
			if bsa, ok := prov["blind_solve_answer"]; ok && bsa != nil {
				switch v := bsa.(type) {
				case map[string]any:
					resp.BlindSolveAnswer = v
				default:
					resp.BlindSolveAnswer = map[string]any{"answer": v}
				}
			}
			if cr, ok := prov["cefr_reasoning"].(string); ok {
				resp.CEFRReasoning = cr
			}
		}
	}

	return resp
}

// AdminReviewBatchResponse is one generation run awaiting review.
type AdminReviewBatchResponse struct {
	Batch     string    `json:"batch"`
	ItemCount int64     `json:"item_count"`
	Kinds     []string  `json:"kinds"`
	CreatedAt time.Time `json:"created_at"`
}

// AdminReviewBatchListResponse is the paginated response for
// GET /admin/review-queue/batches.
type AdminReviewBatchListResponse struct {
	Items []AdminReviewBatchResponse `json:"items"`
	Total int64                      `json:"total"`
}

// AdminApproveBatchRequest is the body of
// POST /admin/review-queue/batches/{id}/approve.
type AdminApproveBatchRequest struct {
	// Reject lists version ids in the batch to leave for a person. Every other
	// item in the batch is approved.
	Reject []uuid.UUID `json:"reject,omitempty"`
	Note   *string     `json:"note,omitempty"`
}

// AdminApproveBatchResponse reports how many versions the batch approval
// published.
type AdminApproveBatchResponse struct {
	Approved int `json:"approved"`
}

// AdminReviewSampleResponse is one auto-published item drawn for spot-check.
type AdminReviewSampleResponse struct {
	VersionID uuid.UUID `json:"version_id"`
	Batch     string    `json:"batch"`
	Kind      string    `json:"kind"`
	CEFRLevel string    `json:"cefr_level"`
	SampledOn time.Time `json:"sampled_on"`
	CreatedAt time.Time `json:"created_at"`
}

// AdminReviewSampleListResponse is the paginated response for
// GET /admin/review-queue/samples.
type AdminReviewSampleListResponse struct {
	Items []AdminReviewSampleResponse `json:"items"`
	Total int64                       `json:"total"`
}

// AdminDecideSampleRequest is the body of
// POST /admin/review-queue/samples/{id}/decide.
type AdminDecideSampleRequest struct {
	// Decision is "kept" (the item stands) or "rejected" (it is unpublished).
	Decision string  `json:"decision"`
	Note     *string `json:"note,omitempty"`
}

func toAdminReviewSampleResponse(sample domain.ReviewSample) AdminReviewSampleResponse {
	return AdminReviewSampleResponse{
		VersionID: sample.VersionID,
		Batch:     sample.Batch,
		Kind:      sample.Kind,
		CEFRLevel: sample.CEFRLevel,
		SampledOn: sample.SampledOn,
		CreatedAt: sample.CreatedAt,
	}
}

func toAdminReviewBatchResponse(batch domain.ReviewBatch) AdminReviewBatchResponse {
	kinds := batch.Kinds
	if kinds == nil {
		kinds = []string{}
	}
	return AdminReviewBatchResponse{
		Batch:     batch.Batch,
		ItemCount: batch.ItemCount,
		Kinds:     kinds,
		CreatedAt: batch.CreatedAt,
	}
}
