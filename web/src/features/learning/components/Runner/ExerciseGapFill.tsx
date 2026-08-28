import React, { useEffect, useRef, useState } from "react";
import { Check, X } from "lucide-react";
import { useTranslation } from "react-i18next";

import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { cn } from "@/lib/utils";

export interface ExerciseGapFillProps {
  prompt: string;
  sentenceBeforeBlank?: string | undefined;
  sentenceAfterBlank?: string | undefined;
  expectedAnswer?: string | null | undefined;
  feedback?: string | null | undefined;
  isSubmitted: boolean;
  isCorrect?: boolean | null | undefined;
  isLoading?: boolean;
  onSubmit: (answerText: string) => void;
  onContinue: () => void;
}

export const ExerciseGapFill: React.FC<ExerciseGapFillProps> = ({
  prompt,
  sentenceBeforeBlank = "",
  sentenceAfterBlank = "",
  expectedAnswer,
  feedback,
  isSubmitted,
  isCorrect,
  isLoading = false,
  onSubmit,
  onContinue,
}) => {
  const { t } = useTranslation();
  const [answer, setAnswer] = useState("");
  const inputRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    if (!isSubmitted && inputRef.current) {
      inputRef.current.focus();
    }
  }, [isSubmitted]);

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    if (isSubmitted) {
      onContinue();
    } else if (answer.trim() && !isLoading) {
      onSubmit(answer.trim());
    }
  };

  return (
    <form onSubmit={handleSubmit} className="space-y-8 max-w-2xl mx-auto py-4">
      {/* Prompt Heading */}
      <div className="space-y-2">
        <h2 className="text-xl md:text-2xl font-bold text-text leading-snug">
          {prompt}
        </h2>
      </div>

      {/* Sentence with Gap Input */}
      <div className="p-6 rounded-2xl border border-border bg-surface-card shadow-sm space-y-4">
        <div className="text-lg md:text-xl leading-relaxed text-text">
          <span>{sentenceBeforeBlank} </span>
          <span className="inline-block align-baseline">
            <Input
              ref={inputRef}
              type="text"
              value={answer}
              disabled={isSubmitted || isLoading}
              onChange={(e) => setAnswer(e.target.value)}
              placeholder={t(
                "runner.gapFillPlaceholder",
                "Type your answer...",
              )}
              className={cn(
                "w-48 sm:w-64 inline-block font-bold text-base h-11 border-2 focus-visible:ring-primary",
                isSubmitted &&
                  isCorrect &&
                  "border-success bg-success/10 text-success-accent",
                isSubmitted &&
                  !isCorrect &&
                  "border-danger bg-danger/10 text-danger-accent",
              )}
            />
          </span>
          <span> {sentenceAfterBlank}</span>
        </div>
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
                <span>{t("runner.incorrect", "Not quite.")}</span>
              </>
            )}
          </div>
          {!isCorrect && expectedAnswer && (
            <p className="mt-1 text-sm text-text font-medium">
              Correct answer:{" "}
              <span className="font-bold underline">{expectedAnswer}</span>
            </p>
          )}
          {feedback && (
            <p className="mt-1 text-sm text-text font-normal">{feedback}</p>
          )}
        </div>
      )}

      {/* Action CTA Bar */}
      <div className="pt-4 flex justify-end">
        {!isSubmitted ? (
          <Button
            type="submit"
            size="lg"
            disabled={!answer.trim() || isLoading}
            isLoading={isLoading}
            className="w-full sm:w-auto min-w-[160px] font-bold"
          >
            {t("runner.checkBtn", "Check Answer")}
          </Button>
        ) : (
          <Button
            type="button"
            size="lg"
            onClick={onContinue}
            className="w-full sm:w-auto min-w-[160px] font-bold"
          >
            {t("runner.continueBtn", "Continue")}
          </Button>
        )}
      </div>
    </form>
  );
};
