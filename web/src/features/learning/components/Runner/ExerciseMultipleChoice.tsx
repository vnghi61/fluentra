import React, { useEffect, useState } from "react";
import { Check, X } from "lucide-react";
import { useTranslation } from "react-i18next";

import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";

import { type AnswerExplanation } from "./ExerciseShell";

export interface OptionItem {
  id: string;
  text: string;
}

export interface ExerciseMultipleChoiceProps {
  prompt: string;
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

export const ExerciseMultipleChoice: React.FC<ExerciseMultipleChoiceProps> = ({
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

  // Keyboard shortcut listener for options 1-4 and Enter to submit/continue
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      // Ignore if user typing in an input
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

  return (
    <div className="space-y-8 max-w-2xl mx-auto py-4">
      {/* Question Prompt */}
      <div className="space-y-2">
        <h2 className="text-xl md:text-2xl font-bold text-text leading-snug">
          {prompt}
        </h2>
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
              "border-primary bg-primary/10 text-text ring-2 ring-primary font-medium";
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
                "w-full flex items-center justify-between p-4 rounded-xl border transition-all text-left min-h-[56px] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary cursor-pointer disabled:cursor-default",
                optionStyle,
              )}
            >
              <div className="flex items-center gap-3">
                <span className="flex items-center justify-center h-7 w-7 rounded-lg border border-border bg-surface-muted text-xs font-bold text-text-muted shrink-0">
                  {index + 1}
                </span>
                <span className="text-base">{option.text}</span>
              </div>

              {isSubmitted && (
                <div>
                  {isSelectedCorrect || isAnswer ? (
                    <Check
                      className="h-5 w-5 text-success"
                      aria-label={t("runner.markCorrect", "Correct")}
                    />
                  ) : isSelectedWrong ? (
                    <X
                      className="h-5 w-5 text-danger"
                      aria-label={t("runner.markIncorrect", "Incorrect")}
                    />
                  ) : null}
                </div>
              )}
            </button>
          );
        })}
      </div>

      {/* Evaluation Feedback */}
      {isSubmitted && (
        <div
          role="status"
          className={cn(
            "p-4 rounded-xl border animate-in fade-in duration-200",
            isCorrect
              ? "border-success/30 bg-success/10 text-success-accent"
              : "border-danger/30 bg-danger/10 text-danger-accent",
          )}
        >
          <div className="flex items-center gap-2 font-bold text-base">
            {isCorrect ? (
              <>
                <Check className="h-5 w-5" aria-hidden="true" />
                <span>{t("runner.correct", "Correct! Well done.")}</span>
              </>
            ) : (
              <>
                <X className="h-5 w-5" aria-hidden="true" />
                <span>
                  {t(
                    "runner.incorrect",
                    "Not quite. Review the correct option above.",
                  )}
                </span>
              </>
            )}
          </div>
          {feedback && (
            <p className="mt-1 text-sm text-text font-normal">{feedback}</p>
          )}
          {explanation && (
            <div className="mt-3 pt-3 border-t border-border/40 space-y-2 text-sm">
              <div className="flex items-start gap-2">
                <span className="inline-flex items-center px-1.5 py-0.5 rounded text-[10px] font-semibold uppercase tracking-wider bg-primary/10 text-primary shrink-0 mt-0.5">
                  EN
                </span>
                <p className="text-text/90 font-medium leading-relaxed">{explanation.text}</p>
              </div>
              <div className="flex items-start gap-2">
                <span className="inline-flex items-center px-1.5 py-0.5 rounded text-[10px] font-semibold uppercase tracking-wider bg-accent/20 text-accent shrink-0 mt-0.5">
                  VI
                </span>
                <p className="text-text-muted leading-relaxed">{explanation.text_vi}</p>
              </div>
            </div>
          )}
        </div>
      )}

      {/* Action CTA Bar */}
      <div className="pt-4 flex justify-end">
        {!isSubmitted ? (
          <Button
            size="lg"
            disabled={!selectedId || isLoading}
            isLoading={isLoading}
            onClick={() => selectedId && onSubmit(selectedId)}
            className="w-full sm:w-auto min-w-[160px] font-bold"
          >
            {t("runner.checkBtn", "Check Answer")}
          </Button>
        ) : (
          <Button
            size="lg"
            onClick={onContinue}
            className="w-full sm:w-auto min-w-[160px] font-bold"
          >
            {t("runner.continueBtn", "Continue")}
          </Button>
        )}
      </div>
    </div>
  );
};
