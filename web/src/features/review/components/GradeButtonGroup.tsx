import React, { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import type { TFunction } from "i18next";

import { cn } from "@/lib/utils";
import type { ReviewGrade } from "../api/reviewApi";

export type NextDueByGrade = Record<ReviewGrade, string>;

export interface GradeButtonGroupProps {
  onGrade: (grade: ReviewGrade) => void;
  disabled?: boolean;
  /** When the card would come back under each grade, as the server scheduled it. */
  nextDue?: NextDueByGrade | undefined;
}

const MINUTE = 60_000;

/**
 * "10 min", "3 d" — how far away a due time is.
 *
 * The labels under the buttons used to be a fixed "1 day / 3 days / 7 days / 14
 * days" that no schedule ever produced: `again` brings a card back in ten
 * minutes, and the others grow with every review. The server schedules each
 * grade and this only formats the answer.
 */
function untilLabel(t: TFunction, dueAt: string, now: number): string {
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

export const GradeButtonGroup: React.FC<GradeButtonGroupProps> = ({
  onGrade,
  disabled = false,
  nextDue,
}) => {
  const { t } = useTranslation();

  // Keyboard shortcut listener: 1 -> again, 2 -> hard, 3 -> good, 4 -> easy
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if (disabled) return;
      if (
        e.target instanceof HTMLInputElement ||
        e.target instanceof HTMLTextAreaElement
      ) {
        return;
      }

      switch (e.key) {
        case "1":
          e.preventDefault();
          onGrade("again");
          break;
        case "2":
          e.preventDefault();
          onGrade("hard");
          break;
        case "3":
          e.preventDefault();
          onGrade("good");
          break;
        case "4":
          e.preventDefault();
          onGrade("easy");
          break;
      }
    };

    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, [disabled, onGrade]);

  const grades: {
    grade: ReviewGrade;
    labelKey: string;
    digit: string;
    colorClass: string;
  }[] = [
    {
      grade: "again",
      labelKey: "review.again",
      digit: "1",
      colorClass:
        "border-danger/30 hover:border-danger hover:bg-danger/10 text-danger-accent",
    },
    {
      grade: "hard",
      labelKey: "review.hard",
      digit: "2",
      colorClass:
        "border-warning/30 hover:border-warning hover:bg-warning/10 text-warning-accent",
    },
    {
      grade: "good",
      labelKey: "review.good",
      digit: "3",
      colorClass:
        "border-primary/30 hover:border-primary hover:bg-primary/10 text-primary-accent",
    },
    {
      grade: "easy",
      labelKey: "review.easy",
      digit: "4",
      colorClass:
        "border-success/30 hover:border-success hover:bg-success/10 text-success-accent",
    },
  ];

  // Read once when the buttons appear. They mount when a card is flipped and
  // unmount when it is graded, so every card measures from its own flip.
  const [now] = useState(() => Date.now());

  return (
    <div className="w-full max-w-2xl mx-auto space-y-2 pt-2">
      <p className="text-center text-sm text-text-muted">
        {t("review.gradePrompt")}
      </p>
      <div
        role="group"
        aria-label={t("review.srsRecallGrading", "SRS recall grading")}
        className="grid grid-cols-2 sm:grid-cols-4 gap-2 sm:gap-3"
      >
        {grades.map(({ grade, labelKey, digit, colorClass }) => (
          <button
            key={grade}
            type="button"
            disabled={disabled}
            onClick={() => onGrade(grade)}
            className={cn(
              "flex flex-col items-center justify-center p-3 rounded-2xl border-2 bg-surface-card transition-all min-h-[56px] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary select-none cursor-pointer disabled:opacity-50 disabled:cursor-not-allowed",
              colorClass,
            )}
          >
            <div className="flex items-center gap-1.5 font-bold text-sm sm:text-base">
              <span className="text-xs opacity-75 font-mono">[{digit}]</span>
              <span>{t(labelKey)}</span>
            </div>
            {nextDue && (
              <span className="text-[11px] text-text-muted font-medium mt-0.5">
                {t("review.reviewIn", {
                  when: untilLabel(t, nextDue[grade], now),
                })}
              </span>
            )}
          </button>
        ))}
      </div>
    </div>
  );
};
