package telemetry

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// Instruments contains the bounded-label metrics shared by platform modules.
type Instruments struct {
	HTTPDuration             metric.Float64Histogram
	HTTPActive               metric.Int64UpDownCounter
	DBQueryDuration          metric.Float64Histogram
	DBPoolConnections        metric.Int64UpDownCounter
	CacheOperationDuration   metric.Float64Histogram
	CacheRequests            metric.Int64Counter
	StorageOperationDuration metric.Float64Histogram
	StorageBytes             metric.Int64Counter
	JobDuration              metric.Float64Histogram
	JobQueueDepth            metric.Int64UpDownCounter
	JobOldestPending         metric.Int64ObservableGauge
	JobAttempts              metric.Int64Counter
}

// NewInstruments creates the standard metric instruments from meter.
func NewInstruments(meter metric.Meter) (Instruments, error) {
	httpDuration, err := meter.Float64Histogram("http_server_request_duration_seconds", metric.WithUnit("s"))
	if err != nil {
		return Instruments{}, fmt.Errorf("create HTTP duration histogram: %w", err)
	}
	httpActive, err := meter.Int64UpDownCounter("http_server_active_requests")
	if err != nil {
		return Instruments{}, fmt.Errorf("create HTTP active counter: %w", err)
	}
	dbQueryDuration, err := meter.Float64Histogram("db_query_duration_seconds", metric.WithUnit("s"))
	if err != nil {
		return Instruments{}, fmt.Errorf("create database duration histogram: %w", err)
	}
	dbPoolConnections, err := meter.Int64UpDownCounter("db_pool_connections")
	if err != nil {
		return Instruments{}, fmt.Errorf("create database pool counter: %w", err)
	}
	cacheOperationDuration, err := meter.Float64Histogram("cache_operation_duration_seconds", metric.WithUnit("s"))
	if err != nil {
		return Instruments{}, fmt.Errorf("create cache duration histogram: %w", err)
	}
	cacheRequests, err := meter.Int64Counter("cache_requests_total")
	if err != nil {
		return Instruments{}, fmt.Errorf("create cache requests counter: %w", err)
	}
	storageOperationDuration, err := meter.Float64Histogram("storage_operation_duration_seconds", metric.WithUnit("s"))
	if err != nil {
		return Instruments{}, fmt.Errorf("create storage duration histogram: %w", err)
	}
	storageBytes, err := meter.Int64Counter("storage_bytes_total", metric.WithUnit("By"))
	if err != nil {
		return Instruments{}, fmt.Errorf("create storage bytes counter: %w", err)
	}
	jobDuration, err := meter.Float64Histogram("job_duration_seconds", metric.WithUnit("s"))
	if err != nil {
		return Instruments{}, fmt.Errorf("create job duration histogram: %w", err)
	}
	jobQueueDepth, err := meter.Int64UpDownCounter("job_queue_depth")
	if err != nil {
		return Instruments{}, fmt.Errorf("create job queue depth counter: %w", err)
	}
	jobOldestPending, err := meter.Int64ObservableGauge("job_oldest_pending_seconds", metric.WithUnit("s"))
	if err != nil {
		return Instruments{}, fmt.Errorf("create oldest job gauge: %w", err)
	}
	jobAttempts, err := meter.Int64Counter("job_attempts_total")
	if err != nil {
		return Instruments{}, fmt.Errorf("create job attempts counter: %w", err)
	}
	return Instruments{
		HTTPDuration:             httpDuration,
		HTTPActive:               httpActive,
		DBQueryDuration:          dbQueryDuration,
		DBPoolConnections:        dbPoolConnections,
		CacheOperationDuration:   cacheOperationDuration,
		CacheRequests:            cacheRequests,
		StorageOperationDuration: storageOperationDuration,
		StorageBytes:             storageBytes,
		JobDuration:              jobDuration,
		JobQueueDepth:            jobQueueDepth,
		JobOldestPending:         jobOldestPending,
		JobAttempts:              jobAttempts,
	}, nil
}

// RecordCacheRequest emits an outcome with bounded module and result labels.
func (i Instruments) RecordCacheRequest(ctx context.Context, module, result string) {
	i.CacheRequests.Add(ctx, 1, metric.WithAttributes(
		attribute.String("module", module), attribute.String("result", result)))
}
