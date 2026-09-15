import React, { useEffect, useRef, useState } from "react";
import { useTranslation } from "react-i18next";

import { Input } from "@/components/ui/input";
import { PronounceButton } from "@/components/ui/pronounce-button";
import { cn } from "@/lib/utils";

import {
  type AnswerExplanation,
  ExerciseActions,
  ExerciseFeedback,
  ExercisePrompt,
} from "./ExerciseShell";

export interface ExerciseSentenceTransformProps {
  prompt: string;
  expectedAnswer?: string | null | undefined;
  feedback?: string | null | undefined;
  explanation?: AnswerExplanation | null | undefined;
  isSubmitted: boolean;
  isCorrect?: boolean | null | undefined;
  isLoading?: boolean;
  onSubmit: (answerText: string) => void;
  onContinue: () => void;
}

/**
 * `grammar_sentence_transform` — rewrite a sentence according to a grammatical rule or prompt.
 *
 * The learner reads a prompt (such as "Rewrite beginning with 'Although': ...")
 * and types their transformed sentence into the input field.
 */
export const ExerciseSentenceTransform: React.FC<
  ExerciseSentenceTransformProps
> = ({
  prompt,
  expectedAnswer,
  feedback,
  explanation,
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

  const handleSubmit = () => {
    if (isSubmitted) {
      onContinue();
    } else if (answer.trim() && !isLoading) {
      onSubmit(answer.trim());
    }
  };

  const handleKeyDown = (e: React.KeyboardEvent<HTMLInputElement>) => {
    if (e.key === "Enter" && !e.shiftKey) {
      e.preventDefault();
      handleSubmit();
    }
  };

  return (
    <div className="space-y-8 max-w-2xl mx-auto py-4">
      {/* Prompt Heading */}
      <div className="space-y-3">
        <div className="flex items-start justify-between gap-4">
          <ExercisePrompt>{prompt}</ExercisePrompt>
          <div className="shrink-0 mt-1">
            <PronounceButton
              text={prompt}
              label={t("runner.listenSentence", "Listen to the sentence")}
            />
          </div>
        </div>
      </div>

      {/* Sentence Transformation Input Card */}
      <div className="p-6 rounded-2xl border border-border bg-surface-card shadow-sm space-y-4">
        <label
          htmlFor="sentence-transform-input"
          className="text-xs font-semibold text-text-muted uppercase tracking-wider block"
        >
          {t("runner.sentenceTransformLabel", "Your rewritten sentence")}
        </label>
        <Input
          id="sentence-transform-input"
          ref={inputRef}
          type="text"
          value={answer}
          disabled={isSubmitted || isLoading}
          onChange={(e) => setAnswer(e.target.value)}
          onKeyDown={handleKeyDown}
          aria-label={t(
            "runner.sentenceTransformLabel",
            "Your rewritten sentence",
          )}
          placeholder={t(
            "runner.sentenceTransformPlaceholder",
            "Type the rewritten sentence...",
          )}
          className={cn(
            "w-full min-h-[48px] px-4 text-base md:text-lg font-medium border-2 focus-visible:ring-primary rounded-xl",
            isSubmitted &&
              isCorrect &&
              "border-success bg-success/10 text-success-accent",
            isSubmitted &&
              !isCorrect &&
              "border-danger bg-danger/10 text-danger-accent",
          )}
        />
      </div>

      {/* Evaluation Feedback */}
      {isSubmitted && (
        <ExerciseFeedback
          isCorrect={isCorrect}
          expectedAnswer={expectedAnswer}
          feedback={feedback}
          explanation={explanation}
        />
      )}

      {/* Action Footer */}
      <ExerciseActions
        isSubmitted={isSubmitted}
        canSubmit={answer.trim().length > 0}
        isLoading={isLoading}
        onSubmit={handleSubmit}
        onContinue={onContinue}
      />
    </div>
  );
};
