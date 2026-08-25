import React, { useEffect } from "react";
import { useTranslation } from "react-i18next";

import { cn } from "@/lib/utils";
import type { ReviewGrade } from "../api/reviewApi";

export interface GradeButtonGroupProps {
  onGrade: (grade: ReviewGrade) => void;
  disabled?: boolean;
}

export const GradeButtonGroup: React.FC<GradeButtonGroupProps> = ({
  onGrade,
  disabled = false,
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
    subKey: string;
    digit: string;
    colorClass: string;
  }[] = [
    {
      grade: "again",
      labelKey: "review.again",
      subKey: "review.againSub",
      digit: "1",
      colorClass:
        "border-danger/30 hover:border-danger hover:bg-danger/10 text-danger-accent",
    },
    {
      grade: "hard",
      labelKey: "review.hard",
      subKey: "review.hardSub",
      digit: "2",
      colorClass:
        "border-warning/30 hover:border-warning hover:bg-warning/10 text-warning-accent",
    },
    {
      grade: "good",
      labelKey: "review.good",
      subKey: "review.goodSub",
      digit: "3",
      colorClass:
        "border-primary/30 hover:border-primary hover:bg-primary/10 text-primary-accent",
    },
    {
      grade: "easy",
      labelKey: "review.easy",
      subKey: "review.easySub",
      digit: "4",
      colorClass:
        "border-success/30 hover:border-success hover:bg-success/10 text-success-accent",
    },
  ];

  return (
    <div
      role="group"
      aria-label="SRS recall grading"
      className="grid grid-cols-2 sm:grid-cols-4 gap-2 sm:gap-3 w-full max-w-2xl mx-auto pt-2"
    >
      {grades.map(({ grade, labelKey, subKey, digit, colorClass }) => (
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
            <span>{t(labelKey, grade.charAt(0).toUpperCase() + grade.slice(1))}</span>
          </div>
          <span className="text-[11px] text-text-muted font-medium mt-0.5">
            {t(subKey, "")}
          </span>
        </button>
      ))}
    </div>
  );
};
