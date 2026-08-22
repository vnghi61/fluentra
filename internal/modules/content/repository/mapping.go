// Package repository implements the persistence layer for the content module.
package repository

import (
	"encoding/json"
	"time"

	"github.com/fluentra/fluentra/internal/generated/content/sqlc"
	"github.com/fluentra/fluentra/internal/modules/content/contract"
	"github.com/fluentra/fluentra/internal/modules/content/domain"
)

func toDomainItem(row sqlc.ContentContentItem) domain.Item {
	return domain.Item{
		ID:               row.ID,
		Kind:             row.Kind,
		Slug:             row.Slug,
		CurrentVersionID: row.CurrentVersionID,
		Status:           domain.AuthoringStatus(row.Status),
		OwnerID:          row.OwnerID,
		CreatedAt:        row.CreatedAt,
		UpdatedAt:        row.UpdatedAt,
	}
}

func toDomainVersion(row sqlc.ContentContentVersion) domain.Version {
	return domain.Version{
		ID:          row.ID,
		ItemID:      row.ItemID,
		Version:     int(row.Version),
		Kind:        row.Kind,
		Body:        json.RawMessage(row.Body),
		CEFRLevel:   row.CefrLevel,
		Status:      domain.AuthoringStatus(row.Status),
		MediaRefs:   row.MediaRefs,
		PublishedAt: row.PublishedAt,
		CreatedAt:   row.CreatedAt,
		UpdatedAt:   row.UpdatedAt,
	}
}

// ToContractVersion converts a domain.Version and its associated tag codes to a *contract.Version.
func ToContractVersion(v domain.Version, tags []string) *contract.Version {
	var pubAt *time.Time
	if v.PublishedAt != nil {
		t := *v.PublishedAt
		pubAt = &t
	}
	return &contract.Version{
		ID:          v.ID,
		ItemID:      v.ItemID,
		Version:     v.Version,
		Kind:        v.Kind,
		Body:        v.Body,
		CEFRLevel:   v.CEFRLevel,
		MediaRefs:   v.MediaRefs,
		Tags:        tags,
		Status:      string(v.Status),
		PublishedAt: pubAt,
	}
}

func toDomainMediaAsset(row sqlc.ContentMediaAsset) domain.MediaAsset {
	return domain.MediaAsset{
		ID:         row.ID,
		ObjectKey:  row.ObjectKey,
		Kind:       row.Kind,
		DurationMS: row.DurationMs,
		Checksum:   row.Checksum,
		Status:     domain.MediaStatus(row.Status),
		ByteSize:   row.ByteSize,
		MIMEType:   row.MimeType,
		CreatedAt:  row.CreatedAt,
		UpdatedAt:  row.UpdatedAt,
	}
}

func toDomainTaxonomy(row sqlc.ContentTaxonomy) domain.Taxonomy {
	return domain.Taxonomy{
		ID:        row.ID,
		Namespace: row.Namespace,
		Code:      row.Code,
		Label:     row.Label,
		ParentID:  row.ParentID,
		CreatedAt: row.CreatedAt,
		UpdatedAt: row.UpdatedAt,
	}
}

func toDomainReview(row sqlc.ContentContentReview) domain.Review {
	return domain.Review{
		ID:         row.ID,
		VersionID:  row.VersionID,
		ReviewerID: row.ReviewerID,
		Decision:   domain.ReviewDecision(row.Decision),
		Comments:   row.Comments,
		CreatedAt:  row.CreatedAt,
	}
}
