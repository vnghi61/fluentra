package vocabulary

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/fluentra/fluentra/internal/generated/vocabulary/sqlc"
	contentcontract "github.com/fluentra/fluentra/internal/modules/content/contract"
	srscontract "github.com/fluentra/fluentra/internal/modules/srs/contract"
	"github.com/fluentra/fluentra/internal/modules/vocabulary/contract"
	"github.com/fluentra/fluentra/internal/modules/vocabulary/repository"
	"github.com/fluentra/fluentra/internal/modules/vocabulary/service"
	vocabularyhttp "github.com/fluentra/fluentra/internal/modules/vocabulary/transport/http"
	"github.com/fluentra/fluentra/internal/platform/ai"
	"github.com/fluentra/fluentra/internal/platform/job"
	"github.com/fluentra/fluentra/internal/shared/apperr"
	"github.com/fluentra/fluentra/internal/shared/clock"
	"github.com/fluentra/fluentra/internal/shared/outbox"
)

// reviewScheduler narrows srs.CardWriter to the one method this module uses, and
// keeps a nil dependency nil rather than turning it into a non-nil interface
// holding a nil pointer.
func reviewScheduler(writer srscontract.CardWriter) service.ReviewScheduler {
	if writer == nil {
		return nil
	}
	return writer
}

// Advisory lock id for the generator, derived from this module's migration
// timestamp so it cannot collide with another module's.
const generateExercisesLockID int64 = 1_700_000_231

// generateInterval is twelve-hourly. The dictionary changes when content is
// authored or a learner's upload is verified, which is not an hourly event, and
// a generator that rewrote the catalogue every hour would drop every cached
// lesson for nothing.
const generateInterval = 12 * time.Hour

// The upload verification job. Hourly, because an upload a learner is waiting
// on should feel answered rather than forgotten.
const verifyUploadsLockID int64 = 1_700_000_271
const enrichQueuedLockID int64 = 1_700_000_272

const verifyUploadsInterval = time.Hour

// Guard is the authorization interface required by HTTP handlers.
type Guard = vocabularyhttp.Guard

// Deps defines dependencies supplied by the composition root.
type Deps struct {
	Pool    *pgxpool.Pool
	Clock   clock.Clock
	Guard   Guard
	Content contentcontract.Reader
	Reviews srscontract.CardWriter

	// The practice generator's dependencies. All three are optional and are
	// supplied only by cmd/worker: the API serves no route that generates
	// exercises, and a module built without them simply has no generator.
	ContentAuthor service.ContentAuthor
	LessonAuthor  service.LessonAuthor
	// GeneratorAuthorID owns the generated content. `content_items.owner_id` is
	// not nullable, and unattributed content is content nobody can be asked about.
	// It owns a learner's verified uploads for the same reason.
	GeneratorAuthorID uuid.UUID

	// The upload pipeline's dependencies, both optional.
	//
	// Dictionary is authoritative on whether an uploaded word exists; AI judges
	// the learner's own wording of the meaning and writes example sentences. A
	// module built with neither serves the upload endpoints — a learner can
	// still submit — and simply verifies nothing until a worker with them runs.
	Dictionary repository.DictionaryLookup
	AI         ai.Client
}

// Module represents the wired vocabulary module.
type Module struct {
	pool      *pgxpool.Pool
	clock     clock.Clock
	queries   *sqlc.Queries
	service   *service.Service
	handler   *vocabularyhttp.Handler
	grader    *service.Grader
	generator *service.Generator
	uploads   *service.Uploads
}

// New constructs and wires the vocabulary module.
func New(deps Deps) *Module {
	timekeeper := deps.Clock
	if timekeeper == nil {
		timekeeper = clock.Real{}
	}

	queries := sqlc.New(deps.Pool)
	repo := repository.New(deps.Pool)

	events := outboxWriter{Writer: outbox.NewWriter()}

	srv := service.New(service.Deps{
		Repo:    repo,
		Content: deps.Content,
		Reviews: reviewScheduler(deps.Reviews),
		Events:  events,
		Clock:   timekeeper,
	})

	// The repository doubles as the sense resolver: the grader schedules the
	// word behind an exercise for review, not the exercise, and the word lives
	// in this module's own tables.
	grader := service.NewGrader(deps.Content, repo)

	uploads := service.NewUploads(srv, repo, service.UploadDeps{
		Dictionary: deps.Dictionary,
		AI:         deps.AI,
		Content:    deps.ContentAuthor,
		AuthorID:   deps.GeneratorAuthorID,
		Pool:       deps.Pool,
	})

	var handler *vocabularyhttp.Handler
	if deps.Guard != nil {
		var err error
		handler, err = vocabularyhttp.NewHandler(srv, deps.Guard, uploads)
		if err != nil {
			panic(fmt.Sprintf("failed to construct vocabulary HTTP handler: %v", err))
		}
	}

	return &Module{
		pool:    deps.Pool,
		clock:   timekeeper,
		queries: queries,
		service: srv,
		handler: handler,
		grader:  grader,
		uploads: uploads,
		generator: service.NewGenerator(repo, service.GeneratorDeps{
			Content:  deps.ContentAuthor,
			Lessons:  deps.LessonAuthor,
			AuthorID: deps.GeneratorAuthorID,
		}),
	}
}

