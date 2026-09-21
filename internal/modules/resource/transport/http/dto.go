package http

import (
	"time"

	"github.com/google/uuid"

	"github.com/fluentra/fluentra/internal/modules/resource/contract"
)

type createUploadIntentRequest struct {
	Filename    string `json:"filename"`
	ContentType string `json:"content_type"`
}

type uploadIntentResponse struct {
	ID        uuid.UUID `json:"id"`
	UploadURL string    `json:"upload_url"`
	ObjectKey string    `json:"object_key"`
	ExpiresAt time.Time `json:"expires_at"`
}

type submitResourceRequest struct {
	ResourceID *uuid.UUID `json:"resource_id,omitempty"`
	URL        *string    `json:"url,omitempty"`
	Title      *string    `json:"title,omitempty"`
}

type submitResourceResponse struct {
	ID     uuid.UUID `json:"id"`
	Status string    `json:"status"`
}

type resourceResponse struct {
	ID               uuid.UUID  `json:"id"`
	Kind             string     `json:"kind"`
	Title            string     `json:"title"`
	Status           string     `json:"status"`
	FailureReason    string     `json:"failure_reason"`
	OriginalFilename string     `json:"original_filename"`
	DeclaredMIME     string     `json:"declared_mime"`
	DetectedMIME     string     `json:"detected_mime"`
	ByteSize         *int64     `json:"byte_size"`
	Checksum         *string    `json:"checksum"`
	SourceURL        *string    `json:"source_url"`
	DownloadURL      *string    `json:"download_url"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
	ValidatedAt      *time.Time                  `json:"validated_at"`
	Renditions       []resourceRenditionResponse `json:"renditions,omitempty"`
}

type resourceRenditionResponse struct {
	Kind       string     `json:"kind"`
	MIMEType   string     `json:"mime_type"`
	Width      *int       `json:"width,omitempty"`
	Height     *int       `json:"height,omitempty"`
	DurationMS *int       `json:"duration_ms,omitempty"`
	ByteSize   *int64     `json:"byte_size,omitempty"`
	URL        *string    `json:"url,omitempty"`
	ExpiresAt  *time.Time `json:"expires_at,omitempty"`
}

type resourceListResponse struct {
	Items    []resourceResponse `json:"items"`
	Total    int                `json:"total"`
	Page     int                `json:"page"`
	PageSize int                `json:"page_size"`
}

func toResourceResponse(r contract.Resource) resourceResponse {
	var renditions []resourceRenditionResponse
	if len(r.Renditions) > 0 {
		renditions = make([]resourceRenditionResponse, 0, len(r.Renditions))
		for _, rend := range r.Renditions {
			if rend.Status == "ready" {
				renditions = append(renditions, resourceRenditionResponse{
					Kind:       rend.Kind,
					MIMEType:   rend.MIMEType,
					Width:      rend.Width,
					Height:     rend.Height,
					DurationMS: rend.DurationMS,
					ByteSize:   rend.ByteSize,
					URL:        rend.URL,
					ExpiresAt:  rend.ExpiresAt,
				})
			}
		}
	}

	return resourceResponse{
		ID:               r.ID,
		Kind:             r.Kind,
		Title:            r.Title,
		Status:           r.Status,
		FailureReason:    r.FailureReason,
		OriginalFilename: r.OriginalFilename,
		DeclaredMIME:     r.DeclaredMIME,
		DetectedMIME:     r.DetectedMIME,
		ByteSize:         r.ByteSize,
		Checksum:         r.Checksum,
		SourceURL:        r.SourceURL,
		DownloadURL:      r.DownloadURL,
		Renditions:       renditions,
		CreatedAt:        r.CreatedAt,
		UpdatedAt:        r.UpdatedAt,
		ValidatedAt:      r.ValidatedAt,
	}
}

func toResourceListResponse(l *contract.ResourceList) resourceListResponse {
	items := make([]resourceResponse, len(l.Items))
	for i, item := range l.Items {
		items[i] = toResourceResponse(item)
	}
	return resourceListResponse{
		Items:    items,
		Total:    l.Total,
		Page:     l.Page,
		PageSize: l.PageSize,
	}
}
