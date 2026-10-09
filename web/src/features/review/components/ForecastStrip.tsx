import React from "react";
import { useTranslation } from "react-i18next";

import {
  Card,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { cn } from "@/lib/utils";
import type { ForecastItem } from "../api/reviewApi";

export interface ForecastStripProps {
  days: ForecastItem[];
  /** How many days to draw. The endpoint returns 30; a week reads better. */
  window?: number;
  className?: string;
}

/**
 * `YYYY-MM-DD` as a date in the reader's own timezone.
 *
 * `new Date("2026-08-28")` is parsed as UTC midnight, which renders as the 27th
 * for anyone west of Greenwich. The forecast is a calendar of the learner's
 * days, so it is built from the parts rather than from the string.
 */
function localDate(iso: string): Date | null {
  const match = /^(\d{4})-(\d{2})-(\d{2})$/.exec(iso);
  if (!match) return null;
  return new Date(Number(match[1]), Number(match[2]) - 1, Number(match[3]));
}

export const ForecastStrip: React.FC<ForecastStripProps> = ({
  days,
  window = 7,
  className,
}) => {
  const { t, i18n } = useTranslation();
  const visible = days.slice(0, window);
  const busiest = visible.reduce((max, day) => Math.max(max, day.due_count), 0);

  const weekday = new Intl.DateTimeFormat(i18n.language, { weekday: "short" });
  const full = new Intl.DateTimeFormat(i18n.language, { dateStyle: "long" });

  if (visible.length === 0) {
    return null;
  }

  return (
    <Card className={cn("flex flex-col p-5 md:p-6", className)}>
      <CardHeader className="space-y-1 p-0">
        <CardTitle className="text-xs font-semibold uppercase tracking-wider text-primary-accent">
          {t("practice.forecast.title", "The week ahead")}
        </CardTitle>
        <CardDescription>
          {t(
            "practice.forecast.desc",
            "Cards already scheduled to come back to you.",
          )}
        </CardDescription>
      </CardHeader>

      <ul className="mt-auto grid grid-cols-7 items-end gap-1.5 pt-5">
        {visible.map((day) => {
          const date = localDate(day.date);
          // A zero-count day still gets a visible baseline, so the row reads
          // as a calendar rather than as gaps.
          const height =
            busiest === 0 ? 4 : Math.max(4, (day.due_count / busiest) * 72);
          return (
            <li
              key={day.date}
              className="flex min-w-0 flex-col items-center gap-1.5"
            >
              <span className="text-xs font-semibold tabular-nums text-text">
                {day.due_count}
              </span>
              <div
                className={cn(
                  "w-full max-w-8 rounded-md",
                  day.due_count > 0 ? "bg-primary" : "bg-border-subtle",
                )}
                style={{ height: `${height}px` }}
                aria-hidden="true"
              />
              <span
                className="w-full truncate text-center text-xs text-text-muted"
                title={date ? full.format(date) : day.date}
              >
                {date ? weekday.format(date) : day.date}
              </span>
            </li>
          );
        })}
      </ul>
    </Card>
  );
};