// CronJobs returns the scheduled work this module owns.
//
// The generator is idempotent, so losing the advisory lock to another replica
// costs nothing, and a run that overlaps the previous one converges on the same
// catalogue rather than duplicating it.
func (m *Module) CronJobs() []job.CronJob {
	return []job.CronJob{
		{
			Name:     "vocabulary.generate_exercises",
			LockID:   generateExercisesLockID,
			Interval: generateInterval,
			Task:     m.generator.GenerateExercises,
		},
		{
			Name:     "vocabulary.verify_uploads",
			LockID:   verifyUploadsLockID,
			Interval: verifyUploadsInterval,
			Task:     m.uploads.VerifyPending,
		},
		{
			Name:     "vocabulary.enrich_queued",
			LockID:   enrichQueuedLockID,
			Interval: verifyUploadsInterval,
			Task:     m.uploads.EnrichQueued,
		},
	}
}

// EnrichQueued sweeps queued words and verifies them when quota is available.
func (m *Module) EnrichQueued(ctx context.Context) error {
	if m.uploads == nil {
		return nil
	}
	return m.uploads.EnrichQueued(ctx)
}

// GenerateExercises runs the practice generator once. Exported so cmd/worker can
// run it at start-up rather than leaving a fresh database with no practice
// content until the first interval elapses.
func (m *Module) GenerateExercises(ctx context.Context) error {
	return m.generator.GenerateExercises(ctx)
}

// Routes mounts the learner vocabulary endpoints on the router.
func (m *Module) Routes(router chi.Router) {
	if m.handler != nil {
		m.handler.Routes(router)
	}
}

// AdminRoutes mounts the staff-facing vocabulary authoring endpoints on the router.
func (m *Module) AdminRoutes(router chi.Router) {
	if m.handler != nil {
		m.handler.AdminRoutes(router)
	}
}

// Reader returns the public Reader contract implementation.
func (m *Module) Reader() contract.Reader {
	return readerAdapter{service: m.service}
}

// Grader returns the vocabulary exercise grader implementing learningcontract.ExerciseGrader.
func (m *Module) Grader() contract.Grader {
	return m.grader
}

type readerAdapter struct {
	service *service.Service
}

func (r readerAdapter) LookupWord(ctx context.Context, lemma string) (*contract.WordDetail, error) {
	words, err := r.service.LookupWord(ctx, lemma)
	if err != nil {
		return nil, err
	}
	if len(words) == 0 {
		return nil, apperr.New(apperr.NotFound, "WORD_NOT_FOUND", "word not found")
	}

	w := words[0]
	senses := make([]contract.WordSense, 0, len(w.Senses))
	for _, s := range w.Senses {
		exJSON, _ := json.Marshal(s.Examples)
		senses = append(senses, contract.WordSense{
			ID:               s.ID,
			WordID:           s.WordID,
			Definition:       s.Definition,
			Register:         s.Register,
			Domain:           s.Domain,
			Examples:         exJSON,
			ContentVersionID: s.ContentVersionID,
			AudioURL:         w.AudioURL,
		})
	}

	return &contract.WordDetail{
		ID:            w.ID,
		Lemma:         w.Lemma,
		Pos:           string(w.POS),
		CEFRLevel:     string(w.CEFRLevel),
		FrequencyRank: w.FrequencyRank,
		IPA:           w.IPA,
		Senses:        senses,
	}, nil
}

func (r readerAdapter) GetSenses(ctx context.Context, senseIDs []uuid.UUID) ([]contract.WordSense, error) {
	senses, err := r.service.GetSenses(ctx, senseIDs)
	if err != nil {
		return nil, err
	}

	result := make([]contract.WordSense, 0, len(senses))
	for _, s := range senses {
		exJSON, _ := json.Marshal(s.Examples)
		result = append(result, contract.WordSense{
			ID:               s.ID,
			WordID:           s.WordID,
			Definition:       s.Definition,
			Register:         s.Register,
			Domain:           s.Domain,
			Examples:         exJSON,
			ContentVersionID: s.ContentVersionID,
		})
	}
	return result, nil
}

type outboxWriter struct {
	*outbox.Writer
}

func (w outboxWriter) Write(
	ctx context.Context, tx service.OutboxTx, aggregate, event string, payload any,
) (uuid.UUID, error) {
	return w.Writer.Write(ctx, outboxTx{tx}, aggregate, event, payload)
}

type outboxTx struct{ inner service.OutboxTx }

func (t outboxTx) Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error) {
	return t.inner.Exec(ctx, sql, arguments...)
}
