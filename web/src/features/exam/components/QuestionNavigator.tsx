import React, { useState } from "react";
import { Check } from "lucide-react";
import { useTranslation } from "react-i18next";
import { cn } from "@/lib/utils";
import type { ExamSkill, SectionActivities } from "../types";
import type { QuestionSlot } from "../parts";

export function QuestionNavigator({
  slots,
  sections,
  sectionLabels,
  currentSection,
  activeQuestionNumber,
  isExamMode,
  onJump,
}: {
  slots: QuestionSlot[];
  sections: SectionActivities[];
  sectionLabels: Record<ExamSkill, string>;
  currentSection: number;
  activeQuestionNumber: number | null;
  isExamMode: boolean;
  onJump: (slot: QuestionSlot) => void;
}): React.JSX.Element {
  const { t } = useTranslation();
  const [filter, setFilter] = useState<"all" | "unanswered" | "answered">(
    "all",
  );
  const tabs = [
    ["all", t("exam.parts.questionFilterAll")],
    ["unanswered", t("exam.parts.questionFilterUnanswered")],
    ["answered", t("exam.parts.questionFilterAnswered")],
  ] as const;
  const visible = slots.filter(
    (slot) => filter === "all" || slot.answered === (filter === "answered"),
  );
  return (
    <div className="space-y-4">
      <h2 className="text-base font-semibold leading-snug text-text">
        {t("exam.parts.questionList")}
      </h2>
      <div
        className="flex flex-wrap gap-2"
        role="group"
        aria-label={t("exam.parts.questionList")}
      >
        {tabs.map(([key, label]) => (
          <button
            key={key}
            type="button"
            aria-pressed={filter === key}
            onClick={() => setFilter(key)}
            className={cn(
              "min-h-11 rounded-lg px-3 text-sm font-semibold",
              filter === key
                ? "bg-primary text-primary-fg"
                : "text-text-muted hover:bg-surface-muted",
            )}
          >
            {label}
          </button>
        ))}
      </div>
      {visible.length === 0 && (
        <p className="text-sm text-text-muted">
          {t("exam.parts.noQuestionsInFilter")}
        </p>
      )}
      {sections.map((section) => {
        const sectionSlots = visible.filter(
          (slot) => slot.section === section.section_position,
        );
        if (!sectionSlots.length) return null;
        const locked =
          isExamMode && section.section_position !== currentSection;
        const currentNumber =
          activeQuestionNumber ??
          slots.find((slot) => slot.section === currentSection)?.number;
        return (
          <section key={section.section_position} className="space-y-2">
            <h3 className="text-xs font-semibold uppercase tracking-wider text-text-muted">
              {sectionLabels[section.skill]}
            </h3>
            <div className="flex flex-wrap gap-2">
              {sectionSlots.map((slot) => (
                <button
                  key={slot.number}
                  type="button"
                  disabled={locked}
                  aria-current={
                    slot.section === currentSection &&
                    slot.number === currentNumber
                      ? "true"
                      : undefined
                  }
                  aria-label={t("exam.parts.goToQuestion", {
                    num: slot.number,
                  })}
                  onClick={() => onJump(slot)}
                  className={cn(
                    "flex min-h-11 min-w-11 items-center justify-center gap-1 rounded-lg border px-2 font-mono text-sm font-semibold",
                    slot.section === currentSection &&
                      slot.number === currentNumber
                      ? "border-primary bg-primary text-primary-fg"
                      : slot.answered
                        ? "border-primary/20 bg-primary/10 text-primary-accent"
                        : "border-border bg-surface-card text-text",
                    locked && "cursor-not-allowed opacity-50",
                  )}
                >
                  {slot.number}
                  {slot.answered && (
                    <>
                      <Check className="h-4 w-4" aria-hidden="true" />
                      <span className="sr-only">
                        {t("exam.parts.questionFilterAnswered")}
                      </span>
                    </>
                  )}
                </button>
              ))}
            </div>
          </section>
        );
      })}
    </div>
  );
}
