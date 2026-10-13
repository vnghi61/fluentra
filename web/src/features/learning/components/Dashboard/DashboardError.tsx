import React from "react";

import { describeLoadError } from "@/api/loadError";
import { ServerErrorState } from "@/components/layout/ServerErrorState";

export interface DashboardErrorProps {
  onRetry: () => unknown;
  error?: Error | null;
}

/**
 * A learner page (the dashboard, progress) that could not load its data.
 *
 * This used to be a red card titled "Unable to Load Dashboard" over
 * `error.message`, which on a cold start was the API's own words —
 * "Authentication required", "Internal Server Error". It is now the error
 * screen the waking screen leads into: what happened in plain words, a timed
 * retry when the server is busy, and a way on to Learn.
 */
export const DashboardError: React.FC<DashboardErrorProps> = ({
  onRetry,
  error,
}) => {
  const { kind, requestId } = describeLoadError(error);
  return (
    <ServerErrorState kind={kind} requestId={requestId} onRetry={onRetry} />
  );
};
