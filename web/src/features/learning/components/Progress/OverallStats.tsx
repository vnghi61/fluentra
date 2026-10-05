import React from "react";
import { BookOpen, CheckCircle2, Trophy } from "lucide-react";
import { useTranslation } from "react-i18next";

import { Card } from "@/components/ui/card";

export interface OverallStatsProps {
  totalActivitiesCompleted: number;
  coursesInProgress: number;
  coursesCompleted: number;
}

/**
 * The headline figures for the progress screen.
 *
 * There are three real figures rendered from GET /me/progress:
 * completed activities count, courses in progress count, and courses completed count.
 * This card previously sat beside "Total Study Time" and "Words Mastered",
 * both computed by multiplying activity counts.
 *
 * TODO(backend): Study minutes, streak, and weekly deltas need dedicated fields
 * on GET /me/progress before those metrics can be displayed (see
 * docs/development/phase-2/P10-learner-web.md §1). Until then, do not render
 * fabricated measurements, "no data" placeholders, or fake cards.
 */
export const OverallStats: React.FC<OverallStatsProps> = ({
  totalActivitiesCompleted,
  coursesInProgress,
  coursesCompleted,
}) => {
  const { t } = useTranslation();

  return (
    <div className="grid grid-cols-2 md:grid-cols-3 gap-3 md:gap-4">
      {/* Activities Completed */}
      <Card className="col-span-2 md:col-span-1 p-4 flex items-center gap-4 border-border-subtle bg-surface-card">
        <div className="h-10 w-10 rounded-xl bg-warning/10 text-warning-accent flex items-center justify-center shrink-0">
          <CheckCircle2 className="h-5 w-5" aria-hidden="true" />
        </div>
        <div>
          <p className="text-xs text-text-muted font-medium">
            {t("progress.kpi.activitiesDone")}
          </p>
          <p
            className="text-2xl font-extrabold text-text mt-0.5"
            data-testid="activities-completed"
          >
            {totalActivitiesCompleted}
          </p>
        </div>
      </Card>

      {/* Courses in Progress */}
      <Card className="p-4 flex items-center gap-4 border-border-subtle bg-surface-card">
        <div className="h-10 w-10 rounded-xl bg-primary/10 text-primary-accent flex items-center justify-center shrink-0">
          <BookOpen className="h-5 w-5" aria-hidden="true" />
        </div>
        <div>
          <p className="text-xs text-text-muted font-medium">
            {t("progress.kpi.coursesInProgress")}
          </p>
          <p
            className="text-2xl font-extrabold text-text mt-0.5"
            data-testid="courses-in-progress"
          >
            {coursesInProgress}
          </p>
        </div>
      </Card>

      {/* Courses Completed */}
      <Card className="p-4 flex items-center gap-4 border-border-subtle bg-surface-card">
        <div className="h-10 w-10 rounded-xl bg-success/10 text-success-accent flex items-center justify-center shrink-0">
          <Trophy className="h-5 w-5" aria-hidden="true" />
        </div>
        <div>
          <p className="text-xs text-text-muted font-medium">
            {t("progress.kpi.coursesCompleted")}
          </p>
          <p
            className="text-2xl font-extrabold text-text mt-0.5"
            data-testid="courses-completed"
          >
            {coursesCompleted}
          </p>
        </div>
      </Card>
    </div>
  );
};
