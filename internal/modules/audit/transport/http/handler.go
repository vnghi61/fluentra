// Package http is the audit module's HTTP surface: the administrative search
// over the trail and the security event stream, and the one operation that
// changes anything — marking an event triaged.
package http

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/fluentra/fluentra/internal/modules/audit/contract"
	"github.com/fluentra/fluentra/internal/modules/audit/domain"
	"github.com/fluentra/fluentra/internal/shared/apperr"
	"github.com/fluentra/fluentra/internal/shared/httpx"
	"github.com/fluentra/fluentra/internal/shared/pagination"
)

// maxBodyBytes is ample for the one request body these handlers accept.
const maxBodyBytes = 4 << 10

// fieldNote is the only member of that body.
const fieldNote = "note"

// Guard is the authorization surface these handlers need.
//
// It takes a plain string rather than rbac's Permission type because `audit`
// does not depend on `rbac`: the arrows in MODULE_INDEX.md §3 run from rbac
// *into* audit, and importing back the other way would be a cycle as well as a
// boundary violation. The composition root adapts the real Authorizer to this,
// which is the one place entitled to see both modules.
type Guard interface {
	Require(ctx context.Context, permission string) error
}

// Trail is the service surface these handlers need.
type Trail interface {
	SearchLogs(ctx context.Context, query domain.LogQuery) ([]domain.LogEntry, bool, error)
	SearchSecurityEvents(ctx context.Context, query domain.SecurityQuery) ([]domain.SecurityRecord, bool, error)
	ResolveSecurityEvent(ctx context.Context, eventID, resolvedBy uuid.UUID, note string) (domain.SecurityRecord, error)
}

// Handler serves the audit operations.
type Handler struct {
	trail Trail
	guard Guard
	now   func() time.Time
}

// NewHandler creates the handler.
func NewHandler(trail Trail, guard Guard, now func() time.Time) *Handler {
	if now == nil {
		now = time.Now
	}
	return &Handler{trail: trail, guard: guard, now: now}
}

// Routes mounts this module's operations.
//
// The paths are written out in full rather than registered inside a
// `Route("/admin", …)` group, because `rbac` already mounts one and chi allows
// exactly one handler per mount point. Role gating for the whole `/admin`
// prefix is the composition root's job — it wraps these with rbac's AdminOnly.
// Each handler still calls Require with its own permission: the middleware
// fences the route, the guard locks the operation, and neither substitutes for
// the other.
func (h *Handler) Routes(router chi.Router) {
	router.Get("/admin/audit-logs", h.searchLogs)
	router.Get("/admin/security-events", h.searchSecurityEvents)
	router.Post("/admin/security-events/{id}/resolve", h.resolveSecurityEvent)
}

func (h *Handler) searchLogs(writer http.ResponseWriter, request *http.Request) {
	if _, err := h.authorise(request, contract.PermissionRead); err != nil {
		httpx.WriteProblem(writer, request, err)
		return
	}

	query, err := h.parseLogQuery(request)
	if err != nil {
		httpx.WriteProblem(writer, request, err)
		return
	}

	entries, hasMore, err := h.trail.SearchLogs(request.Context(), query)
	if err != nil {
		httpx.WriteProblem(writer, request, err)
		return
	}

	items := make([]auditLogResponse, 0, len(entries))
	for _, entry := range entries {
		items = append(items, toAuditLogResponse(entry))
	}

	page, err := buildPage(query.Limit, hasMore, len(entries) > 0, func() (time.Time, uuid.UUID) {
		last := entries[len(entries)-1]
		return last.CreatedAt, last.ID
	})
	if err != nil {
		httpx.WriteProblem(writer, request, err)
		return
	}
	httpx.WriteJSON(writer, request, http.StatusOK, auditLogPageResponse{Items: items, Page: page})
}

func (h *Handler) searchSecurityEvents(writer http.ResponseWriter, request *http.Request) {
	if _, err := h.authorise(request, contract.PermissionRead); err != nil {
		httpx.WriteProblem(writer, request, err)
		return
	}

	query, err := h.parseSecurityQuery(request)
	if err != nil {
		httpx.WriteProblem(writer, request, err)
		return
	}

	records, hasMore, err := h.trail.SearchSecurityEvents(request.Context(), query)
	if err != nil {
		httpx.WriteProblem(writer, request, err)
		return
	}

	items := make([]securityEventResponse, 0, len(records))
	for _, record := range records {
		items = append(items, toSecurityEventResponse(record))
	}

	page, err := buildPage(query.Limit, hasMore, len(records) > 0, func() (time.Time, uuid.UUID) {
		last := records[len(records)-1]
		return last.CreatedAt, last.ID
	})
	if err != nil {
		httpx.WriteProblem(writer, request, err)
		return
	}
	httpx.WriteJSON(writer, request, http.StatusOK, securityEventPageResponse{Items: items, Page: page})
}

