package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/fluentra/fluentra/internal/modules/audit/contract"
	"github.com/fluentra/fluentra/internal/modules/audit/domain"
)

// Delivery is one outbox event as the consumer needs it.
//
// It is a local type rather than shared/outbox.Event or eventbus.Message so
// that the consumer can be tested without either, and so the composition root
// is the only place that knows which of them is in use. The fields are the
// intersection of both, which is not a coincidence: the outbox publisher's
// Event and the bus's Message describe the same delivery.
type Delivery struct {
	// ID is the outbox event id, and the deduplication key. Redelivery is not
	// an edge case here — it is the documented behaviour of at-least-once
	// dispatch, and this value is the whole reason it is survivable.
	ID      uuid.UUID
	Topic   string
	Payload json.RawMessage
}

// envelope is the shape audit expects of every outbox payload.
//
// It is a structural convention rather than a shared type, and it has to be:
// every arrow in MODULE_INDEX.md §3 points *into* this module, so `audit` may
// not import user/contract or rbac/contract to unmarshal what they published.
// Reading the payload by field name is what lets the trail record a module's
// events without depending on it — and it is why every emitting module writes
// `occurred_at`, `actor_id` and `changed_fields` under those names.
//
// A field the payload does not carry stays nil and is simply not recorded. An
// unknown module publishing an event audit has never seen still produces a
// usable entry: the action, the time, and whatever of the convention it
// happened to follow.
type envelope struct {
	OccurredAt    time.Time      `json:"occurred_at"`
	ActorID       *uuid.UUID     `json:"actor_id"`
	UserID        *uuid.UUID     `json:"user_id"`
	ChangedFields []string       `json:"changed_fields"`
	Before        map[string]any `json:"before"`
	After         map[string]any `json:"after"`
	Severity      string         `json:"severity"`
}

// The envelope's member names, as they appear on the wire. They are constants
// because envelopeFields below has to stay in step with the struct tags above,
// and a typo there would silently duplicate a field into the detail.
const (
	fieldOccurredAt    = "occurred_at"
	fieldActorID       = "actor_id"
	fieldUserID        = "user_id"
	fieldChangedFields = "changed_fields"
	fieldBefore        = "before"
	fieldAfter         = "after"
	fieldSeverity      = "severity"
)

// The action catalogue: every topic this module records, and the name each
// entry is stored under (`audit`'s AGENT.md §10).
//
// They are declared here as string literals rather than imported from the
// modules that publish them, for the boundary reason set out on `envelope`
// above. Adding an event to this list is what makes it auditable.
const (
	topicProfileUpdated     = "user.profile_updated"
	topicPreferencesUpdated = "user.preferences_updated"
	topicUserSuspended      = "user.suspended"
	topicDeletionRequested  = "user.deletion_requested"
	topicUserDeleted        = "user.deleted"
	topicRoleAssigned       = "rbac.role_assigned"
	topicRoleRevoked        = "rbac.role_revoked"
	topicAccessDenied       = "rbac.access_denied"
	topicSecurityEvent      = "auth.security_event"
)

// targetTypeUser is the target kind for an event about an account.
const targetTypeUser = "user"

// securityTopics are the events that describe a security occurrence rather
// than a state change, and so belong in the other stream.
//
// It is an explicit set rather than a pattern match on the topic. A rule like
// "anything ending in _denied" reads well until the first event that ends in
// _denied and is not a security matter, and by then the misfiled rows are
// already in a table nobody can UPDATE.
var securityTopics = map[string]struct{}{
	topicSecurityEvent: {},
	topicAccessDenied:  {},
}

// SubscribedTopics is the list the composition root subscribes this consumer
// to, in P1.5.
//
// The names are string literals, not constants imported from the emitting
// modules, for the boundary reason set out on `envelope` above. The cost is
// that renaming an event in `user` does not break this list at compile time;
// what catches it instead is the integration test, which drives a real change
// through a real outbox and asserts a row appears.
func SubscribedTopics() []string {
	return []string{
		topicProfileUpdated,
		topicPreferencesUpdated,
		topicUserSuspended,
		topicDeletionRequested,
		topicUserDeleted,
		topicRoleAssigned,
		topicRoleRevoked,
		topicAccessDenied,
		topicSecurityEvent,
	}
}

// Consume files one delivered event.
//
// Returning an error asks the publisher to redeliver, which is the right
// answer for a database that is down and the wrong one for a payload that will
// never parse — a permanent failure retried forever blocks the queue behind
// it. So a malformed payload is logged and acknowledged, and only a write
// failure is returned.
func (s *Service) Consume(ctx context.Context, delivery Delivery) error {
	if !domain.ValidName(delivery.Topic) {
		slog.ErrorContext(ctx, "dropping an outbox event whose topic is not an audit action",
			"module", "audit", "op", "Consume", "topic", delivery.Topic,
			"event_id", delivery.ID.String())
		return nil
	}

	var decoded envelope
	if len(delivery.Payload) > 0 {
		if err := json.Unmarshal(delivery.Payload, &decoded); err != nil {
			// Acknowledged deliberately: the payload will not parse on the
			// hundredth attempt either.
			slog.ErrorContext(ctx, "dropping an outbox event whose payload could not be read",
				"module", "audit", "op", "Consume", "topic", delivery.Topic,
				"event_id", delivery.ID.String(), "error", err)
			return nil
		}
	}

	occurredAt := s.occurredAt(delivery, decoded)

	if _, isSecurity := securityTopics[delivery.Topic]; isSecurity {
		return s.consumeSecurityEvent(ctx, delivery, decoded, occurredAt)
	}
	return s.consumeAuditLog(ctx, delivery, decoded, occurredAt)
}

