import React from "react";
import { Eye } from "lucide-react";
import { useTranslation } from "react-i18next";

import { Button } from "@/components/ui/button";
import { PronounceButton } from "@/components/ui/pronounce-button";

export interface FlashcardFrontProps {
  word: string;
  ipa?: string;
  audioUrl?: string | null;
  /** The page crediting the recording; without it the recording does not play. */
  audioAttribution?: string | null;
  audioLicence?: string | null;
  onFlip: () => void;
  /** The reveal button, so the session can put focus on a new card. */
  revealRef?: React.Ref<HTMLButtonElement> | undefined;
}

/**
 * The question side of a review card.
 *
 * The reveal is a real button rather than the whole face acting as one: the
 * face also holds the pronounce button, and a button inside a button is two
 * controls a screen reader cannot tell apart. A tap anywhere else on the face
 * still turns the card, because that is what a thumb expects of a flashcard.
 *
 * The button's name carries the word, so when focus lands on it for a new card
 * a screen reader announces what is being asked, not only "Show answer".
 */
export const FlashcardFront: React.FC<FlashcardFrontProps> = ({
  word,
  ipa,
  audioUrl,
  audioAttribution,
  audioLicence,
  onFlip,
  revealRef,
}) => {
  const { t } = useTranslation();

  return (
    <div
      // Pointer convenience only; the keyboard path is the button below.
      onClick={(event) => {
        if ((event.target as Element).closest("button, a")) return;
        onFlip();
      }}
      className="flex h-full min-h-[300px] w-full cursor-pointer select-none flex-col items-center gap-6 rounded-[var(--radius-card)] border border-border-subtle bg-surface-card p-5 text-center shadow-[var(--shadow-card)] transition-colors hover:border-primary/40 motion-reduce:transition-none sm:min-h-[340px] sm:p-8"
    >
      <p className="text-xs font-semibold uppercase tracking-wider text-text-muted">
        {t("review.frontPrompt")}
      </p>

      <div className="flex w-full flex-1 flex-col items-center justify-center gap-3">
        <h2 className="max-w-full break-words text-4xl font-bold leading-tight tracking-tight text-text">
          {word}
        </h2>

        {/*
          The IPA and the speaker are one row, but the speaker does not depend
          on the IPA being there: a sense authored without a transcription must
          not silently lose its audio too. PronounceButton falls back to speech
          synthesis, so the button is always the real thing.
        */}
        <div className="flex flex-wrap items-center justify-center gap-2">
          {ipa && (
            <span className="font-mono text-base text-primary-accent">
              {ipa}
            </span>
          )}
          <PronounceButton
            text={word}
            audioUrl={audioUrl}
            audioAttribution={audioAttribution}
            audioLicence={audioLicence}
            label={t("review.listenBtn", "Pronounce word")}
          />
        </div>
      </div>

      <Button
        ref={revealRef}
        size="lg"
        onClick={onFlip}
        aria-label={t("review.showAnswerFor", { word })}
        aria-keyshortcuts="Space Enter"
        className="w-full font-semibold sm:w-auto sm:min-w-60"
      >
        <Eye className="h-4 w-4" aria-hidden="true" />
        {t("review.showAnswer")}
        <kbd
          aria-hidden="true"
          className="ml-1 hidden rounded-md border border-primary-fg/30 px-1.5 font-mono text-xs leading-5 md:inline"
        >
          {t("review.spaceKey")}
        </kbd>
      </Button>
    </div>
  );
};
