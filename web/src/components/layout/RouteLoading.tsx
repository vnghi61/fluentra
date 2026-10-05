import * as React from "react";

import { useTranslation } from "react-i18next";

import { Skeleton } from "@/components/ui/skeleton";

/**
 * Branded route transition loading state.
 *
 * Renders immediately when mounted because the router delays display via
 * defaultPendingMs: 150 to prevent visual flash on fast responses.
 * Announces status to screen readers.
 */
export function RouteLoading(): React.JSX.Element {
  const { t } = useTranslation();
  const loadingText = t("app.loading", "Loading...");

  return (
    <div
      role="status"
      aria-label={loadingText}
      className="relative w-full overflow-x-hidden min-h-[200px]"
    >
      <span className="sr-only">{loadingText}</span>

      {/* Indeterminate top progress bar */}
      <div
        className="fixed top-0 left-0 right-0 z-50 h-0.5 w-full overflow-hidden bg-primary/20"
        aria-hidden="true"
      >
        <div className="h-full w-2/5 animate-route-progress bg-primary" />
      </div>

      {/* Skeleton placeholder layout: heading bar, two cards, and a list */}
      <div className="mx-auto w-full max-w-5xl space-y-6 p-4 sm:p-6">
        {/* Heading bar */}
        <div className="space-y-2">
          <Skeleton className="h-8 w-48 rounded-lg" />
          <Skeleton className="h-4 w-72 rounded-md" />
        </div>

        {/* Two cards */}
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
          <div className="space-y-4 rounded-xl border border-border-subtle bg-surface-card p-6 shadow-xs">
            <Skeleton className="h-6 w-32 rounded-md" />
            <Skeleton className="h-4 w-full rounded-md" />
            <Skeleton className="h-4 w-3/4 rounded-md" />
            <Skeleton className="h-10 w-28 rounded-lg" />
          </div>
          <div className="space-y-4 rounded-xl border border-border-subtle bg-surface-card p-6 shadow-xs">
            <Skeleton className="h-6 w-36 rounded-md" />
            <Skeleton className="h-4 w-full rounded-md" />
            <Skeleton className="h-4 w-2/3 rounded-md" />
            <Skeleton className="h-10 w-28 rounded-lg" />
          </div>
        </div>

        {/* List */}
        <div className="space-y-3 rounded-xl border border-border-subtle bg-surface-card p-6 shadow-xs">
          <Skeleton className="h-5 w-40 rounded-md" />
          <div className="space-y-2 pt-2">
            <Skeleton className="h-12 w-full rounded-lg" />
            <Skeleton className="h-12 w-full rounded-lg" />
            <Skeleton className="h-12 w-full rounded-lg" />
          </div>
        </div>
      </div>
    </div>
  );
}
