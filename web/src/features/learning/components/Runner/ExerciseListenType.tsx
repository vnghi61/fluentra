import React, { useEffect, useRef, useState } from "react";
import { useTranslation } from "react-i18next";

import { Input } from "@/components/ui/input";
import { PronounceButton } from "@/components/ui/pronounce-button";
import { cn } from "@/lib/utils";

import {
  ExerciseActions,
  ExerciseFeedback,
  ExercisePrompt,
} from "./ExerciseShell";

/**
 * `vocab_listen_type` — hear a word, spell it.
 *
 * The word is not on screen; that is the exercise. It is, however, in the page:
 * synthesis happens in the browser, so the text has to reach it, and a learner
 * who opens the developer tools can read it. The alternative is server-side TTS,
 * which means building `platform/media` first. Grading runs on the server against
 * the stored body either way, so the only thing at stake is a learner choosing to
 * skip their own practice.
 */
export interface ExerciseListenTypeProps {
  prompt: string;
  /** The word the browser speaks. Deliberately client-visible; see above. */
  audioText: string;
  audioUrl?: string | null | undefined;
  ipa?: string | undefined;
  /** A definition, so a learner stuck on the sound is not also stuck on sense. */
  hint?: string | undefined;
  expectedAnswer?: string | null | undefined;
  feedback?: string | null | undefined;
  isSubmitted: boolean;
  isCorrect?: boolean | null | undefined;
  isLoading?: boolean;
  onSubmit: (answerText: string) => void;
  onContinue: () => void;
}

export const ExerciseListenType: React.FC<ExerciseListenTypeProps> = ({
  prompt,
  audioText,
  audioUrl,
  ipa,
  hint,
  expectedAnswer,
  feedback,
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
    if (!isSubmitted) inputRef.current?.focus();
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
      <ExercisePrompt>{prompt}</ExercisePrompt>

      <div className="p-6 rounded-2xl border border-border bg-surface-card shadow-sm space-y-5">
        {/*
          The speaker is the question, so it is large and centred rather than
          tucked beside a label. Nothing else on this card can be read as the
          thing to press first.
        */}
        <div className="flex flex-col items-center gap-2">
          <PronounceButton
            text={audioText}
            audioUrl={audioUrl}
            size="md"
            className="h-16 w-16 min-h-[64px] min-w-[64px] border-2 border-primary/30 bg-primary/5"
            label={t("runner.playWord", "Play the word")}
          />
          <p className="text-xs font-medium text-text-muted">
            {t("runner.tapToHear", "Tap to hear the word again")}
          </p>
        </div>

        {/* Revealed after answering: before that, it would spell the word out. */}
        {isSubmitted && ipa && (
          <p className="text-center font-mono text-sm text-primary-accent">
            {ipa}
          </p>
        )}

        <Input
          ref={inputRef}
          type="text"
          value={answer}
          autoComplete="off"
          autoCorrect="off"
          autoCapitalize="off"
          spellCheck={false}
          disabled={isSubmitted || isLoading}
          onChange={(e) => setAnswer(e.target.value)}
          aria-label={t("runner.listenTypeLabel", "The word you heard")}
          placeholder={t("runner.listenTypePlaceholder", "Type what you hear…")}
          className={cn(
            "mx-auto block w-full max-w-sm text-center text-lg font-bold h-12 border-2 focus-visible:ring-primary",
            isSubmitted &&
              isCorrect &&
              "border-success bg-success/10 text-success-accent",
            isSubmitted &&
              !isCorrect &&
              "border-danger bg-danger/10 text-danger-accent",
          )}
        />

        {hint && (
          <p className="text-center text-sm italic text-text-muted">
            {t("runner.meaningLabel", "Meaning")}: {hint}
          </p>
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
        canSubmit={answer.trim() !== ""}
        isLoading={isLoading}
        onSubmit={() => onSubmit(answer.trim())}
        onContinue={onContinue}
      />
    </form>
  );
};
