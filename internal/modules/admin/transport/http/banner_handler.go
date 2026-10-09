package http

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	admindomain "github.com/fluentra/fluentra/internal/modules/admin/domain"
	"github.com/fluentra/fluentra/internal/shared/apperr"
	"github.com/fluentra/fluentra/internal/shared/httpx"
)

// permissionBanners guards the banner admin operations. It is the permission
// the flag operations use: both are the platform's own switches, and a new
// permission would need an RBAC seed and docs change to mean anything.
const permissionBanners = "system.flags"

// bannerBodyLimit bounds a banner write; the longest valid body is ~3 KB.
const bannerBodyLimit = 16 << 10

// bannerRequest is the wire shape of BannerRequest. Pointers mark the fields
// whose absence means "use the default" rather than the zero value.
type bannerRequest struct {
	Kind        *string    `json:"kind"`
	Title       string     `json:"title"`
	Body        *string    `json:"body"`
	CTALabel    *string    `json:"cta_label"`
	CTAURL      *string    `json:"cta_url"`
	Tone        *string    `json:"tone"`
	IsActive    *bool      `json:"is_active"`
	Priority    *int       `json:"priority"`
	Dismissible *bool      `json:"dismissible"`
	StartsAt    *time.Time `json:"starts_at"`
	EndsAt      *time.Time `json:"ends_at"`
	ImageKey    *string    `json:"image_key"`
	ImageAlt    *string    `json:"image_alt"`
}

func (b bannerRequest) toInput() admindomain.BannerInput {
	in := admindomain.BannerInput{
		Title:       b.Title,
		CTALabel:    b.CTALabel,
		CTAURL:      b.CTAURL,
		Tone:        admindomain.BannerToneInfo,
		Dismissible: true,
		StartsAt:    b.StartsAt,
		EndsAt:      b.EndsAt,
		ImageKey:    b.ImageKey,
	}
	if b.ImageAlt != nil {
		in.ImageAlt = *b.ImageAlt
	}
	if b.Kind != nil {
		in.Kind = admindomain.BannerKind(*b.Kind)
	}
	if b.Body != nil {
		in.Body = *b.Body
	}
	if b.Tone != nil {
		in.Tone = admindomain.BannerTone(*b.Tone)
	}
	if b.IsActive != nil {
		in.IsActive = *b.IsActive
	}
	if b.Priority != nil {
		in.Priority = *b.Priority
	}
	if b.Dismissible != nil {
		in.Dismissible = *b.Dismissible
	}
	return in
}

// BannerDTO is the wire shape of Banner, the administrator's view.
type BannerDTO struct {
	ID          string  `json:"id"`
	Kind        string  `json:"kind"`
	Title       string  `json:"title"`
	Body        string  `json:"body"`
	CTALabel    *string `json:"cta_label"`
	CTAURL      *string `json:"cta_url"`
	Tone        string  `json:"tone"`
	IsActive    bool    `json:"is_active"`
	Priority    int     `json:"priority"`
	Dismissible bool    `json:"dismissible"`
	StartsAt    *string `json:"starts_at"`
	EndsAt      *string `json:"ends_at"`
	ImageKey    *string `json:"image_key"`
	ImageURL    *string `json:"image_url"`
	ImageAlt    string  `json:"image_alt"`
	ImageWidth  *int    `json:"image_width"`
	ImageHeight *int    `json:"image_height"`
	CreatedBy   string  `json:"created_by"`
	CreatedAt   string  `json:"created_at"`
	UpdatedAt   string  `json:"updated_at"`
}

// ActiveBannerDTO is the wire shape of ActiveBanner, the learner's view: no
// author, no priority, no window.
type ActiveBannerDTO struct {
	ID          string  `json:"id"`
	Kind        string  `json:"kind"`
	Title       string  `json:"title"`
	Body        string  `json:"body"`
	CTALabel    *string `json:"cta_label"`
	CTAURL      *string `json:"cta_url"`
	Tone        string  `json:"tone"`
	Dismissible bool    `json:"dismissible"`
	ImageURL    *string `json:"image_url"`
	ImageAlt    string  `json:"image_alt"`
	ImageWidth  *int    `json:"image_width"`
	ImageHeight *int    `json:"image_height"`
}

