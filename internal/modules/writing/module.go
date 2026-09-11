package writing

import (
	"context"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	contentcontract "github.com/fluentra/fluentra/internal/modules/content/contract"
	learningcontract "github.com/fluentra/fluentra/internal/modules/learning/contract"
	"github.com/fluentra/fluentra/internal/modules/writing/contract"
	writingjob "github.com/fluentra/fluentra/internal/modules/writing/job"
	writingrepo "github.com/fluentra/fluentra/internal/modules/writing/repository"
	"github.com/fluentra/fluentra/internal/modules/writing/service"
	writinghttp "github.com/fluentra/fluentra/internal/modules/writing/transport/http"
	"github.com/fluentra/fluentra/internal/platform/ai"
	"github.com/fluentra/fluentra/internal/platform/job"
	"github.com/fluentra/fluentra/internal/shared/clock"
	"github.com/fluentra/fluentra/internal/shared/dbx"
)

// WorkerNudger signals a background worker to wake up after a job is enqueued.
type WorkerNudger = service.WorkerNudger

// Deps defines dependencies supplied by the composition root.
type Deps struct {
	Pool         *pgxpool.Pool
	Enqueuer     job.Enqueuer
	Content      contentcontract.Reader
	AI           ai.Client
	Counter      learningcontract.AttemptCounter
	Attempts     learningcontract.AttemptReader
	Completer    learningcontract.AsyncGradingCompleter
	WorkerNudger WorkerNudger
	Clock        clock.Clock
	DailyLimit   int
}

// Module represents the wired writing module.
type Module struct {
	grader  *service.Grader
	repo    writingrepo.Repository
	handler *writinghttp.Handler
}

// New constructs a writing module.
func New(deps Deps) *Module {
	var enqueuer service.JobEnqueuer
	if deps.Pool != nil && deps.Enqueuer != nil {
		enqueuer = jobEnqueuerAdapter{
			pool:     deps.Pool,
			enqueuer: deps.Enqueuer,
		}
	}

	repo := writingrepo.New(deps.Pool)
	handler := writinghttp.NewHandler(repo)

	grader := service.NewGraderWithDeps(service.GraderDeps{
		Content:    deps.Content,
		AI:         deps.AI,
		Counter:    deps.Counter,
		Attempts:   deps.Attempts,
		Completer:  deps.Completer,
		Feedback:   repo,
		Enqueuer:   enqueuer,
		Nudger:     deps.WorkerNudger,
		Clock:      deps.Clock,
		DailyLimit: deps.DailyLimit,
	})

	return &Module{
		grader:  grader,
		repo:    repo,
		handler: handler,
	}
}

// Grader exposes the writing exercise grader.
func (m *Module) Grader() contract.Grader {
	return m.grader
}

// Routes mounts the writing endpoints on the router.
func (m *Module) Routes(r chi.Router) {
	if m.handler != nil {
		m.handler.Routes(r)
	}
}

// FeedbackReader exposes the feedback reading contract.
func (m *Module) FeedbackReader() contract.FeedbackReader {
	return m.repo
}

// GradeSubmissionWorker returns the River worker for grading writing submissions.
func (m *Module) GradeSubmissionWorker() *writingjob.GradeSubmissionWorker {
	return writingjob.NewGradeSubmissionWorker(m.grader)
}

type jobEnqueuerAdapter struct {
	pool     *pgxpool.Pool
	enqueuer job.Enqueuer
}

func (a jobEnqueuerAdapter) EnqueueGradeSubmission(ctx context.Context, attemptID uuid.UUID) error {
	return dbx.InTx(ctx, a.pool, func(txCtx context.Context, tx pgx.Tx) error {
		args := writingjob.GradeSubmissionArgs{AttemptID: attemptID}
		_, err := a.enqueuer.EnqueueTx(txCtx, tx, args, nil)
		return err
	})
}
