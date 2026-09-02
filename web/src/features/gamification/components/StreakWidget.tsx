import React from "react";
import { Clock, Flame, Shield, ShieldAlert } from "lucide-react";
import { useTranslation } from "react-i18next";

import { Button } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { useUseStreakFreeze } from "../api/gamificationApi";
import type { Streak } from "../api/gamificationApi";

export interface StreakWidgetProps {
  streak: Streak;
}

export const StreakWidget: React.FC<StreakWidgetProps> = ({ streak }) => {
  const { t } = useTranslation();
  const freezeMutation = useUseStreakFreeze();

  const {
    current = 0,
    longest = 0,
    freezes_available = 0,
    hours_remaining = 0,
  } = streak;

  const hasActiveStreak = current > 0;

  const handleUseFreeze = () => {
    if (freezes_available <= 0 || freezeMutation.isPending) return;
    freezeMutation.mutate();
  };

  return (
    <Card className="border-border/60 bg-gradient-to-br from-surface-card to-surface/40 shadow-sm">
      <CardHeader className="pb-3">
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-2.5">
            <div
              className={`flex h-9 w-9 items-center justify-center rounded-xl shadow-inner ${
                hasActiveStreak
                  ? "bg-amber-500/10 text-amber-500"
                  : "bg-surface-raised text-text-muted"
              }`}
            >
              <Flame
                className={`h-5 w-5 ${hasActiveStreak ? "fill-amber-500/20" : ""}`}
                aria-hidden="true"
              />
            </div>
            <div>
              <CardTitle className="text-lg font-bold text-text">
                {t("gamification.streakTitle", "Day Streak")}
              </CardTitle>
              <p className="text-xs text-text-muted">
                {hasActiveStreak
                  ? t("gamification.streakBest", "Best: {{longest}} days", { longest })
                  : t("gamification.streakEmpty", "Practice daily to start your streak")}
              </p>
            </div>
          </div>

          <div className="text-right">
            <span className="text-2xl font-extrabold tracking-tight text-text">
              {current}
            </span>
            <span className="ml-1 text-xs font-semibold text-text-muted">
              {t("gamification.days", "days")}
            </span>
          </div>
        </div>
      </CardHeader>

      <CardContent className="space-y-3 pt-0">
        {/* Day Boundary Countdown & Freezes info */}
        <div className="grid grid-cols-2 gap-2 text-xs">
          <div className="flex items-center gap-1.5 rounded-lg bg-surface/60 p-2 text-text-muted">
            <Clock className="h-4 w-4 text-primary shrink-0" aria-hidden="true" />
            <span className="truncate">
              {hours_remaining > 0
                ? t("gamification.hoursLeft", "{{hours}}h left today", {
                    hours: hours_remaining,
                  })
                : t("gamification.todayEnded", "Resets at midnight")}
            </span>
          </div>

          <div className="flex items-center justify-between rounded-lg bg-surface/60 p-2 text-text-muted">
            <div className="flex items-center gap-1.5 truncate">
              {freezes_available > 0 ? (
                <Shield className="h-4 w-4 text-emerald-500 shrink-0" aria-hidden="true" />
              ) : (
                <ShieldAlert className="h-4 w-4 text-text-muted shrink-0" aria-hidden="true" />
              )}
              <span className="truncate">
                {t("gamification.freezesCount", "{{count}} freeze", {
                  count: freezes_available,
                })}
              </span>
            </div>

            {freezes_available > 0 && (
              <Button
                variant="ghost"
                size="sm"
                // 44px of touch area inside a 24px-tall row: the negative
                // margin lets the hit region overflow the compact grid cell
                // without making the cell taller than the Clock cell beside
                // it. R1 is about what a thumb can hit, not what is painted.
                className="min-h-[44px] -my-2.5 px-2 text-[11px] text-primary hover:text-primary-hover"
                onClick={handleUseFreeze}
                disabled={freezeMutation.isPending}
              >
                {t("gamification.useFreeze", "Use")}
              </Button>
            )}
          </div>
        </div>
      </CardContent>
    </Card>
  );
};