func toBannerDTO(b admindomain.Banner) BannerDTO {
	dto := BannerDTO{
		ID:          b.ID.String(),
		Kind:        string(kindOf(b)),
		Title:       b.Title,
		Body:        b.Body,
		CTALabel:    b.CTALabel,
		CTAURL:      b.CTAURL,
		Tone:        string(b.Tone),
		IsActive:    b.IsActive,
		Priority:    b.Priority,
		Dismissible: b.Dismissible,
		StartsAt:    formatOptionalTime(b.StartsAt),
		EndsAt:      formatOptionalTime(b.EndsAt),
		CreatedBy:   b.CreatedBy.String(),
		CreatedAt:   b.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:   b.UpdatedAt.UTC().Format(time.RFC3339),
	}
	if b.Image != nil {
		key, width, height := b.Image.Key, b.Image.Width, b.Image.Height
		dto.ImageKey = &key
		dto.ImageAlt = b.Image.Alt
		dto.ImageWidth = &width
		dto.ImageHeight = &height
		if b.Image.URL != "" {
			url := b.Image.URL
			dto.ImageURL = &url
		}
	}
	return dto
}

func toActiveBannerDTO(b admindomain.Banner) ActiveBannerDTO {
	dto := ActiveBannerDTO{
		ID:          b.ID.String(),
		Kind:        string(kindOf(b)),
		Title:       b.Title,
		Body:        b.Body,
		CTALabel:    b.CTALabel,
		CTAURL:      b.CTAURL,
		Tone:        string(b.Tone),
		Dismissible: b.Dismissible,
	}
	// An image whose URL could not be signed is left out entirely: the
	// learner gets a text banner, not a broken picture.
	if b.Image != nil && b.Image.URL != "" {
		url, width, height := b.Image.URL, b.Image.Width, b.Image.Height
		dto.ImageURL = &url
		dto.ImageAlt = b.Image.Alt
		dto.ImageWidth = &width
		dto.ImageHeight = &height
	}
	return dto
}

// kindOf answers announcement for a banner read before kinds existed.
func kindOf(b admindomain.Banner) admindomain.BannerKind {
	if b.Kind == "" {
		return admindomain.BannerKindAnnouncement
	}
	return b.Kind
}

func formatOptionalTime(t *time.Time) *string {
	if t == nil {
		return nil
	}
	formatted := t.UTC().Format(time.RFC3339)
	return &formatted
}

// listActiveBanners serves any signed-in account; there is no permission
// beyond holding a session.
func (h *Handler) listActiveBanners(w http.ResponseWriter, r *http.Request) {
	if _, ok := httpx.ActorFrom(r.Context()); !ok {
		httpx.WriteProblem(w, r, apperr.New(
			apperr.Unauthenticated, "UNAUTHENTICATED", "Provide a valid access token."))
		return
	}

	banners, err := h.service.ListActiveBanners(r.Context())
	if err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}

	items := make([]ActiveBannerDTO, 0, len(banners))
	for _, banner := range banners {
		items = append(items, toActiveBannerDTO(banner))
	}
	httpx.WriteJSON(w, r, http.StatusOK, map[string]any{keyItems: items})
}

func (h *Handler) listBanners(w http.ResponseWriter, r *http.Request) {
	if _, err := h.authorise(r, permissionBanners); err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}

	banners, err := h.service.ListBanners(r.Context())
	if err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}

	items := make([]BannerDTO, 0, len(banners))
	for _, banner := range banners {
		items = append(items, toBannerDTO(banner))
	}
	httpx.WriteJSON(w, r, http.StatusOK, map[string]any{keyItems: items})
}

