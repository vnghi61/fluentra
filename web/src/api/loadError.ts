import { ApiError } from "./client";

/** How a page's failed load should read: see ServerErrorState. */
export type LoadErrorKind = "server" | "offline" | "request";

export interface LoadErrorInfo {
  kind: LoadErrorKind;
  /** The API's request_id, when the server answered. */
  requestId?: string | undefined;
}

/**
 * Sorts a failed load into what the learner can do about it.
 *
 * Only the browser saying it is offline is `offline`: that screen retries on
 * the `online` event, which never comes while the device has a connection. A
 * request that got no answer while online (fetch's "Failed to fetch": the host
 * is down or restarting) is `server`, with the rest: an API answer of 5xx or
 * 429 (busy, restarting or rate limited) and anything thrown that is not an
 * API answer. Those retry on a timer. Any other API refusal is `request`.
 */
export function describeLoadError(error: unknown): LoadErrorInfo {
  if (typeof navigator !== "undefined" && navigator.onLine === false) {
    return { kind: "offline" };
  }

  if (error instanceof ApiError) {
    const { status, request_id: requestId } = error.problem;
    const kind = status >= 500 || status === 429 ? "server" : "request";
    return { kind, requestId };
  }
  return { kind: "server" };
}
