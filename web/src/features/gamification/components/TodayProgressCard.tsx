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
        "rounded-2xl border border-border-subtle bg-surface-card p-4 md:p-5 shadow-[0_2px_8px_rgba(15,23,42,0.04)] space-y-4",
        className,
      )}
    >
      {/* Header: Title and Ghost Leaderboard Link */}
      <div className="flex items-center justify-between gap-2">
        <h3 className="text-base md:text-lg font-semibold text-text">
          {t("gamification.todayProgress.title", "Today's Progress")}
        </h3>
        <Link
          to="/progress"
          className="inline-flex items-center text-xs md:text-sm font-medium text-primary-accent hover:underline min-h-[44px]"
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

      {/* 3-column Stat Row (grid-cols-3) */}
      <div
        className={cn("grid gap-3", hasQuests ? "grid-cols-3" : "grid-cols-2")}
      >
        {/* Streak */}
        <div className="flex flex-col gap-1 rounded-xl border border-border-subtle bg-surface-muted/50 p-2.5 min-w-0">
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
        <div className="flex flex-col gap-1 rounded-xl border border-border-subtle bg-surface-muted/50 p-2.5 min-w-0">
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
          <div className="flex flex-col gap-1 rounded-xl border border-border-subtle bg-surface-muted/50 p-2.5 min-w-0">
            <div className="flex items-center gap-1.5 text-xs text-text-muted font-medium">
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
