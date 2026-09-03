import React from "react";
import { Check, X } from "lucide-react";
import { useTranslation } from "react-i18next";

import { Button } from "@/components/ui/button";
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
  const { t } = useTranslation();
  const partial = !isCorrect && score !== undefined && score > 0;

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
          <>
            <Check className="h-5 w-5" aria-hidden="true" />
            <span>{t("runner.correct", "Correct! Well done.")}</span>
          </>
        ) : (
          <>
            <X className="h-5 w-5" aria-hidden="true" />
            <span>
              {partial
                ? `${t("runner.partial", "Partly right")} — ${score}%`
                : t("runner.incorrect", "Not quite.")}
            </span>
          </>
        )}
      </div>

      {!isCorrect && expectedAnswer && (
        <p className="mt-1 text-sm text-text font-medium">
          {t("runner.correctAnswerLabel", "Correct answer")}:{" "}
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
  );
};

/** The heading every exercise opens with. */
export const ExercisePrompt: React.FC<{ children: React.ReactNode }> = ({
  children,
}) => (
  <h2 className="text-xl md:text-2xl font-bold text-text leading-snug">
    {children}
  </h2>
);
