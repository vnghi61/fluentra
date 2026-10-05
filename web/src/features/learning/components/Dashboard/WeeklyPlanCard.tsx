import React, { useState } from "react";
import { Link } from "@tanstack/react-router";
import { CalendarDays, CheckCircle2, ChevronRight, Circle } from "lucide-react";
import { useTranslation } from "react-i18next";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Progress } from "@/components/ui/progress";
import { cn } from "@/lib/utils";
import { useWeeklyPlan, type WeeklyPlanItem } from "../../api/placement";

export interface WeeklyPlanCardProps {
  className?: string | undefined;
}

/**
 * This week's plan: fixed for the week, with the minutes studied and the items
 * done read when the card loads.
 */
export const WeeklyPlanCard: React.FC<WeeklyPlanCardProps> = ({
  className,
}) => {
  const { t, i18n } = useTranslation();
  const [expanded, setExpanded] = useState(false);
  const [mobileExpanded, setMobileExpanded] = useState(false);
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

  const visibleItems =
    expanded || mobileExpanded ? plan.items : plan.items.slice(0, 5);

  return (
    <Card
      className={cn(
        "bg-surface-card border border-border-subtle rounded-2xl shadow-[0_2px_8px_rgba(15,23,42,0.04)]",
        className,
      )}
    >
      {/* Below md: Mobile collapsed full-width button row */}
      <button
        type="button"
        aria-expanded={mobileExpanded}
        onClick={() => setMobileExpanded((value) => !value)}
        className="flex md:hidden w-full items-center justify-between gap-3 p-4 text-left font-medium min-h-[44px]"
      >
        <div className="flex items-center gap-2 min-w-0">
          <CalendarDays
            className="h-5 w-5 text-primary shrink-0"
            aria-hidden="true"
          />
          <span className="text-sm font-semibold text-text truncate">
            {t("weeklyPlan.mobileRow", "This week's plan")}
          </span>
        </div>
        <div className="flex items-center gap-1.5 shrink-0 text-xs font-medium text-text-muted">
          <span>
            {t("weeklyPlan.minutesShort", {
              done: plan.progress.minutes,
              goal: plan.minutes_goal,
              defaultValue: `${plan.progress.minutes} / ${plan.minutes_goal} min`,
            })}
          </span>
          <ChevronRight
            className={cn(
              "h-4 w-4 transition-transform duration-200 text-text-muted motion-reduce:transition-none",
              mobileExpanded && "rotate-90",
            )}
            aria-hidden="true"
          />
        </div>
      </button>

      {/* From md up: Desktop compact header */}
      <div className="hidden md:block p-4 pb-3 md:p-5 md:pb-3 space-y-3">
        <div className="flex flex-wrap items-center justify-between gap-2">
          <div className="flex items-center gap-2">
            <CalendarDays className="h-5 w-5 text-primary" aria-hidden="true" />
            <h3 className="text-base font-semibold text-text">
              {t("weeklyPlan.title", "This week's plan")}
            </h3>
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
          <Progress
            value={percent}
            className="h-2"
            aria-label={t("weeklyPlan.progressLabel")}
          />
        </div>
      </div>

      {/* Items list: appears only when expanded on mobile, always visible on md+ */}
      <div
        className={cn(
          mobileExpanded ? "block" : "hidden md:block",
          "p-4 pt-0 md:p-5 md:pt-0 border-t border-border-subtle md:border-t-0",
        )}
      >
        <ul className="divide-y divide-border-subtle lg:divide-y-0 lg:grid lg:grid-cols-2 lg:gap-x-6 lg:gap-y-1">
          {visibleItems.map((item, index) => (
            <li
              key={`${item.kind}-${item.lesson_id ?? index}`}
              className="flex min-h-[44px] items-center justify-between gap-2 py-1.5 lg:border-b lg:border-border-subtle/50"
            >
              <div className="flex min-w-0 flex-1 items-center gap-2">
                {item.done ? (
                  <CheckCircle2
                    className="h-5 w-5 shrink-0 text-success"
                    aria-label={t("weeklyPlan.done")}
                  />
                ) : (
                  <Circle
                    className="h-5 w-5 shrink-0 text-text-muted"
                    aria-hidden="true"
                  />
                )}
                <div className="min-w-0">
                  {item.lesson_id ? (
                    <Link
                      to="/learn/lesson/$lessonId"
                      params={{ lessonId: item.lesson_id }}
                      className="flex min-h-[44px] items-center text-sm font-medium text-text hover:text-primary-accent"
                    >
                      <span
                        className="truncate"
                        title={item.title ?? kindLabels[item.kind]}
                      >
                        {item.title ?? kindLabels[item.kind]}
                      </span>
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
        {plan.items.length > 5 && (
          <div className="hidden md:block pt-2">
            <Button
              variant="ghost"
              className="w-full text-xs font-medium text-text-muted hover:text-text"
              aria-expanded={expanded}
              onClick={() => setExpanded((value) => !value)}
            >
              {expanded
                ? t("weeklyPlan.showLess", "Show less")
                : t("weeklyPlan.viewAll", "View all")}
            </Button>
          </div>
        )}
      </div>
    </Card>
  );
};
