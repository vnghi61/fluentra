import * as React from "react";
import { useRouter } from "@tanstack/react-router";
import { RefreshCw } from "lucide-react";
import { useTranslation } from "react-i18next";

import { describeLoadError } from "@/api/loadError";
import { ServerErrorState } from "@/components/layout/ServerErrorState";
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

  const handleRetry = async () => {
    reset();
    await router.invalidate();
  };

  const handleReload = () => {
    if (typeof window !== "undefined") {
      window.location.reload();
    }
  };

  const { kind: loadKind, requestId } = describeLoadError(error);

  return (
    <div
      role={kind === "chunk" ? "alert" : undefined}
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

      {/* Offline and every other failure share the error screen the waking
          screen leads into; the raw error is not for learners, the request id
          is what support needs. */}
      {kind !== "chunk" && (
        <ServerErrorState
          kind={loadKind}
          requestId={requestId}
          onRetry={handleRetry}
          className="min-h-0 py-0"
        />
      )}
    </div>
  );
}
