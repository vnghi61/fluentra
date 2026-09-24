// Package exam wires the exam module.
package exam

import (
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	examcontract "github.com/fluentra/fluentra/internal/modules/exam/contract"
	examjob "github.com/fluentra/fluentra/internal/modules/exam/job"
	examrepo "github.com/fluentra/fluentra/internal/modules/exam/repository"
	"github.com/fluentra/fluentra/internal/modules/exam/service"
	examhttp "github.com/fluentra/fluentra/internal/modules/exam/transport/http"
	learningcontract "github.com/fluentra/fluentra/internal/modules/learning/contract"
	lessoncontract "github.com/fluentra/fluentra/internal/modules/lesson/contract"
	questionbankcontract "github.com/fluentra/fluentra/internal/modules/questionbank/contract"
	platformjob "github.com/fluentra/fluentra/internal/platform/job"
	"github.com/fluentra/fluentra/internal/shared/clock"
)

// WorkerNudger signals a background worker to wake up after a job is enqueued.
type WorkerNudger = service.WorkerNudger

// PoolDrawer draws items for an exam sitting across the 4 sections.
type PoolDrawer = service.PoolDrawer

// SectionActivities represents drawn activities for a section.
type SectionActivities = service.SectionActivities

// SittingActivityDTO represents an activity inside a sitting.
type SittingActivityDTO = service.SittingActivityDTO

// Deps defines dependencies supplied by the composition root.
type Deps struct {
	Pool         *pgxpool.Pool
	Learning     learningcontract.SittingAnswerSubmitter
	Attempts     learningcontract.AttemptOutcomeReader
	Exposures    learningcontract.ItemExposureRecorder
	Lesson       lessoncontract.Reader
	Questionbank questionbankcontract.Reader
	Drawer       PoolDrawer
	Enqueuer     platformjob.Enqueuer
	WorkerNudger WorkerNudger
	Clock        clock.Clock
	DailyLimit   int
	// BankAuthor generates the daily job's questions (WO 22 Stage O). Nil
	// leaves the daily generation job a no-op.
	BankAuthor questionbankcontract.Author
}

// Module represents the wired exam module.
type Module struct {
	service *service.Service
	handler *examhttp.Handler
}

// New constructs and wires an exam module.
func New(deps Deps) *Module {
	repo := examrepo.New(deps.Pool)

	svc := service.New(service.Deps{
		Pool:         deps.Pool,
		Repo:         repo,
		Learning:     deps.Learning,
		Attempts:     deps.Attempts,
		Exposures:    deps.Exposures,
		Lesson:       deps.Lesson,
		Questionbank: deps.Questionbank,
		Drawer:       deps.Drawer,
		Clock:        deps.Clock,
		DailyLimit:   deps.DailyLimit,
		Enqueuer:     deps.Enqueuer,
		Nudger:       deps.WorkerNudger,
		BankAuthor:   deps.BankAuthor,
	})

	handler := examhttp.NewHandler(svc)

	return &Module{
		service: svc,
		handler: handler,
	}
}

// Reader exposes the public read contract for exams.
func (m *Module) Reader() examcontract.Reader {
	return m.service
}

// Service returns the underlying exam service.
func (m *Module) Service() *service.Service {
	return m.service
}

// Routes mounts exam endpoints on the router.
func (m *Module) Routes(r chi.Router) {
	if m.handler != nil {
		m.handler.Routes(r)
	}
}

// AdminRoutes mounts exam admin endpoints on the router.
func (m *Module) AdminRoutes(r chi.Router) {
	if m.handler != nil {
		m.handler.AdminRoutes(r)
	}
}

// ExpireAttemptWorker returns the River worker for processing scheduled expired attempts.
func (m *Module) ExpireAttemptWorker() *examjob.ExpireAttemptWorker {
	return examjob.NewExpireAttemptWorker(m.service)
}

// SweepJob returns the scheduled 1-minute sweep cron job.
func (m *Module) SweepJob() platformjob.CronJob {
	return examjob.SweepExpiredJob(m.service)
}

// DailyGenerationJob returns the daily generation cron job (WO 22 Stage O).
func (m *Module) DailyGenerationJob() platformjob.CronJob {
	return examjob.DailyGenerationJob(m.service)
}
