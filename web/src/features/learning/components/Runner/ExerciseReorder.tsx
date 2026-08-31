import React, { useState } from "react";
import { RotateCcw } from "lucide-react";
import { useTranslation } from "react-i18next";

import { Button } from "@/components/ui/button";
import { PronounceButton } from "@/components/ui/pronounce-button";
import { cn } from "@/lib/utils";

import {
  ExerciseActions,
  ExerciseFeedback,
  ExercisePrompt,
} from "./ExerciseShell";

/**
 * `vocab_reorder` — rebuild a sentence from shuffled words.
 *
 * Tap to add a word to the answer, tap it again in the answer to take it back.
 * Same reasoning as the matching kind: dragging tiles is worse than tapping on
 * a phone, and tapping is keyboard-reachable for free.
 *
 * Tokens are addressed by index, not by text. Sentences repeat words — "to",
 * "the", "a" — and keying on the text makes two identical tiles the same tile:
 * placing one removes both, and the answer can never be completed.
 */
export interface ExerciseReorderProps {
  prompt: string;
  /** The sentence's words, already shuffled by the author. */
  tokens: string[];
  /** Bolded in the tiles, so the word being practised is findable. */
  targetWord?: string | undefined;
  expectedAnswer?: string | null | undefined;
  feedback?: string | null | undefined;
  isSubmitted: boolean;
  isCorrect?: boolean | null | undefined;
  isLoading?: boolean;
  onSubmit: (sentence: string) => void;
  onContinue: () => void;
}

export const ExerciseReorder: React.FC<ExerciseReorderProps> = ({
  prompt,
  tokens,
  targetWord,
  expectedAnswer,
  feedback,
  isSubmitted,
  isCorrect,
  isLoading = false,
  onSubmit,
  onContinue,
}) => {
  const { t } = useTranslation();
  // Indices into `tokens`, in the order the learner placed them.
  const [placed, setPlaced] = useState<number[]>([]);

  const sentence = placed.map((index) => tokens[index]).join(" ");
  const isTarget = (token: string) =>
    Boolean(targetWord) &&
    token.toLowerCase().replace(/[^a-z]/g, "") ===
      targetWord?.toLowerCase().replace(/[^a-z]/g, "");

  const place = (index: number) => {
    if (isSubmitted || placed.includes(index)) return;
    setPlaced((current) => [...current, index]);
  };

  const remove = (position: number) => {
    if (isSubmitted) return;
    setPlaced((current) => current.filter((_, i) => i !== position));
  };

  return (
    <div className="space-y-8 max-w-2xl mx-auto py-4">
      <ExercisePrompt>{prompt}</ExercisePrompt>

      {/* The answer being built. Keeps its height so the layout does not jump
          as the first word is placed. */}
      <div
        className={cn(
          "min-h-[96px] p-4 rounded-2xl border-2 border-dashed bg-surface-card flex flex-wrap items-start content-start gap-2",
          isSubmitted && isCorrect && "border-success bg-success/5",
          isSubmitted && !isCorrect && "border-danger bg-danger/5",
          !isSubmitted && "border-border",
        )}
        role="group"
        aria-label={t("runner.reorderAnswer", "Your sentence")}
      >
        {placed.length === 0 && (
          <p className="text-sm text-text-muted italic py-2">
            {t("runner.reorderEmpty", "Tap the words below in the right order")}
          </p>
        )}
        {placed.map((tokenIndex, position) => (
          <button
            key={`${tokenIndex}-${position}`}
            type="button"
            disabled={isSubmitted || isLoading}
            onClick={() => remove(position)}
            className="px-3 py-2 rounded-lg border border-primary/40 bg-primary/10 text-base font-medium text-text min-h-[44px] cursor-pointer disabled:cursor-default focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary"
          >
            {tokens[tokenIndex]}
          </button>
        ))}
      </div>

      {/* The word bank. */}
      <div
        className="flex flex-wrap gap-2"
        role="group"
        aria-label={t("runner.reorderBank", "Available words")}
      >
        {tokens.map((token, index) => {
          const used = placed.includes(index);
          return (
            <button
              key={`${token}-${index}`}
              type="button"
              disabled={used || isSubmitted || isLoading}
              onClick={() => place(index)}
              className={cn(
                "px-3 py-2 rounded-lg border text-base min-h-[44px] transition-all cursor-pointer disabled:cursor-default focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary",
                used
                  ? "border-dashed border-border bg-surface-muted/40 text-transparent select-none"
                  : "border-border bg-surface-card text-text hover:border-primary/50",
                !used && isTarget(token) && "font-bold text-primary-accent",
              )}
              // A used tile keeps its width so the bank does not reflow under
              // the learner's finger between taps.
              aria-hidden={used}
            >
              {token}
            </button>
          );
        })}
      </div>

      <div className="flex items-center justify-between gap-3">
        {isSubmitted && expectedAnswer ? (
          <div className="flex items-center gap-1">
            <PronounceButton
              text={expectedAnswer}
              label={t("runner.listenFullSentence", "Listen to the full sentence")}
            />
            <span className="text-sm text-text-muted">
              {t("runner.listenFullSentence", "Listen to the full sentence")}
            </span>
          </div>
        ) : (
          <span />
        )}
        {!isSubmitted && placed.length > 0 && (
          <Button
            type="button"
            variant="ghost"
            size="sm"
            onClick={() => setPlaced([])}
            className="gap-1.5"
          >
            <RotateCcw className="h-3.5 w-3.5" aria-hidden="true" />
            {t("runner.reorderReset", "Start over")}
          </Button>
        )}
      </div>

      {isSubmitted && (
        <ExerciseFeedback
          isCorrect={isCorrect}
          expectedAnswer={expectedAnswer}
          feedback={feedback}
        />
      )}

      <ExerciseActions
        isSubmitted={isSubmitted}
        canSubmit={placed.length === tokens.length}
        isLoading={isLoading}
        onSubmit={() => onSubmit(sentence)}
        onContinue={onContinue}
      />
    </div>
  );
};
