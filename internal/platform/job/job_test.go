package job_test

import (
	"context"
	"testing"

	"github.com/fluentra/fluentra/internal/platform/job"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestDefaultQueues(t *testing.T) {
	queues := job.DefaultQueues()
	expected := []string{job.QueueDefault, job.QueueAI, job.QueueMedia, job.QueueNotify, job.QueueBatch}

	for _, q := range expected {
		if _, ok := queues[q]; !ok {
			t.Errorf("expected queue %q to be present in DefaultQueues()", q)
		}
	}
}

func TestEnqueuer_NilTx_Fails(t *testing.T) {
	client := job.NewClient(nil)
	_, err := client.EnqueueTx(context.Background(), nil, nil, nil)
	if err == nil {
		t.Error("expected error when transaction is nil")
	}
}

// TestParseQueues_ReadsConcurrencyFromConfiguration is the acceptance for
// "concurrency comes from config": changing WORKER_QUEUES changes what the
// worker does, with no code change and no hardcoded fallback.
func TestParseQueues_ReadsConcurrencyFromConfiguration(t *testing.T) {
	t.Parallel()
	queues, err := job.ParseQueues("default:10,ai:4,media:2,notify:10,batch:2")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	want := map[string]int{
		job.QueueDefault: 10, job.QueueAI: 4, job.QueueMedia: 2, job.QueueNotify: 10, job.QueueBatch: 2,
	}
	if len(queues) != len(want) {
		t.Fatalf("queues = %v, want %d entries", queues, len(want))
	}
	for name, concurrency := range want {
		if queues[name] != concurrency {
			t.Errorf("queue %q concurrency = %d, want %d", name, queues[name], concurrency)
		}
	}

	changed, err := job.ParseQueues("default:1,ai:99")
	if err != nil {
		t.Fatalf("parse changed spec: %v", err)
	}
	if changed[job.QueueDefault] != 1 || changed[job.QueueAI] != 99 || len(changed) != 2 {
		t.Errorf("changed config did not change the result: %v", changed)
	}
}

func TestParseQueues_RejectsMalformedSpecifications(t *testing.T) {
	t.Parallel()
	for name, spec := range map[string]string{
		"empty":            "",
		"only spaces":      "   ",
		"missing colon":    "default",
		"missing name":     ":10",
		"not a number":     "default:many",
		"zero concurrency": "default:0",
		"negative":         "default:-1",
		"duplicate queue":  "default:1,default:2",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if _, err := job.ParseQueues(spec); err == nil {
				t.Errorf("spec %q was accepted", spec)
			}
		})
	}
}

func TestNewWorker_RequiresPoolAndQueues(t *testing.T) {
	t.Parallel()
	if _, err := job.NewWorker(job.WorkerOptions{Queues: map[string]int{"default": 1}}); err == nil {
		t.Error("expected an error when no pool is supplied")
	}
	if _, err := job.NewWorker(job.WorkerOptions{Pool: &pgxpool.Pool{}}); err == nil {
		t.Error("expected an error when no queue is declared")
	}
}