func (h *Handler) resolveSecurityEvent(writer http.ResponseWriter, request *http.Request) {
	actor, err := h.authorise(request, contract.PermissionManage)
	if err != nil {
		httpx.WriteProblem(writer, request, err)
		return
	}

	eventID, err := uuid.Parse(chi.URLParam(request, "id"))
	if err != nil {
		// A malformed id is a 404 rather than a 400: the resource it names does
		// not exist, and saying "that is not a uuid" tells a caller probing the
		// surface something about its shape.
		httpx.WriteProblem(writer, request, domain.ErrEventNotFound)
		return
	}

	note, err := decodeNote(request)
	if err != nil {
		httpx.WriteProblem(writer, request, err)
		return
	}

	record, err := h.trail.ResolveSecurityEvent(request.Context(), eventID, actor.UserID, note)
	if err != nil {
		httpx.WriteProblem(writer, request, err)
		return
	}
	httpx.WriteJSON(writer, request, http.StatusOK, toSecurityEventResponse(record))
}

// authorise resolves the caller and applies the guard. Doing both in one place
// is what stops a handler being written with one and not the other.
func (h *Handler) authorise(request *http.Request, permission string) (httpx.Actor, error) {
	actor, ok := httpx.ActorFrom(request.Context())
	if !ok {
		return httpx.Actor{}, apperr.New(
			apperr.Unauthenticated, "UNAUTHENTICATED", "Provide a valid access token.")
	}
	if err := h.guard.Require(request.Context(), permission); err != nil {
		return httpx.Actor{}, err
	}
	return actor, nil
}

// -------------------------------------------------------------- query input

func (h *Handler) parseLogQuery(request *http.Request) (domain.LogQuery, error) {
	values := request.URL.Query()

	window, err := h.parseWindow(values)
	if err != nil {
		return domain.LogQuery{}, err
	}
	limit, err := parseLimit(values.Get("limit"))
	if err != nil {
		return domain.LogQuery{}, err
	}
	position, err := parseCursor(values.Get("cursor"))
	if err != nil {
		return domain.LogQuery{}, err
	}
	actorID, err := optionalUUID(values.Get("actor_id"), "actor_id")
	if err != nil {
		return domain.LogQuery{}, err
	}
	action, err := optionalName(values.Get("action"), "action")
	if err != nil {
		return domain.LogQuery{}, err
	}

	return domain.LogQuery{
		Window:     window,
		ActorID:    actorID,
		Action:     action,
		TargetType: optionalString(values.Get("target_type")),
		TargetID:   optionalString(values.Get("target_id")),
		After:      position,
		Limit:      limit,
	}, nil
}

func (h *Handler) parseSecurityQuery(request *http.Request) (domain.SecurityQuery, error) {
	values := request.URL.Query()

	window, err := h.parseWindow(values)
	if err != nil {
		return domain.SecurityQuery{}, err
	}
	limit, err := parseLimit(values.Get("limit"))
	if err != nil {
		return domain.SecurityQuery{}, err
	}
	position, err := parseCursor(values.Get("cursor"))
	if err != nil {
		return domain.SecurityQuery{}, err
	}
	userID, err := optionalUUID(values.Get("user_id"), "user_id")
	if err != nil {
		return domain.SecurityQuery{}, err
	}
	kind, err := optionalName(values.Get("kind"), "kind")
	if err != nil {
		return domain.SecurityQuery{}, err
	}

	query := domain.SecurityQuery{
		Window: window, Kind: kind, UserID: userID, After: position, Limit: limit,
	}
	if raw := values.Get("severity"); raw != "" {
		severity := contract.Severity(raw)
		if !severity.Valid() {
			return domain.SecurityQuery{}, invalidField("severity", "ENUM",
				"severity must be one of low, medium, high, critical.")
		}
		query.Severity = &severity
	}
	if raw := values.Get("resolved"); raw != "" {
		resolved, err := strconv.ParseBool(raw)
		if err != nil {
			return domain.SecurityQuery{}, invalidField("resolved", "TYPE", "resolved must be true or false.")
		}
		query.Resolved = &resolved
	}
	return query, nil
}

