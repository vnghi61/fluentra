import React from "react";
import { ArrowRight, Check, X } from "lucide-react";
import { useTranslation } from "react-i18next";

import { Button } from "@/components/ui/button";
import { pickLocalized, prefersVietnamese } from "@/lib/localized";
import { cn } from "@/lib/utils";

/**
 * The two pieces every exercise ends with: the verdict, and the button.
 *
 * They were copied into each renderer while there were three. At seven the copy
 * is the problem — a change to how a wrong answer reads has to be made in seven
 * places, and the one that gets missed is the one a learner sees. The three
 * original renderers still carry their own copies; they are working code with
 * tests against them, and rewriting them is not what adding four kinds needs.
 */

export interface AnswerExplanation {
  text: string;
  text_vi: string;
}

export interface ExerciseFeedbackProps {
  isCorrect?: boolean | null | undefined;
  /** Revealed only once the learner has answered. */
  expectedAnswer?: string | null | undefined;
  feedback?: string | null | undefined;
  /**
   * A partial score out of 100, for kinds that can be partly right.
   *
   * Matching is the only one today. "Incorrect" is a poor description of four
   * pairs out of five, and telling the learner which it is costs one line.
   */
  score?: number | undefined;
  explanation?: AnswerExplanation | null | undefined;
}

export const ExerciseFeedback: React.FC<ExerciseFeedbackProps> = ({
  isCorrect,
  expectedAnswer,
  feedback,
  score,
  explanation,
}) => {
  const { t, i18n } = useTranslation();
  const partial = !isCorrect && score !== undefined && score > 0;
  const verdict = isCorrect
    ? t("runner.correct", "Correct! Well done.")
    : partial
      ? `${t("runner.partial", "Partly right")} — ${score}%`
      : t("runner.incorrect", "Not quite.");

  const showFeedback =
    Boolean(feedback?.trim()) &&
    !prefersVietnamese(i18n.language) &&
    feedback!.trim().toLowerCase() !== verdict.trim().toLowerCase();

  const explanationText = explanation
    ? pickLocalized(explanation.text, explanation.text_vi, i18n.language)
    : "";

  return (
    <div
      role="status"
      className={cn(
        "p-4 rounded-xl border animate-in fade-in duration-200",
        isCorrect
          ? "border-success/30 bg-success/10 text-success-accent"
          : partial
            ? "border-warning/30 bg-warning/10 text-text"
            : "border-danger/30 bg-danger/10 text-danger-accent",
      )}
    >
      <div className="flex items-center gap-2 font-bold text-base">
        {isCorrect ? (
          <Check className="h-5 w-5" aria-hidden="true" />
        ) : (
          <X className="h-5 w-5" aria-hidden="true" />
        )}
        <span>{verdict}</span>
      </div>

      {!isCorrect && expectedAnswer && (
        <p className="mt-1 text-base leading-relaxed text-text font-medium">
          {t("runner.correctAnswerLabel", "Correct answer")}:{" "}
          <span className="font-bold underline">{expectedAnswer}</span>
        </p>
      )}
      {showFeedback && (
        <p className="mt-1 text-base leading-relaxed text-text font-normal">
          {feedback}
        </p>
      )}

      {explanationText ? (
        <div className="mt-3 pt-3 border-t border-border/40 space-y-1 text-base">
          <span className="text-xs font-semibold uppercase tracking-wider text-text-muted">
            {t("runner.explanationLabel", "Explanation")}
          </span>
          <p className="text-text/90 font-medium leading-relaxed">
            {explanationText}
          </p>
        </div>
      ) : null}
    </div>
  );
};

export interface ExerciseActionsProps {
  isSubmitted: boolean;
  /** Disables Check while the answer is incomplete. */
  canSubmit: boolean;
  isLoading?: boolean;
  onSubmit: () => void;
  onContinue: () => void;
}

export const ExerciseActions: React.FC<ExerciseActionsProps> = ({
  isSubmitted,
  canSubmit,
  isLoading = false,
  onSubmit,
  onContinue,
}) => {
  const { t } = useTranslation();

  return (
    <div className="pt-4 flex justify-end">
      {!isSubmitted ? (
        <Button
          size="lg"
          disabled={!canSubmit || isLoading}
          isLoading={isLoading}
          onClick={onSubmit}
          className="w-full sm:w-auto min-w-[160px]"
        >
          {t("runner.checkBtn", "Check Answer")}
          <ArrowRight className="h-4 w-4" aria-hidden="true" />
        </Button>
      ) : (
        <Button
          size="lg"
          onClick={onContinue}
          className="w-full sm:w-auto min-w-[160px]"
        >
          {t("runner.continueBtn", "Continue")}
          <ArrowRight className="h-4 w-4" aria-hidden="true" />
        </Button>
      )}
    </div>
  );
};

/** The heading every exercise opens with. */
export const ExercisePrompt: React.FC<{ children: React.ReactNode }> = ({
  children,
}) => (
  <h2 className="min-w-0 break-words text-lg md:text-xl font-semibold text-text leading-snug">
    {children}
  </h2>
);
