import React from "react";
import { Link } from "@tanstack/react-router";
import { CalendarDays, CheckCircle2, Circle } from "lucide-react";
import { useTranslation } from "react-i18next";

import { Badge } from "@/components/ui/badge";
import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { cn } from "@/lib/utils";
import { useWeeklyPlan, type WeeklyPlanItem } from "../../api/placement";

export interface WeeklyPlanCardProps {
  className?: string | undefined;
}

/**
 * This week's plan: fixed for the week, with the minutes studied and the items
 * done read when the card loads.
 */
export const WeeklyPlanCard: React.FC<WeeklyPlanCardProps> = ({ className }) => {
  const { t, i18n } = useTranslation();
  const { data: plan } = useWeeklyPlan();
  if (!plan) return null;

  const kindLabels: Record<WeeklyPlanItem["kind"], string> = {
    lesson: t("weeklyPlan.kind.lesson"),
    daily_practice: t("weeklyPlan.kind.dailyPractice"),
    reviews: t("weeklyPlan.kind.reviews"),
    writing: t("weeklyPlan.kind.writing"),
    speaking: t("weeklyPlan.kind.speaking"),
  };
  const percent = Math.min(
    100,
    Math.round((plan.progress.minutes / Math.max(1, plan.minutes_goal)) * 100),
  );
  const weekOf = new Date(`${plan.week_start}T00:00:00`).toLocaleDateString(
    i18n.language,
    { day: "numeric", month: "long" },
  );

  return (
    <Card className={cn("border-border bg-surface-card", className)}>
      <CardHeader className="space-y-3">
        <div className="flex flex-wrap items-center justify-between gap-2">
          <div className="flex items-center gap-2">
            <CalendarDays className="h-5 w-5 text-primary" aria-hidden="true" />
            <CardTitle className="text-lg font-bold text-text">
              {t("weeklyPlan.title")}
            </CardTitle>
          </div>
          <span className="text-sm text-text-muted">
            {t("weeklyPlan.weekOf", { date: weekOf })}
          </span>
        </div>
        <div className="space-y-1.5">
          <div className="flex flex-wrap justify-between gap-2 text-sm">
            <span className="text-text">
              {t("weeklyPlan.minutes", {
                done: plan.progress.minutes,
                goal: plan.minutes_goal,
              })}
            </span>
            <span className="text-text-muted">
              {t("weeklyPlan.itemsDone", {
                done: plan.progress.items_done,
                total: plan.progress.items_total,
              })}
            </span>
          </div>
          <div
            className="h-2 w-full overflow-hidden rounded-full bg-border-subtle"
            role="progressbar"
            aria-label={t("weeklyPlan.progressLabel")}
            aria-valuemin={0}
            aria-valuemax={100}
            aria-valuenow={percent}
          >
            <div
              className="h-full rounded-full bg-primary"
              style={{ width: `${percent}%` }}
            />
          </div>
        </div>
      </CardHeader>
      <CardContent>
        <ul className="divide-y divide-border-subtle">
          {plan.items.map((item, index) => (
            <li
              key={`${item.kind}-${item.lesson_id ?? index}`}
              className="flex items-center justify-between gap-3 py-2"
            >
              <div className="flex min-w-0 items-center gap-3">
                {item.done ? (
                  <CheckCircle2
                    className="h-5 w-5 shrink-0 text-success"
                    aria-label={t("weeklyPlan.done")}
                  />
                ) : (
                  <Circle className="h-5 w-5 shrink-0 text-text-muted" aria-hidden="true" />
                )}
                <div className="min-w-0">
                  {item.lesson_id ? (
                    <Link
                      to="/learn/lesson/$lessonId"
                      params={{ lessonId: item.lesson_id }}
                      className="inline-flex min-h-[44px] items-center truncate text-sm font-medium text-text hover:text-primary"
                    >
                      {item.title ?? kindLabels[item.kind]}
                    </Link>
                  ) : (
                    <span className="block text-sm font-medium text-text">
                      {kindLabels[item.kind]}
                    </span>
                  )}
                  {item.reviews ? (
                    <span className="block text-xs text-text-muted">
                      {t("weeklyPlan.reviews", { reviews: item.reviews })}
                    </span>
                  ) : null}
                </div>
              </div>
              <div className="flex shrink-0 items-center gap-2">
                {item.focus && (
                  <Badge variant="warning">{t("weeklyPlan.focus")}</Badge>
                )}
                <span className="text-xs text-text-muted">
                  {t("weeklyPlan.itemMinutes", { minutes: item.minutes })}
                </span>
              </div>
            </li>
          ))}
        </ul>
      </CardContent>
    </Card>
  );
};
