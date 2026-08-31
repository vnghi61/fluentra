// Package gamification wires the XP, streak, badge, quest and leaderboard
// module. `New(deps)` is the only symbol cmd/ imports.
package gamification

import (
	"context"
	"fmt"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/fluentra/fluentra/internal/modules/gamification/contract"
	"github.com/fluentra/fluentra/internal/modules/gamification/repository"
	"github.com/fluentra/fluentra/internal/modules/gamification/service"
	gamificationhttp "github.com/fluentra/fluentra/internal/modules/gamification/transport/http"
	usercontract "github.com/fluentra/fluentra/internal/modules/user/contract"
	"github.com/fluentra/fluentra/internal/platform/job"
	"github.com/fluentra/fluentra/internal/shared/clock"
	"github.com/fluentra/fluentra/internal/shared/eventbus"
	"github.com/fluentra/fluentra/internal/shared/outbox"
)

// Advisory lock ids, derived from this module's migration timestamp so they
// cannot collide with another module's.
const (
	sweepStreaksLockID     int64 = 1_700_000_261
	buildLeaderboardLockID int64 = 1_700_000_262
)

// sweepInterval is hourly, because the streak day boundary is per learner: a
// job that ran once a day would run at the wrong time for most of the world.
const sweepInterval = time.Hour

// leaderboardInterval rebuilds the standings often enough that they feel live
// without ranking on the request path.
const leaderboardInterval = 15 * time.Minute

// Guard is the authorization interface the HTTP handlers enforce with.
type Guard = gamificationhttp.Guard

// Deps are supplied by the composition root.
type Deps struct {
	Pool  *pgxpool.Pool
	Clock clock.Clock
	Guard Guard
	Users usercontract.Reader
}

// Module is the wired gamification module.
type Module struct {
	service *service.Service
	handler *gamificationhttp.Handler
}

// New constructs and wires the module.
func New(deps Deps) *Module {
	timekeeper := deps.Clock
	if timekeeper == nil {
		timekeeper = clock.Real{}
	}

	srv := service.New(service.Deps{
		Pool:   deps.Pool,
		Repo:   repository.New(deps.Pool),
		Users:  deps.Users,
		Events: outboxWriter{Writer: outbox.NewWriter()},
		Clock:  timekeeper,
	})

	// The handler exists only when there is a Guard to enforce with. cmd/worker
	// builds this module for its jobs and its consumer and serves no HTTP, so
	// it passes none — and constructing the handler regardless would panic the
	// worker at boot on GUARD_REQUIRED, exactly as it once did for srs.
	var handler *gamificationhttp.Handler
	if deps.Guard != nil {
		built, err := gamificationhttp.NewHandler(srv, deps.Guard)
		if err != nil {
			panic(fmt.Sprintf("failed to construct gamification HTTP handler: %v", err))
		}
		handler = built
	}

	return &Module{service: srv, handler: handler}
}

// Routes mounts the endpoints. A module built without a Guard mounts nothing,
// rather than serving unguarded routes.
func (m *Module) Routes(router chi.Router) {
	if m.handler != nil {
		m.handler.Routes(router)
	}
}

// Reader is the read side other modules and the dashboard use.
func (m *Module) Reader() contract.Reader { return m.service }

// Awarder lets an already-asynchronous caller pay XP directly.
//
// The event path stays the normal route in. This exists for the vocabulary
// verification job, which runs on a schedule, owns its own transaction, and has
// no learning request behind it to block.
func (m *Module) Awarder() contract.Awarder { return m.service }

// Subscribe registers the event consumer on every topic gamification reacts to.
func (m *Module) Subscribe(bus eventbus.EventBus) error {
	for _, topic := range service.SubscribedTopics() {
		if err := bus.Subscribe(topic, m.handleMessage); err != nil {
			return fmt.Errorf("subscribe gamification consumer to %s: %w", topic, err)
		}
	}
	return nil
}

func (m *Module) handleMessage(ctx context.Context, message eventbus.Message) error {
	return m.service.Consume(ctx, service.Delivery{
		ID:      message.ID,
		Topic:   message.Topic,
		Payload: message.Payload,
	})
}

// CronJobs returns the scheduled work this module owns. Both are idempotent, so
// losing the advisory lock to another replica costs nothing.
func (m *Module) CronJobs() []job.CronJob {
	return []job.CronJob{
		{
			Name:     "gamification.sweep_streaks",
			LockID:   sweepStreaksLockID,
			Interval: sweepInterval,
			Task:     m.service.SweepStreaks,
		},
		{
			Name:     "gamification.build_leaderboard",
			LockID:   buildLeaderboardLockID,
			Interval: leaderboardInterval,
			Task:     m.service.BuildLeaderboard,
		},
	}
}

type outboxWriter struct{ *outbox.Writer }

func (w outboxWriter) Write(
	ctx context.Context, tx service.OutboxTx, aggregate, event string, payload any,
) (uuid.UUID, error) {
	return w.Writer.Write(ctx, outboxTx{tx}, aggregate, event, payload)
}

type outboxTx struct{ inner service.OutboxTx }

func (t outboxTx) Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error) {
	return t.inner.Exec(ctx, sql, arguments...)
}
