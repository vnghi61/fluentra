package vocabulary

import (
	"context"
	"encoding/json"
	"fmt"

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

// Guard is the authorization interface required by HTTP handlers.
type Guard = vocabularyhttp.Guard

// Deps defines dependencies supplied by the composition root.
type Deps struct {
	Pool    *pgxpool.Pool
	Clock   clock.Clock
	Guard   Guard
	Content contentcontract.Reader
	Reviews srscontract.CardWriter
}

// Module represents the wired vocabulary module.
type Module struct {
	pool    *pgxpool.Pool
	clock   clock.Clock
	queries *sqlc.Queries
	service *service.Service
	handler *vocabularyhttp.Handler
	grader  *service.Grader
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

	grader := service.NewGrader(deps.Content)

	var handler *vocabularyhttp.Handler
	if deps.Guard != nil {
		var err error
		handler, err = vocabularyhttp.NewHandler(srv, deps.Guard)
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
	}
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
