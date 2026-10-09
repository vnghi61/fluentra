import React from "react";
import { Card } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";

/** Mirrors the Dashboard grid so the page does not jump when data arrives. */
export const DashboardSkeleton: React.FC = () => {
  return (
    <div
      className="mx-auto w-full space-y-4 lg:space-y-5"
      data-testid="dashboard-skeleton"
    >
      {/* Greeting and the two compact tiles */}
      <div className="flex flex-col gap-4 md:flex-row md:items-end md:justify-between">
        <div className="space-y-2">
          <Skeleton className="h-8 w-64 max-w-full" />
          <Skeleton className="h-4 w-96 max-w-full" />
        </div>
        <div className="hidden gap-3 md:grid md:w-[400px] md:grid-cols-2 lg:w-[420px]">
          <Skeleton className="h-16 rounded-[var(--radius-card)]" />
          <Skeleton className="h-16 rounded-[var(--radius-card)]" />
        </div>
      </div>

      {/* Row 1: hero 2fr | today's progress 1fr */}
      <div className="grid grid-cols-1 gap-4 lg:grid-cols-[minmax(0,2fr)_minmax(0,1fr)] lg:gap-5">
        <Card className="space-y-4 p-5 md:p-6">
          <Skeleton className="h-4 w-32" />
          <Skeleton className="h-5 w-40 rounded-full" />
          <Skeleton className="h-7 w-3/4" />
          <Skeleton className="h-2 w-1/2 rounded-full" />
          <Skeleton className="h-11 w-40 rounded-xl" />
        </Card>
        <Card className="space-y-4 p-5 md:p-6">
          <div className="flex items-center justify-between">
            <Skeleton className="h-5 w-36" />
            <Skeleton className="h-4 w-24" />
          </div>
          <Skeleton className="h-2 w-full rounded-full" />
          <div className="grid grid-cols-2 gap-3 pt-2">
            <Skeleton className="h-16 rounded-xl" />
            <Skeleton className="h-16 rounded-xl" />
          </div>
        </Card>
      </div>

      {/* Row 2: practice set | reviews & path */}
      <div className="grid grid-cols-1 gap-4 lg:grid-cols-2 lg:gap-5">
        {[0, 1].map((i) => (
          <Card key={i} className="space-y-4 p-5 md:p-6">
            <Skeleton className="h-5 w-40" />
            <Skeleton className="h-4 w-full" />
            <div className="flex flex-wrap gap-2">
              <Skeleton className="h-7 w-24 rounded-lg" />
              <Skeleton className="h-7 w-24 rounded-lg" />
              <Skeleton className="h-7 w-20 rounded-lg" />
            </div>
            <Skeleton className="h-11 w-44 rounded-xl" />
          </Card>
        ))}
      </div>

      {/* Row 3: weekly plan | CEFR skills */}
      <div className="grid grid-cols-1 gap-4 lg:gap-5 xl:grid-cols-[minmax(0,1.35fr)_minmax(0,1fr)]">
        <Card className="space-y-4 p-5 md:p-6">
          <div className="flex items-center justify-between">
            <Skeleton className="h-5 w-36" />
            <Skeleton className="h-4 w-28" />
          </div>
          <Skeleton className="h-2 w-full rounded-full" />
          <div className="space-y-3 pt-2">
            <Skeleton className="h-10 w-full" />
            <Skeleton className="h-10 w-full" />
            <Skeleton className="h-10 w-full" />
          </div>
        </Card>
        <Card className="space-y-4 p-5 md:p-6">
          <Skeleton className="h-5 w-44" />
          <div className="space-y-3 pt-2">
            {Array.from({ length: 6 }).map((_, i) => (
              <Skeleton key={i} className="h-3 w-full rounded-full" />
            ))}
          </div>
        </Card>
      </div>
    </div>
  );
};
