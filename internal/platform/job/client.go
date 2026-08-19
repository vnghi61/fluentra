package job

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"github.com/riverqueue/river/rivertype"
)

// The five River queues. Work is separated by latency profile so a slow bulk job
// can never starve an interactive one; DefaultQueues gives each its concurrency.
const (
	QueueDefault = "default"
	QueueAI      = "ai"
	QueueMedia   = "media"
	QueueNotify  = "notify"
	QueueBatch   = "batch"
)

// DefaultQueues returns the configured 5 queue names and default concurrency levels.
func DefaultQueues() map[string]int {
	return map[string]int{
		QueueDefault: 10,
		QueueAI:      5,
		QueueMedia:   3,
		QueueNotify:  10,
		QueueBatch:   2,
	}
}

// Enqueuer requires taking the database transaction as a mandatory parameter (BR-JOB-01).
type Enqueuer interface {
	EnqueueTx(
		ctx context.Context, tx pgx.Tx, args river.JobArgs, opts *river.InsertOpts,
	) (*rivertype.JobInsertResult, error)
}

// Client wraps River queue enqueueing operations.
type Client struct {
	riverClient *river.Client[pgx.Tx]
}

// NewClient creates a job enqueuer client facade.
func NewClient(riverClient *river.Client[pgx.Tx]) *Client {
	return &Client{riverClient: riverClient}
}

// EnqueueTx enqueues a job inside the caller's transaction.
func (c *Client) EnqueueTx(
	ctx context.Context, tx pgx.Tx, args river.JobArgs, opts *river.InsertOpts,
) (*rivertype.JobInsertResult, error) {
	if tx == nil {
		return nil, errors.New("job enqueueing requires an active transaction")
	}
	if c.riverClient == nil {
		return nil, errors.New("river client is not initialized")
	}
	res, err := c.riverClient.InsertTx(ctx, tx, args, opts)
	if err != nil {
		return nil, fmt.Errorf("insert river job: %w", err)
	}
	return res, nil
}

// NewClientFromPool creates a job client using the connection pool.
//
// Schema is not optional. River's tables live in `ops` — MigrateUp puts them
// there and the worker's own config says so — and a client left with the zero
// Config looks for `river_job` on the default search_path instead. It compiles,
// it starts, and every insert fails at runtime: the API answered 500 on
// `POST /me/export` for exactly this reason, with no row written and no job
// queued. An enqueue-only client that cannot see the queue is the same class of
// defect as a query with no caller, and it is why this line is asserted in
// TestNewClientFromPool_InsertsIntoTheSchemaTheMigratorCreated.
func NewClientFromPool(pool *pgxpool.Pool) (*Client, error) {
	riverClient, err := river.NewClient(riverpgxv5.New(pool), &river.Config{Schema: Schema})
	if err != nil {
		return nil, fmt.Errorf("create river client: %w", err)
	}
	return NewClient(riverClient), nil
}
