import React from "react";
import { Link } from "@tanstack/react-router";
import { Flame, Target, Trophy, Zap } from "lucide-react";
import { useTranslation } from "react-i18next";

import { Progress } from "@/components/ui/progress";
import { cn } from "@/lib/utils";
import type { GamificationSummary } from "../api/gamificationApi";

export interface TodayProgressCardProps {
  summary: GamificationSummary;
  className?: string | undefined;
}

export const TodayProgressCard: React.FC<TodayProgressCardProps> = ({
  summary,
  className,
}) => {
  const { t } = useTranslation();

  // Level progress math & clamping (consistent with GamificationSummarySection)
  const totalXp = Math.max(0, summary.total_xp ?? 0);
  const level = Math.max(1, summary.level ?? 1);
  const levelStartXp = Math.max(0, summary.level_start_xp ?? 0);
  const nextLevelXp = Math.max(
    levelStartXp + 1,
    summary.next_level_xp ?? levelStartXp + 100,
  );
  const xpToNextLevel = Math.max(0, nextLevelXp - totalXp);

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

  // Quests completion: counted only from quests where every progress[k] >= steps[k]
  const quests = summary.quests ?? [];
  const hasQuests = quests.length > 0;
  const completedQuests = quests.filter((q) => {
    const steps = q.steps;
    if (!steps || typeof steps !== "object") return false;
    const entries = Object.entries(steps);
    if (entries.length === 0) return false;
    return entries.every(([k, target]) => {
      const prog = (q.progress as Record<string, number> | undefined)?.[k] ?? 0;
      return typeof target === "number" && prog >= target;
    });
  }).length;

  return (
    <div
      className={cn(
        "@container h-full space-y-4 rounded-[var(--radius-card)] border border-border-subtle bg-surface-card p-4 shadow-[var(--shadow-card)] md:p-5",
        className,
      )}
    >
      {/* Header: Title and Ghost Leaderboard Link */}
      <div className="flex items-center justify-between gap-2">
        <h2 className="text-base font-bold tracking-tight text-text md:text-lg">
          {t("gamification.todayProgress.title", "Today's Progress")}
        </h2>
        <Link
          to="/leaderboard"
          className="inline-flex shrink-0 items-center whitespace-nowrap text-sm font-medium text-primary-accent hover:underline min-h-[44px] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary rounded-md"
        >
          {t("gamification.todayProgress.leaderboardLink", "Leaderboard →")}
        </Link>
      </div>

      {/* Daily XP Row */}
      <div className="space-y-2">
        <div className="flex items-center justify-between text-xs">
          <div className="flex items-center gap-1.5 font-medium text-text-muted">
            <Target
              className="h-4 w-4 shrink-0 text-primary-accent"
              aria-hidden="true"
            />
            <span>{t("gamification.dailyGoal", "Daily Goal")}</span>
          </div>
          <div className="flex items-center gap-2">
            <span className="font-semibold text-text">
              {xpToday} / {dailyGoalXp} XP
            </span>
            <span className="font-medium text-text-muted">{goalPercent}%</span>
          </div>
        </div>
        <Progress
          value={goalPercent}
          className="h-2 bg-surface-muted transition-[width] duration-300 motion-reduce:transition-none"
          aria-label={t("gamification.dailyGoalTooltip", "Daily XP Goal")}
        />
      </div>

      {/* Stat row. Three tiles need the card's own width (~28rem); in a
        narrower card (the dashboard's side column, a phone) streak and level
        share a row and quests take a full-width row of their own, so no label
        is cut off or wrapped to three lines. */}
      <div
        className={cn("grid grid-cols-2 gap-3", hasQuests && "@md:grid-cols-3")}
      >
        {/* Streak */}
        <div className="flex min-w-0 flex-col gap-1 rounded-xl border border-border-subtle bg-surface-muted/50 p-3">
          <div className="flex items-center gap-1.5 text-xs text-text-muted font-medium">
            <Flame
              className={cn(
                "h-4 w-4 shrink-0",
                hasStreak
                  ? "text-amber-500 fill-amber-500/20"
                  : "text-text-muted",
              )}
              aria-hidden="true"
            />
            <span className="min-w-0 break-words">
              {t("gamification.streakTitle", "Day Streak")}
            </span>
          </div>
          {hasStreak ? (
            <p className="text-sm font-bold text-text truncate">
              {currentStreak} {t("gamification.days", "days")}
            </p>
          ) : (
            <p className="text-xs text-text-muted leading-tight">
              {t(
                "gamification.todayProgress.streakStart",
                "Start your streak today",
              )}
            </p>
          )}
        </div>

        {/* Level: Level N shown once, not repeated, plus XP to next level */}
        <div className="flex min-w-0 flex-col gap-1 rounded-xl border border-border-subtle bg-surface-muted/50 p-3">
          <div className="flex items-center gap-1.5">
            <Zap className="h-4 w-4 shrink-0 text-xp" aria-hidden="true" />
            <span className="text-sm font-bold text-text truncate">
              {t("gamification.levelLabel", {
                level,
                defaultValue: `Level ${level}`,
              })}
            </span>
          </div>
          <p className="text-xs text-text-muted truncate">
            {t("gamification.todayProgress.xpToNext", "{{xp}} XP to next", {
              xp: xpToNextLevel,
            })}
          </p>
        </div>

        {/* Quests completed: omitted when quests is empty */}
        {hasQuests && (
          <div className="col-span-2 flex min-w-0 items-center justify-between gap-2 rounded-xl border border-border-subtle bg-surface-muted/50 p-3 @md:col-span-1 @md:flex-col @md:items-start @md:justify-start @md:gap-1">
            <div className="flex min-w-0 items-center gap-1.5 text-xs text-text-muted font-medium">
              <Trophy
                className="h-4 w-4 shrink-0 text-mission"
                aria-hidden="true"
              />
              <span className="truncate">
                {t("gamification.todayProgress.quests", "Quests")}
              </span>
            </div>
            <p className="text-sm font-bold text-text truncate">
              {completedQuests} / {quests.length}
            </p>
          </div>
        )}
      </div>
    </div>
  );
};

TodayProgressCard.displayName = "TodayProgressCard";
