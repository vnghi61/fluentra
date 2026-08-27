import React, { useEffect, useState } from "react";
import { ArrowRight, Check, RotateCw, X } from "lucide-react";
import { useTranslation } from "react-i18next";

import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";

export interface ExerciseFlashcardProps {
  prompt: string;
  targetWord?: string;
  ipa?: string;
  definition?: string;
  exampleSentence?: string;
  isLoading?: boolean;
  isSubmitted: boolean;
  isCorrect?: boolean | null | undefined;
  /**
   * The learner's own verdict on whether they recalled the word.
   *
   * A flashcard has no answer to mark, so the only honest grade is the one the
   * learner gives. The card used to report nothing at all: it advanced without
   * submitting, which left the attempt the runner had opened sitting in
   * `in_progress` for ever, kept the activity out of progress, and scheduled no
   * review card for a word the learner had just studied.
   */
  onSubmit: (knewIt: boolean) => void;
  onContinue: () => void;
}

export const ExerciseFlashcard: React.FC<ExerciseFlashcardProps> = ({
  prompt,
  targetWord,
  ipa,
  definition,
  exampleSentence,
  isLoading = false,
  isSubmitted,
  isCorrect,
  onSubmit,
  onContinue,
}) => {
  const { t } = useTranslation();
  const [isFlipped, setIsFlipped] = useState(false);

  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key !== " " && e.key !== "Enter") return;
      e.preventDefault();
      // Space flips, and once graded it continues. It deliberately does not
      // answer the recall question: which verdict a bare keypress meant is a
      // guess, and guessing it wrong writes a review schedule for the learner.
      if (!isFlipped) setIsFlipped(true);
      else if (isSubmitted) onContinue();
    };

    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, [isFlipped, isSubmitted, onContinue]);

  return (
    <div className="space-y-8 max-w-xl mx-auto py-4">
      <div className="text-center space-y-1">
        <h2 className="text-xl md:text-2xl font-bold text-text">
          {prompt || t("runner.flipPrompt", "Study this key term")}
        </h2>
      </div>

      {/* Interactive Flashcard with Flip Animation */}
      <div
        role="button"
        tabIndex={0}
        aria-label="Flashcard"
        onClick={() => setIsFlipped((prev) => !prev)}
        onKeyDown={(e) => {
          if (e.key === "Enter" || e.key === " ") {
            e.preventDefault();
            setIsFlipped((prev) => !prev);
          }
        }}
        className={cn(
          "min-h-[280px] p-8 rounded-3xl border-2 cursor-pointer transition-all duration-300 flex flex-col items-center justify-center text-center shadow-lg select-none",
          isFlipped
            ? "border-primary/50 bg-gradient-to-br from-surface-card to-primary/10"
            : "border-border bg-surface-card hover:border-primary/40",
        )}
      >
        {!isFlipped ? (
          <div className="space-y-4 animate-in fade-in">
            <h3 className="text-3xl md:text-4xl font-extrabold text-text tracking-tight">
              {targetWord}
            </h3>
            {ipa && (
              <p className="font-mono text-sm text-primary-accent tracking-wide">
                {ipa}
              </p>
            )}
            <p className="text-xs text-text-muted mt-6 flex items-center justify-center gap-1.5 font-medium">
              <RotateCw className="h-3.5 w-3.5" aria-hidden="true" />
              {t("runner.flipPrompt", "Press Space or tap to flip card")}
            </p>
          </div>
        ) : (
          <div className="space-y-4 animate-in fade-in">
            <p className="text-xl md:text-2xl font-semibold text-text">
              {definition}
            </p>
            {exampleSentence && (
              <p className="text-sm text-text-muted italic border-t border-border-subtle pt-3 max-w-md">
                &ldquo;{exampleSentence}&rdquo;
              </p>
            )}
          </div>
        )}
      </div>

      {isSubmitted && (
        <div
          role="status"
          className={cn(
            "rounded-xl border p-4 text-sm font-medium",
            isCorrect
              ? "border-success bg-success/10 text-success-accent"
              : "border-border bg-surface-muted/40 text-text-muted",
          )}
        >
          {isCorrect
            ? t("runner.correct", "Correct! Well done.")
            : t(
                "runner.flashcardAgain",
                "Noted — this word will come back sooner.",
              )}
        </div>
      )}

      {/* Action Bar */}
      <div className="pt-4">
        {!isFlipped ? (
          <div className="flex justify-end">
            <Button
              size="lg"
              disabled={isLoading}
              onClick={() => setIsFlipped(true)}
              className="w-full sm:w-auto min-w-[160px] min-h-[44px] font-bold gap-2"
            >
              {t("runner.flipCardBtn", "Flip Card")}
              <ArrowRight className="h-4 w-4" aria-hidden="true" />
            </Button>
          </div>
        ) : isSubmitted ? (
          <div className="flex justify-end">
            <Button
              size="lg"
              disabled={isLoading}
              onClick={onContinue}
              className="w-full sm:w-auto min-w-[160px] min-h-[44px] font-bold gap-2"
            >
              {t("runner.continueBtn", "Continue")}
              <ArrowRight className="h-4 w-4" aria-hidden="true" />
            </Button>
          </div>
        ) : (
          <div className="space-y-3">
            <p className="text-sm text-text-muted text-center font-medium">
              {t("runner.recallPrompt", "Did you recall this word?")}
            </p>
            <div className="flex flex-col sm:flex-row gap-3 sm:justify-end">
              <Button
                size="lg"
                variant="outline"
                disabled={isLoading}
                onClick={() => onSubmit(false)}
                className="w-full sm:w-auto min-w-[160px] min-h-[44px] font-bold gap-2"
              >
                <X className="h-4 w-4" aria-hidden="true" />
                {t("runner.notYetBtn", "Not yet")}
              </Button>
              <Button
                size="lg"
                disabled={isLoading}
                onClick={() => onSubmit(true)}
                className="w-full sm:w-auto min-w-[160px] min-h-[44px] font-bold gap-2"
              >
                <Check className="h-4 w-4" aria-hidden="true" />
                {t("runner.knewItBtn", "I knew it")}
              </Button>
            </div>
          </div>
        )}
      </div>
    </div>
  );
};
