package service_test

import (
	"context"
	"errors"
	"slices"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/fluentra/fluentra/internal/modules/rbac/contract"
	"github.com/fluentra/fluentra/internal/modules/rbac/domain"
	"github.com/fluentra/fluentra/internal/modules/rbac/service"
)

// fakeRepo is an in-memory role store. It counts its calls so a test can show
// that a cached resolution did not reach the database.
type fakeRepo struct {
	mu sync.Mutex

	assignments map[uuid.UUID][]contract.Role
	rolePerms   map[contract.Role][]contract.Permission

	calls  map[string]int
	failOn map[string]error
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{
		assignments: map[uuid.UUID][]contract.Role{},
		rolePerms: map[contract.Role][]contract.Permission{
			contract.RoleAdmin: contract.All(),
			contract.RoleUser:  {},
		},
		calls:  map[string]int{},
		failOn: map[string]error{},
	}
}

func (f *fakeRepo) record(method string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls[method]++
	return f.failOn[method]
}

func (f *fakeRepo) callCount(method string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls[method]
}

func (f *fakeRepo) RolesOf(_ context.Context, userID uuid.UUID) ([]contract.Role, error) {
	if err := f.record("RolesOf"); err != nil {
		return nil, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]contract.Role(nil), f.assignments[userID]...), nil
}

