import React from "react";
import { CheckCircle2 } from "lucide-react";
import { useTranslation } from "react-i18next";

import { Card } from "@/components/ui/card";

export interface OverallStatsProps {
  totalActivitiesCompleted: number;
}

/**
 * The headline figures for the progress screen.
 *
 * There is one, because GET /me/progress returns one. This card previously sat
 * beside "Total Study Time" and "Words Mastered", both computed from this same
 * number by multiplying it — three minutes and four words per activity — and both
 * rendered in the same weight as the real figure, which is what made them read as
 * measurements. Bring either back when the endpoint returns it.
 */
export const OverallStats: React.FC<OverallStatsProps> = ({
  totalActivitiesCompleted,
}) => {
  const { t } = useTranslation();

  return (
    <div className="grid grid-cols-1 sm:grid-cols-3 gap-4">
      <Card className="p-5 flex items-center gap-4 border-border-subtle bg-surface-card">
        <div className="h-12 w-12 rounded-2xl bg-warning/10 text-warning-accent flex items-center justify-center shrink-0">
          <CheckCircle2 className="h-6 w-6" aria-hidden="true" />
        </div>
        <div>
          <p className="text-xs text-text-muted font-medium">
            {t("progress.activitiesDone")}
          </p>
          <p className="text-2xl font-extrabold text-text mt-0.5">
            {totalActivitiesCompleted}
          </p>
        </div>
      </Card>
    </div>
  );
};
