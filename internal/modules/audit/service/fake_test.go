package service

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/fluentra/fluentra/internal/modules/audit/domain"
)

// errRepository is what a fake returns when a test asks it to fail. It stands
// in for "the database is unreachable", which is the case BR-AUDIT-02 is about.
var errRepository = errors.New("the database is unreachable")

// fakeRepository is an in-memory stand-in that keeps the one behaviour that
// actually matters here: the unique index on (event_id, created_at). Without
// modelling it the idempotency tests would pass against a fake that cannot
// fail the way the real table does.
type fakeRepository struct {
	mu sync.Mutex

	logs   []domain.LogEntry
	events []domain.SecurityRecord

	logKeys   map[string]struct{}
	eventKeys map[string]struct{}

	failInsert   bool
	failSearch   bool
	partitions   []int
	retentions   []time.Duration
	failEnsure   bool
	detachResult []string
}

func newFakeRepository() *fakeRepository {
	return &fakeRepository{
		logKeys:   make(map[string]struct{}),
		eventKeys: make(map[string]struct{}),
	}
}

func key(eventID uuid.UUID, createdAt time.Time) string {
	return eventID.String() + "@" + createdAt.UTC().Format(time.RFC3339Nano)
}

func (f *fakeRepository) InsertAuditLog(_ context.Context, entry domain.LogEntry) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.failInsert {
		return false, errRepository
	}
	dedupe := key(entry.EventID, entry.CreatedAt)
	if _, exists := f.logKeys[dedupe]; exists {
		return false, nil
	}
	f.logKeys[dedupe] = struct{}{}
	f.logs = append(f.logs, entry)
	return true, nil
}

func (f *fakeRepository) InsertSecurityEvent(_ context.Context, record domain.SecurityRecord) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.failInsert {
		return false, errRepository
	}
	dedupe := key(record.EventID, record.CreatedAt)
	if _, exists := f.eventKeys[dedupe]; exists {
		return false, nil
	}
	f.eventKeys[dedupe] = struct{}{}
	f.events = append(f.events, record)
	return true, nil
}

func (f *fakeRepository) SearchAuditLogs(_ context.Context, query domain.LogQuery) ([]domain.LogEntry, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.failSearch {
		return nil, errRepository
	}
	if query.Limit >= len(f.logs) {
		return append([]domain.LogEntry(nil), f.logs...), nil
	}
	return append([]domain.LogEntry(nil), f.logs[:query.Limit]...), nil
}

func (f *fakeRepository) SearchSecurityEvents(
	_ context.Context, query domain.SecurityQuery,
) ([]domain.SecurityRecord, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.failSearch {
		return nil, errRepository
	}
	if query.Limit >= len(f.events) {
		return append([]domain.SecurityRecord(nil), f.events...), nil
	}
	return append([]domain.SecurityRecord(nil), f.events[:query.Limit]...), nil
}

func (f *fakeRepository) GetSecurityEvent(_ context.Context, eventID uuid.UUID) (domain.SecurityRecord, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, record := range f.events {
		if record.ID == eventID {
			return record, nil
		}
	}
	return domain.SecurityRecord{}, domain.ErrEventNotFound
}

func (f *fakeRepository) ResolveSecurityEvent(_ context.Context, resolution domain.Resolution) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for index, record := range f.events {
		if record.ID != resolution.ID {
			continue
		}
		if record.ResolvedAt != nil {
			// Exactly what the `resolved_at IS NULL` predicate does.
			return false, nil
		}
		at := resolution.At
		by := resolution.By
		note := resolution.Note
		f.events[index].ResolvedAt = &at
		f.events[index].ResolvedBy = &by
		f.events[index].ResolutionNote = &note
		return true, nil
	}
	return false, nil
}

func (f *fakeRepository) EnsurePartitions(_ context.Context, monthsAhead int) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.failEnsure {
		return 0, errRepository
	}
	f.partitions = append(f.partitions, monthsAhead)
	return monthsAhead + 1, nil
}

func (f *fakeRepository) DetachExpiredPartitions(_ context.Context, retain time.Duration) ([]string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.retentions = append(f.retentions, retain)
	return f.detachResult, nil
}

func (f *fakeRepository) logCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.logs)
}

func (f *fakeRepository) eventCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.events)
}

func (f *fakeRepository) lastLog() domain.LogEntry {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.logs) == 0 {
		return domain.LogEntry{}
	}
	return f.logs[len(f.logs)-1]
}

func (f *fakeRepository) lastEvent() domain.SecurityRecord {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.events) == 0 {
		return domain.SecurityRecord{}
	}
	return f.events[len(f.events)-1]
}
