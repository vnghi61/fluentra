import React from "react";
import { Flame } from "lucide-react";
import { useTranslation } from "react-i18next";

import { ProgressRing } from "@/components/ui/progress-ring";
import { StatTile } from "@/components/ui/stat-tile";
import { cn } from "@/lib/utils";
import type { GamificationSummary } from "../api/gamificationApi";

export interface DailyGoalTilesProps {
  summary: GamificationSummary;
  className?: string | undefined;
}

/**
 * The two compact tiles beside the Dashboard greeting: the current streak and
 * today's XP against the daily goal, both straight from /me/gamification.
 */
export function DailyGoalTiles({
  summary,
  className,
}: DailyGoalTilesProps): React.JSX.Element {
  const { t } = useTranslation();
  const streak = Math.max(0, summary.streak?.current ?? 0);
  const xpToday = Math.max(0, summary.xp_today ?? 0);
  const goal = Math.max(1, summary.daily_goal_xp ?? 50);
  const percent = Math.min(100, Math.round((xpToday / goal) * 100));

  return (
    <div className={cn("grid grid-cols-2 gap-3", className)}>
      <StatTile
        size="sm"
        icon={Flame}
        iconClassName={
          streak > 0
            ? "bg-amber-500/15 text-amber-600 dark:text-amber-300"
            : "bg-surface-muted text-text-muted"
        }
        value={streak}
        label={t("overview.dashboard.streakDays", { count: streak })}
      />
      <StatTile
        size="sm"
        visual={
          <ProgressRing
            value={percent}
            size={40}
            strokeWidth={4}
            label={t("overview.dashboard.xpRingLabel", {
              xp: xpToday,
              goal,
            })}
          >
            <span className="text-xs font-bold tabular-nums text-text">
              {percent}%
            </span>
          </ProgressRing>
        }
        value={`${xpToday} / ${goal} XP`}
        label={t("overview.dashboard.goalToday")}
      />
    </div>
  );
}
