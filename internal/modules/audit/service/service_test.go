package service

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/fluentra/fluentra/internal/modules/audit/contract"
	"github.com/fluentra/fluentra/internal/modules/audit/domain"
	"github.com/fluentra/fluentra/internal/shared/apperr"
	"github.com/fluentra/fluentra/internal/shared/clock"
	"github.com/fluentra/fluentra/internal/shared/httpx"
)

var fixedNow = time.Date(2026, 8, 10, 12, 0, 0, 0, time.UTC)

// Literals these tests reach for repeatedly. The topic names come from the
// catalogue in consumer.go, which is the same package.
const (
	testFieldDisplayName = "display_name"
	testRoleAdmin        = "admin"
	testFieldTimezone    = "timezone"
)

// contextWithClientIP produces a context carrying a resolved client address.
//
// It runs the real ClientIPResolver middleware rather than reaching for a
// setter, because httpx deliberately exports none: the address is something
// middleware establishes from a request, and a test that could inject one
// directly would be testing a path production does not have.
func contextWithClientIP(t *testing.T, address netip.Addr) context.Context {
	t.Helper()

	resolver, err := httpx.NewClientIPResolver(nil)
	if err != nil {
		t.Fatalf("NewClientIPResolver: %v", err)
	}
	request := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	request.RemoteAddr = net.JoinHostPort(address.String(), "51000")

	var captured context.Context
	resolver.Middleware(http.HandlerFunc(func(_ http.ResponseWriter, inner *http.Request) {
		captured = inner.Context()
	})).ServeHTTP(httptest.NewRecorder(), request)
	if captured == nil {
		t.Fatal("the client-ip middleware did not run")
	}
	return captured
}

func newService(t *testing.T, repo *fakeRepository) *Service {
	t.Helper()
	frozen := &clock.Fake{}
	frozen.Set(fixedNow)
	return New(Deps{Repo: repo, Clock: frozen, IPHashKey: []byte("test-key-not-a-secret")})
}

// TestRecordDoesNotFailTheCallerWhenTheDatabaseIsDown is BR-AUDIT-02 as a
// test. The interface has no error to return, so what this actually proves is
// that the implementation does not panic or block when the write fails — the
// caller carries on.
func TestRecordDoesNotFailTheCallerWhenTheDatabaseIsDown(t *testing.T) {
	t.Parallel()

	repo := newFakeRepository()
	repo.failInsert = true
	service := newService(t, repo)

	service.Record(context.Background(), contract.Entry{Action: topicProfileUpdated})
	service.RecordSecurityEvent(context.Background(), contract.SecurityEvent{
		Kind: topicAccessDenied, Severity: contract.SeverityMedium,
	})

	if repo.logCount() != 0 || repo.eventCount() != 0 {
		t.Fatal("the fake accepted a write it was told to fail")
	}
}

func TestRecordRefusesAMalformedAction(t *testing.T) {
	t.Parallel()

	repo := newFakeRepository()
	service := newService(t, repo)

	for _, action := range []string{"", "profileUpdated", "user", "user.profile.updated"} {
		service.Record(context.Background(), contract.Entry{Action: action})
	}
	if repo.logCount() != 0 {
		t.Errorf("%d malformed actions were recorded; the search filters on exact names", repo.logCount())
	}
}

// TestRecordRedactsBeforeItStores is the one that keeps personal data out of a
// table nobody can UPDATE and which is kept for two years.
func TestRecordRedactsBeforeItStores(t *testing.T) {
	t.Parallel()

	repo := newFakeRepository()
	service := newService(t, repo)

	service.Record(context.Background(), contract.Entry{
		Action: topicProfileUpdated,
		Before: map[string]any{testFieldDisplayName: "Nghi", testFieldTimezone: "UTC"},
		After:  map[string]any{testFieldDisplayName: "Nghi Nguyen", testFieldTimezone: "Asia/Ho_Chi_Minh"},
		Meta:   map[string]any{"reason": "support request", "email": "learner@example.com"},
	})

	stored := repo.lastLog()
	if stored.Before[testFieldDisplayName] == "Nghi" || stored.After[testFieldDisplayName] == "Nghi Nguyen" {
		t.Errorf("a display name reached the table: before=%v after=%v", stored.Before, stored.After)
	}
	if stored.Meta["email"] == "learner@example.com" {
		t.Errorf("an email address reached the table through Meta: %v", stored.Meta)
	}
	if stored.After[testFieldTimezone] != "Asia/Ho_Chi_Minh" {
		t.Errorf("timezone was redacted too; it is not personal data: %v", stored.After)
	}
	if stored.Meta["reason"] != "support request" {
		t.Errorf("the stated reason was lost: %v", stored.Meta)
	}
	want := []string{testFieldDisplayName, testFieldTimezone}
	if len(stored.ChangedFields) != len(want) {
		t.Errorf("changed_fields = %v, want %v", stored.ChangedFields, want)
	}
}

