// Package repository maps audit rows to domain values. It holds no rules: what
// may be written and what must be redacted is decided in domain, against data
// that reaches this package already cleaned.
package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	sqlcaudit "github.com/fluentra/fluentra/internal/generated/audit/sqlc"
	"github.com/fluentra/fluentra/internal/modules/audit/contract"
	"github.com/fluentra/fluentra/internal/modules/audit/domain"
	"github.com/fluentra/fluentra/internal/shared/dbx"
)

// Repository reads and writes the audit tables.
type Repository struct {
	queries *sqlcaudit.Queries
}

// New creates a repository over db.
func New(db dbx.Querier) *Repository {
	return &Repository{queries: sqlcaudit.New(db)}
}

// InsertAuditLog writes one entry and reports whether it was new.
//
// False means the unique index on (event_id, created_at) rejected a
// redelivery, which is the normal and correct outcome of at-least-once
// delivery — not an error, and not something to retry.
func (r *Repository) InsertAuditLog(ctx context.Context, entry domain.LogEntry) (bool, error) {
	before, err := marshalObject(entry.Before)
	if err != nil {
		return false, fmt.Errorf("marshal audit before: %w", err)
	}
	after, err := marshalObject(entry.After)
	if err != nil {
		return false, fmt.Errorf("marshal audit after: %w", err)
	}
	meta, err := marshalObject(entry.Meta)
	if err != nil {
		return false, fmt.Errorf("marshal audit meta: %w", err)
	}
	if meta == nil {
		// The column is NOT NULL: an entry with no context stores an empty
		// object rather than nothing, so a reader never has to handle both.
		meta = []byte(`{}`)
	}

	var role *sqlcaudit.AuditActorRole
	if entry.ActorRole != nil {
		converted := sqlcaudit.AuditActorRole(*entry.ActorRole)
		role = &converted
	}

	fields := entry.ChangedFields
	if fields == nil {
		fields = []string{}
	}

	affected, err := r.queries.InsertAuditLog(ctx, sqlcaudit.InsertAuditLogParams{
		ID:            entry.ID,
		CreatedAt:     entry.CreatedAt,
		EventID:       entry.EventID,
		ActorID:       entry.ActorID,
		ActorRole:     role,
		Action:        entry.Action,
		TargetType:    entry.TargetType,
		TargetID:      entry.TargetID,
		ChangedFields: fields,
		Before:        before,
		After:         after,
		Meta:          meta,
		IpHash:        entry.IPHash,
		TraceID:       entry.TraceID,
	})
	if err != nil {
		return false, fmt.Errorf("insert audit log: %w", err)
	}
	return affected > 0, nil
}

// InsertSecurityEvent writes one event and reports whether it was new.
func (r *Repository) InsertSecurityEvent(ctx context.Context, record domain.SecurityRecord) (bool, error) {
	detail, err := marshalObject(record.Detail)
	if err != nil {
		return false, fmt.Errorf("marshal security detail: %w", err)
	}
	if detail == nil {
		detail = []byte(`{}`)
	}

	affected, err := r.queries.InsertSecurityEvent(ctx, sqlcaudit.InsertSecurityEventParams{
		ID:        record.ID,
		CreatedAt: record.CreatedAt,
		EventID:   record.EventID,
		Kind:      record.Kind,
		Severity:  sqlcaudit.AuditEventSeverity(record.Severity),
		UserID:    record.UserID,
		Detail:    detail,
		IpHash:    record.IPHash,
		TraceID:   record.TraceID,
	})
	if err != nil {
		return false, fmt.Errorf("insert security event: %w", err)
	}
	return affected > 0, nil
}

// SearchAuditLogs returns one page of the trail, newest first.
func (r *Repository) SearchAuditLogs(ctx context.Context, query domain.LogQuery) ([]domain.LogEntry, error) {
	params := sqlcaudit.SearchAuditLogsParams{
		WindowStart: query.Window.Start,
		WindowEnd:   query.Window.End,
		ActorID:     query.ActorID,
		Action:      query.Action,
		TargetType:  query.TargetType,
		TargetID:    query.TargetID,
		RowLimit:    boundedInt32(query.Limit),
	}
	if query.After != nil {
		params.CursorCreatedAt = &query.After.CreatedAt
		cursorID := query.After.ID
		params.CursorID = &cursorID
	}

	rows, err := r.queries.SearchAuditLogs(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("search audit logs: %w", err)
	}
	entries := make([]domain.LogEntry, 0, len(rows))
	for _, row := range rows {
		entry, err := toLogEntry(row)
		if err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}
	return entries, nil
}

// SearchSecurityEvents returns one page of the security stream, newest first.
func (r *Repository) SearchSecurityEvents(
	ctx context.Context, query domain.SecurityQuery,
) ([]domain.SecurityRecord, error) {
	params := sqlcaudit.SearchSecurityEventsParams{
		WindowStart: query.Window.Start,
		WindowEnd:   query.Window.End,
		Kind:        query.Kind,
		UserID:      query.UserID,
		Resolved:    query.Resolved,
		RowLimit:    boundedInt32(query.Limit),
	}
	if query.Severity != nil {
		severity := sqlcaudit.AuditEventSeverity(*query.Severity)
		params.Severity = &severity
	}
	if query.After != nil {
		params.CursorCreatedAt = &query.After.CreatedAt
		cursorID := query.After.ID
		params.CursorID = &cursorID
	}

	rows, err := r.queries.SearchSecurityEvents(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("search security events: %w", err)
	}
	records := make([]domain.SecurityRecord, 0, len(rows))
	for _, row := range rows {
		record, err := toSecurityRecord(row)
		if err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	return records, nil
}

// GetSecurityEvent reads one event. It returns domain.ErrEventNotFound rather
// than pgx.ErrNoRows so the service does not have to know what drives it.
func (r *Repository) GetSecurityEvent(ctx context.Context, id uuid.UUID) (domain.SecurityRecord, error) {
	row, err := r.queries.GetSecurityEventByID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.SecurityRecord{}, domain.ErrEventNotFound
		}
		return domain.SecurityRecord{}, fmt.Errorf("get security event: %w", err)
	}
	return toSecurityRecord(row)
}

