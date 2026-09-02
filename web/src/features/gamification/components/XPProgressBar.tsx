import React from "react";
import { CheckCircle2, Sparkles, Target, Zap } from "lucide-react";
import { useTranslation } from "react-i18next";

import { Badge } from "@/components/ui/badge";
import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Progress } from "@/components/ui/progress";
import type { GamificationSummary } from "../api/gamificationApi";

export interface XPProgressBarProps {
  summary: GamificationSummary;
}

export const XPProgressBar: React.FC<XPProgressBarProps> = ({ summary }) => {
  const { t } = useTranslation();
  const {
    total_xp = 0,
    level = 1,
    level_start_xp = 0,
    next_level_xp = 100,
    xp_today = 0,
    daily_goal_xp = 50,
  } = summary;

  // Compute progress within current level
  const levelRange = Math.max(1, next_level_xp - level_start_xp);
  const currentLevelProgress = Math.max(0, total_xp - level_start_xp);
  const levelPercentage = Math.min(
    100,
    Math.max(0, Math.round((currentLevelProgress / levelRange) * 100)),
  );

  // Compute daily goal status
  const goalReached = xp_today >= daily_goal_xp;

  return (
    <Card className="border-border/60 bg-gradient-to-br from-surface-card to-surface/40 shadow-sm">
      <CardHeader className="pb-3">
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-2.5">
            <div className="flex h-9 w-9 items-center justify-center rounded-xl bg-primary/10 text-primary shadow-inner">
              <Zap className="h-5 w-5 fill-primary/20" aria-hidden="true" />
            </div>
            <div>
              <div className="flex items-center gap-2">
                <CardTitle className="text-lg font-bold text-text">
                  {t("gamification.levelTitle", "Level {{level}}", { level })}
                </CardTitle>
                <Badge variant="secondary" className="px-2 py-0.5 text-xs font-semibold">
                  {t("gamification.xpCount", "{{count}} XP", { count: total_xp })}
                </Badge>
              </div>
              <p className="text-xs text-text-muted">
                {t(
                  "gamification.levelProgressText",
                  "{{current}} / {{target}} XP to Level {{nextLevel}}",
                  {
                    current: currentLevelProgress,
                    target: levelRange,
                    nextLevel: level + 1,
                  },
                )}
              </p>
            </div>
          </div>

          {/* Daily Goal Pill */}
          <div
            className="flex items-center gap-1.5 rounded-full border border-border/80 bg-surface px-3 py-1 text-xs font-medium"
            title={t("gamification.dailyGoalTooltip", "Daily XP Goal")}
          >
            {goalReached ? (
              <CheckCircle2 className="h-3.5 w-3.5 text-success" aria-hidden="true" />
            ) : (
              <Target className="h-3.5 w-3.5 text-primary" aria-hidden="true" />
            )}
            <span className="text-text">
              {t("gamification.todayXP", "{{today}}/{{goal}} XP today", {
                today: xp_today,
                goal: daily_goal_xp,
              })}
            </span>
          </div>
        </div>
      </CardHeader>

      <CardContent className="space-y-3 pt-0">
        {/* Level Progress Bar */}
        <div className="space-y-1">
          <div className="flex justify-between text-xs font-medium text-text-muted">
            <span>{t("gamification.levelLabel", "Level {{level}}", { level })}</span>
            <span>{levelPercentage}%</span>
          </div>
          <Progress
            value={levelPercentage}
            className="h-2.5 bg-surface-raised"
            aria-label={t("gamification.levelProgressAria", "Progress towards Level {{level}}", {
              level: level + 1,
            })}
          />
        </div>

        {/* Daily Goal Sub-bar */}
        <div className="flex items-center justify-between rounded-lg bg-surface/60 px-3 py-2 text-xs">
          <div className="flex items-center gap-1.5 text-text-muted">
            <Sparkles className="h-3.5 w-3.5 text-amber-500" aria-hidden="true" />
            <span>{t("gamification.dailyGoal", "Daily Goal")}</span>
          </div>
          <span className="font-semibold text-text">
            {goalReached
              ? t("gamification.goalAchieved", "Achieved ({{today}} XP)", { today: xp_today })
              : t("gamification.goalRemaining", "{{remaining}} XP remaining", {
                  remaining: Math.max(0, daily_goal_xp - xp_today),
                })}
          </span>
        </div>
      </CardContent>
    </Card>
  );
};