// TestRecordTakesTheActorFromContext is the rule that a caller cannot attribute
// an action to somebody else: there is no actor field on Entry to lie in.
func TestRecordTakesTheActorFromContext(t *testing.T) {
	t.Parallel()

	repo := newFakeRepository()
	service := newService(t, repo)

	actorID := uuid.New()
	ctx := httpx.WithActor(context.Background(), httpx.Actor{UserID: actorID, Role: testRoleAdmin})
	service.Record(ctx, contract.Entry{
		Action: "user.read_profile", TargetType: targetTypeUser, TargetID: actorID.String(),
	})

	stored := repo.lastLog()
	if stored.ActorID == nil || *stored.ActorID != actorID {
		t.Fatalf("actor_id = %v, want %s", stored.ActorID, actorID)
	}
	if stored.ActorRole == nil || *stored.ActorRole != contract.ActorRoleAdmin {
		t.Errorf("actor_role = %v, want admin recorded as it was at the time", stored.ActorRole)
	}
}

func TestRecordWithoutAnActorRecordsNoActor(t *testing.T) {
	t.Parallel()

	repo := newFakeRepository()
	service := newService(t, repo)

	service.Record(context.Background(), contract.Entry{Action: "system.rotated_partitions"})

	stored := repo.lastLog()
	if stored.ActorID != nil || stored.ActorRole != nil {
		t.Errorf("an entry with no caller got actor %v/%v", stored.ActorID, stored.ActorRole)
	}
}

func TestRecordHashesTheClientAddress(t *testing.T) {
	t.Parallel()

	repo := newFakeRepository()
	service := newService(t, repo)

	ctx := contextWithClientIP(t, netip.MustParseAddr("203.0.113.7"))
	service.Record(ctx, contract.Entry{Action: topicProfileUpdated})

	stored := repo.lastLog()
	if stored.IPHash == nil {
		t.Fatal("no ip_hash was recorded")
	}
	if len(*stored.IPHash) != 64 {
		t.Errorf("ip_hash = %q, want a 64-character hex digest", *stored.IPHash)
	}
	if *stored.IPHash == "203.0.113.7" {
		t.Error("the raw address was stored")
	}
}

// TestRecordStoresNoAddressWithoutAKey: a keyless deployment records nothing
// rather than an unkeyed digest that only looks like protection.
func TestRecordStoresNoAddressWithoutAKey(t *testing.T) {
	t.Parallel()

	repo := newFakeRepository()
	frozen := &clock.Fake{}
	frozen.Set(fixedNow)
	service := New(Deps{Repo: repo, Clock: frozen})

	ctx := contextWithClientIP(t, netip.MustParseAddr("203.0.113.7"))
	service.Record(ctx, contract.Entry{Action: topicProfileUpdated})

	if stored := repo.lastLog(); stored.IPHash != nil {
		t.Errorf("ip_hash = %v with no key configured, want nothing", *stored.IPHash)
	}
}

func TestRecordSecurityEventDefaultsAnUnknownSeverity(t *testing.T) {
	t.Parallel()

	repo := newFakeRepository()
	service := newService(t, repo)

	service.RecordSecurityEvent(context.Background(), contract.SecurityEvent{
		Kind: "auth.login_failed", Severity: contract.Severity("catastrophic"),
	})

	if stored := repo.lastEvent(); stored.Severity != contract.SeverityLow {
		t.Errorf("severity = %q, want it filed quietly rather than dropped", stored.Severity)
	}
}

// ------------------------------------------------------------- the consumer

func delivery(t *testing.T, topic string, payload any) Delivery {
	t.Helper()
	encoded, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	eventID, err := uuid.NewV7()
	if err != nil {
		t.Fatalf("NewV7: %v", err)
	}
	return Delivery{ID: eventID, Topic: topic, Payload: encoded}
}

