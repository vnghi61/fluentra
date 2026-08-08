package telemetry

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/log/global"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/log"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.39.0"
)

// Config identifies this process in every exported telemetry signal.
type Config struct {
	ServiceName string
	Version     string
	Environment string
	CommitSHA   string
	Endpoint    string
}

// Provider owns global OpenTelemetry providers and their shutdown lifecycle.
type Provider struct {
	tracerProvider *trace.TracerProvider
	meterProvider  *metric.MeterProvider
	loggerProvider *log.LoggerProvider
	instruments    Instruments
}

var defaultInstruments struct {
	sync.RWMutex
	value *Instruments
}

// NewProvider configures OTLP tracing, metrics, and logs with W3C propagation.
func NewProvider(ctx context.Context, config Config) (*Provider, error) {
	resources, err := resource.New(ctx, resource.WithAttributes(
		semconv.ServiceName(config.ServiceName),
		semconv.ServiceVersion(config.Version),
		semconv.DeploymentEnvironmentName(config.Environment),
		attribute.String("git.commit.sha", config.CommitSHA),
	))
	if err != nil {
		return nil, fmt.Errorf("create telemetry resource: %w", err)
	}

	traceExporter, err := otlptracegrpc.New(ctx, otlptracegrpc.WithEndpoint(config.Endpoint), otlptracegrpc.WithInsecure())
	if err != nil {
		return nil, fmt.Errorf("create OTLP trace exporter: %w", err)
	}
	metricExporter, err := otlpmetricgrpc.New(ctx, otlpmetricgrpc.WithEndpoint(config.Endpoint), otlpmetricgrpc.WithInsecure())
	if err != nil {
		return nil, fmt.Errorf("create OTLP metric exporter: %w", err)
	}
	logExporter, err := otlploggrpc.New(ctx, otlploggrpc.WithEndpoint(config.Endpoint), otlploggrpc.WithInsecure())
	if err != nil {
		return nil, fmt.Errorf("create OTLP log exporter: %w", err)
	}

	tracerProvider := trace.NewTracerProvider(trace.WithBatcher(traceExporter), trace.WithResource(resources))
	meterProvider := metric.NewMeterProvider(metric.WithReader(metric.NewPeriodicReader(metricExporter)), metric.WithResource(resources))
	loggerProvider := log.NewLoggerProvider(log.WithProcessor(log.NewBatchProcessor(logExporter)), log.WithResource(resources))
	instruments, err := NewInstruments(meterProvider.Meter("fluentra"))
	if err != nil {
		shutdownCtx, cancel := context.WithCancel(context.Background())
		cancel()
		return nil, errors.Join(fmt.Errorf("create telemetry instruments: %w", err), tracerProvider.Shutdown(shutdownCtx), meterProvider.Shutdown(shutdownCtx), loggerProvider.Shutdown(shutdownCtx))
	}

	provider := &Provider{
		tracerProvider: tracerProvider,
		meterProvider:  meterProvider,
		loggerProvider: loggerProvider,
		instruments:    instruments,
	}
	otel.SetTracerProvider(tracerProvider)
	otel.SetMeterProvider(meterProvider)
	global.SetLoggerProvider(loggerProvider)
	otel.SetTextMapPropagator(propagation.TraceContext{})
	setDefaultInstruments(instruments)
	slog.SetDefault(provider.Logger("fluentra"))
	return provider, nil
}

// Instruments returns this provider's shared bounded-label metric instruments.
func (p *Provider) Instruments() Instruments { return p.instruments }

// Shutdown flushes pending telemetry before the process exits.
func (p *Provider) Shutdown(ctx context.Context) error {
	var errs []error
	if p.tracerProvider != nil {
		errs = append(errs, p.tracerProvider.Shutdown(ctx))
	}
	if p.meterProvider != nil {
		errs = append(errs, p.meterProvider.Shutdown(ctx))
	}
	if p.loggerProvider != nil {
		errs = append(errs, p.loggerProvider.Shutdown(ctx))
	}
	return errors.Join(errs...)
}

func setDefaultInstruments(instruments Instruments) {
	defaultInstruments.Lock()
	defer defaultInstruments.Unlock()
	defaultInstruments.value = &instruments
}

func currentInstruments() (Instruments, bool) {
	defaultInstruments.RLock()
	defer defaultInstruments.RUnlock()
	if defaultInstruments.value == nil {
		return Instruments{}, false
	}
	return *defaultInstruments.value, true
}
