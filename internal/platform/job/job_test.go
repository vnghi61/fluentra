package job_test

import (
	"context"
	"testing"

	"github.com/fluentra/fluentra/internal/platform/job"
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
