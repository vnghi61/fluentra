/**
 * Waking a host that sleeps.
 *
 * The API runs on Render's free tier, which stops a service after a stretch of
 * inactivity and starts it again on the next request. That first request does
 * not fail fast — it hangs for the length of a cold boot, and while the process
 * is coming up the platform's proxy answers 502 or 503. Both look like an
 * outage to a client written for a server that is always there, and the first
 * visitor of the morning got an error page.
 *
 * Two things fix it, and they are different things:
 *
 *   - `wakeUp()` is a nudge. The app fires it on load so the boot starts while
 *     the visitor is still reading the page, rather than when they first ask
 *     for data.
 *   - `isColdStart()` classifies a failure so `apiFetch` can retry instead of
 *     surfacing it. A sleeping host is not a broken one.
 *
 * The status is observable so the UI can say "starting the server" rather than
 * showing a spinner that means nothing. Nothing here retries forever: past the
 * budget it is an outage, and saying so is more useful than spinning.
 */

/** How long to keep trying before calling it an outage. */
const WAKE_BUDGET_MS = 75_000;

/** Delay between attempts, growing, capped. Render cold boots run 30-60s. */
const FIRST_DELAY_MS = 1_000;
const MAX_DELAY_MS = 5_000;

/** How long a request may run before we admit out loud that we are waiting. */
export const SLOW_REQUEST_MS = 3_000;

export type WakeStatus = "unknown" | "waking" | "awake" | "unreachable";

let status: WakeStatus = "unknown";
let inFlight: Promise<boolean> | null = null;
const listeners = new Set<(status: WakeStatus) => void>();

function setStatus(next: WakeStatus): void {
  if (status === next) return;
  status = next;
  for (const listener of listeners) listener(status);
}

export function wakeStatus(): WakeStatus {
  return status;
}

/** Subscribe to status changes. Returns its own unsubscribe. */
export function onWakeStatus(
  listener: (status: WakeStatus) => void,
): () => void {
  listeners.add(listener);
  return () => listeners.delete(listener);
}

/**
 * A response or error that means "the host is asleep or still booting", not
 * "the request was wrong".
 *
 * 502/503/504 come from the platform proxy while the process starts. A thrown
 * TypeError is what `fetch` gives for a connection that never opened — which is
 * also what a suspended service looks like from the browser. A 4xx is never a
 * cold start: the server answered, and it had an opinion.
 */
export function isColdStartStatus(status: number): boolean {
  return status === 502 || status === 503 || status === 504;
}

export function isNetworkError(error: unknown): boolean {
  return error instanceof TypeError;
}

const sleep = (ms: number): Promise<void> =>
  new Promise((resolve) => setTimeout(resolve, ms));

/**
 * Pings until the API answers, or until the budget runs out.
 *
 * Single-flight: a page that fires five queries at a cold host must start one
 * boot, not five. Once awake it resolves immediately — the check is a cached
 * fact for the rest of the session, because a host that answered is not going
 * back to sleep while the tab is open and asking.
 */
export async function wakeUp(): Promise<boolean> {
  if (status === "awake") return true;
  if (inFlight) return inFlight;

  inFlight = (async () => {
    setStatus("waking");
    const deadline = Date.now() + WAKE_BUDGET_MS;
    let delay = FIRST_DELAY_MS;

    while (Date.now() < deadline) {
      try {
        const response = await fetch("/api/v1/ping", {
          method: "GET",
          headers: { Accept: "application/json" },
          // The ping's job is to reach the origin, not to be served from a
          // cache that would report a sleeping host as awake.
          cache: "no-store",
        });
        if (response.ok) {
          setStatus("awake");
          return true;
        }
        if (!isColdStartStatus(response.status)) {
          // The server answered with an opinion. It is up; whatever is wrong
          // is not something more pinging fixes.
          setStatus("awake");
          return true;
        }
      } catch (error) {
        if (!isNetworkError(error)) {
          setStatus("awake");
          return true;
        }
      }
      await sleep(delay);
      delay = Math.min(delay * 2, MAX_DELAY_MS);
    }

    setStatus("unreachable");
    return false;
  })().finally(() => {
    inFlight = null;
  });

  return inFlight;
}

/** Forgets that the host was awake. For tests, and for a failed retry. */
export function resetWakeState(): void {
  status = "unknown";
  inFlight = null;
}