func (f *fakeRepo) PermissionsOf(_ context.Context, userID uuid.UUID) ([]contract.Permission, error) {
	if err := f.record("PermissionsOf"); err != nil {
		return nil, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	seen := map[contract.Permission]struct{}{}
	var permissions []contract.Permission
	for _, role := range f.assignments[userID] {
		for _, permission := range f.rolePerms[role] {
			if _, duplicate := seen[permission]; duplicate {
				continue
			}
			seen[permission] = struct{}{}
			permissions = append(permissions, permission)
		}
	}
	return permissions, nil
}

func (f *fakeRepo) ListRoles(_ context.Context) ([]domain.RoleWithPermissions, error) {
	if err := f.record("ListRoles"); err != nil {
		return nil, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	return []domain.RoleWithPermissions{
		{Name: contract.RoleAdmin, Description: "Full administrative access.",
			Permissions: f.rolePerms[contract.RoleAdmin]},
		{Name: contract.RoleUser, Description: "A learner.", Permissions: f.rolePerms[contract.RoleUser]},
	}, nil
}

func (f *fakeRepo) RoleExists(_ context.Context, role contract.Role) (bool, error) {
	if err := f.record("RoleExists"); err != nil {
		return false, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	_, ok := f.rolePerms[role]
	return ok, nil
}

func (f *fakeRepo) AssignRole(
	_ context.Context, userID uuid.UUID, role contract.Role, _ *uuid.UUID,
) (bool, error) {
	if err := f.record("AssignRole"); err != nil {
		return false, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if slices.Contains(f.assignments[userID], role) {
		return false, nil
	}
	f.assignments[userID] = append(f.assignments[userID], role)
	slices.Sort(f.assignments[userID])
	return true, nil
}

func (f *fakeRepo) RevokeRole(_ context.Context, userID uuid.UUID, role contract.Role) (bool, error) {
	if err := f.record("RevokeRole"); err != nil {
		return false, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	held := f.assignments[userID]
	index := slices.Index(held, role)
	if index < 0 {
		return false, nil
	}
	f.assignments[userID] = slices.Delete(held, index, index+1)
	return true, nil
}

func (f *fakeRepo) LockAndCountRole(_ context.Context, role contract.Role) (int, error) {
	if err := f.record("LockAndCountRole"); err != nil {
		return 0, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	count := 0
	for _, roles := range f.assignments {
		if slices.Contains(roles, role) {
			count++
		}
	}
	return count, nil
}

func (f *fakeRepo) DeleteAssignmentsForUser(_ context.Context, userID uuid.UUID) (int, error) {
	if err := f.record("DeleteAssignmentsForUser"); err != nil {
		return 0, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	removed := len(f.assignments[userID])
	delete(f.assignments, userID)
	return removed, nil
}

// WithTx returns the same fake: what these tests check is which calls a use
// case makes and in what order, and a fake that imitated rollback would only
// be testing itself.
func (f *fakeRepo) WithTx(pgx.Tx) service.Repository { return f }

// fakeCache is an in-memory permission cache that records what happened to it,
// so a test can assert a revocation busted the key rather than waiting out a
// five-minute TTL.
type fakeCache struct {
	mu      sync.Mutex
	entries map[string][]string
	deleted []string
	getErr  error
	setErr  error
	delErr  error
	gets    int
}

func newFakeCache() *fakeCache { return &fakeCache{entries: map[string][]string{}} }

func (c *fakeCache) Get(_ context.Context, key string) ([]string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.gets++
	if c.getErr != nil {
		return nil, c.getErr
	}
	value, ok := c.entries[key]
	if !ok {
		return nil, errors.New("cache miss")
	}
	return append([]string(nil), value...), nil
}

func (c *fakeCache) Set(_ context.Context, key string, value []string, _ time.Duration) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.setErr != nil {
		return c.setErr
	}
	c.entries[key] = append([]string(nil), value...)
	return nil
}

func (c *fakeCache) Delete(_ context.Context, keys ...string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.delErr != nil {
		return c.delErr
	}
	for _, key := range keys {
		delete(c.entries, key)
		c.deleted = append(c.deleted, key)
	}
	return nil
}

func (c *fakeCache) stored(key string) ([]string, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	value, ok := c.entries[key]
	return value, ok
}

func (c *fakeCache) deleteCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.deleted)
}

// fakeBeginner satisfies dbx.Beginner with a transaction that always commits.
type fakeBeginner struct {
	begins    int
	rollbacks int
	commitErr error
}

func (b *fakeBeginner) BeginTx(context.Context, pgx.TxOptions) (pgx.Tx, error) {
	b.begins++
	return &fakeTx{owner: b}, nil
}

type fakeTx struct{ owner *fakeBeginner }

func (t *fakeTx) Begin(context.Context) (pgx.Tx, error) {
	panic("nested transactions are not modelled")
}
func (t *fakeTx) Commit(context.Context) error   { return t.owner.commitErr }
func (t *fakeTx) Rollback(context.Context) error { t.owner.rollbacks++; return nil }
func (t *fakeTx) Conn() *pgx.Conn                { return nil }

func (t *fakeTx) CopyFrom(context.Context, pgx.Identifier, []string, pgx.CopyFromSource) (int64, error) {
	panic("CopyFrom is not modelled")
}
func (t *fakeTx) SendBatch(context.Context, *pgx.Batch) pgx.BatchResults {
	panic("SendBatch is not modelled")
}
func (t *fakeTx) LargeObjects() pgx.LargeObjects { panic("LargeObjects is not modelled") }

func (t *fakeTx) Prepare(context.Context, string, string) (*pgconn.StatementDescription, error) {
	panic("Prepare is not modelled")
}

func (t *fakeTx) Exec(context.Context, string, ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}
func (t *fakeTx) Query(context.Context, string, ...any) (pgx.Rows, error) {
	panic("Query is not modelled")
}
func (t *fakeTx) QueryRow(context.Context, string, ...any) pgx.Row { panic("QueryRow is not modelled") }

// recordingEvents captures what was published.
type recordingEvents struct {
	mu       sync.Mutex
	written  []writtenEvent
	writeErr error
}

type writtenEvent struct {
	Aggregate string
	Event     string
	Payload   any
}

func (r *recordingEvents) Write(
	_ context.Context, tx service.OutboxTx, aggregate, event string, payload any,
) (uuid.UUID, error) {
	if tx == nil {
		return uuid.Nil, errors.New("the outbox was handed a nil transaction")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.writeErr != nil {
		return uuid.Nil, r.writeErr
	}
	r.written = append(r.written, writtenEvent{Aggregate: aggregate, Event: event, Payload: payload})
	return uuid.New(), nil
}

func (r *recordingEvents) events() []writtenEvent {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]writtenEvent(nil), r.written...)
}

// FirstHolderOf answers with whoever the fixture assigned the role first, and
// uuid.Nil when nobody holds it — the same "nobody, and that is not an error"
// the real repository returns for a database with no administrator yet.
func (f *fakeRepo) FirstHolderOf(_ context.Context, role contract.Role) (uuid.UUID, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for userID, roles := range f.assignments {
		for _, held := range roles {
			if held == role {
				return userID, nil
			}
		}
	}
	return uuid.Nil, nil
}
