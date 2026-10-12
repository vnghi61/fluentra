import React, { useEffect, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { PenLine } from "lucide-react";

import { PronounceButton } from "@/components/ui/pronounce-button";
import { cn } from "@/lib/utils";

import {
  type AnswerExplanation,
  ExerciseActions,
  ExerciseFeedback,
  ExercisePrompt,
} from "./ExerciseShell";
import { Textarea } from "@/components/ui/textarea";

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
  const inputRef = useRef<HTMLTextAreaElement>(null);

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

  const handleKeyDown = (e: React.KeyboardEvent<HTMLTextAreaElement>) => {
    if (e.key === "Enter" && !e.shiftKey) {
      e.preventDefault();
      handleSubmit();
    }
  };

  return (
    <div className="w-full min-w-0 space-y-6 max-w-4xl mx-auto">
      {/* Prompt Heading */}
      <div className="space-y-3">
        <span className="inline-flex items-center gap-2 rounded-xl bg-primary/10 px-3 py-2 text-sm font-semibold text-primary-accent">
          <PenLine className="h-4 w-4" aria-hidden="true" />
          {t("runner.sentenceTransformKind", "Rewrite the sentence")}
        </span>
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
      <div className="p-4 sm:p-6 rounded-2xl border border-border-subtle bg-surface-card shadow-sm space-y-3">
        <label
          htmlFor="sentence-transform-input"
          className="text-xs font-semibold text-text-muted uppercase tracking-wider block"
        >
          {t("runner.sentenceTransformLabel", "Your rewritten sentence")}
        </label>
        <Textarea
          id="sentence-transform-input"
          ref={inputRef}
          rows={3}
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
            "min-h-[104px] resize-y rounded-xl bg-surface-card p-4 font-medium md:text-lg disabled:opacity-70",
            isSubmitted &&
              isCorrect &&
              "border-success bg-success/10 text-success-accent",
            isSubmitted &&
              !isCorrect &&
              "border-danger bg-danger/10 text-danger-accent",
          )}
        />
        <p className="text-right text-xs text-text-muted">
          {t("runner.answerCharacters", { count: answer.length })}
        </p>
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
