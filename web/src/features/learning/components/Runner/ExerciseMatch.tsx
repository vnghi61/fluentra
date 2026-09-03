import React, { useState } from "react";
import { Check, X } from "lucide-react";
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
 * `vocab_match` — pair each word with its meaning.
 *
 * Tap a word, then tap a definition. Not drag-and-drop: this runs on phones as
 * much as on desktops, and a drag across two columns on a 375px screen is a
 * fiddly way to answer a vocabulary question. Tapping is also what a keyboard
 * and a screen reader can drive without any extra machinery.
 *
 * A pair can be undone by tapping either half again, because the alternative —
 * being stuck with a wrong pair until submitting — makes the learner answer the
 * interface rather than the question.
 */
export interface MatchOption {
  id: string;
  text: string;
}

export interface ExerciseMatchProps {
  prompt: string;
  words: MatchOption[];
  definitions: MatchOption[];
  /** word id → definition id, revealed after answering. */
  correctPairs?: Record<string, string> | undefined;
  feedback?: string | null | undefined;
  /** Out of 100. Matching is the one kind that can be partly right. */
  score?: number | undefined;
  isSubmitted: boolean;
  isCorrect?: boolean | null | undefined;
  isLoading?: boolean;
  explanation?: import("./ExerciseShell").AnswerExplanation | null | undefined;
  onSubmit: (pairs: Record<string, string>) => void;
  onContinue: () => void;
}

