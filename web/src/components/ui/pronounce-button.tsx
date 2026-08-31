import React, { useCallback, useEffect, useRef, useState } from "react";
import { Volume2, VolumeX } from "lucide-react";
import { useTranslation } from "react-i18next";

import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";

/**
 * Speaks a word or a sentence out loud.
 *
 * Two sources, in that order:
 *
 *  1. `audioUrl` — a recorded asset, when the content version carries one.
 *  2. The browser's own speech synthesis, on the text itself.
 *
 * The fallback is the reason this component exists. The previous button lived
 * inside `FlashcardFront` and rendered only when the body had both an `ipa` and
 * an `audio_url`. No seeded sense has ever carried an `audio_url` — the column
 * is `words.audio_asset_id` and nothing populates it — so on every card in the
 * curated deck the control was absent, and a learner looking at `/dɪˈlɪʃ.əs/`
 * had no way to hear it. Synthesis is not as good as a recording, but it is the
 * difference between a pronunciation feature and no pronunciation feature, and
 * it costs nothing to serve.
 *
 * When neither source is usable the button renders disabled with a struck-out
 * icon rather than disappearing, so the absence is legible instead of looking
 * like a layout that forgot something.
 */
export interface PronounceButtonProps {
  /** The text to speak. Required — it is what synthesis falls back to. */
  text: string;
  /** A recorded pronunciation, preferred over synthesis when it plays. */
  audioUrl?: string | null | undefined;
  /** BCP-47 tag handed to the synthesiser. The material is English. */
  lang?: string;
  size?: "sm" | "md";
  className?: string;
  /** Overrides the default title/aria-label, e.g. "Listen to the sentence". */
  label?: string;
}

/** Whether this browser can synthesise speech at all. */
function canSynthesise(): boolean {
  return (
    typeof window !== "undefined" &&
    "speechSynthesis" in window &&
    typeof window.SpeechSynthesisUtterance === "function"
  );
}

export const PronounceButton: React.FC<PronounceButtonProps> = ({
  text,
  audioUrl,
  lang = "en-US",
  size = "sm",
  className,
  label,
}) => {
  const { t } = useTranslation();
  const [isPlaying, setIsPlaying] = useState(false);
  const [failed, setFailed] = useState(false);
  const audioRef = useRef<HTMLAudioElement | null>(null);

  // A card can be advanced mid-utterance. Without this, the previous word keeps
  // talking over the next one, which is worse than silence.
  useEffect(() => {
    return () => {
      audioRef.current?.pause();
      audioRef.current = null;
      if (canSynthesise()) window.speechSynthesis.cancel();
    };
  }, [text, audioUrl]);

  const speak = useCallback(() => {
    if (!canSynthesise()) {
      setFailed(true);
      setIsPlaying(false);
      return;
    }
    try {
      // Cancel first: queued utterances otherwise pile up on repeated taps and
      // play back-to-back long after the learner has moved on.
      window.speechSynthesis.cancel();
      const utterance = new SpeechSynthesisUtterance(text);
      utterance.lang = lang;
      utterance.rate = 0.9;
      utterance.onend = () => setIsPlaying(false);
      utterance.onerror = () => {
        setFailed(true);
        setIsPlaying(false);
      };
      setIsPlaying(true);
      window.speechSynthesis.speak(utterance);
    } catch {
      setFailed(true);
      setIsPlaying(false);
    }
  }, [text, lang]);

  const handleClick = useCallback(
    (e: React.MouseEvent) => {
      // These buttons sit inside cards that flip on click. Without this, hearing
      // the word also turns the card over and gives away the answer.
      e.stopPropagation();
      e.preventDefault();

      if (!text.trim()) return;

      if (!audioUrl) {
        speak();
        return;
      }

      try {
        const audio = new Audio(audioUrl);
        audioRef.current = audio;
        audio.onended = () => setIsPlaying(false);
        // A broken or missing asset falls through to synthesis rather than
        // reporting failure: the learner wanted to hear the word, and the
        // reason the recording is unavailable is not their problem.
        audio.onerror = () => speak();
        setIsPlaying(true);
        void audio.play().catch(() => speak());
      } catch {
        speak();
      }
    },
    [audioUrl, speak, text],
  );

  const unavailable = failed || !text.trim();
  const title =
    label ??
    (unavailable
      ? t("pronounce.unavailable", "Pronunciation unavailable")
      : t("pronounce.listen", "Listen to the pronunciation"));

  return (
    <Button
      type="button"
      variant="ghost"
      size={size}
      disabled={unavailable}
      onClick={handleClick}
      onKeyDown={(e) => e.stopPropagation()}
      title={title}
      aria-label={title}
      className={cn(
        "h-11 w-11 shrink-0 rounded-full p-0 min-h-[44px] min-w-[44px]",
        className,
      )}
    >
      {unavailable ? (
        <VolumeX className="h-4 w-4 text-text-muted" aria-hidden="true" />
      ) : (
        <Volume2
          className={cn(
            "h-4 w-4 text-primary-accent",
            isPlaying && "animate-pulse",
          )}
          aria-hidden="true"
        />
      )}
    </Button>
  );
};
