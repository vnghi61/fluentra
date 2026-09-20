// Package job holds the payment module's background work: sweeping orders
// that expired unpaid, and the daily reconciliation against SePay.
package job

import (
	"context"
	"time"

	"github.com/fluentra/fluentra/internal/modules/payment/service"
	"github.com/fluentra/fluentra/internal/platform/job"
)

const (
	// ExpirySweepLockID is the advisory lock ID for sweeping expired unpaid orders (Work Order 15 §10.5).
	ExpirySweepLockID int64 = 1_700_000_751

	// ReconciliationLockID is the advisory lock ID for daily SePay bank transaction reconciliation (Work Order 15 §10.6).
	ReconciliationLockID int64 = 1_700_000_752

	expirySweepInterval    = 1 * time.Hour
	reconciliationInterval = 24 * time.Hour
)

// ExpirySweepWorker sweeps expired unpaid orders.
type ExpirySweepWorker struct {
	svc service.Service
}

// NewExpirySweepWorker creates an expiry sweep worker.
func NewExpirySweepWorker(svc service.Service) *ExpirySweepWorker {
	return &ExpirySweepWorker{svc: svc}
}

// CronJob returns the scheduled hourly sweep task.
func (w *ExpirySweepWorker) CronJob() job.CronJob {
	return job.CronJob{
		Name:     "payment.expiry_sweep",
		LockID:   ExpirySweepLockID,
		Interval: expirySweepInterval,
		Task:     w.Sweep,
	}
}

// Sweep invokes order expiration sweep on the payment service.
func (w *ExpirySweepWorker) Sweep(ctx context.Context) error {
	_, err := w.svc.SweepExpiredOrders(ctx)
	return err
}

// ReconciliationWorker runs daily SePay transaction reconciliation.
type ReconciliationWorker struct {
	svc service.Service
}

// NewReconciliationWorker creates a reconciliation worker.
func NewReconciliationWorker(svc service.Service) *ReconciliationWorker {
	return &ReconciliationWorker{svc: svc}
}

// CronJob returns the scheduled daily reconciliation task.
func (w *ReconciliationWorker) CronJob() job.CronJob {
	return job.CronJob{
		Name:     "payment.reconciliation",
		LockID:   ReconciliationLockID,
		Interval: reconciliationInterval,
		Task:     w.Reconcile,
	}
}

// Reconcile invokes bank reconciliation on the payment service.
func (w *ReconciliationWorker) Reconcile(ctx context.Context) error {
	return w.svc.Reconcile(ctx)
}