// TestConsumeIsIdempotentOnTheEventID is the acceptance criterion "a duplicate
// event produces one row", at the service level. The integration test proves
// the same thing against the real unique index.
func TestConsumeIsIdempotentOnTheEventID(t *testing.T) {
	t.Parallel()

	repo := newFakeRepository()
	service := newService(t, repo)

	event := delivery(t, topicProfileUpdated, map[string]any{
		fieldUserID:      uuid.New(),
		fieldActorID:     uuid.New(),
		"changed_fields": []string{testFieldDisplayName},
		"occurred_at":    fixedNow,
	})

	for range 5 {
		if err := service.Consume(context.Background(), event); err != nil {
			t.Fatalf("Consume: %v", err)
		}
	}
	if repo.logCount() != 1 {
		t.Errorf("five deliveries produced %d rows, want 1", repo.logCount())
	}
}

// TestConsumeIsIdempotentWithoutAnOccurredAt is the subtler half. created_at is
// the partition key and half the dedup key, so an event carrying no timestamp
// must still resolve to the same one every time — from its own version 7 id,
// not from the clock.
func TestConsumeIsIdempotentWithoutAnOccurredAt(t *testing.T) {
	t.Parallel()

	repo := newFakeRepository()
	moving := &clock.Fake{}
	moving.Set(fixedNow)
	service := New(Deps{Repo: repo, Clock: moving})

	event := delivery(t, "rbac.role_assigned", map[string]any{fieldUserID: uuid.New()})

	if err := service.Consume(context.Background(), event); err != nil {
		t.Fatalf("first Consume: %v", err)
	}
	// Time passes between deliveries, which is the realistic case: a redelivery
	// happens seconds or hours later.
	moving.Advance(36 * time.Hour)
	if err := service.Consume(context.Background(), event); err != nil {
		t.Fatalf("second Consume: %v", err)
	}

	if repo.logCount() != 1 {
		t.Errorf("a redelivery an hour later produced %d rows, want 1", repo.logCount())
	}
}

func TestConsumeRoutesSecurityTopicsToTheOtherStream(t *testing.T) {
	t.Parallel()

	repo := newFakeRepository()
	service := newService(t, repo)

	if err := service.Consume(context.Background(), delivery(t, topicAccessDenied, map[string]any{
		fieldUserID: uuid.New(), "permission": "user.suspend", "severity": "medium",
	})); err != nil {
		t.Fatalf("Consume: %v", err)
	}
	if err := service.Consume(context.Background(), delivery(t, topicProfileUpdated, map[string]any{
		fieldUserID: uuid.New(),
	})); err != nil {
		t.Fatalf("Consume: %v", err)
	}

	if repo.eventCount() != 1 {
		t.Errorf("security stream has %d rows, want 1", repo.eventCount())
	}
	if repo.logCount() != 1 {
		t.Errorf("audit trail has %d rows, want 1", repo.logCount())
	}
	if stored := repo.lastEvent(); stored.Detail["permission"] != "user.suspend" {
		t.Errorf("detail = %v, want the permission that was refused", stored.Detail)
	}
}

// TestConsumeAcknowledgesAnUnreadablePayload: a payload that will not parse
// will not parse on the hundredth attempt either, and retrying it forever
// blocks the queue behind it.
func TestConsumeAcknowledgesAnUnreadablePayload(t *testing.T) {
	t.Parallel()

	repo := newFakeRepository()
	service := newService(t, repo)

	eventID, err := uuid.NewV7()
	if err != nil {
		t.Fatalf("NewV7: %v", err)
	}
	broken := Delivery{ID: eventID, Topic: topicProfileUpdated, Payload: []byte("{not json")}

	if err := service.Consume(context.Background(), broken); err != nil {
		t.Errorf("Consume returned %v, which asks the publisher to redeliver forever", err)
	}
	if repo.logCount() != 0 {
		t.Error("a row was written from a payload that could not be read")
	}
}

// TestConsumeReturnsAWriteFailureSoItIsRedelivered is the other side of that
// judgement: a transient failure must not be acknowledged.
func TestConsumeReturnsAWriteFailureSoItIsRedelivered(t *testing.T) {
	t.Parallel()

	repo := newFakeRepository()
	repo.failInsert = true
	service := newService(t, repo)

	err := service.Consume(context.Background(), delivery(t, topicProfileUpdated, map[string]any{}))
	if err == nil {
		t.Error("Consume acknowledged an event it failed to write; the record would be lost")
	}
}

