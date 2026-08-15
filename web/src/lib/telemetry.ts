import {
  context,
  propagation,
  trace,
  type Span,
  type Tracer,
} from "@opentelemetry/api";

/**
 * Browser tracing.
 *
 * The SDK is loaded lazily and the whole module is optional: nothing here is on
 * the critical path to first paint, and a browser with no collector reachable
 * must still render the application.
 *
 * The reason this exists at all is that `api/client.ts` used to build its own
 * `traceparent` header from `crypto.randomUUID()`. That produced a syntactically
 * valid header naming a trace and a parent span that had never existed, so the
 * server started a child of nothing: every request became an orphan root, and
 * the result *looked* exactly like working distributed tracing. Only a real SDK
 * can emit a header the server can join, because only a real SDK has actually
 * started the span the header refers to.
 */

const TRACER_NAME = "fluentra-web";

let initialised = false;
let tracerRef: Tracer | null = null;

export interface TelemetryConfig {
  /** OTLP/HTTP traces endpoint, e.g. http://localhost:4318/v1/traces. */
  endpoint: string;
  serviceName: string;
  environment: string;
}

/**
 * Starts the browser tracer. Safe to call more than once; the second call is a
 * no-op. Import cost is deferred until this runs, so the SDK is not in the
 * initial chunk.
 */
export async function startTelemetry(config: TelemetryConfig): Promise<void> {
  if (initialised) return;
  initialised = true;

  try {
    const [
      { WebTracerProvider, BatchSpanProcessor },
      { OTLPTraceExporter },
      { ZoneContextManager },
      { registerInstrumentations },
      { FetchInstrumentation },
      { resourceFromAttributes },
      semconv,
    ] = await Promise.all([
      import("@opentelemetry/sdk-trace-web"),
      import("@opentelemetry/exporter-trace-otlp-http"),
      import("@opentelemetry/context-zone"),
      import("@opentelemetry/instrumentation"),
      import("@opentelemetry/instrumentation-fetch"),
      import("@opentelemetry/resources"),
      import("@opentelemetry/semantic-conventions"),
    ]);

    const provider = new WebTracerProvider({
      resource: resourceFromAttributes({
        [semconv.ATTR_SERVICE_NAME]: config.serviceName,
        "deployment.environment.name": config.environment,
      }),
      spanProcessors: [
        new BatchSpanProcessor(new OTLPTraceExporter({ url: config.endpoint })),
      ],
    });

    provider.register({ contextManager: new ZoneContextManager() });

    registerInstrumentations({
      instrumentations: [
        new FetchInstrumentation({
          // Without this the SDK will not attach traceparent to same-origin
          // XHR/fetch, which is every call this application makes.
          propagateTraceHeaderCorsUrls: [/.*/],
          clearTimingResources: true,
        }),
      ],
    });

    tracerRef = trace.getTracer(TRACER_NAME);
  } catch {
    // A missing or unreachable collector must not break the application. The
    // no-op API returned by @opentelemetry/api takes over from here.
    tracerRef = null;
  }
}

/** The tracer, or the API's no-op tracer when telemetry never started. */
export function tracer(): Tracer {
  return tracerRef ?? trace.getTracer(TRACER_NAME);
}

/**
 * Writes the current trace context into headers.
 *
 * When a span is active this yields a `traceparent` the server can join. When
 * none is, it writes nothing — which is the honest outcome, and unlike the old
 * hand-rolled header it never invents a parent.
 */
export function injectTraceContext(
  headers: Record<string, string>,
): Record<string, string> {
  propagation.inject(context.active(), headers);
  return headers;
}

/** Runs `fn` inside a span named `name`, ending the span whatever happens. */
export async function withSpan<T>(
  name: string,
  fn: (span: Span) => Promise<T>,
): Promise<T> {
  const span = tracer().startSpan(name);
  try {
    return await context.with(trace.setSpan(context.active(), span), () =>
      fn(span),
    );
  } finally {
    span.end();
  }
}
