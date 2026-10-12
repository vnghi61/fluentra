import React, { useState } from "react";
import { Link } from "@tanstack/react-router";
import {
  ArrowRight,
  BookOpen,
  CalendarDays,
  CheckCircle2,
  ChevronDown,
  ChevronRight,
  Circle,
  Headphones,
  Layers,
  ListChecks,
  MessageCircle,
  Mic,
  PenLine,
  Target,
  Zap,
  type LucideIcon,
} from "lucide-react";
import { useTranslation } from "react-i18next";

import { Badge } from "@/components/ui/badge";
import { Card } from "@/components/ui/card";
import { Progress } from "@/components/ui/progress";
import { MD_UP, useMediaQuery } from "@/hooks/useMediaQuery";
import { cn } from "@/lib/utils";
import { useWeeklyPlan, type WeeklyPlanItem } from "../../api/placement";

export interface WeeklyPlanCardProps {
  className?: string | undefined;
}

/** Rows shown before "show more": three on a phone, two full columns above. */
const MOBILE_LIMIT = 3;
const DESKTOP_LIMIT = 6;

interface Tone {
  Icon: LucideIcon;
  tile: string;
}

const skillTones: Record<string, Tone> = {
  speaking: {
    Icon: Mic,
    tile: "bg-violet-500/10 text-violet-600 dark:bg-violet-400/15 dark:text-violet-300",
  },
  vocabulary: {
    Icon: BookOpen,
    tile: "bg-emerald-500/10 text-emerald-600 dark:bg-emerald-400/15 dark:text-emerald-300",
  },
  listening: {
    Icon: Headphones,
    tile: "bg-orange-500/10 text-orange-600 dark:bg-orange-400/15 dark:text-orange-300",
  },
  reading: {
    Icon: ListChecks,
    tile: "bg-amber-500/10 text-amber-600 dark:bg-amber-400/15 dark:text-amber-300",
  },
  grammar: {
    Icon: Layers,
    tile: "bg-sky-500/10 text-sky-600 dark:bg-sky-400/15 dark:text-sky-300",
  },
  writing: {
    Icon: PenLine,
    tile: "bg-purple-500/10 text-purple-600 dark:bg-purple-400/15 dark:text-purple-300",
  },
};

const kindTones: Record<WeeklyPlanItem["kind"], Tone> = {
  lesson: {
    Icon: MessageCircle,
    tile: "bg-primary/10 text-primary-accent",
  },
  daily_practice: {
    Icon: Zap,
    tile: "bg-sky-500/10 text-sky-600 dark:bg-sky-400/15 dark:text-sky-300",
  },
  reviews: {
    Icon: Target,
    tile: "bg-rose-500/10 text-rose-600 dark:bg-rose-400/15 dark:text-rose-300",
  },
  writing: skillTones.writing!,
  speaking: skillTones.speaking!,
};

function toneFor(item: WeeklyPlanItem): Tone {
  if (item.kind === "reviews") return kindTones.reviews;
  return (item.skill && skillTones[item.skill]) || kindTones[item.kind];
}

/**
 * This week's plan: fixed for the week, with the minutes studied and the items
 * done read when the card loads. Each row opens the place the item is done.
 */