// TestConsumeTakesTheActorFromThePayload — the consumer runs in the worker,
// where nobody is signed in, so an actor in the context (there should be none)
// must not be mistaken for the person who acted.
func TestConsumeTakesTheActorFromThePayload(t *testing.T) {
	t.Parallel()

	repo := newFakeRepository()
	service := newService(t, repo)

	payloadActor := uuid.New()
	contextActor := uuid.New()
	ctx := httpx.WithActor(context.Background(), httpx.Actor{UserID: contextActor, Role: testRoleAdmin})

	if err := service.Consume(ctx, delivery(t, "rbac.role_assigned", map[string]any{
		fieldActorID: payloadActor, fieldUserID: uuid.New(), "role": "admin",
	})); err != nil {
		t.Fatalf("Consume: %v", err)
	}

	stored := repo.lastLog()
	if stored.ActorID == nil || *stored.ActorID != payloadActor {
		t.Fatalf("actor_id = %v, want the payload's actor %s", stored.ActorID, payloadActor)
	}
	if stored.ActorRole != nil {
		t.Errorf("actor_role = %v; the worker has no basis to claim one", *stored.ActorRole)
	}
}

func TestConsumeDropsATopicThatIsNotAnActionName(t *testing.T) {
	t.Parallel()

	repo := newFakeRepository()
	service := newService(t, repo)

	eventID, _ := uuid.NewV7()
	if err := service.Consume(context.Background(), Delivery{
		ID: eventID, Topic: "not-an-action", Payload: []byte(`{}`),
	}); err != nil {
		t.Errorf("Consume returned %v; a malformed topic is permanent, not transient", err)
	}
	if repo.logCount() != 0 {
		t.Error("a row was written under a topic the search could never match")
	}
}

func TestSubscribedTopicsCoverWhatUserAndRBACPublish(t *testing.T) {
	t.Parallel()

	// These names are duplicated from user/contract and rbac/contract on
	// purpose — audit may not import them. This asserts the duplication is
	// still correct; the integration test asserts it end to end.
	required := []string{
		topicProfileUpdated, "user.preferences_updated",
		"rbac.role_assigned", "rbac.role_revoked",
	}
	subscribed := make(map[string]struct{}, len(SubscribedTopics()))
	for _, topic := range SubscribedTopics() {
		subscribed[topic] = struct{}{}
		if !domain.ValidName(topic) {
			t.Errorf("subscribed topic %q is not a valid action name and could never be stored", topic)
		}
	}
	for _, topic := range required {
		if _, found := subscribed[topic]; !found {
			t.Errorf("audit does not subscribe to %q, so those writes would never be audited", topic)
		}
	}
}

// ------------------------------------------------------------ search & triage

func TestSearchReportsHasMoreWithoutReturningTheProbeRow(t *testing.T) {
	t.Parallel()

	repo := newFakeRepository()
	service := newService(t, repo)

	for range 5 {
		service.Record(context.Background(), contract.Entry{Action: topicProfileUpdated})
	}

	entries, hasMore, err := service.SearchLogs(context.Background(), domain.LogQuery{Limit: 2})
	if err != nil {
		t.Fatalf("SearchLogs: %v", err)
	}
	if len(entries) != 2 {
		t.Errorf("returned %d entries, want the 2 asked for and not the probe row", len(entries))
	}
	if !hasMore {
		t.Error("has_more = false with 5 rows and a limit of 2")
	}

	all, hasMore, err := service.SearchLogs(context.Background(), domain.LogQuery{Limit: 50})
	if err != nil {
		t.Fatalf("SearchLogs: %v", err)
	}
	if len(all) != 5 || hasMore {
		t.Errorf("last page = %d entries, has_more = %v; want 5 and false", len(all), hasMore)
	}
}

