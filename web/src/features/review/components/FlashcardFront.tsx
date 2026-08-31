import React from "react";
import { RotateCw } from "lucide-react";
import { useTranslation } from "react-i18next";

import { PronounceButton } from "@/components/ui/pronounce-button";

export interface FlashcardFrontProps {
  word: string;
  ipa?: string;
  audioUrl?: string | null;
  onFlip: () => void;
}

export const FlashcardFront: React.FC<FlashcardFrontProps> = ({
  word,
  ipa,
  audioUrl,
  onFlip,
}) => {
  const { t } = useTranslation();

  return (
    <div
      role="button"
      tabIndex={0}
      aria-label={`${word}. ${t("review.flipHint", "Space to reveal answer")}`}
      onClick={onFlip}
      onKeyDown={(e) => {
        if (e.key === "Enter" || e.key === " ") {
          e.preventDefault();
          onFlip();
        }
      }}
      className="w-full min-h-[300px] p-8 rounded-3xl border-2 border-border bg-surface-card hover:border-primary/40 transition-all flex flex-col items-center justify-center text-center cursor-pointer shadow-md select-none focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary"
    >
      <div className="space-y-4 max-w-md mx-auto">
        <h2 className="text-4xl md:text-5xl font-extrabold text-text tracking-tight">
          {word}
        </h2>

        {/*
          The IPA and the speaker are one row, but the speaker no longer depends
          on the IPA being there. Both used to be inside a single `{ipa && ...}`,
          so a sense authored without a transcription silently lost its audio
          too — and no seeded sense carries an `audio_url` at all, which is why
          the control was never on screen. PronounceButton falls back to speech
          synthesis, so the button is now always the real thing.
        */}
        <div className="flex items-center justify-center gap-2">
          {ipa && (
            <span className="font-mono text-base text-primary-accent">
              {ipa}
            </span>
          )}
          <PronounceButton
            text={word}
            audioUrl={audioUrl}
            label={t("review.listenBtn", "Pronounce word")}
          />
        </div>

        <div className="pt-6">
          <span className="inline-flex items-center gap-1.5 text-xs font-semibold text-text-muted bg-surface-muted px-3 py-1.5 rounded-full">
            <RotateCw className="h-3.5 w-3.5" aria-hidden="true" />
            {t("review.flipHint", "Space to reveal answer")}
          </span>
        </div>
      </div>
    </div>
  );
};
