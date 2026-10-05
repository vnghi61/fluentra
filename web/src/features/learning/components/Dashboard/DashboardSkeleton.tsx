import React from "react";
import { Card } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";

export const DashboardSkeleton: React.FC = () => {
  return (
    <div
      className="mx-auto w-full max-w-[1240px] space-y-3 md:space-y-4 lg:space-y-5"
      data-testid="dashboard-skeleton"
    >
      {/* Header skeleton */}
      <div className="space-y-2">
        <Skeleton className="h-8 w-64 max-w-full" />
        <Skeleton className="h-4 w-96 max-w-full" />
      </div>

      {/* Row 1: Hero 2fr plus side 1fr */}
      <div className="grid grid-cols-1 lg:grid-cols-[2fr_1fr] gap-3 md:gap-4 lg:gap-5">
        <Card className="rounded-2xl p-5 md:p-6 space-y-4">
          <div className="flex justify-between items-center">
            <Skeleton className="h-4 w-32" />
            <Skeleton className="h-5 w-20 rounded-full" />
          </div>
          <Skeleton className="h-7 w-3/4" />
          <Skeleton className="h-4 w-1/2" />
          <Skeleton className="h-11 w-36 rounded-lg" />
        </Card>

        <Card className="rounded-2xl p-5 md:p-6 space-y-4">
          <div className="flex justify-between items-center">
            <Skeleton className="h-5 w-36" />
            <Skeleton className="h-4 w-24" />
          </div>
          <Skeleton className="h-2 w-full rounded-full" />
          <div className="grid grid-cols-3 gap-2 pt-2">
            <Skeleton className="h-14 rounded-xl" />
            <Skeleton className="h-14 rounded-xl" />
            <Skeleton className="h-14 rounded-xl" />
          </div>
        </Card>
      </div>

      {/* Two 2-col rows (Row 2 & Row 3) + One full-width row (Row 4) */}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-3 md:gap-4 lg:gap-5">
        {/* Row 2: Daily practice & Reviews */}
        <Card className="rounded-2xl p-5 md:p-6 space-y-4">
          <Skeleton className="h-4 w-28" />
          <Skeleton className="h-6 w-48" />
          <Skeleton className="h-4 w-full" />
        </Card>

        <Card className="rounded-2xl p-5 md:p-6 space-y-4">
          <Skeleton className="h-4 w-28" />
          <Skeleton className="h-6 w-36" />
          <Skeleton className="h-10 w-32 rounded-lg" />
        </Card>

        {/* Row 3: Foundation next & Skill progress */}
        <Card className="rounded-2xl p-5 md:p-6 space-y-4">
          <Skeleton className="h-4 w-28" />
          <Skeleton className="h-6 w-40" />
          <Skeleton className="h-4 w-3/4" />
        </Card>

        <Card className="rounded-2xl p-5 md:p-6 space-y-4">
          <div className="flex justify-between items-center">
            <Skeleton className="h-5 w-32" />
            <Skeleton className="h-4 w-20" />
          </div>
          <div className="space-y-3 pt-2">
            <Skeleton className="h-3 w-full" />
            <Skeleton className="h-3 w-full" />
            <Skeleton className="h-3 w-full" />
          </div>
        </Card>

        {/* Row 4: One full-width row for weekly plan */}
        <Card className="rounded-2xl p-5 md:p-6 space-y-4 lg:col-span-2">
          <div className="flex justify-between items-center">
            <Skeleton className="h-5 w-36" />
            <Skeleton className="h-4 w-28" />
          </div>
          <Skeleton className="h-2 w-full rounded-full" />
          <div className="grid grid-cols-1 lg:grid-cols-2 gap-3 pt-2">
            <Skeleton className="h-8 w-full" />
            <Skeleton className="h-8 w-full" />
          </div>
        </Card>
      </div>
    </div>
  );
};
