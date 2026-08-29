import React, { useEffect, useState } from "react";
import { ArrowRight, Check, RotateCw, X } from "lucide-react";
import { useTranslation } from "react-i18next";

import { Button } from "@/components/ui/button";
import { FlipCard } from "@/components/ui/flip-card";
import { cn } from "@/lib/utils";

export interface ExerciseFlashcardProps {
  prompt: string;
  targetWord?: string;
  ipa?: string;
  definition?: string;
  /** The gloss in the learner's own language, when the content carries one. */
  definitionVi?: string;
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
  definitionVi,
  exampleSentence,
  isLoading = false,
  isSubmitted,
  isCorrect,
  onSubmit,
  onContinue,
}) => {
  const { t, i18n } = useTranslation();

  // Both faces carry the same box so the card keeps its shape mid-turn.
  const faceClass =
    "min-h-[280px] p-8 rounded-3xl border-2 flex flex-col items-center justify-center text-center shadow-lg select-none h-full";

  // `startsWith`, not equality: i18next resolves to "vi" here but a browser or a
  // stored preference can hand back "vi-VN".
  const prefersVietnamese = i18n.language.toLowerCase().startsWith("vi");
  const gloss = definitionVi?.trim() ? definitionVi : undefined;
  const leadDefinition = prefersVietnamese && gloss ? gloss : definition;
  const secondDefinition =
    prefersVietnamese && gloss ? definition : (gloss ?? undefined);
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
      <FlipCard
        flipped={isFlipped}
        onClick={() => setIsFlipped((prev) => !prev)}
        label={t("runner.flashcard", "Flashcard")}
        className="cursor-pointer"
        front={
          <div className={cn(faceClass, "border-border bg-surface-card")}>
            <div className="space-y-4">
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
          </div>
        }
        back={
          <div
            className={cn(
              faceClass,
              "border-primary/50 bg-gradient-to-br from-surface-card to-primary/10",
            )}
          >
            <div className="space-y-4">
              {/*
                The learner's own language leads when they have chosen it.

                An English definition is the thing being learned, so it is never
                dropped — but a learner reading the interface in Vietnamese is
                being asked to define an unknown word with more unknown words,
                and the gloss they can already read is what makes the card land.
                When the interface is in English the order is simply reversed.
              */}
              <p className="text-xl md:text-2xl font-semibold text-text">
                {leadDefinition}
              </p>
              {secondDefinition && (
                <p className="text-base text-text-muted">{secondDefinition}</p>
              )}
              {exampleSentence && (
                <p className="text-sm text-text-muted italic border-t border-border-subtle pt-3 max-w-md">
                  &ldquo;{exampleSentence}&rdquo;
                </p>
              )}
            </div>
          </div>
        }
      />

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
