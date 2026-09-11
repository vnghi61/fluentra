import React, { useState } from "react";
import { PenTool } from "lucide-react";
import { useTranslation } from "react-i18next";

import { cn } from "@/lib/utils";

import { GuestNotice } from "../GuestNotice";
import {
  type AnswerExplanation,
  ExerciseActions,
  ExerciseFeedback,
  ExercisePrompt,
} from "./ExerciseShell";

export interface ExerciseWritingProps {
  /** The writing prompt / task description. */
  prompt: string;
  /** Guidelines, constraints, or evaluation rubric. */
  rubric?: string | undefined;
  /** Minimum required word count. */
  minWords?: number | undefined;
  sampleAnswer?: string | undefined;
  feedback?: string | null | undefined;
  score?: number | null | undefined;
  isSubmitted: boolean;
  isCorrect?: boolean | null | undefined;
  isLoading?: boolean;
  isGuest?: boolean;
  explanation?: AnswerExplanation | null | undefined;
  onSubmit: (answerText: string) => void;
  onContinue: () => void;
}

function countWords(text: string): number {
  const trimmed = text.trim();
  if (!trimmed) return 0;
  return trimmed.split(/\s+/).length;
}

/**
 * `writing_prompt` — compose open-ended written text evaluated via AI.
 *
 * Provides a rich writing prompt, optional rubric/minimum word guidelines,
 * live word-counter, and interactive grading feedback.
 */
export const ExerciseWriting: React.FC<ExerciseWritingProps> = ({
  prompt,
  rubric,
  minWords = 0,
  sampleAnswer,
  feedback,
  score,
  isSubmitted,
  isCorrect,
  isLoading = false,
  isGuest = false,
  explanation,
  onSubmit,
  onContinue,
}) => {
  const { t } = useTranslation();
  const [text, setText] = useState("");

  const words = countWords(text);
  const meetsMinWords = minWords <= 0 || words >= minWords;
  const canSubmit = text.trim().length > 0;

  const handleSubmit = () => {
    if (canSubmit && !isLoading) {
      onSubmit(text.trim());
    }
  };

  return (
    <div className="space-y-6 max-w-2xl mx-auto py-4">
      {/* Prompt Heading */}
      <ExercisePrompt>{prompt}</ExercisePrompt>

      {/* Rubric / Instructions Card */}
      {rubric && (
        <div className="rounded-2xl border border-border bg-surface-card/70 shadow-sm p-4 sm:p-5 space-y-2">
          <div className="flex items-center gap-2 text-xs font-semibold uppercase tracking-wider text-primary">
            <PenTool className="h-3.5 w-3.5" aria-hidden="true" />
            <span>
              {t("runner.writingInstructions", "Instructions & Rubric")}
            </span>
          </div>
          <p className="text-sm text-text-muted leading-relaxed whitespace-pre-line">
            {rubric}
          </p>
        </div>
      )}

      {/* Writing Textarea or Guest Notice */}
      {isGuest ? (
        <GuestNotice />
      ) : (
        <div className="space-y-2">
          <label htmlFor="writing-response" className="sr-only">
            {prompt}
          </label>
          <textarea
            id="writing-response"
            value={text}
            disabled={isSubmitted || isLoading}
            onChange={(e) => setText(e.target.value)}
            placeholder={t(
              "runner.writingPlaceholder",
              "Write your response here...",
            )}
            rows={6}
            className={cn(
              "w-full rounded-xl border border-border bg-surface-card p-4 text-base text-text leading-relaxed",
              "focus:outline-none focus:ring-2 focus:ring-primary focus:border-transparent transition-all shadow-sm resize-y",
              "min-h-[140px]",
              isSubmitted && isCorrect && "border-success/50 bg-success/5",
              isSubmitted &&
                isCorrect === false &&
                "border-danger/50 bg-danger/5",
            )}
          />

          {/* Word Counter */}
          <div className="flex items-center justify-between text-xs text-text-muted px-1">
            <span>
              {minWords > 0 ? (
                <span
                  className={cn(
                    "font-medium",
                    meetsMinWords ? "text-success" : "text-text-muted",
                  )}
                >
                  {t(
                    "runner.wordCountWithMin",
                    `${words} / ${minWords} words minimum`,
                    {
                      count: words,
                      min: minWords,
                    },
                  )}
                </span>
              ) : (
                <span>
                  {t("runner.wordCount", `${words} words`, { count: words })}
                </span>
              )}
            </span>
            {minWords > 0 && !meetsMinWords && (
              <span className="text-warning text-xs">
                {t(
                  "runner.wordsRemaining",
                  `${Math.max(0, minWords - words)} more needed`,
                  {
                    remaining: Math.max(0, minWords - words),
                  },
                )}
              </span>
            )}
          </div>
        </div>
      )}

      {/* Feedback Panel */}
      {isSubmitted && (
        <ExerciseFeedback
          isCorrect={isCorrect}
          feedback={feedback}
          expectedAnswer={!isCorrect ? sampleAnswer : undefined}
          score={typeof score === "number" ? score : undefined}
          explanation={explanation}
        />
      )}

      {/* Action Bar */}
      <ExerciseActions
        isSubmitted={isSubmitted || isGuest}
        canSubmit={canSubmit}
        isLoading={isLoading}
        onSubmit={handleSubmit}
        onContinue={onContinue}
      />
    </div>
  );
};
