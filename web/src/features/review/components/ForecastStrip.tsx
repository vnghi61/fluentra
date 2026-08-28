import React from "react";
import { useTranslation } from "react-i18next";

import {
  Card,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import type { ForecastItem } from "../api/reviewApi";

export interface ForecastStripProps {
  days: ForecastItem[];
  /** How many days to draw. The endpoint returns 30; a week reads better. */
  window?: number;
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
    <Card>
      <CardHeader>
        <CardTitle className="text-base font-semibold">
          {t("practice.forecast.title", "The week ahead")}
        </CardTitle>
        <CardDescription>
          {t(
            "practice.forecast.desc",
            "Cards already scheduled to come back to you.",
          )}
        </CardDescription>
      </CardHeader>

      {/* Scrolls inside itself: seven columns do not fit a 320px viewport. */}
      <div className="overflow-x-auto px-6 pb-6">
        <ul className="flex items-end gap-3 min-w-max">
          {visible.map((day) => {
            const date = localDate(day.date);
            // A zero-count day still gets a visible baseline, so the row reads
            // as a calendar rather than as gaps.
            const height =
              busiest === 0 ? 4 : Math.max(4, (day.due_count / busiest) * 64);
            return (
              <li
                key={day.date}
                className="flex flex-col items-center gap-1.5 w-12"
              >
                <span className="text-xs font-semibold text-text tabular-nums">
                  {day.due_count}
                </span>
                <div
                  className={
                    day.due_count > 0
                      ? "w-8 rounded-t bg-primary"
                      : "w-8 rounded-t bg-border-subtle"
                  }
                  style={{ height: `${height}px` }}
                  aria-hidden="true"
                />
                <span
                  className="text-xs text-text-muted"
                  title={date ? full.format(date) : day.date}
                >
                  {date ? weekday.format(date) : day.date}
                </span>
              </li>
            );
          })}
        </ul>
      </div>
    </Card>
  );
};
