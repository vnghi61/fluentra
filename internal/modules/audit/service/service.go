// Package service holds the audit use cases: recording what happened,
// searching it back, and triaging a security event.
package service

import (
	"context"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/fluentra/fluentra/internal/modules/audit/contract"
	"github.com/fluentra/fluentra/internal/modules/audit/domain"
	"github.com/fluentra/fluentra/internal/shared/clock"
	"github.com/fluentra/fluentra/internal/shared/httpx"
	"github.com/fluentra/fluentra/internal/shared/id"
)

// recordTimeout bounds a best-effort write.
//
// Record runs on a context deliberately detached from the request's, because
// the caller may well have finished responding by the time it runs and a
// cancelled context would silently drop the entry. Detached is not unbounded,
// so it gets its own deadline.
const recordTimeout = 5 * time.Second

// Repository is the persistence surface this service needs, declared by the
// consumer so the rules can be exercised without a database.
//
// There is no WithTx here, and that is not an omission. Rule L4 forbids a
// transaction spanning two modules, so an audit write is never inside a
// caller's transaction — the events that must not be lost arrive through the
// outbox, which is exactly the mechanism that exists because the shared
// transaction is not allowed. Every method below is one statement.
type Repository interface {
	InsertAuditLog(ctx context.Context, entry domain.LogEntry) (bool, error)
	InsertSecurityEvent(ctx context.Context, record domain.SecurityRecord) (bool, error)
	SearchAuditLogs(ctx context.Context, query domain.LogQuery) ([]domain.LogEntry, error)
	SearchSecurityEvents(ctx context.Context, query domain.SecurityQuery) ([]domain.SecurityRecord, error)
	GetSecurityEvent(ctx context.Context, eventID uuid.UUID) (domain.SecurityRecord, error)
	ResolveSecurityEvent(ctx context.Context, resolution domain.Resolution) (bool, error)
	EnsurePartitions(ctx context.Context, monthsAhead int) (int, error)
	DetachExpiredPartitions(ctx context.Context, retain time.Duration) ([]string, error)
}

// Service implements the audit use cases.
type Service struct {
	repo  Repository
	clock clock.Clock
	ipKey []byte
}

// Deps are the service's collaborators.
type Deps struct {
	Repo  Repository
	Clock clock.Clock

	// IPHashKey keys the HMAC that turns a client address into the stored
	// hash. Leave it empty and no address is recorded at all: an unkeyed
	// digest of an IPv4 address is reversible by anybody willing to hash four
	// billion values, so storing one would be storing the address with extra
	// steps.
	IPHashKey []byte
}

// New creates the audit service.
func New(deps Deps) *Service {
	timekeeper := deps.Clock
	if timekeeper == nil {
		timekeeper = clock.Real{}
	}
	return &Service{repo: deps.Repo, clock: timekeeper, ipKey: deps.IPHashKey}
}

// The service is what other modules get when they ask for the audit contract.
var (
	_ contract.Recorder         = (*Service)(nil)
	_ contract.SecurityRecorder = (*Service)(nil)
)

// Record files one action, best effort.
//
// It returns nothing, and every failure below ends in a log rather than in the
// caller's error path. That is BR-AUDIT-02: an audit write must not be able to
// fail the operation it describes. The failure is loud in the log, and an
// action whose record must survive a database outage should be emitted as an
// outbox event in the same transaction as the change instead — see
// consumer.go.
func (s *Service) Record(ctx context.Context, entry contract.Entry) {
	if !domain.ValidName(entry.Action) {
		slog.ErrorContext(ctx, "refusing an audit entry with a malformed action",
			"module", "audit", "op", "Record", "action", entry.Action)
		return
	}

	eventID, err := id.NewUUIDv7(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "could not generate an audit event id",
			"module", "audit", "op", "Record", "action", entry.Action, "error", err)
		return
	}

	// Detached from the request: see recordTimeout.
	detached, cancel := context.WithTimeout(context.WithoutCancel(ctx), recordTimeout)
	defer cancel()

	built, err := s.buildEntry(ctx, eventID, s.clock.Now(), entry)
	if err != nil {
		slog.ErrorContext(ctx, "could not build an audit entry",
			"module", "audit", "op", "Record", "action", entry.Action, "error", err)
		return
	}

	if _, err := s.repo.InsertAuditLog(detached, built); err != nil {
		slog.ErrorContext(ctx, "audit entry was not recorded; the operation it describes was not affected",
			"module", "audit", "op", "Record", "action", entry.Action, "error", err)
	}
}