export const ExerciseMatch: React.FC<ExerciseMatchProps> = ({
  prompt,
  words,
  definitions,
  correctPairs,
  feedback,
  score,
  isSubmitted,
  isCorrect,
  isLoading = false,
  explanation,
  onSubmit,
  onContinue,
}) => {
  const { t } = useTranslation();
  const [pairs, setPairs] = useState<Record<string, string>>({});
  const [activeWord, setActiveWord] = useState<string | null>(null);

  const definitionOf = (wordId: string) => pairs[wordId];
  const wordFor = (definitionId: string) =>
    Object.keys(pairs).find((wordId) => pairs[wordId] === definitionId);

  const handleWord = (wordId: string) => {
    if (isSubmitted) return;
    // Tapping a paired word breaks the pair; that is the undo.
    if (definitionOf(wordId)) {
      setPairs((current) => {
        const next = { ...current };
        delete next[wordId];
        return next;
      });
      setActiveWord(wordId);
      return;
    }
    setActiveWord((current) => (current === wordId ? null : wordId));
  };

  const handleDefinition = (definitionId: string) => {
    if (isSubmitted) return;
    const owner = wordFor(definitionId);
    if (owner) {
      setPairs((current) => {
        const next = { ...current };
        delete next[owner];
        return next;
      });
      return;
    }
    if (!activeWord) return;
    setPairs((current) => ({ ...current, [activeWord]: definitionId }));
    setActiveWord(null);
  };

  /** The number a paired row shows, so the two columns read as pairs. */
  const badgeFor = (wordId: string | undefined) =>
    wordId ? words.findIndex((w) => w.id === wordId) + 1 : 0;

  const verdictOf = (wordId: string): "right" | "wrong" | null => {
    if (!isSubmitted || !correctPairs) return null;
    return pairs[wordId] === correctPairs[wordId] ? "right" : "wrong";
  };

  const complete = Object.keys(pairs).length === words.length;

  return (
    <div className="space-y-8 max-w-3xl mx-auto py-4">
      <ExercisePrompt>{prompt}</ExercisePrompt>

      <div className="grid grid-cols-1 sm:grid-cols-2 gap-3 sm:gap-4">
        {/* Words */}
        <div
          className="space-y-2"
          role="group"
          aria-label={t("runner.matchWords", "Words")}
        >
          {words.map((word, index) => {
            const paired = definitionOf(word.id);
            const verdict = verdictOf(word.id);
            const active = activeWord === word.id;

            return (
              <div key={word.id} className="flex items-center gap-1">
                <button
                  type="button"
                  disabled={isLoading || isSubmitted}
                  aria-pressed={active || Boolean(paired)}
                  onClick={() => handleWord(word.id)}
                  className={cn(
                    "flex-1 flex items-center gap-3 p-3 rounded-xl border text-left min-h-[56px] transition-all cursor-pointer disabled:cursor-default focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary",
                    verdict === "right" && "border-success bg-success/10",
                    verdict === "wrong" && "border-danger bg-danger/10",
                    !verdict &&
                      active &&
                      "border-primary bg-primary/10 ring-2 ring-primary",
                    !verdict &&
                      !active &&
                      paired &&
                      "border-primary/40 bg-primary/5",
                    !verdict &&
                      !active &&
                      !paired &&
                      "border-border bg-surface-card hover:border-primary/50",
                  )}
                >
                  <span className="flex items-center justify-center h-7 w-7 shrink-0 rounded-lg border border-border bg-surface-muted text-xs font-bold text-text-muted">
                    {index + 1}
                  </span>
                  <span className="text-base font-semibold text-text">
                    {word.text}
                  </span>
                  {verdict === "right" && (
                    <Check
                      className="ml-auto h-5 w-5 text-success"
                      aria-label={t("runner.markCorrect", "Correct")}
                    />
                  )}
                  {verdict === "wrong" && (
                    <X
                      className="ml-auto h-5 w-5 text-danger"
                      aria-label={t("runner.markIncorrect", "Incorrect")}
                    />
                  )}
                </button>
                <PronounceButton text={word.text} />
              </div>
            );
          })}
        </div>

        {/* Definitions */}
        <div
          className="space-y-2"
          role="group"
          aria-label={t("runner.matchDefinitions", "Meanings")}
        >
          {definitions.map((definition) => {
            const owner = wordFor(definition.id);
            const badge = badgeFor(owner);

            return (
              <button
                key={definition.id}
                type="button"
                disabled={isLoading || isSubmitted || (!activeWord && !owner)}
                onClick={() => handleDefinition(definition.id)}
                className={cn(
                  "w-full flex items-center gap-3 p-3 rounded-xl border text-left min-h-[56px] transition-all cursor-pointer disabled:cursor-default focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary",
                  owner
                    ? "border-primary/40 bg-primary/5"
                    : "border-border bg-surface-card",
                  !owner && activeWord && "hover:border-primary/50",
                  !owner && !activeWord && "opacity-70",
                )}
              >
                <span
                  className={cn(
                    "flex items-center justify-center h-7 w-7 shrink-0 rounded-lg border text-xs font-bold",
                    badge
                      ? "border-primary bg-primary text-primary-fg"
                      : "border-dashed border-border text-text-muted",
                  )}
                >
                  {badge || "?"}
                </span>
                <span className="text-sm text-text">{definition.text}</span>
              </button>
            );
          })}
        </div>
      </div>

      {!isSubmitted && (
        <div className="flex items-center justify-between gap-3">
          <p className="text-sm text-text-muted">
            {activeWord
              ? t("runner.matchPickMeaning", "Now pick its meaning")
              : t("runner.matchPickWord", "Pick a word to start")}{" "}
            · {Object.keys(pairs).length}/{words.length}
          </p>
          {Object.keys(pairs).length > 0 && (
            <Button
              type="button"
              variant="ghost"
              size="sm"
              onClick={() => {
                setPairs({});
                setActiveWord(null);
              }}
            >
              {t("runner.matchClear", "Clear all")}
            </Button>
          )}
        </div>
      )}

      {isSubmitted && (
        <ExerciseFeedback
          isCorrect={isCorrect}
          feedback={feedback}
          score={score}
          explanation={explanation}
        />
      )}

      <ExerciseActions
        isSubmitted={isSubmitted}
        canSubmit={complete}
        isLoading={isLoading}
        onSubmit={() => onSubmit(pairs)}
        onContinue={onContinue}
      />
    </div>
  );
};
