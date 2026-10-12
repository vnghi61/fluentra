import React from "react";
import { ChevronDown, Flame, Target, Zap } from "lucide-react";
import { useTranslation } from "react-i18next";

import { Progress } from "@/components/ui/progress";
import { Button } from "@/components/ui/button";
import {
  useUseStreakFreeze,
  type GamificationSummary,
} from "../api/gamificationApi";
import { LeaderboardWidget } from "./LeaderboardWidget";
import { QuestsWidget } from "./QuestsWidget";

export interface GamificationSummarySectionProps {
  summary: GamificationSummary;
}

export const GamificationSummarySection: React.FC<
  GamificationSummarySectionProps
> = ({ summary }) => {
  const { t } = useTranslation();
  const freeze = useUseStreakFreeze();

  // Level progress math & clamping
  const totalXp = Math.max(0, summary.total_xp ?? 0);
  const level = Math.max(1, summary.level ?? 1);
  const levelStartXp = Math.max(0, summary.level_start_xp ?? 0);
  const nextLevelXp = Math.max(
    levelStartXp + 1,
    summary.next_level_xp ?? levelStartXp + 100,
  );
  const levelSpan = Math.max(1, nextLevelXp - levelStartXp);
  const currentLevelXp = Math.max(0, totalXp - levelStartXp);
  const levelPercent = Math.min(
    100,
    Math.max(0, Math.round((currentLevelXp / levelSpan) * 100)),
  );

  // Daily XP goal math & clamping
  const xpToday = Math.max(0, summary.xp_today ?? 0);
  const dailyGoalXp = Math.max(1, summary.daily_goal_xp ?? 50);
  const goalPercent = Math.min(
    100,
    Math.max(0, Math.round((xpToday / dailyGoalXp) * 100)),
  );

  // Streak days
  const currentStreak = Math.max(0, summary.streak?.current ?? 0);
  const hasStreak = currentStreak > 0;

  return (
    <div className="space-y-4 md:space-y-5">
      {/* Top compact stats grid: 3 stats across (Daily Goal, Streak, Level XP) */}
      <div className="grid grid-cols-1 sm:grid-cols-3 gap-3">
        {/* 1. Daily XP Goal */}
        <div className="rounded-2xl border border-border-subtle bg-surface-card p-4 flex flex-col gap-3 min-w-0">
          <div className="flex items-center justify-between gap-1.5">
            <div className="flex h-8 w-8 items-center justify-center rounded-xl bg-primary/10 text-primary-accent shrink-0">
              <Target className="h-5 w-5" aria-hidden="true" />
            </div>
            <span className="text-xs font-semibold text-text-muted truncate">
              {goalPercent}%
            </span>
          </div>
          <div className="space-y-1 min-w-0">
            <p className="text-sm text-text-muted truncate font-medium">
              {t("gamification.dailyGoal", "Daily Goal")}
            </p>
            <p className="text-2xl font-bold leading-tight tabular-nums text-text truncate">
              {xpToday} / {dailyGoalXp} XP
            </p>
            <Progress
              value={goalPercent}
              className="h-1.5 bg-surface-muted"
              aria-label={t("gamification.dailyGoalTooltip", "Daily XP Goal")}
            />
            <p className="text-sm text-text-muted">
              {t("gamification.goalRemaining", "{{remaining}} XP remaining", {
                remaining: Math.max(0, dailyGoalXp - xpToday),
              })}
            </p>
          </div>
        </div>

        {/* 2. Streak Days */}
        <div className="rounded-2xl border border-border-subtle bg-surface-card p-4 flex flex-col gap-3 min-w-0">
          <div className="flex items-center justify-between gap-1.5">
            <div
              className={`flex h-8 w-8 items-center justify-center rounded-xl shrink-0 ${
                hasStreak
                  ? "bg-amber-500/10 text-amber-500"
                  : "bg-surface-muted text-text-muted"
              }`}
            >
              <Flame
                className={`h-5 w-5 ${hasStreak ? "fill-amber-500/20" : ""}`}
                aria-hidden="true"
              />
            </div>
            {summary.streak?.longest !== undefined &&
              summary.streak.longest > 0 && (
                <span className="text-sm text-text-muted truncate">
                  {t("gamification.streakBest", {
                    longest: summary.streak.longest,
                    defaultValue: `Best: ${summary.streak.longest}`,
                  })}
                </span>
              )}
          </div>
          <div className="space-y-1 min-w-0">
            <p className="text-sm text-text-muted truncate font-medium">
              {t("gamification.streakTitle", "Day Streak")}
            </p>
            <p className="text-2xl font-bold leading-tight tabular-nums text-text truncate">
              {currentStreak} {t("gamification.days", "days")}
            </p>
            <p className="text-sm text-text-muted">
              {hasStreak
                ? t("gamification.hoursLeft", "{{hours}}h left today", {
                    hours: summary.streak.hours_remaining,
                  })
                : t(
                    "gamification.streakEmpty",
                    "Practice daily to start your streak",
                  )}
            </p>
            {summary.streak && summary.streak.freezes_available > 0 && (
              <Button
                variant="ghost"
                size="sm"
                className="px-0 text-primary-accent"
                disabled={freeze.isPending}
                isLoading={freeze.isPending}
                aria-label={t("gamification.useStreakFreeze")}
                onClick={() => freeze.mutate()}
              >
                {t("gamification.useFreeze", "Use")} ·{" "}
                {t("gamification.freezesCount", "{{count}} freeze", {
                  count: summary.streak.freezes_available,
                })}
              </Button>
            )}
            {freeze.isError && (
              <p role="alert" className="text-sm text-danger-accent">
                {t("states.errorTitle", "Something went wrong")}
              </p>
            )}
            {freeze.isSuccess && (
              <p role="status" className="text-sm text-success-accent">
                {t("gamification.freezeSuccess")}
              </p>
            )}
          </div>
        </div>

        {/* 3. Level Progress */}
        <div className="rounded-2xl border border-border-subtle bg-surface-card p-4 flex flex-col gap-3 min-w-0">
          <div className="flex items-center justify-between gap-1.5">
            <div className="flex h-8 w-8 items-center justify-center rounded-xl bg-xp/10 text-xp shrink-0">
              <Zap className="h-5 w-5" aria-hidden="true" />
            </div>
            <span className="text-xs font-semibold text-text-muted truncate">
              {levelPercent}%
            </span>
          </div>
          <div className="space-y-1 min-w-0">
            <p className="text-sm text-text-muted truncate font-medium">
              {t("gamification.levelLabel", {
                level,
                defaultValue: `Level ${level}`,
              })}
            </p>
            <p className="text-2xl font-bold leading-tight tabular-nums text-text truncate">
              {currentLevelXp} / {levelSpan} XP
            </p>
            <Progress
              value={levelPercent}
              className="h-1.5 bg-surface-muted"
              aria-label={t(
                "gamification.levelProgressAria",
                "Progress towards Level {{level}}",
                { level: level + 1 },
              )}
            />
            <p className="text-sm text-text-muted">
              {t("gamification.levelRemaining", {
                count: Math.max(0, nextLevelXp - totalXp),
                level: level + 1,
              })}
            </p>
          </div>
        </div>
      </div>

      {/* Quests / Active Missions */}
      <QuestsWidget quests={summary.quests} />

      {/* Secondary standings remain accessible without a card around cards. */}
      <details className="group">
        <summary className="min-h-[44px] cursor-pointer font-semibold text-sm text-primary-accent flex items-center justify-between gap-2 select-none list-none [&::-webkit-details-marker]:hidden focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary rounded-lg px-1">
          <span>
            {t(
              "gamification.leaderboardDetails",
              "Standings and league participation",
            )}
          </span>
          <ChevronDown
            className="h-4 w-4 text-text-muted transition-transform duration-200 group-open:rotate-180 motion-reduce:transition-none"
            aria-hidden="true"
          />
        </summary>
        <div className="pt-2">
          <LeaderboardWidget currentLeague={summary.league ?? undefined} />
        </div>
      </details>
    </div>
  );
};