// RecordSecurityEvent files one security event, on the same terms.
func (s *Service) RecordSecurityEvent(ctx context.Context, event contract.SecurityEvent) {
	if !domain.ValidName(event.Kind) {
		slog.ErrorContext(ctx, "refusing a security event with a malformed kind",
			"module", "audit", "op", "RecordSecurityEvent", "kind", event.Kind)
		return
	}

	eventID, err := id.NewUUIDv7(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "could not generate a security event id",
			"module", "audit", "op", "RecordSecurityEvent", "kind", event.Kind, "error", err)
		return
	}

	detached, cancel := context.WithTimeout(context.WithoutCancel(ctx), recordTimeout)
	defer cancel()

	record, err := s.buildSecurityRecord(ctx, eventID, s.clock.Now(), event)
	if err != nil {
		slog.ErrorContext(ctx, "could not build a security event",
			"module", "audit", "op", "RecordSecurityEvent", "kind", event.Kind, "error", err)
		return
	}

	if _, err := s.repo.InsertSecurityEvent(detached, record); err != nil {
		slog.ErrorContext(ctx, "security event was not recorded",
			"module", "audit", "op", "RecordSecurityEvent", "kind", event.Kind, "error", err)
	}
}

// buildEntry turns a caller's Entry into the row that will be written. It is
// the single place redaction happens, so there is no path to the table that
// skips it.
func (s *Service) buildEntry(
	ctx context.Context, eventID uuid.UUID, occurredAt time.Time, entry contract.Entry,
) (domain.LogEntry, error) {
	rowID, err := id.NewUUIDv7(ctx)
	if err != nil {
		return domain.LogEntry{}, err
	}

	fields := entry.ChangedFields
	if fields == nil {
		fields = domain.ChangedFields(entry.Before, entry.After)
	}

	built := domain.LogEntry{
		ID:            rowID,
		CreatedAt:     occurredAt.UTC(),
		EventID:       eventID,
		Action:        entry.Action,
		ChangedFields: domain.NormaliseFields(fields),
		Before:        domain.Redact(entry.Before),
		After:         domain.Redact(entry.After),
		Meta:          domain.Redact(entry.Meta),
	}
	if entry.TargetType != "" {
		targetType := entry.TargetType
		built.TargetType = &targetType
		if entry.TargetID != "" {
			targetID := entry.TargetID
			built.TargetID = &targetID
		}
	}
	if actor, ok := httpx.ActorFrom(ctx); ok {
		actorID := actor.UserID
		built.ActorID = &actorID
		if role := contract.ActorRole(actor.Role); role.Valid() {
			built.ActorRole = &role
		}
	}
	if hashed := domain.HashIP(httpx.ClientIP(ctx), s.ipKey); hashed != "" {
		built.IPHash = &hashed
	}
	if traceID := httpx.TraceID(ctx); traceID != "" {
		built.TraceID = &traceID
	}
	return built, nil
}

