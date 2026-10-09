import type { TFunction } from "i18next";

const MINUTE = 60_000;

/**
 * "10 min", "3 d" — how far away a due time is.
 *
 * The labels under the grade buttons used to be a fixed "1 day / 3 days / 7
 * days / 14 days" that no schedule ever produced: `again` brings a card back in
 * ten minutes, and the others grow with every review. The server schedules each
 * grade and this only formats the answer.
 */
export function untilLabel(t: TFunction, dueAt: string, now: number): string {
  const minutes = Math.max(1, Math.round((Date.parse(dueAt) - now) / MINUTE));
  if (minutes < 60) return t("review.inMinutes", { count: minutes });
  const hours = Math.round(minutes / 60);
  if (hours < 24) return t("review.inHours", { count: hours });
  const days = Math.round(hours / 24);
  if (days < 31) return t("review.inDays", { count: days });
  const months = Math.round(days / 30);
  if (months < 12) return t("review.inMonths", { count: months });
  return t("review.inYears", { count: Math.round(days / 365) });
}