export const WeeklyPlanCard: React.FC<WeeklyPlanCardProps> = ({
  className,
}) => {
  const { t, i18n } = useTranslation();
  const [expanded, setExpanded] = useState(false);
  const isDesktop = useMediaQuery(MD_UP);
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

  const limit = isDesktop ? DESKTOP_LIMIT : MOBILE_LIMIT;
  const hiddenCount = Math.max(0, plan.items.length - limit);
  const visibleItems = expanded ? plan.items : plan.items.slice(0, limit);

  const subtitle = (item: WeeklyPlanItem): string => {
    const parts: string[] = [];
    if (item.skill) parts.push(t(`skills.${item.skill}`, item.skill));
    if (item.reviews) {
      parts.push(t("weeklyPlan.reviews", { reviews: item.reviews }));
    } else if (item.title) {
      parts.push(kindLabels[item.kind]);
    }
    return parts.join(" · ");
  };

  const rowBody = (item: WeeklyPlanItem) => {
    const { Icon, tile } = toneFor(item);
    const label = item.title ?? kindLabels[item.kind];
    const sub = subtitle(item);
    return (
      <>
        {item.done ? (
          <CheckCircle2
            className="h-5 w-5 shrink-0 text-success"
            aria-label={t("weeklyPlan.done")}
          />
        ) : (
          <Circle
            className="h-5 w-5 shrink-0 text-text-muted/60"
            aria-hidden="true"
          />
        )}
        <span
          className={cn(
            "flex h-9 w-9 shrink-0 items-center justify-center rounded-xl",
            tile,
          )}
          aria-hidden="true"
        >
          <Icon className="h-[18px] w-[18px]" />
        </span>
        <span className="min-w-0 flex-1">
          {/* Two lines before an ellipsis, and the focus tag wraps below the
            title rather than squeezing it to "Lesson …" on a phone. */}
          <span className="flex min-w-0 flex-wrap items-center gap-x-2 gap-y-0.5">
            <span
              className="line-clamp-2 min-w-0 break-words text-sm font-semibold text-text group-hover:text-primary-accent"
              title={label}
            >
              {label}
            </span>
            {item.focus && (
              <Badge variant="warning" className="shrink-0">
                {t("weeklyPlan.focus")}
              </Badge>
            )}
          </span>
          {sub && (
            <span className="block truncate text-xs text-text-muted">
              {sub}
            </span>
          )}
        </span>
        <span className="flex shrink-0 items-center gap-1 text-xs text-text-muted md:group-hover:hidden md:group-focus-visible:hidden">
          {t("weeklyPlan.itemMinutes", { minutes: item.minutes })}
          <ChevronRight className="h-4 w-4" aria-hidden="true" />
        </span>
        {!item.done && (
          <span
            className="hidden shrink-0 items-center gap-1 rounded-full bg-primary px-3 py-1.5 text-xs font-semibold text-primary-fg md:group-hover:inline-flex md:group-focus-visible:inline-flex"
            aria-hidden="true"
          >
            {t("weeklyPlan.start", "Start")}
            <ArrowRight className="h-4 w-4" />
          </span>
        )}
      </>
    );
  };

  const rowClass =
    "group flex min-h-[56px] w-full items-center gap-3 rounded-xl border border-transparent px-2 py-2 transition-colors hover:border-primary/30 hover:bg-primary/5 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary motion-reduce:transition-none";

  const renderRow = (item: WeeklyPlanItem) => {
    const body = rowBody(item);
    // The row's name is the item, not every piece of text inside it.
    const label = item.title ?? kindLabels[item.kind];
    const name = item.done ? `${label}, ${t("weeklyPlan.done")}` : label;
    if (item.lesson_id) {
      return (
        <Link
          to="/learn/lesson/$lessonId"
          params={{ lessonId: item.lesson_id }}
          aria-label={name}
          className={rowClass}
        >
          {body}
        </Link>
      );
    }
    const to =
      item.kind === "daily_practice"
        ? "/practice/daily"
        : item.kind === "reviews"
          ? "/practice/review"
          : item.kind === "writing"
            ? "/my-writing"
            : item.kind === "speaking"
              ? "/my-speaking"
              : null;
    if (to) {
      return (
        <Link to={to} aria-label={name} className={rowClass}>
          {body}
        </Link>
      );
    }
    return <div className={cn(rowClass, "hover:bg-transparent")}>{body}</div>;
  };

  return (
    <Card className={cn("@container h-full p-4 md:p-5", className)}>
      <div className="space-y-3">
        <div className="flex flex-wrap items-center justify-between gap-x-3 gap-y-1">
          <div className="flex items-center gap-2">
            <CalendarDays
              className="h-5 w-5 text-primary-accent"
              aria-hidden="true"
            />
            <h2 className="text-base font-bold tracking-tight text-text md:text-lg">
              {t("weeklyPlan.title", "This week's plan")}
            </h2>
          </div>
          <span className="text-xs text-text-muted sm:text-sm">
            {t("weeklyPlan.weekOf", { date: weekOf })}
          </span>
        </div>
        <div className="space-y-2">
          <div className="flex flex-wrap justify-between gap-2 text-sm">
            <span className="font-semibold text-text">
              {t("weeklyPlan.minutes", {
                done: plan.progress.minutes,
                goal: plan.minutes_goal,
              })}
            </span>
            <span className="font-medium text-text-muted">
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

      <ul className="mt-3 grid grid-cols-1 gap-x-5 gap-y-1 @3xl:grid-cols-2 @3xl:[&>li:nth-child(odd)]:border-r @3xl:[&>li:nth-child(odd)]:border-border-subtle @3xl:[&>li:nth-child(odd)]:pr-5">
        {visibleItems.map((item, index) => (
          <li key={`${item.kind}-${item.lesson_id ?? index}`}>
            {renderRow(item)}
          </li>
        ))}
      </ul>

      {hiddenCount > 0 && (
        <div className="mt-3 flex justify-center md:justify-end">
          <button
            type="button"
            aria-expanded={expanded}
            onClick={() => setExpanded((value) => !value)}
            className="inline-flex min-h-[44px] items-center gap-1 rounded-full border border-border-subtle px-4 text-sm font-medium text-primary-accent transition-colors hover:bg-primary/5 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary md:border-transparent md:px-2"
          >
            {expanded
              ? t("weeklyPlan.showLess", "Show less")
              : t("weeklyPlan.showMore", {
                  count: hiddenCount,
                  defaultValue: `Show ${hiddenCount} more`,
                })}
            <ChevronDown
              className={cn(
                "h-4 w-4 transition-transform motion-reduce:transition-none",
                expanded && "rotate-180",
              )}
              aria-hidden="true"
            />
          </button>
        </div>
      )}
    </Card>
  );
};