// parseWindow reads the optional bounds and lets the domain apply the defaults
// that keep the search inside a bounded set of partitions.
func (h *Handler) parseWindow(values map[string][]string) (domain.Window, error) {
	from, err := optionalTime(get(values, "from"), "from")
	if err != nil {
		return domain.Window{}, err
	}
	to, err := optionalTime(get(values, "to"), "to")
	if err != nil {
		return domain.Window{}, err
	}
	return domain.NewWindow(from, to, h.now())
}

func get(values map[string][]string, key string) string {
	if found := values[key]; len(found) > 0 {
		return found[0]
	}
	return ""
}

func parseLimit(raw string) (int, error) {
	if raw == "" {
		return domain.DefaultLimit, nil
	}
	limit, err := strconv.Atoi(raw)
	if err != nil || limit < 1 || limit > domain.MaxLimit {
		return 0, invalidField("limit", "RANGE", "limit must be between 1 and 100.")
	}
	return limit, nil
}

// parseCursor turns the opaque page token back into a keyset position.
//
// A cursor that does not decode is a 400, not a 422: it is not a field a
// person typed, it is a token the previous response handed out, and the client
// that mangled it has a bug rather than a validation problem.
func parseCursor(raw string) (*domain.Position, error) {
	if raw == "" {
		return nil, nil
	}
	cursor, err := pagination.Decode[time.Time](raw)
	if err != nil {
		return nil, apperr.New(apperr.BadRequest, "BAD_REQUEST", "The cursor is not valid.")
	}
	id, err := uuid.Parse(cursor.ID)
	if err != nil {
		return nil, apperr.New(apperr.BadRequest, "BAD_REQUEST", "The cursor is not valid.")
	}
	return &domain.Position{CreatedAt: cursor.SortValue, ID: id}, nil
}

func optionalString(raw string) *string {
	if raw == "" {
		return nil
	}
	value := raw
	return &value
}

func optionalName(raw, field string) (*string, error) {
	if raw == "" {
		return nil, nil
	}
	if !domain.ValidName(raw) {
		return nil, invalidField(field, "PATTERN", field+" must be named <module>.<name>.")
	}
	value := raw
	return &value, nil
}

func optionalUUID(raw, field string) (*uuid.UUID, error) {
	if raw == "" {
		return nil, nil
	}
	parsed, err := uuid.Parse(raw)
	if err != nil {
		return nil, invalidField(field, "TYPE", field+" must be a UUID.")
	}
	return &parsed, nil
}

func optionalTime(raw, field string) (*time.Time, error) {
	if raw == "" {
		return nil, nil
	}
	parsed, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return nil, invalidField(field, "TYPE", field+" must be an RFC 3339 timestamp.")
	}
	utc := parsed.UTC()
	return &utc, nil
}

func invalidField(field, code, message string) error {
	return apperr.New(apperr.Validation, "VALIDATION_FAILED",
		"One or more request fields are invalid.").
		WithFields(apperr.FieldViolation{Field: field, Code: code, Message: message})
}

// decodeNote reads the one member the resolve request carries. Unknown members
// are a 422 naming the field, for the same reason they are on PATCH /me: a
// misspelled field that returns 200 is indistinguishable from success.
func decodeNote(request *http.Request) (string, error) {
	limited := http.MaxBytesReader(nil, request.Body, maxBodyBytes)
	decoder := json.NewDecoder(limited)

	var decoded map[string]json.RawMessage
	if err := decoder.Decode(&decoded); err != nil {
		var maxBytesError *http.MaxBytesError
		if errors.As(err, &maxBytesError) {
			return "", apperr.New(apperr.TooLarge, "PAYLOAD_TOO_LARGE", "Request body exceeds the maximum size.")
		}
		return "", apperr.Wrap(err, apperr.BadRequest, "MALFORMED_REQUEST", "Request body must be a JSON object.")
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return "", apperr.New(apperr.BadRequest, "MALFORMED_REQUEST", "Request body must contain one JSON value.")
	}

	for field := range decoded {
		if field != fieldNote {
			return "", invalidField(field, "UNKNOWN_FIELD", field+" is not a field of this resource.")
		}
	}

	raw, present := decoded[fieldNote]
	if !present {
		return "", invalidField(fieldNote, "REQUIRED", "note is required.")
	}
	var note string
	if err := json.Unmarshal(raw, &note); err != nil {
		return "", invalidField(fieldNote, "TYPE", "note has the wrong type.")
	}
	return note, nil
}

