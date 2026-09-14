// Package speaking wires the speaking module.
package speaking

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	contentcontract "github.com/fluentra/fluentra/internal/modules/content/contract"
	learningcontract "github.com/fluentra/fluentra/internal/modules/learning/contract"
	"github.com/fluentra/fluentra/internal/modules/speaking/contract"
	speakingjob "github.com/fluentra/fluentra/internal/modules/speaking/job"
	speakingrepo "github.com/fluentra/fluentra/internal/modules/speaking/repository"
	"github.com/fluentra/fluentra/internal/modules/speaking/service"
	speakinghttp "github.com/fluentra/fluentra/internal/modules/speaking/transport/http"
	usercontract "github.com/fluentra/fluentra/internal/modules/user/contract"
	"github.com/fluentra/fluentra/internal/platform/ai"
	platformjob "github.com/fluentra/fluentra/internal/platform/job"
	"github.com/fluentra/fluentra/internal/platform/media"
	"github.com/fluentra/fluentra/internal/platform/storage"
	"github.com/fluentra/fluentra/internal/shared/clock"
	"github.com/fluentra/fluentra/internal/shared/dbx"
	"github.com/fluentra/fluentra/internal/shared/eventbus"
)

// WorkerNudger signals a background worker to wake up after a job is enqueued.
type WorkerNudger = service.WorkerNudger

// Deps defines dependencies supplied by the composition root.
type Deps struct {
	Pool         *pgxpool.Pool
	Enqueuer     platformjob.Enqueuer
	Storage      storage.Store
	Transcriber  media.Transcriber
	AI           ai.Client
	Content      contentcontract.Reader
	Counter      learningcontract.AttemptCounter
	Attempts     learningcontract.AttemptReader
	Completer    learningcontract.AsyncGradingCompleter
	WorkerNudger WorkerNudger
	Clock        clock.Clock
	DailyLimit   int
	ASRModel     string
	Bucket       string
}

// Module represents the wired speaking module.
type Module struct {
	service *service.Service
	grader  *service.Grader
	handler *speakinghttp.Handler
}

// New constructs and wires a speaking module.
func New(deps Deps) *Module {
	repo := speakingrepo.New(deps.Pool)

	svc := service.New(service.Deps{
		Repo:       repo,
		Storage:    deps.Storage,
		Counter:    deps.Counter,
		Clock:      deps.Clock,
		DailyLimit: deps.DailyLimit,
		Bucket:     deps.Bucket,
	})

	var enqueuer service.JobEnqueuer
	if deps.Pool != nil && deps.Enqueuer != nil {
		enqueuer = jobEnqueuerAdapter{
			pool:     deps.Pool,
			enqueuer: deps.Enqueuer,
		}
	}

	grader := service.NewGrader(service.GraderDeps{
		Content:     deps.Content,
		Attempts:    deps.Attempts,
		Completer:   deps.Completer,
		Storage:     deps.Storage,
		Transcriber: deps.Transcriber,
		AI:          deps.AI,
		Enqueuer:    enqueuer,
		Feedback:    repo,
		Counter:     deps.Counter,
		Nudger:      deps.WorkerNudger,
		Clock:       deps.Clock,
		Bucket:      deps.Bucket,
		DailyLimit:  deps.DailyLimit,
		ASRModel:    deps.ASRModel,
	})

	handler := speakinghttp.NewHandler(svc)

	return &Module{
		service: svc,
		grader:  grader,
		handler: handler,
	}
}

// Grader exposes the speaking exercise grader.
func (m *Module) Grader() contract.Grader {
	return m.grader
}

// Service returns the underlying speaking service.
func (m *Module) Service() *service.Service {
	return m.service
}

// Routes mounts speaking endpoints on the router.
func (m *Module) Routes(r chi.Router) {
	if m.handler != nil {
		m.handler.Routes(r)
	}
}

// FeedbackReader exposes the feedback reading contract.
func (m *Module) FeedbackReader() contract.FeedbackReader {
	return m.service
}

// RecordingCleaner exposes GDPR / erasure recording cleanup.
func (m *Module) RecordingCleaner() contract.RecordingCleaner {
	return m.service
}

// GradeRecordingWorker returns the River worker for processing audio recordings.
func (m *Module) GradeRecordingWorker() *speakingjob.GradeRecordingWorker {
	return speakingjob.NewGradeRecordingWorker(m.grader)
}

// PurgeJob returns the scheduled 90-day retention cron job.
func (m *Module) PurgeJob() platformjob.CronJob {
	return speakingjob.PurgeJob(m.service)
}

// Subscribe registers the module's consumers in the worker.
//
// Account erasure anonymises the user row, so the feedback rows stay and the
// cascade never runs; nothing but this consumer removes the recordings in storage.
func (m *Module) Subscribe(bus eventbus.EventBus) error {
	if err := bus.Subscribe(usercontract.EventDeleted, m.handleUserDeleted); err != nil {
		return fmt.Errorf("subscribe speaking consumer to %s: %w", usercontract.EventDeleted, err)
	}
	return nil
}

func (m *Module) handleUserDeleted(ctx context.Context, msg eventbus.Message) error {
	var payload usercontract.UserDeleted
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		return fmt.Errorf("decode %s payload: %w", usercontract.EventDeleted, err)
	}
	if payload.UserID == uuid.Nil {
		return nil
	}
	return m.service.DeleteUserRecordings(ctx, payload.UserID)
}

type jobEnqueuerAdapter struct {
	pool     *pgxpool.Pool
	enqueuer platformjob.Enqueuer
}

func (a jobEnqueuerAdapter) EnqueueGradeRecording(ctx context.Context, attemptID uuid.UUID) error {
	return dbx.InTx(ctx, a.pool, func(txCtx context.Context, tx pgx.Tx) error {
		args := speakingjob.GradeRecordingArgs{AttemptID: attemptID}
		_, err := a.enqueuer.EnqueueTx(txCtx, tx, args, nil)
		return err
	})
}
