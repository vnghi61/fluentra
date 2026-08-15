import { injectTraceContext } from "@/lib/telemetry";

/**
 * Fetch wrapper. It adds a request id and lets the OpenTelemetry propagator add
 * the trace context.
 *
 * It deliberately does **not** construct `traceparent` itself. The previous
 * version generated a random trace id per request, which produced a well-formed
 * header pointing at a span that had never existed — the server dutifully
 * started a child of nothing and every request became its own orphan trace. It
 * looked like distributed tracing on a dashboard, which is exactly why the bug
 * survived. Only the SDK can emit a header that joins, because only the SDK has
 * started the span it names.
 */

export interface RequestOptions extends Omit<RequestInit, "headers"> {
  headers?: Record<string, string>;
}

/** RFC 9457 Problem Details, the only error shape this API returns. */
export interface ProblemDetails {
  type?: string;
  title: string;
  status: number;
  detail?: string;
  instance?: string;
  code?: string;
}

export class ApiError extends Error {
  readonly problem: ProblemDetails;

  constructor(problem: ProblemDetails) {
    super(problem.title);
    this.name = "ApiError";
    this.problem = problem;
  }
}

export function generateRequestID(): string {
  return crypto.randomUUID();
}

function isProblemDetails(value: unknown): value is ProblemDetails {
  return (
    typeof value === "object" &&
    value !== null &&
    typeof (value as { title?: unknown }).title === "string" &&
    typeof (value as { status?: unknown }).status === "number"
  );
}

export async function apiFetch<T>(
  endpoint: string,
  options: RequestOptions = {},
): Promise<T> {
  const headers: Record<string, string> = {
    "Content-Type": "application/json",
    "X-Request-Id": generateRequestID(),
    ...options.headers,
  };

  const response = await fetch(endpoint, {
    ...options,
    headers: injectTraceContext(headers),
  });

  if (!response.ok) {
    let body: unknown;
    try {
      body = await response.json();
    } catch {
      body = undefined;
    }
    throw new ApiError(
      isProblemDetails(body)
        ? body
        : {
            title: response.statusText || "Request failed",
            status: response.status,
          },
    );
  }

  if (response.status === 204) {
    return undefined as T;
  }
  return (await response.json()) as T;
}