// ------------------------------------------------------------- response DTOs
//
// These mirror api/openapi/components/audit.yaml. They are hand-written, so
// the contract test in this package validates real responses against the spec.

type pageResponse struct {
	NextCursor *string `json:"next_cursor,omitempty"`
	HasMore    bool    `json:"has_more"`
	Limit      int     `json:"limit"`
}

type auditLogResponse struct {
	ID            uuid.UUID      `json:"id"`
	CreatedAt     time.Time      `json:"created_at"`
	ActorID       *uuid.UUID     `json:"actor_id"`
	ActorRole     *string        `json:"actor_role"`
	Action        string         `json:"action"`
	TargetType    *string        `json:"target_type"`
	TargetID      *string        `json:"target_id"`
	ChangedFields []string       `json:"changed_fields"`
	Before        map[string]any `json:"before"`
	After         map[string]any `json:"after"`
	Meta          map[string]any `json:"meta"`
	TraceID       *string        `json:"trace_id"`
}

type auditLogPageResponse struct {
	Items []auditLogResponse `json:"items"`
	Page  pageResponse       `json:"page"`
}

type securityEventResponse struct {
	ID             uuid.UUID      `json:"id"`
	CreatedAt      time.Time      `json:"created_at"`
	Kind           string         `json:"kind"`
	Severity       string         `json:"severity"`
	UserID         *uuid.UUID     `json:"user_id"`
	Detail         map[string]any `json:"detail"`
	TraceID        *string        `json:"trace_id"`
	ResolvedAt     *time.Time     `json:"resolved_at"`
	ResolvedBy     *uuid.UUID     `json:"resolved_by"`
	ResolutionNote *string        `json:"resolution_note"`
}

type securityEventPageResponse struct {
	Items []securityEventResponse `json:"items"`
	Page  pageResponse            `json:"page"`
}

// toAuditLogResponse renders one entry.
//
// ip_hash is deliberately not on the response. It is a pseudonymous identifier
// that correlates a person's activity across sessions, it is not needed to
// read the trail, and the fewer places it exists the fewer places it leaks
// from. It stays in the table for the security queries that need it.
func toAuditLogResponse(entry domain.LogEntry) auditLogResponse {
	rendered := auditLogResponse{
		ID:            entry.ID,
		CreatedAt:     entry.CreatedAt.UTC(),
		ActorID:       entry.ActorID,
		Action:        entry.Action,
		TargetType:    entry.TargetType,
		TargetID:      entry.TargetID,
		ChangedFields: entry.ChangedFields,
		Before:        entry.Before,
		After:         entry.After,
		Meta:          entry.Meta,
		TraceID:       entry.TraceID,
	}
	if rendered.ChangedFields == nil {
		rendered.ChangedFields = []string{}
	}
	if rendered.Meta == nil {
		rendered.Meta = map[string]any{}
	}
	if entry.ActorRole != nil {
		role := entry.ActorRole.String()
		rendered.ActorRole = &role
	}
	return rendered
}

func toSecurityEventResponse(record domain.SecurityRecord) securityEventResponse {
	rendered := securityEventResponse{
		ID:             record.ID,
		CreatedAt:      record.CreatedAt.UTC(),
		Kind:           record.Kind,
		Severity:       record.Severity.String(),
		UserID:         record.UserID,
		Detail:         record.Detail,
		TraceID:        record.TraceID,
		ResolvedAt:     record.ResolvedAt,
		ResolvedBy:     record.ResolvedBy,
		ResolutionNote: record.ResolutionNote,
	}
	if rendered.Detail == nil {
		rendered.Detail = map[string]any{}
	}
	return rendered
}

// buildPage assembles the pagination block, encoding a cursor only when there
// is another page to reach with it.
func buildPage(limit int, hasMore, hasItems bool, last func() (time.Time, uuid.UUID)) (pageResponse, error) {
	page := pageResponse{HasMore: hasMore, Limit: limit}
	if !hasMore || !hasItems {
		return page, nil
	}
	createdAt, id := last()
	encoded, err := pagination.Encode(pagination.Cursor[time.Time]{
		SortValue: createdAt, ID: id.String(),
	})
	if err != nil {
		return pageResponse{}, apperr.Wrap(err, apperr.Internal, "INTERNAL_ERROR",
			"An unexpected error occurred.")
	}
	page.NextCursor = &encoded
	return page, nil
}