// ResolveSecurityEvent closes an open event and reports whether it did.
//
// False means the row was already resolved, or is not there. The statement
// carries `resolved_at IS NULL`, so two administrators resolving at the same
// moment do not need a transaction between them: exactly one update matches.
func (r *Repository) ResolveSecurityEvent(ctx context.Context, resolution domain.Resolution) (bool, error) {
	resolvedAt := resolution.At
	resolvedBy := resolution.By
	note := resolution.Note
	affected, err := r.queries.ResolveSecurityEvent(ctx, sqlcaudit.ResolveSecurityEventParams{
		ID:             resolution.ID,
		CreatedAt:      resolution.CreatedAt,
		ResolvedAt:     &resolvedAt,
		ResolvedBy:     &resolvedBy,
		ResolutionNote: &note,
	})
	if err != nil {
		return false, fmt.Errorf("resolve security event: %w", err)
	}
	return affected > 0, nil
}

// EnsurePartitions creates the current month's partition and the next
// monthsAhead, returning how many were made.
func (r *Repository) EnsurePartitions(ctx context.Context, monthsAhead int) (int, error) {
	created, err := r.queries.EnsureAuditPartitions(ctx, boundedInt32(monthsAhead))
	if err != nil {
		return 0, fmt.Errorf("ensure audit partitions: %w", err)
	}
	return int(created), nil
}

// DetachExpiredPartitions detaches every partition whose whole month has
// passed out of the retention period, returning their names.
func (r *Repository) DetachExpiredPartitions(ctx context.Context, retain time.Duration) ([]string, error) {
	interval := pgtype.Interval{Microseconds: retain.Microseconds(), Valid: true}
	detached, err := r.queries.DetachExpiredAuditPartitions(ctx, interval)
	if err != nil {
		return nil, fmt.Errorf("detach expired audit partitions: %w", err)
	}
	return detached, nil
}

// ---------------------------------------------------------------- mapping

func toLogEntry(row sqlcaudit.AuditAuditLog) (domain.LogEntry, error) {
	before, err := unmarshalObject(row.Before)
	if err != nil {
		return domain.LogEntry{}, fmt.Errorf("decode audit before: %w", err)
	}
	after, err := unmarshalObject(row.After)
	if err != nil {
		return domain.LogEntry{}, fmt.Errorf("decode audit after: %w", err)
	}
	meta, err := unmarshalObject(row.Meta)
	if err != nil {
		return domain.LogEntry{}, fmt.Errorf("decode audit meta: %w", err)
	}
	if meta == nil {
		meta = map[string]any{}
	}

	var role *contract.ActorRole
	if row.ActorRole != nil {
		converted := contract.ActorRole(*row.ActorRole)
		role = &converted
	}
	fields := row.ChangedFields
	if fields == nil {
		fields = []string{}
	}

	return domain.LogEntry{
		ID:            row.ID,
		CreatedAt:     row.CreatedAt,
		EventID:       row.EventID,
		ActorID:       row.ActorID,
		ActorRole:     role,
		Action:        row.Action,
		TargetType:    row.TargetType,
		TargetID:      row.TargetID,
		ChangedFields: fields,
		Before:        before,
		After:         after,
		Meta:          meta,
		IPHash:        row.IpHash,
		TraceID:       row.TraceID,
	}, nil
}

func toSecurityRecord(row sqlcaudit.AuditSecurityEvent) (domain.SecurityRecord, error) {
	detail, err := unmarshalObject(row.Detail)
	if err != nil {
		return domain.SecurityRecord{}, fmt.Errorf("decode security detail: %w", err)
	}
	if detail == nil {
		detail = map[string]any{}
	}
	return domain.SecurityRecord{
		ID:             row.ID,
		CreatedAt:      row.CreatedAt,
		UpdatedAt:      row.UpdatedAt,
		EventID:        row.EventID,
		Kind:           row.Kind,
		Severity:       contract.Severity(row.Severity),
		UserID:         row.UserID,
		Detail:         detail,
		IPHash:         row.IpHash,
		TraceID:        row.TraceID,
		ResolvedAt:     row.ResolvedAt,
		ResolvedBy:     row.ResolvedBy,
		ResolutionNote: row.ResolutionNote,
	}, nil
}

// boundedInt32 narrows an int for the generated query parameters.
//
// Every value that reaches it is already bounded — the domain clamps a page
// size to 100 and the lookahead is a constant — so this never actually clamps
// anything. It exists so the narrowing is provably safe on a 64-bit build
// rather than safe by argument, which is the difference between the compiler
// checking it and a reader checking it.
func boundedInt32(value int) int32 {
	switch {
	case value < 0:
		return 0
	case value > math.MaxInt32:
		return math.MaxInt32
	default:
		return int32(value)
	}
}

// marshalObject returns nil for an empty map, so an absent diff is stored as
// SQL NULL rather than as `{}`. The two mean different things to a reader:
// null is "the emitting module sent no values", `{}` would be "it sent an
// empty set of them".
func marshalObject(values map[string]any) ([]byte, error) {
	if len(values) == 0 {
		return nil, nil
	}
	return json.Marshal(values)
}

func unmarshalObject(raw []byte) (map[string]any, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	var values map[string]any
	if err := json.Unmarshal(raw, &values); err != nil {
		return nil, err
	}
	return values, nil
}
