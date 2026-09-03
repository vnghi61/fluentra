import React, { useEffect, useRef, useState } from "react";
import { Check, X } from "lucide-react";
import { useTranslation } from "react-i18next";

import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { PronounceButton } from "@/components/ui/pronounce-button";
import { cn } from "@/lib/utils";

import { type AnswerExplanation } from "./ExerciseShell";

export interface ExerciseGapFillProps {
  prompt: string;
  sentenceBeforeBlank?: string | undefined;
  sentenceAfterBlank?: string | undefined;
  expectedAnswer?: string | null | undefined;
  feedback?: string | null | undefined;
  isSubmitted: boolean;
  isCorrect?: boolean | null | undefined;
  isLoading?: boolean;
  explanation?: AnswerExplanation | null | undefined;
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
  explanation,
  onSubmit,
  onContinue,
}) => {
  const { t } = useTranslation();
  const [answer, setAnswer] = useState("");
  const inputRef = useRef<HTMLInputElement>(null);

  // What the speaker says.
  //
  // Before the answer is in, the blank is a pause: reading the missing word out
  // loud would hand the learner the answer and turn the exercise into a
  // dictation. Afterwards the sentence is spoken whole, with the correct word
  // in place, because hearing the finished sentence is the part worth hearing —
  // and by then there is nothing left to give away.
  const spokenSentence = [
    sentenceBeforeBlank,
    isSubmitted ? (expectedAnswer ?? "") : "...",
    sentenceAfterBlank,
  ]
    .filter((part) => part.trim() !== "")
    .join(" ")
    .trim();

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
        <div className="flex items-center justify-end">
          <PronounceButton
            text={spokenSentence}
            label={
              isSubmitted
                ? t("runner.listenFullSentence", "Listen to the full sentence")
                : t("runner.listenSentence", "Listen to the sentence")
            }
          />
        </div>
        {/*
          Laid out as a wrapping flex row, not as inline text with a box dropped
          into it.

          A 44px-tall bordered input inside `leading-relaxed` prose sits on a
          baseline it does not fit: the line box stretches around it, the words
          after the gap hang at the wrong height, and at narrow widths the fixed
          `w-48` input took most of a line and pushed a single word onto the
          next. Flex items centre on each other and wrap as units, so the
          sentence reads as a sentence at every width.
        */}
        <div className="flex flex-wrap items-center gap-x-2 gap-y-3 text-lg md:text-xl leading-relaxed text-text">
          {sentenceBeforeBlank && <span>{sentenceBeforeBlank}</span>}
          <Input
            ref={inputRef}
            type="text"
            value={answer}
            disabled={isSubmitted || isLoading}
            onChange={(e) => setAnswer(e.target.value)}
            aria-label={t("runner.gapFillLabel", "Your answer for the blank")}
            placeholder={t("runner.gapFillPlaceholder", "Type your answer...")}
            className={cn(
              "w-36 sm:w-48 shrink-0 text-center font-bold text-base h-11 border-2 focus-visible:ring-primary",
              isSubmitted &&
                isCorrect &&
                "border-success bg-success/10 text-success-accent",
              isSubmitted &&
                !isCorrect &&
                "border-danger bg-danger/10 text-danger-accent",
            )}
          />
          {sentenceAfterBlank && <span>{sentenceAfterBlank}</span>}
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
