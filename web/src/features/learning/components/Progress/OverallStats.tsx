import React from "react";
import { BookOpen, CheckCircle2, Clock, Trophy } from "lucide-react";
import { useTranslation } from "react-i18next";

import { StatTile } from "@/components/ui/stat-tile";
import { cn } from "@/lib/utils";

export interface OverallStatsProps {
  totalActivitiesCompleted: number;
  coursesInProgress: number;
  coursesCompleted: number;
  /**
   * Minutes from learning sessions this week and the weekly goal, from
   * GET /me/weekly-plan (`progress.minutes`, `minutes_goal`). Omitted while
   * that loads or when it fails: the tile is left out, not zeroed.
   */
  studyThisWeek?: { minutes: number; goal: number } | undefined;
}

/**
 * The headline figures for the progress screen, each one measured:
 * completed activities, courses in progress and courses completed from
 * GET /me/progress, and this week's study minutes from GET /me/weekly-plan.
 *
 * Nothing is derived. Total all-time study time, words mastered and the
 * "vs last week" deltas in the design have no API behind them (see
 * the P10 learner-web card) and are not drawn.
 */
export const OverallStats: React.FC<OverallStatsProps> = ({
  totalActivitiesCompleted,
  coursesInProgress,
  coursesCompleted,
  studyThisWeek,
}) => {
  const { t } = useTranslation();

  const duration = (minutes: number) => {
    const h = Math.floor(minutes / 60);
    const m = minutes % 60;
    return h > 0
      ? t("overview.progress.hoursMinutes", { h, m })
      : t("overview.progress.minutesOnly", { m });
  };

  return (
    <div
      className={cn(
        "grid grid-cols-2 gap-3 md:gap-4",
        studyThisWeek ? "lg:grid-cols-4" : "md:grid-cols-3",
      )}
    >
      <StatTile
        icon={CheckCircle2}
        iconClassName="bg-warning/10 text-warning-accent"
        label={t("progress.kpi.activitiesDone")}
        value={totalActivitiesCompleted}
        valueTestId="activities-completed"
        labelFirst
        className={cn(!studyThisWeek && "col-span-2 md:col-span-1")}
      />
      <StatTile
        icon={BookOpen}
        label={t("progress.kpi.coursesInProgress")}
        value={coursesInProgress}
        valueTestId="courses-in-progress"
        labelFirst
      />
      <StatTile
        icon={Trophy}
        iconClassName="bg-success/10 text-success-accent"
        label={t("progress.kpi.coursesCompleted")}
        value={coursesCompleted}
        valueTestId="courses-completed"
        labelFirst
      />
      {studyThisWeek && (
        <StatTile
          icon={Clock}
          iconClassName="bg-violet-500/10 text-violet-700 dark:bg-violet-400/15 dark:text-violet-300"
          label={t("overview.progress.studyThisWeek")}
          value={duration(studyThisWeek.minutes)}
          hint={t("overview.progress.weeklyGoal", {
            goal: duration(studyThisWeek.goal),
          })}
          valueTestId="study-this-week"
          labelFirst
        />
      )}
    </div>
  );
};
