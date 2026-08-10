package audit

import (
	"context"
	"fmt"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/fluentra/fluentra/internal/modules/audit/contract"
	"github.com/fluentra/fluentra/internal/modules/audit/repository"
	"github.com/fluentra/fluentra/internal/modules/audit/service"
	audithttp "github.com/fluentra/fluentra/internal/modules/audit/transport/http"
	"github.com/fluentra/fluentra/internal/platform/job"
	"github.com/fluentra/fluentra/internal/shared/clock"
	"github.com/fluentra/fluentra/internal/shared/eventbus"
)

// Advisory lock ids for this module's scheduled work. They are module-local
// constants in a global namespace, so they are spaced far from zero and
// documented here: two jobs sharing an id would silently never run together,
// and the symptom is a job that "sometimes does not happen".
const (
	rotatePartitionsLockID int64 = 1_700_000_030
	enforceRetentionLockID int64 = 1_700_000_031
)

// How often the scheduled work runs.
//
// Both are far more frequent than they need to be, and deliberately so. Making
// a partition three months early is free; discovering at 00:00 on the first of
// the month that nobody made one is an outage on the write path of every
// module that emits an event.
const (
	rotateInterval    = 6 * time.Hour
	retentionInterval = 24 * time.Hour
)

// Guard is the authorization surface this module's admin operations need,
// re-exported so the composition root can name it without reaching into
// transport/http.
//
// It takes a permission as a string because `audit` does not depend on `rbac`:
// every arrow in MODULE_INDEX.md §3 points into this module. The composition
// root adapts the real Authorizer, and it is the only place entitled to see
// both.
type Guard = audithttp.Guard

// Deps are what the composition root supplies.
type Deps struct {
	Pool  *pgxpool.Pool
	Clock clock.Clock

	// Guard authorises the admin operations.
	//
	// It may resolve its authorizer lazily, and in the composition root it
	// does. `rbac` records into `audit`, so `audit` is constructed first; but
	// `audit`'s own admin operations are guarded by `rbac`. That circle is real
	// and an indirection here is where it is broken, rather than by building
	// the two in an order that only works while `rbac` happens not to need a
	// recorder yet.
	Guard Guard

	// IPHashKey keys the HMAC that turns a client address into a stored hash.
	// Empty means no address is recorded at all, which is the safe default: an
	// unkeyed digest of an IPv4 address is trivially reversible.
	IPHashKey []byte
}

// Module is the audit module, assembled.
type Module struct {
	service *service.Service
	handler *audithttp.Handler
}

// New wires the module.
//
// There are none of the two adapters the other modules need in here. No
// repositoryAdapter, because the service declares no WithTx: rule L4 means an
// audit write is never inside another module's transaction, so there is no
// transaction to join. No outboxWriter, because this module publishes nothing
// — every arrow in MODULE_INDEX.md §3 points into it.
func New(deps Deps) *Module {
	timekeeper := deps.Clock
	if timekeeper == nil {
		timekeeper = clock.Real{}
	}

	trail := service.New(service.Deps{
		Repo:      repository.New(deps.Pool),
		Clock:     timekeeper,
		IPHashKey: deps.IPHashKey,
	})

	return &Module{
		service: trail,
		handler: audithttp.NewHandler(trail, deps.Guard, timekeeper.Now),
	}
}

// Routes mounts the module's HTTP operations.
//
// They are `/admin` paths but this does not mount an `/admin` group: `rbac`
// already does, and chi allows one handler per mount point. The composition
// root wraps these in rbac's AdminOnly.
func (m *Module) Routes(router chi.Router) { m.handler.Routes(router) }

// Recorder is the best-effort write surface every other module calls.
func (m *Module) Recorder() contract.Recorder { return m.service }

// SecurityRecorder is the same for the security event stream.
func (m *Module) SecurityRecorder() contract.SecurityRecorder { return m.service }

// Subscribe registers the outbox consumer for every topic this module records.
//
// This is what closes the WP1 gate: the events `user` and `rbac` already write
// into ops.outbox_events in the same transaction as their business writes
// become rows in audit.audit_logs on the other side of the publisher.
func (m *Module) Subscribe(bus eventbus.EventBus) error {
	for _, topic := range service.SubscribedTopics() {
		if err := bus.Subscribe(topic, m.handleMessage); err != nil {
			return fmt.Errorf("subscribe audit consumer to %s: %w", topic, err)
		}
	}
	return nil
}

// handleMessage adapts the bus's Message to the consumer's Delivery. The
// service takes its own type so it can be tested without the bus.
func (m *Module) handleMessage(ctx context.Context, message eventbus.Message) error {
	return m.service.Consume(ctx, service.Delivery{
		ID:      message.ID,
		Topic:   message.Topic,
		Payload: message.Payload,
	})
}

// CronJobs returns the scheduled work this module owns, for the worker to
// register. Both are idempotent, so losing the advisory lock to another
// replica costs nothing.
func (m *Module) CronJobs() []job.CronJob {
	return []job.CronJob{
		{
			Name:     "audit.rotate_partitions",
			LockID:   rotatePartitionsLockID,
			Interval: rotateInterval,
			Task:     m.service.RotatePartitions,
		},
		{
			Name:     "audit.enforce_retention",
			LockID:   enforceRetentionLockID,
			Interval: retentionInterval,
			Task:     m.service.EnforceRetention,
		},
	}
}

// RotatePartitions creates the partitions the coming months need. It is
// exported so the composition root can run it once at start-up rather than
// waiting for the first tick — a deployment onto a database whose partitions
// have lapsed would otherwise refuse writes until then.
func (m *Module) RotatePartitions(ctx context.Context) error {
	return m.service.RotatePartitions(ctx)
}
