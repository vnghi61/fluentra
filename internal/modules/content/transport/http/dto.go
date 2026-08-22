// Package http implements the HTTP transport handlers and DTOs for the content module.
package http

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"

	"github.com/fluentra/fluentra/internal/modules/content/contract"
	"github.com/fluentra/fluentra/internal/modules/content/domain"
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

// EstimateLevelResponse payload returned by AI level estimation.
type EstimateLevelResponse struct {
	EstimatedLevel string `json:"estimated_level"`
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