func (s *Service) buildSecurityRecord(
	ctx context.Context, eventID uuid.UUID, occurredAt time.Time, event contract.SecurityEvent,
) (domain.SecurityRecord, error) {
	rowID, err := id.NewUUIDv7(ctx)
	if err != nil {
		return domain.SecurityRecord{}, err
	}

	severity := event.Severity
	if !severity.Valid() {
		// Filing it quietly beats losing it. A miscategorised event is visible
		// on the dashboard at the wrong height; a dropped one is not visible
		// anywhere.
		severity = contract.SeverityLow
	}

	record := domain.SecurityRecord{
		ID:        rowID,
		CreatedAt: occurredAt.UTC(),
		UpdatedAt: occurredAt.UTC(),
		EventID:   eventID,
		Kind:      event.Kind,
		Severity:  severity,
		Detail:    domain.Redact(event.Detail),
	}
	if event.UserID != uuid.Nil {
		userID := event.UserID
		record.UserID = &userID
	}
	if hashed := domain.HashIP(httpx.ClientIP(ctx), s.ipKey); hashed != "" {
		record.IPHash = &hashed
	}
	if traceID := httpx.TraceID(ctx); traceID != "" {
		record.TraceID = &traceID
	}
	return record, nil
}

// SearchLogs returns one page of the trail, and whether another follows.
//
// It reads one row more than the caller asked for and discards it. That is how
// `has_more` is answered without a second COUNT query — and a COUNT over a
// partitioned table with a 90-day window is not a cheap thing to do on every
// page of a screen somebody is scrolling.
func (s *Service) SearchLogs(
	ctx context.Context, query domain.LogQuery,
) (entries []domain.LogEntry, hasMore bool, err error) {
	query.Limit = domain.NormaliseLimit(query.Limit)

	probe := query
	probe.Limit = query.Limit + 1
	rows, err := s.repo.SearchAuditLogs(ctx, probe)
	if err != nil {
		return nil, false, err
	}
	if len(rows) > query.Limit {
		return rows[:query.Limit], true, nil
	}
	return rows, false, nil
}

// SearchSecurityEvents returns one page of the security stream, on the same
// terms.
func (s *Service) SearchSecurityEvents(
	ctx context.Context, query domain.SecurityQuery,
) (records []domain.SecurityRecord, hasMore bool, err error) {
	query.Limit = domain.NormaliseLimit(query.Limit)

	probe := query
	probe.Limit = query.Limit + 1
	rows, err := s.repo.SearchSecurityEvents(ctx, probe)
	if err != nil {
		return nil, false, err
	}
	if len(rows) > query.Limit {
		return rows[:query.Limit], true, nil
	}
	return rows, false, nil
}

// ResolveSecurityEvent marks an event triaged and returns it as it now stands.
//
// The update carries `resolved_at IS NULL`, so it is the arbiter between two
// administrators closing the same event at once: exactly one statement matches
// a row. The read that follows a miss is only there to tell the loser whether
// they lost the race (409) or named an event that does not exist (404) — it is
// not what decides the outcome.
func (s *Service) ResolveSecurityEvent(
	ctx context.Context, eventID, resolvedBy uuid.UUID, note string,
) (domain.SecurityRecord, error) {
	trimmed, err := domain.ValidateNote(note)
	if err != nil {
		return domain.SecurityRecord{}, err
	}

	existing, err := s.repo.GetSecurityEvent(ctx, eventID)
	if err != nil {
		return domain.SecurityRecord{}, err
	}
	if existing.Resolved() {
		return domain.SecurityRecord{}, domain.ErrAlreadyResolved
	}

	resolvedAt := s.clock.Now().UTC()
	updated, err := s.repo.ResolveSecurityEvent(ctx, domain.Resolution{
		ID:        existing.ID,
		CreatedAt: existing.CreatedAt,
		By:        resolvedBy,
		Note:      trimmed,
		At:        resolvedAt,
	})
	if err != nil {
		return domain.SecurityRecord{}, err
	}
	if !updated {
		// Somebody else resolved it between the read and the write.
		return domain.SecurityRecord{}, domain.ErrAlreadyResolved
	}

	existing.ResolvedAt = &resolvedAt
	existing.ResolvedBy = &resolvedBy
	existing.ResolutionNote = &trimmed
	existing.UpdatedAt = resolvedAt
	return existing, nil
}