// occurredAt decides which partition the entry lands in, and it must give the
// same answer every time an event is delivered.
//
// That is not a nicety. created_at is the partition key and half of the unique
// index on (event_id, created_at); a redelivery that computed a different
// timestamp would write a second row in a second partition and the index that
// exists to prevent exactly that would never see the collision. So: the
// emitting module's own occurred_at first, then the millisecond timestamp
// inside the version 7 event id, and only then the clock — which is
// non-deterministic, and reached only by an event that has neither.
func (s *Service) occurredAt(delivery Delivery, decoded envelope) time.Time {
	if !decoded.OccurredAt.IsZero() {
		return decoded.OccurredAt.UTC()
	}
	if fromID, ok := domain.TimeFromUUIDv7(delivery.ID); ok {
		return fromID
	}
	return s.clock.Now().UTC()
}

func (s *Service) consumeAuditLog(
	ctx context.Context, delivery Delivery, decoded envelope, occurredAt time.Time,
) error {
	entry := contract.Entry{
		Action:        delivery.Topic,
		Before:        decoded.Before,
		After:         decoded.After,
		ChangedFields: decoded.ChangedFields,
		Meta:          map[string]any{"source": "outbox"},
	}
	// The subject of a user- or role-scoped event is the account named in it.
	if decoded.UserID != nil {
		entry.TargetType = targetTypeUser
		entry.TargetID = decoded.UserID.String()
	}

	// The event id is the deduplication key, so it is used as-is rather than
	// generated: that is what makes a redelivery collide instead of duplicate.
	built, err := s.buildEntry(ctx, delivery.ID, occurredAt, entry)
	if err != nil {
		return fmt.Errorf("build audit entry for %s: %w", delivery.Topic, err)
	}
	// The actor comes from the payload here, not from ctx: the consumer runs
	// in the worker, where there is no request and nobody is signed in. The
	// actor is whoever the emitting module recorded at the time.
	built.ActorID = nil
	built.ActorRole = nil
	if decoded.ActorID != nil && *decoded.ActorID != uuid.Nil {
		actorID := *decoded.ActorID
		built.ActorID = &actorID
	}

	inserted, err := s.repo.InsertAuditLog(ctx, built)
	if err != nil {
		return fmt.Errorf("record audit entry for %s: %w", delivery.Topic, err)
	}
	if !inserted {
		slog.DebugContext(ctx, "audit entry already recorded for this event",
			"module", "audit", "op", "Consume", "topic", delivery.Topic,
			"event_id", delivery.ID.String())
	}
	return nil
}

func (s *Service) consumeSecurityEvent(
	ctx context.Context, delivery Delivery, decoded envelope, occurredAt time.Time,
) error {
	event := contract.SecurityEvent{
		Kind:     delivery.Topic,
		Severity: contract.Severity(decoded.Severity),
		Detail:   decoded.After,
	}
	if decoded.UserID != nil {
		event.UserID = *decoded.UserID
	}
	if event.Detail == nil {
		// Everything the payload carried that is not part of the envelope is
		// the detail. Redaction still applies to it.
		event.Detail = detailFromPayload(delivery.Payload)
	}

	record, err := s.buildSecurityRecord(ctx, delivery.ID, occurredAt, event)
	if err != nil {
		return fmt.Errorf("build security event for %s: %w", delivery.Topic, err)
	}

	inserted, err := s.repo.InsertSecurityEvent(ctx, record)
	if err != nil {
		return fmt.Errorf("record security event for %s: %w", delivery.Topic, err)
	}
	if !inserted {
		slog.DebugContext(ctx, "security event already recorded for this event",
			"module", "audit", "op", "Consume", "topic", delivery.Topic,
			"event_id", delivery.ID.String())
	}
	return nil
}

// envelopeFields are the payload members the envelope already accounts for.
// They are dropped from the detail so it does not repeat them.
var envelopeFields = map[string]struct{}{
	fieldOccurredAt: {}, fieldActorID: {}, fieldUserID: {},
	fieldChangedFields: {}, fieldBefore: {}, fieldAfter: {}, fieldSeverity: {},
}

func detailFromPayload(payload json.RawMessage) map[string]any {
	if len(payload) == 0 {
		return nil
	}
	var raw map[string]any
	if err := json.Unmarshal(payload, &raw); err != nil {
		return nil
	}
	for field := range envelopeFields {
		delete(raw, field)
	}
	if len(raw) == 0 {
		return nil
	}
	return raw
}
