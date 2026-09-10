import React, { useEffect, useState } from "react";
import { BookOpen, Check, X } from "lucide-react";
import { useTranslation } from "react-i18next";

import { cn } from "@/lib/utils";

import {
  type AnswerExplanation,
  ExerciseActions,
  ExerciseFeedback,
  ExercisePrompt,
} from "./ExerciseShell";
import type { OptionItem } from "./ExerciseMultipleChoice";

export interface ExerciseReadingProps {
  passageTitle?: string | undefined;
  /** The reading passage the learner reads. */
  passage: string;
  /** The comprehension question prompt. */
  prompt: string;
  /** The multiple choice options to answer the prompt. */
  options: OptionItem[];
  correctOptionId?: string | null | undefined;
  feedback?: string | null | undefined;
  isSubmitted: boolean;
  isCorrect?: boolean | null | undefined;
  isLoading?: boolean;
  explanation?: AnswerExplanation | null | undefined;
  onSubmit: (selectedOptionId: string) => void;
  onContinue: () => void;
}

/**
 * `reading_comprehension` — answer questions based on an authored passage.
 *
 * Graded synchronously from an authored answer key (no AI call).
 * Displays a styled passage card and multiple choice questions with touch-friendly targets.
 */
export const ExerciseReading: React.FC<ExerciseReadingProps> = ({
  passageTitle,
  passage,
  prompt,
  options,
  correctOptionId,
  feedback,
  isSubmitted,
  isCorrect,
  isLoading = false,
  explanation,
  onSubmit,
  onContinue,
}) => {
  const { t } = useTranslation();
  const [selectedId, setSelectedId] = useState<string | null>(null);

  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if (
        e.target instanceof HTMLInputElement ||
        e.target instanceof HTMLTextAreaElement
      ) {
        return;
      }

      if (!isSubmitted) {
        const digit = parseInt(e.key, 10);
        if (digit >= 1 && digit <= options.length) {
          e.preventDefault();
          setSelectedId(options[digit - 1]?.id || null);
        } else if (e.key === "Enter" && selectedId && !isLoading) {
          e.preventDefault();
          onSubmit(selectedId);
        }
      } else if (e.key === "Enter" || e.key === " ") {
        e.preventDefault();
        onContinue();
      }
    };

    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, [options, selectedId, isSubmitted, isLoading, onSubmit, onContinue]);

  const handleSubmit = () => {
    if (selectedId && !isLoading) {
      onSubmit(selectedId);
    }
  };

  return (
    <div className="space-y-6 max-w-2xl mx-auto py-4">
      {/* Passage Card */}
      <div className="rounded-2xl border border-border bg-surface-card shadow-sm overflow-hidden">
        <div className="px-5 py-3 border-b border-border/60 bg-surface/40 flex items-center gap-2">
          <BookOpen
            className="h-4 w-4 text-primary shrink-0"
            aria-hidden="true"
          />
          <h3 className="text-sm font-semibold text-text uppercase tracking-wider">
            {passageTitle || t("runner.readingPassage", "Reading Passage")}
          </h3>
        </div>
        <div className="p-5 md:p-6">
          <p className="text-base md:text-lg leading-relaxed text-text font-normal whitespace-pre-line">
            {passage}
          </p>
        </div>
      </div>

      {/* Comprehension Question Prompt */}
      <div className="pt-2">
        <ExercisePrompt>{prompt}</ExercisePrompt>
      </div>

      {/* Options List */}
      <div className="space-y-3" role="radiogroup" aria-label={prompt}>
        {options.map((option, index) => {
          const isSelected = selectedId === option.id;
          const isAnswer = correctOptionId === option.id;
          const isSelectedWrong = isSubmitted && isSelected && !isCorrect;
          const isSelectedCorrect = isSubmitted && isSelected && isCorrect;

          let optionStyle =
            "border-border bg-surface-card hover:border-primary/50 text-text";
          if (isSubmitted) {
            if (isSelectedCorrect || isAnswer) {
              optionStyle =
                "border-success bg-success/10 text-text font-semibold";
            } else if (isSelectedWrong) {
              optionStyle =
                "border-danger bg-danger/10 text-text line-through opacity-80";
            } else {
              optionStyle = "border-border opacity-50 text-text-muted";
            }
          } else if (isSelected) {
            optionStyle =
              "border-primary bg-primary/10 text-text font-semibold ring-1 ring-primary";
          }

          return (
            <button
              key={option.id}
              type="button"
              role="radio"
              aria-checked={isSelected}
              disabled={isSubmitted || isLoading}
              onClick={() => setSelectedId(option.id)}
              className={cn(
                "w-full text-left p-4 rounded-xl border transition-all duration-150 flex items-center justify-between group cursor-pointer min-h-[44px] text-base",
                optionStyle,
                isSubmitted && "cursor-default",
              )}
            >
              <div className="flex items-center gap-3">
                <span
                  className={cn(
                    "flex items-center justify-center w-7 h-7 rounded-lg text-xs font-bold border transition-colors shrink-0",
                    isSelected && !isSubmitted
                      ? "bg-primary text-primary-foreground border-primary"
                      : "bg-surface text-text-muted border-border group-hover:border-primary/40",
                    isSubmitted &&
                      (isSelectedCorrect || isAnswer) &&
                      "bg-success text-success-foreground border-success",
                    isSubmitted &&
                      isSelectedWrong &&
                      "bg-danger text-danger-foreground border-danger",
                  )}
                >
                  {index + 1}
                </span>
                <span className="leading-snug">{option.text}</span>
              </div>

              {isSubmitted && (isSelectedCorrect || isAnswer) && (
                <Check
                  className="h-5 w-5 text-success shrink-0 ml-2"
                  aria-hidden="true"
                />
              )}
              {isSubmitted && isSelectedWrong && (
                <X
                  className="h-5 w-5 text-danger shrink-0 ml-2"
                  aria-hidden="true"
                />
              )}
            </button>
          );
        })}
      </div>

      {/* Feedback Panel */}
      {isSubmitted && (
        <ExerciseFeedback
          isCorrect={isCorrect}
          feedback={feedback}
          explanation={explanation}
        />
      )}

      {/* Action Bar */}
      <ExerciseActions
        isSubmitted={isSubmitted}
        canSubmit={selectedId !== null}
        isLoading={isLoading}
        onSubmit={handleSubmit}
        onContinue={onContinue}
      />
    </div>
  );
};
