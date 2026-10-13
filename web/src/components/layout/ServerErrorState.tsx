import * as React from "react";
import { Link } from "@tanstack/react-router";
import { Loader2, RotateCcw } from "lucide-react";
import { useTranslation } from "react-i18next";

import { cn } from "@/lib/utils";

import { OrbitBrandMark } from "./OrbitBrandMark";
import { systemStateActionClass } from "./SystemStatePage";

/**
 * What kind of failure a page met, for the copy and the recovery:
 *
 * - `server`: the API failed (5xx, a rate limit, a host restarting). It says
 *   so plainly and tries again by itself every `autoRetrySeconds`.
 * - `offline`: the request never left the device. It waits for the browser to
 *   come back online and then tries again.
 * - `request`: the API refused (a 4xx other than a rate limit). Telling the
 *   learner the server is busy would be untrue, and retrying on a timer would
 *   be pointless, so it only offers the buttons.
 */
export type ServerErrorKind = "server" | "offline" | "request";

export interface ServerErrorStateProps {
  kind: ServerErrorKind;
  onRetry: () => unknown;
  /** The API's request_id, which support can look up. Never the raw error. */
  requestId?: string | undefined;
  autoRetrySeconds?: number;
  className?: string | undefined;
}

const DEFAULT_AUTO_RETRY_SECONDS = 10;

/**
 * A page that could not load its data, drawn like the "Starting the server…"
 * screen it follows on a cold start: the same orbiting mark, stopped, with an
 * amber body for "something is wrong". It replaces error cards that printed
 * the API's own words ("Authentication required", "Internal Server Error").
 */
export function ServerErrorState({
  kind,
  onRetry,
  requestId,
  autoRetrySeconds = DEFAULT_AUTO_RETRY_SECONDS,
  className,
}: ServerErrorStateProps): React.JSX.Element {
  const { t } = useTranslation();
  const [busy, setBusy] = React.useState(false);
  const [left, setLeft] = React.useState(autoRetrySeconds);
  const mounted = React.useRef(true);
  React.useEffect(
    () => () => {
      mounted.current = false;
    },
    [],
  );

  const retry = React.useCallback(async () => {
    setBusy(true);
    try {
      await onRetry();
    } finally {
      // Still here means the retry failed too: count down again.
      if (mounted.current) {
        setBusy(false);
        setLeft(autoRetrySeconds);
      }
    }
  }, [onRetry, autoRetrySeconds]);

  // A busy server is retried on a timer the learner can see.
  React.useEffect(() => {
    if (kind !== "server" || busy) return;
    const timer = window.setTimeout(() => {
      if (left <= 1) void retry();
      else setLeft(left - 1);
    }, 1000);
    return () => window.clearTimeout(timer);
  }, [kind, busy, left, retry]);

  // An offline device is retried the moment it reconnects.
  React.useEffect(() => {
    if (kind !== "offline") return;
    const online = () => void retry();
    window.addEventListener("online", online);
    return () => window.removeEventListener("online", online);
  }, [kind, retry]);

  const title =
    kind === "offline"
      ? t("errorState.offlineTitle", "You're offline")
      : kind === "request"
        ? t("errorState.requestTitle", "This page didn't load")
        : t("errorState.serverTitle", "The server is a little busy");
  const description =
    kind === "offline"
      ? t(
          "errorState.offlineDescription",
          "Check your Wi-Fi or mobile data. The page reloads by itself once you're back online.",
        )
      : kind === "request"
        ? t(
            "errorState.requestDescription",
            "Something went wrong loading this page. Try again, or carry on learning.",
          )
        : t(
            "errorState.serverDescription",
            "We couldn't load this page's data yet. Your learning progress is safe.",
          );

  return (
    <div
      className={cn(
        "flex min-h-[60vh] w-full flex-1 flex-col items-center justify-center overflow-x-hidden px-4 py-10 text-center",
        className,
      )}
    >
      <OrbitBrandMark size="sm" alert />
      {/* Announced once; the countdown below is outside it so a screen reader
          does not read it every second. */}
      <div role="alert" className="flex flex-col items-center">
        <h1 className="mt-5 text-2xl font-bold leading-tight tracking-tight text-text md:text-3xl">
          {title}
        </h1>
        <p className="mt-3 max-w-md text-base leading-relaxed text-text-muted">
          {description}
        </p>
      </div>

      <div className="mt-7 flex w-full flex-col gap-3 sm:w-auto sm:flex-row">
        <button
          type="button"
          onClick={() => void retry()}
          disabled={busy}
          className={`${systemStateActionClass} gap-2 bg-primary text-primary-fg hover:bg-primary-hover disabled:cursor-progress disabled:opacity-75`}
        >
          {busy ? (
            <Loader2
              className="h-4 w-4 motion-safe:animate-spin"
              aria-hidden="true"
            />
          ) : (
            <RotateCcw className="h-4 w-4" aria-hidden="true" />
          )}
          {busy
            ? t("errorState.retrying", "Trying again…")
            : t("errorState.retryNow", "Try again now")}
        </button>
        <Link
          to="/learn"
          className={`${systemStateActionClass} border border-border bg-transparent text-text hover:bg-surface-muted`}
        >
          {t("errorState.goToLearn", "Go to Learn")}
        </Link>
      </div>

      {kind !== "request" && !busy && (
        <p className="mt-4 inline-flex items-center gap-2 text-sm text-text-muted">
          <span
            className="h-2 w-2 shrink-0 rounded-full bg-warning"
            aria-hidden="true"
          />
          {kind === "offline"
            ? t("errorState.waitingForNetwork", "Waiting for a connection…")
            : t("errorState.autoRetry", {
                count: left,
                defaultValue: "Trying again in {{count}}s",
              })}
        </p>
      )}

      {kind !== "offline" && requestId && (
        <p className="mt-5 text-xs text-text-muted">
          {t("errorState.reference", "Reference")}{" "}
          <code className="select-all font-mono">{requestId}</code>
        </p>
      )}
    </div>
  );
}
