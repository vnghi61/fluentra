import React, { useState } from "react";
import { RotateCw, Volume2, VolumeX } from "lucide-react";
import { useTranslation } from "react-i18next";

import { Button } from "@/components/ui/button";

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
  const [audioError, setAudioError] = useState(false);
  const [isPlaying, setIsPlaying] = useState(false);

  const handlePlayAudio = (e: React.MouseEvent) => {
    e.stopPropagation();
    if (!audioUrl || audioError) return;

    try {
      setIsPlaying(true);
      const audio = new Audio(audioUrl);
      audio.onended = () => setIsPlaying(false);
      audio.onerror = () => {
        setAudioError(true);
        setIsPlaying(false);
      };
      void audio.play().catch(() => {
        setAudioError(true);
        setIsPlaying(false);
      });
    } catch {
      setAudioError(true);
      setIsPlaying(false);
    }
  };

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

        {ipa && (
          <div className="flex items-center justify-center gap-2">
            <span className="font-mono text-base text-primary-accent">
              {ipa}
            </span>
            {audioUrl && (
              <Button
                type="button"
                variant="ghost"
                size="sm"
                disabled={audioError || isPlaying}
                onClick={handlePlayAudio}
                className="h-11 w-11 p-0 rounded-full min-h-[44px] min-w-[44px]"
                title={
                  audioError
                    ? t("review.audioFailed", "Audio unavailable")
                    : t("review.listenBtn", "Pronounce word")
                }
              >
                {audioError ? (
                  <VolumeX
                    className="h-4 w-4 text-text-muted"
                    aria-hidden="true"
                  />
                ) : (
                  <Volume2
                    className={`h-4 w-4 text-primary-accent ${isPlaying ? "animate-pulse" : ""}`}
                    aria-hidden="true"
                  />
                )}
              </Button>
            )}
          </div>
        )}

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
