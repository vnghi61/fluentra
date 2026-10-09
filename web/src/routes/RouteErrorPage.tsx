import * as React from "react";
import { Link, useRouter } from "@tanstack/react-router";
import { RefreshCw, WifiOff } from "lucide-react";
import { useTranslation } from "react-i18next";

import { ApiError } from "@/api/client";
import {
  SystemStatePage,
  systemStateActionClass,
} from "@/components/layout/SystemStatePage";
import { Button } from "@/components/ui/button";
import { tracer } from "@/lib/telemetry";

export interface RouteErrorPageProps {
  error: Error;
  reset: () => void;
  info?: unknown;
}

export type RouteErrorKind = "chunk" | "offline" | "generic";

const CHUNK_ERROR_PATTERN =
  /Failed to fetch dynamically imported module|Importing a module script failed|error loading dynamically imported module|ChunkLoadError|Loading chunk [\w-]+ failed/i;

const NETWORK_ERROR_PATTERN = /fetch|network/i;

export function classifyError(error: unknown): RouteErrorKind {
  const message = error instanceof Error ? error.message : String(error);

  // Chunk loading errors checked first
  if (CHUNK_ERROR_PATTERN.test(message)) {
    return "chunk";
  }

  // Offline / network failures checked next
  const isNavigatorOffline =
    typeof navigator !== "undefined" && navigator.onLine === false;
  const isNetworkTypeError =
    error instanceof TypeError && NETWORK_ERROR_PATTERN.test(message);

  if (isNavigatorOffline || isNetworkTypeError) {
    return "offline";
  }

  return "generic";
}

export function RouteErrorPage({
  error,
  reset,
}: RouteErrorPageProps): React.JSX.Element {
  const { t } = useTranslation();
  const router = useRouter();

  React.useEffect(() => {
    const span = tracer().startSpan("ui.route_error");
    if (error instanceof Error) {
      span.recordException(error);
    } else {
      span.setAttribute("error.message", String(error));
    }
    span.end();
  }, [error]);

  const kind = classifyError(error);

  const handleRetry = () => {
    reset();
    void router.invalidate();
  };

  const handleReload = () => {
    if (typeof window !== "undefined") {
      window.location.reload();
    }
  };

  const problem = error instanceof ApiError ? error.problem : undefined;
  const requestId = problem
    ? ((problem as unknown as Record<string, unknown>).request_id as
        string | undefined)
    : undefined;

  return (
    <div
      role="alert"
      className="flex min-h-[60vh] w-full flex-col items-center justify-center overflow-x-hidden px-4 py-12 text-center"
    >
      {kind === "chunk" && (
        <div className="flex max-w-md flex-col items-center">
          <div className="flex h-14 w-14 items-center justify-center rounded-2xl bg-primary/10 text-primary-accent">
            <RefreshCw className="h-7 w-7" aria-hidden="true" />
          </div>
          <h1 className="mt-5 text-2xl font-bold leading-tight tracking-tight text-text md:text-3xl">
            {t("routeError.updateTitle", "A new version is available")}
          </h1>
          <p className="mt-2 text-base leading-relaxed text-text-muted">
            {t(
              "routeError.updateDescription",
              "Fluentra was just updated. Reload to get the latest version.",
            )}
          </p>
          <div className="mt-6">
            <Button variant="primary" onClick={handleReload}>
              {t("routeError.reload", "Reload")}
            </Button>
          </div>
        </div>
      )}

      {kind === "offline" && (
        <div className="flex max-w-md flex-col items-center">
          <div className="flex h-14 w-14 items-center justify-center rounded-2xl bg-warning/10 text-warning-accent">
            <WifiOff className="h-7 w-7" aria-hidden="true" />
          </div>
          <h1 className="mt-5 text-2xl font-bold leading-tight tracking-tight text-text md:text-3xl">
            {t("routeError.offlineTitle", "You are offline")}
          </h1>
          <p className="mt-2 text-base leading-relaxed text-text-muted">
            {t(
              "routeError.offlineDescription",
              "Check your internet connection and try again.",
            )}
          </p>
          <div className="mt-6 flex items-center gap-3">
            <Button variant="outline" onClick={handleRetry}>
              {t("app.tryAgain", "Try again")}
            </Button>
          </div>
        </div>
      )}

      {kind === "generic" && (
        <SystemStatePage
          variant="500"
          actions={
            <>
              <button
                type="button"
                onClick={handleRetry}
                className={`${systemStateActionClass} bg-primary text-primary-fg hover:bg-primary-hover`}
              >
                {t("systemState.retry", "Try again")}
              </button>
              <Link
                to="/"
                className={`${systemStateActionClass} border border-border bg-transparent text-text hover:bg-surface-muted`}
              >
                {t("systemState.home", "Back to home")}
              </Link>
            </>
          }
        >
          {/* The request id is what support needs; the raw error is not for learners. */}
          {typeof requestId === "string" && requestId && (
            <p className="mt-6 text-xs text-text-muted">
              {t("routeError.requestId", "Request ID")}:{" "}
              <code className="rounded bg-surface-muted px-1.5 py-0.5 font-mono select-all">
                {requestId}
              </code>
            </p>
          )}
        </SystemStatePage>
      )}
    </div>
  );
}
