import * as React from "react";
import { Link, useRouter } from "@tanstack/react-router";
import { AlertTriangle, RefreshCw, WifiOff } from "lucide-react";
import { useTranslation } from "react-i18next";

import { ApiError } from "@/api/client";
import { Button } from "@/components/ui/button";
import { getErrorMessage } from "@/lib/errors/catalogue";
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

  const isApi = error instanceof ApiError;
  const problem = isApi ? error.problem : undefined;
  const errorCode = problem?.code;
  const cataloguedMessage = problem ? getErrorMessage(problem) : undefined;
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
          <h1 className="mt-5 text-2xl font-bold tracking-tight text-text">
            {t("routeError.updateTitle", "A new version is available")}
          </h1>
          <p className="mt-2 text-sm text-text-muted">
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
          <h1 className="mt-5 text-2xl font-bold tracking-tight text-text">
            {t("routeError.offlineTitle", "You are offline")}
          </h1>
          <p className="mt-2 text-sm text-text-muted">
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
        <div className="flex max-w-md flex-col items-center">
          <div className="flex h-14 w-14 items-center justify-center rounded-2xl bg-danger/10 text-danger-accent">
            <AlertTriangle className="h-7 w-7" aria-hidden="true" />
          </div>
          <h1 className="mt-5 text-2xl font-bold tracking-tight text-text">
            {t("routeError.genericTitle", "Something went wrong")}
          </h1>
          <p className="mt-2 text-sm text-text-muted">
            {t(
              "routeError.genericDescription",
              "An unexpected error occurred. Please try again in a moment.",
            )}
          </p>
          <div className="mt-6 flex flex-wrap items-center justify-center gap-3">
            <Button variant="primary" onClick={handleRetry}>
              {t("app.tryAgain", "Try again")}
            </Button>
            <Link
              to="/"
              className="inline-flex min-h-[44px] items-center justify-center rounded-lg border border-border bg-transparent px-4 text-sm font-medium text-text transition-colors hover:bg-surface-muted focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary focus-visible:ring-offset-2"
            >
              {t("routeError.home", "Back to home")}
            </Link>
          </div>

          <details className="mt-8 w-full rounded-xl border border-border-subtle bg-surface-card p-4 text-left">
            <summary className="cursor-pointer text-xs font-semibold text-text-muted select-none hover:text-text">
              {t("routeError.details", "Technical details")}
            </summary>
            <div className="mt-3 flex flex-col gap-2 border-t border-border-subtle pt-3 text-xs text-text-muted">
              <div>
                <span className="font-medium text-text">
                  {t("routeError.error", "Error")}:{" "}
                </span>
                <span className="break-all">
                  {error.message || String(error)}
                </span>
              </div>
              {errorCode && (
                <div>
                  <span className="font-medium text-text">
                    {t("routeError.code", "Code")}:{" "}
                  </span>
                  <span className="font-mono">{errorCode}</span>
                  {cataloguedMessage && (
                    <p className="mt-1 text-xs text-text-muted">
                      {cataloguedMessage}
                    </p>
                  )}
                </div>
              )}
              {typeof requestId === "string" && requestId && (
                <div>
                  <span className="font-medium text-text">
                    {t("routeError.requestId", "Request ID")}:{" "}
                  </span>
                  <code className="rounded bg-surface-muted px-1.5 py-0.5 font-mono select-all">
                    {requestId}
                  </code>
                </div>
              )}
            </div>
          </details>
        </div>
      )}
    </div>
  );
}