func TestSecurityEventSearchProbesForAnotherPageToo(t *testing.T) {
	t.Parallel()

	repo := newFakeRepository()
	service := newService(t, repo)

	for range 4 {
		service.RecordSecurityEvent(context.Background(), contract.SecurityEvent{
			Kind: topicAccessDenied, Severity: contract.SeverityLow,
		})
	}

	events, hasMore, err := service.SearchSecurityEvents(context.Background(), domain.SecurityQuery{Limit: 2})
	if err != nil {
		t.Fatalf("SearchSecurityEvents: %v", err)
	}
	if len(events) != 2 || !hasMore {
		t.Errorf("page = %d events, has_more = %v; want 2 and true", len(events), hasMore)
	}
	if events[0].Resolved() {
		t.Error("a freshly recorded event reports itself resolved")
	}

	all, hasMore, err := service.SearchSecurityEvents(context.Background(), domain.SecurityQuery{})
	if err != nil {
		t.Fatalf("SearchSecurityEvents: %v", err)
	}
	if len(all) != 4 || hasMore {
		t.Errorf("last page = %d events, has_more = %v; want 4 and false", len(all), hasMore)
	}
}

func TestResolveSecurityEventIsAConflictTheSecondTime(t *testing.T) {
	t.Parallel()

	repo := newFakeRepository()
	service := newService(t, repo)

	service.RecordSecurityEvent(context.Background(), contract.SecurityEvent{
		Kind: topicAccessDenied, Severity: contract.SeverityMedium,
	})
	stored := repo.lastEvent()
	admin := uuid.New()

	resolved, err := service.ResolveSecurityEvent(context.Background(), stored.ID, admin, "Known load test.")
	if err != nil {
		t.Fatalf("first resolve: %v", err)
	}
	if !resolved.Resolved() || resolved.ResolvedBy == nil || *resolved.ResolvedBy != admin {
		t.Fatalf("resolved record = %+v", resolved)
	}
	if resolved.ResolutionNote == nil || *resolved.ResolutionNote != "Known load test." {
		t.Errorf("note = %v, want the explanation given", resolved.ResolutionNote)
	}

	_, err = service.ResolveSecurityEvent(context.Background(), stored.ID, uuid.New(), "A second opinion.")
	if !apperr.Is(err, apperr.Conflict) {
		t.Errorf("second resolve error = %v, want a conflict rather than an overwrite", err)
	}
}

func TestResolveSecurityEventRejectsAnEmptyNote(t *testing.T) {
	t.Parallel()

	repo := newFakeRepository()
	service := newService(t, repo)

	_, err := service.ResolveSecurityEvent(context.Background(), uuid.New(), uuid.New(), "   ")
	if !apperr.Is(err, apperr.Validation) {
		t.Errorf("error = %v, want a validation failure; a closed event must explain itself", err)
	}
}

func TestResolveSecurityEventIsNotFoundForAnUnknownID(t *testing.T) {
	t.Parallel()

	repo := newFakeRepository()
	service := newService(t, repo)

	_, err := service.ResolveSecurityEvent(context.Background(), uuid.New(), uuid.New(), "Triaged.")
	if !apperr.Is(err, apperr.NotFound) {
		t.Errorf("error = %v, want not found", err)
	}
}

// --------------------------------------------------------------- partitions

func TestRotatePartitionsAsksForTheDocumentedLookahead(t *testing.T) {
	t.Parallel()

	repo := newFakeRepository()
	service := newService(t, repo)

	if err := service.RotatePartitions(context.Background()); err != nil {
		t.Fatalf("RotatePartitions: %v", err)
	}
	if len(repo.partitions) != 1 || repo.partitions[0] != PartitionsAhead {
		t.Errorf("asked for %v months ahead, want [%d]", repo.partitions, PartitionsAhead)
	}
}

func TestEnforceRetentionAsksForTwoYears(t *testing.T) {
	t.Parallel()

	repo := newFakeRepository()
	repo.detachResult = []string{"audit_logs_y2024m01"}
	service := newService(t, repo)

	if err := service.EnforceRetention(context.Background()); err != nil {
		t.Fatalf("EnforceRetention: %v", err)
	}
	if len(repo.retentions) != 1 || repo.retentions[0] != RetentionPeriod {
		t.Errorf("retention = %v, want [%s] (BR-AUDIT-05)", repo.retentions, RetentionPeriod)
	}
}

func TestRotatePartitionsSurfacesAFailure(t *testing.T) {
	t.Parallel()

	repo := newFakeRepository()
	repo.failEnsure = true
	service := newService(t, repo)

	if err := service.RotatePartitions(context.Background()); err == nil {
		t.Error("a partition that could not be created was reported as success")
	}
}
