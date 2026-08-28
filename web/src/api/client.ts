import { type ProblemDetails } from "@/lib/errors/catalogue";
import { injectTraceContext } from "@/lib/telemetry";
import { isColdStartStatus, isNetworkError, wakeUp } from "./wake";

export type { ProblemDetails };

/**
 * Fetch wrapper. It adds a request id and lets the OpenTelemetry propagator add
 * the trace context.
 */

export interface RequestOptions extends Omit<RequestInit, "headers"> {
  headers?: Record<string, string> | undefined;
  /** Internal flag to avoid infinite retry loops on 401 */
  _isRetry?: boolean | undefined;
  /**
   * Internal flag: this call has already waited out one cold start. Separate
   * from `_isRetry` because the two retries are for different things and a
   * request can legitimately need both — a token refresh after the host woke.
   */
  _wakeRetried?: boolean | undefined;
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

/** Token and refresh hooks configured by the session store */
type TokenGetter = () => string | null;
type RefreshHandler = () => Promise<string | null>;

let tokenGetter: TokenGetter = () => null;
let refreshHandler: RefreshHandler | null = null;
let inFlightRefresh: Promise<string | null> | null = null;

export function configureAuthInterceptor(options: {
  getToken: TokenGetter;
  onRefresh: RefreshHandler;
}): void {
  tokenGetter = options.getToken;
  refreshHandler = options.onRefresh;
}

/** Single-flight refresh: ten concurrent 401s trigger exactly one refresh */
export async function executeSingleFlightRefresh(): Promise<string | null> {
  if (inFlightRefresh) {
    return inFlightRefresh;
  }
  if (!refreshHandler) {
    return null;
  }
  inFlightRefresh = refreshHandler().finally(() => {
    inFlightRefresh = null;
  });
  return inFlightRefresh;
}

function isAuthBypassEndpoint(endpoint: string): boolean {
  return (
    endpoint.includes("/auth/refresh") ||
    endpoint.includes("/auth/login") ||
    endpoint.includes("/auth/register") ||
    endpoint.includes("/auth/forgot-password") ||
    endpoint.includes("/auth/reset-password")
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

  const currentToken = tokenGetter();
  if (currentToken && !headers["Authorization"]) {
    headers["Authorization"] = `Bearer ${currentToken}`;
  }

  let response: Response;
  try {
    response = await fetch(endpoint, {
      ...options,
      headers: injectTraceContext(headers),
    });
  } catch (error) {
    // A connection that never opened is what a suspended host looks like from
    // the browser. Wake it and try once more before calling it a failure.
    if (isNetworkError(error) && !options._wakeRetried) {
      await wakeUp();
      return apiFetch<T>(endpoint, { ...options, _wakeRetried: true });
    }
    throw error;
  }

  // 502/503/504 while the platform starts the process. Same treatment: this is
  // a host that is not ready yet, not a request that was wrong.
  if (isColdStartStatus(response.status) && !options._wakeRetried) {
    await wakeUp();
    return apiFetch<T>(endpoint, { ...options, _wakeRetried: true });
  }

  if (!response.ok) {
    let body: unknown;
    try {
      body = await response.json();
    } catch {
      body = undefined;
    }

    const problem: ProblemDetails = isProblemDetails(body)
      ? body
      : {
          title: response.statusText || "Request failed",
          status: response.status,
        };

    // Single-flight 401 refresh interceptor
    if (
      response.status === 401 &&
      !options._isRetry &&
      !isAuthBypassEndpoint(endpoint) &&
      refreshHandler
    ) {
      const newToken = await executeSingleFlightRefresh();
      if (newToken) {
        return apiFetch<T>(endpoint, {
          ...options,
          _isRetry: true,
          _wakeRetried: options._wakeRetried,
          headers: {
            ...options.headers,
            Authorization: `Bearer ${newToken}`,
          },
        });
      }
    }

    throw new ApiError(problem);
  }

  if (response.status === 204) {
    return undefined as T;
  }
  return (await response.json()) as T;
}
