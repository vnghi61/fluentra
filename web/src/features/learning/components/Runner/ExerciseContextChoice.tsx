import React, { useState } from "react";
import { Check, X } from "lucide-react";
import { useTranslation } from "react-i18next";

import { PronounceButton } from "@/components/ui/pronounce-button";
import { cn } from "@/lib/utils";

import {
  ExerciseActions,
  ExerciseFeedback,
  ExercisePrompt,
} from "./ExerciseShell";
import type { OptionItem } from "./ExerciseMultipleChoice";

/**
 * `vocab_context_choice` — which meaning does this sentence use?
 *
 * Structurally a multiple choice, and graded by exactly the same path. What
 * differs is the stem: the question is a sentence with the word in it, not a
 * definition to match. That is the point of the kind — a learner who knows
 * "leisure" from a glossary still has to recognise it in use — and it is why
 * the sentence gets a card of its own with the word highlighted and audible,
 * rather than being squeezed into the prompt line.
 */
import type { AnswerExplanation } from "./ExerciseShell";

export interface ExerciseContextChoiceProps {
  prompt: string;
  /** The sentence the word appears in. */
  sentence: string;
  targetWord?: string | undefined;
  options: OptionItem[];
  correctOptionId?: string | null | undefined;
  feedback?: string | null | undefined;
  isSubmitted: boolean;
  isCorrect?: boolean | null | undefined;
  isLoading?: boolean;
  explanation?: AnswerExplanation | null | undefined;
  onSubmit: (selectedOptionId: string) => void;
  onContinue: () => void;
}

/** Splits the sentence so the target word can be emphasised in place. */
function highlight(sentence: string, word?: string): React.ReactNode {
  const target = word?.trim();
  if (!target) return sentence;

  const escaped = target.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
  const pattern = new RegExp(`(${escaped}\\w*)`, "gi");
  return sentence.split(pattern).map((part, index) =>
    // `split` with a capturing group puts the matches at the odd indices.
    index % 2 === 1 ? (
      <strong key={index} className="font-extrabold text-primary-accent">
        {part}
      </strong>
    ) : (
      <React.Fragment key={index}>{part}</React.Fragment>
    ),
  );
}

export const ExerciseContextChoice: React.FC<ExerciseContextChoiceProps> = ({
  prompt,
  sentence,
  targetWord,
  options,
  correctOptionId,
  feedback,
  isSubmitted,
  isCorrect,
  isLoading = false,
  explanation,
  onSubmit,
  onContinue,
}) => {
  const { t } = useTranslation();
  const [selectedId, setSelectedId] = useState<string | null>(null);

  return (
    <div className="space-y-8 max-w-2xl mx-auto py-4">
      <ExercisePrompt>{prompt}</ExercisePrompt>

      <div className="p-5 rounded-2xl border border-border bg-surface-card shadow-sm flex items-start justify-between gap-3">
        <p className="text-lg md:text-xl leading-relaxed text-text italic">
          &ldquo;{highlight(sentence, targetWord)}&rdquo;
        </p>
        {/*
          The sentence, not the word: hearing the word in place is what tells a
          learner which sense is being used, and the sentence contains the word
          anyway.
        */}
        <PronounceButton
          text={sentence}
          label={t("runner.listenSentence", "Listen to the sentence")}
        />
      </div>

      <div className="space-y-3" role="radiogroup" aria-label={prompt}>
        {options.map((option, index) => {
          const isSelected = selectedId === option.id;
          const isAnswer = correctOptionId === option.id;
          const selectedWrong = isSubmitted && isSelected && !isCorrect;

          let style = "border-border bg-surface-card hover:border-primary/50";
          if (isSubmitted) {
            if (isAnswer) style = "border-success bg-success/10 font-semibold";
            else if (selectedWrong)
              style = "border-danger bg-danger/10 line-through opacity-80";
            else style = "border-border opacity-50";
          } else if (isSelected) {
            style = "border-primary bg-primary/10 ring-2 ring-primary";
          }

          return (
            <button
              key={option.id}
              type="button"
              role="radio"
              aria-checked={isSelected}
              disabled={isSubmitted || isLoading}
              onClick={() => setSelectedId(option.id)}
              className={cn(
                "w-full flex items-center justify-between gap-3 p-4 rounded-xl border text-left min-h-[56px] transition-all text-text cursor-pointer disabled:cursor-default focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary",
                style,
              )}
            >
              <div className="flex items-center gap-3">
                <span className="flex items-center justify-center h-7 w-7 shrink-0 rounded-lg border border-border bg-surface-muted text-xs font-bold text-text-muted">
                  {index + 1}
                </span>
                <span className="text-base">{option.text}</span>
              </div>
              {isSubmitted && isAnswer && (
                <Check
                  className="h-5 w-5 shrink-0 text-success"
                  aria-label={t("runner.markCorrect", "Correct")}
                />
              )}
              {selectedWrong && !isAnswer && (
                <X
                  className="h-5 w-5 shrink-0 text-danger"
                  aria-label={t("runner.markIncorrect", "Incorrect")}
                />
              )}
            </button>
          );
        })}
      </div>

      {isSubmitted && (
        <ExerciseFeedback
          isCorrect={isCorrect}
          feedback={feedback}
          explanation={explanation}
        />
      )}

      <ExerciseActions
        isSubmitted={isSubmitted}
        canSubmit={Boolean(selectedId)}
        isLoading={isLoading}
        onSubmit={() => selectedId && onSubmit(selectedId)}
        onContinue={onContinue}
      />
    </div>
  );
};
