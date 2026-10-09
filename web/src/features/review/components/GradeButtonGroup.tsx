import React, { useEffect, useState } from "react";
import { Loader2 } from "lucide-react";
import { useTranslation } from "react-i18next";

import { cn } from "@/lib/utils";
import type { ReviewGrade } from "../api/reviewApi";
import { untilLabel } from "../model/interval";
import { GRADE_SHORTCUTS, isForeignKeyEvent } from "../model/shortcuts";

export type NextDueByGrade = Record<ReviewGrade, string>;

export interface GradeButtonGroupProps {
  onGrade: (grade: ReviewGrade) => void;
  disabled?: boolean;
  /** When the card would come back under each grade, as the server scheduled it. */
  nextDue?: NextDueByGrade | undefined;
  /** The grade on its way to the server, drawn as pressed with a spinner. */
  pendingGrade?: ReviewGrade | null | undefined;
  className?: string;
}

const grades: {
  grade: ReviewGrade;
  labelKey: string;
  digit: string;
  tone: string;
  pressed: string;
}[] = [
  {
    grade: "again",
    labelKey: "review.again",
    digit: "1",
    tone: "border-danger/30 text-danger-accent hover:border-danger hover:bg-danger/10",
    pressed: "border-danger bg-danger/10",
  },
  {
    grade: "hard",
    labelKey: "review.hard",
    digit: "2",
    tone: "border-warning/30 text-warning-accent hover:border-warning hover:bg-warning/10",
    pressed: "border-warning bg-warning/10",
  },
  {
    grade: "good",
    labelKey: "review.good",
    digit: "3",
    tone: "border-primary/30 text-primary-accent hover:border-primary hover:bg-primary/10",
    pressed: "border-primary bg-primary/10",
  },
  {
    grade: "easy",
    labelKey: "review.easy",
    digit: "4",
    tone: "border-success/30 text-success-accent hover:border-success hover:bg-success/10",
    pressed: "border-success bg-success/10",
  },
];

export const GradeButtonGroup: React.FC<GradeButtonGroupProps> = ({
  onGrade,
  disabled = false,
  nextDue,
  pendingGrade,
  className,
}) => {
  const { t } = useTranslation();

  // 1 -> again, 2 -> hard, 3 -> good, 4 -> easy. The listener lives here, and
  // this group mounts only once the card is flipped, so a digit cannot grade a
  // card whose answer the learner has not seen.
  useEffect(() => {
    const handleKeyDown = (event: KeyboardEvent) => {
      if (disabled || isForeignKeyEvent(event)) return;
      const grade = GRADE_SHORTCUTS[event.key];
      if (!grade) return;
      event.preventDefault();
      onGrade(grade);
    };

    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, [disabled, onGrade]);

  // Read once when the buttons appear. They mount when a card is flipped and
  // unmount when it is graded, so every card measures from its own flip.
  const [now] = useState(() => Date.now());

  return (
    <div className={cn("w-full space-y-2", className)}>
      <p className="text-center text-sm font-medium text-text">
        {t("review.gradePromptShort")}
        <span className="hidden font-normal text-text-muted md:inline">
          {" "}
          {t("review.gradePromptDetail")}
        </span>
      </p>
      <div
        role="group"
        aria-label={t("review.srsRecallGrading", "SRS recall grading")}
        className="grid grid-cols-2 gap-2 sm:grid-cols-4 sm:gap-3"
      >
        {grades.map(({ grade, labelKey, digit, tone, pressed }) => {
          const isPending = pendingGrade === grade;
          return (
            <button
              key={grade}
              type="button"
              disabled={disabled}
              aria-busy={isPending || undefined}
              aria-keyshortcuts={digit}
              onClick={() => onGrade(grade)}
              className={cn(
                "relative flex min-h-16 select-none flex-col items-center justify-center gap-0.5 rounded-xl border-2 bg-surface-card px-2 py-2.5 text-center transition-colors duration-150 cursor-pointer",
                "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary focus-visible:ring-offset-2 focus-visible:ring-offset-surface",
                "disabled:cursor-not-allowed motion-reduce:transition-none",
                tone,
                isPending ? pressed : "disabled:opacity-50",
              )}
            >
              <span className="flex items-center gap-1.5 text-sm font-semibold sm:text-base">
                {isPending && (
                  <Loader2
                    className="h-4 w-4 animate-spin motion-reduce:animate-none"
                    aria-hidden="true"
                  />
                )}
                {t(labelKey)}
              </span>
              {nextDue && (
                <span className="text-xs font-medium text-text-muted">
                  {t("review.reviewIn", {
                    when: untilLabel(t, nextDue[grade], now),
                  })}
                </span>
              )}
              {/* The shortcut, for a keyboard; a phone has none to press. */}
              <kbd
                aria-hidden="true"
                className="absolute right-1.5 top-1.5 hidden min-w-5 items-center justify-center rounded-md border border-border-subtle bg-surface-muted px-1 font-mono text-xs leading-5 text-text-muted md:inline-flex"
              >
                {digit}
              </kbd>
            </button>
          );
        })}
      </div>
    </div>
  );
};