func (h *Handler) createBanner(w http.ResponseWriter, r *http.Request) {
	actor, err := h.authorise(r, permissionBanners)
	if err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}

	var payload bannerRequest
	if err := httpx.DecodeJSONLimit(r, &payload, bannerBodyLimit); err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}

	banner, err := h.service.CreateBanner(r.Context(), actor.UserID, payload.toInput())
	if err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}
	httpx.WriteJSON(w, r, http.StatusCreated, toBannerDTO(banner))
}

func (h *Handler) updateBanner(w http.ResponseWriter, r *http.Request) {
	actor, err := h.authorise(r, permissionBanners)
	if err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}

	id, err := parseBannerID(r)
	if err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}

	var payload bannerRequest
	if err := httpx.DecodeJSONLimit(r, &payload, bannerBodyLimit); err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}

	banner, err := h.service.UpdateBanner(r.Context(), actor.UserID, id, payload.toInput())
	if err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}
	httpx.WriteJSON(w, r, http.StatusOK, toBannerDTO(banner))
}

func (h *Handler) deleteBanner(w http.ResponseWriter, r *http.Request) {
	actor, err := h.authorise(r, permissionBanners)
	if err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}

	id, err := parseBannerID(r)
	if err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}

	if err := h.service.DeleteBanner(r.Context(), actor.UserID, id); err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}
	w.Header().Set("X-Request-Id", httpx.RequestID(r.Context()))
	w.WriteHeader(http.StatusNoContent)
}

// bannerImageBodyLimit bounds the two small image requests.
const bannerImageBodyLimit = 1 << 10

// BannerImageUploadIntentDTO is the wire shape of BannerImageUploadIntent.
type BannerImageUploadIntentDTO struct {
	UploadURL   string            `json:"upload_url"`
	Method      string            `json:"method"`
	FormData    map[string]string `json:"form_data,omitempty"`
	FileField   string            `json:"file_field,omitempty"`
	ObjectKey   string            `json:"object_key"`
	ExpiresAt   string            `json:"expires_at"`
	MaxBytes    int64             `json:"max_bytes"`
	ContentType string            `json:"content_type"`
}

// BannerImageDTO is the wire shape of BannerImage.
type BannerImageDTO struct {
	ImageKey string `json:"image_key"`
	ImageURL string `json:"image_url"`
	Width    int    `json:"width"`
	Height   int    `json:"height"`
}

func (h *Handler) requestBannerImageUpload(w http.ResponseWriter, r *http.Request) {
	actor, err := h.authorise(r, permissionBanners)
	if err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}

	var payload struct {
		ContentType string `json:"content_type"`
	}
	if err := httpx.DecodeJSONLimit(r, &payload, bannerImageBodyLimit); err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}

	intent, err := h.service.RequestBannerImageUpload(r.Context(), actor.UserID, payload.ContentType)
	if err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}
	httpx.WriteJSON(w, r, http.StatusOK, BannerImageUploadIntentDTO{
		UploadURL:   intent.URL,
		Method:      intent.Method,
		FormData:    intent.FormData,
		FileField:   intent.FileField,
		ObjectKey:   intent.ObjectKey,
		ExpiresAt:   intent.ExpiresAt.UTC().Format(time.RFC3339),
		MaxBytes:    intent.MaxBytes,
		ContentType: intent.ContentType,
	})
}

func (h *Handler) processBannerImage(w http.ResponseWriter, r *http.Request) {
	actor, err := h.authorise(r, permissionBanners)
	if err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}

	var payload struct {
		ObjectKey string `json:"object_key"`
	}
	if err := httpx.DecodeJSONLimit(r, &payload, bannerImageBodyLimit); err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}

	image, err := h.service.ProcessBannerImage(r.Context(), actor.UserID, payload.ObjectKey)
	if err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}
	httpx.WriteJSON(w, r, http.StatusCreated, BannerImageDTO{
		ImageKey: image.Key,
		ImageURL: image.URL,
		Width:    image.Width,
		Height:   image.Height,
	})
}

func parseBannerID(r *http.Request) (uuid.UUID, error) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		return uuid.Nil, apperr.New(apperr.Validation, "INVALID_BANNER_ID", "banner id must be a valid UUID")
	}
	return id, nil
}
