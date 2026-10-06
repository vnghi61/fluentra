import React from "react";
import { Card } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";

export const LearnSkeleton: React.FC = () => {
  return (
    <div
      className="max-w-[1440px] mx-auto w-full min-w-0 space-y-8 animate-pulse"
      data-testid="learn-skeleton"
    >
      {/* Overview Card Skeleton */}
      <Card className="p-6 md:p-8 space-y-4 border-border-subtle bg-surface-card">
        <div className="flex flex-col md:flex-row md:items-start md:justify-between gap-6">
          <div className="flex-1 space-y-3 min-w-0">
            <div className="flex items-center gap-2">
              <Skeleton className="h-4 w-32" />
              <Skeleton className="h-5 w-20 rounded-full" />
            </div>
            <Skeleton className="h-8 md:h-9 w-2/3" />
            <Skeleton className="h-4 w-5/6" />
            <div className="pt-2 space-y-2">
              <div className="flex justify-between">
                <Skeleton className="h-4 w-40" />
                <Skeleton className="h-4 w-10" />
              </div>
              <Skeleton className="h-2.5 w-full rounded-full" />
            </div>
          </div>
          <Skeleton className="hidden md:flex h-[140px] w-[140px] rounded-full shrink-0 self-center" />
        </div>
      </Card>

      {/* Search and Filter Row Skeleton */}
      <div className="flex flex-col md:flex-row gap-3">
        <Skeleton className="h-11 flex-1 rounded-xl" />
        <Skeleton className="h-11 md:w-[220px] rounded-xl" />
      </div>

      {/* Unit Containers Skeleton */}
      <div className="space-y-4">
        {[1, 2].map((idx) => (
          <div
            key={idx}
            className="rounded-2xl border border-border-subtle bg-surface-card overflow-hidden"
          >
            <div className="p-4 md:px-5 flex items-center justify-between min-h-[44px]">
              <div className="flex items-center gap-3 w-1/2">
                <Skeleton className="h-5 w-16" />
                <Skeleton className="h-5 w-40" />
              </div>
              <div className="flex items-center gap-2">
                <Skeleton className="h-4 w-24" />
                <Skeleton className="h-4 w-4 rounded-full" />
              </div>
            </div>
            {idx === 1 && (
              <div className="border-t border-border-subtle divide-y divide-border-subtle">
                {[1, 2].map((lessonIdx) => (
                  <div
                    key={lessonIdx}
                    className="p-3.5 sm:px-4 sm:py-3 flex flex-col sm:flex-row sm:items-center justify-between gap-3 min-h-[44px]"
                  >
                    <div className="flex items-start gap-3 min-w-0 flex-1">
                      <Skeleton className="h-5 w-5 rounded-full shrink-0 mt-0.5" />
                      <div className="space-y-1.5 flex-1 min-w-0">
                        <Skeleton className="h-5 w-48 max-w-full" />
                        <Skeleton className="h-3.5 w-28" />
                      </div>
                    </div>
                    <div className="flex items-center self-end sm:self-center shrink-0 w-full sm:w-auto">
                      <Skeleton className="h-11 min-h-[44px] w-full sm:w-28 rounded-lg" />
                    </div>
                  </div>
                ))}
              </div>
            )}
          </div>
        ))}
      </div>
    </div>
  );
};
