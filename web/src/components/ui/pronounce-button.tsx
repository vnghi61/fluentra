import React, { useCallback, useEffect, useRef, useState } from "react";
import { createPortal } from "react-dom";
import { Volume2, VolumeX } from "lucide-react";
import { useTranslation } from "react-i18next";

import { Button } from "@/components/ui/button";
import {
  creditLabel,
  findRecordings,
  isSingleWord,
  playFirstRecording,
  type Recording,
} from "@/lib/recorded-pronunciation";
import { cancelSpeech, speakText } from "@/lib/speech";
import { reachableStorageUrl } from "@/lib/storage-url";
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
  /**
   * The page crediting the recording. A recording whose licence requires
   * attribution must not play without it, so an `audioUrl` with no attribution
   * is treated as no recording at all.
   */
  audioAttribution?: string | null | undefined;
  /** The licence's short name, shown beside the credit. */
  audioLicence?: string | null | undefined;
  /** BCP-47 tag handed to the synthesiser. The material is English. */
  lang?: string;
  size?: "sm" | "md";
  className?: string;
  /** Overrides the default title/aria-label, e.g. "Listen to the sentence". */
  label?: string;
}

export const PronounceButton: React.FC<PronounceButtonProps> = ({
  text,
  audioUrl,
  audioAttribution,
  audioLicence,
  lang = "en-US",
  size = "sm",
  className,
  label,
}) => {
  const { t } = useTranslation();
  const [isPlaying, setIsPlaying] = useState(false);
  const [failed, setFailed] = useState(false);
  // Nothing could be heard: neither the device's voice nor a recording. The
  // button stays usable — this is usually the phone's media volume or its
  // text-to-speech service, which the learner can fix.
  const [silent, setSilent] = useState(false);
  // The recording that is playing, credited while it does (CC BY-SA).
  const [credit, setCredit] = useState<Recording | null>(null);
  const audioRef = useRef<HTMLAudioElement | null>(null);
  const stopRecordingRef = useRef<(() => void) | null>(null);
  // Bumped by every tap and every card change, so a recording looked up for an
  // earlier tap or an earlier card never starts playing late.
  const requestRef = useRef(0);

  const stopRecording = () => {
    stopRecordingRef.current?.();
    stopRecordingRef.current = null;
  };

  // A recording whose licence requires attribution is not playable without it:
  // playing the file would breach CC BY-SA. The card model drops such a URL
  // before it gets here; this is the second lock on the same door.
  const playableAudioUrl =
    audioUrl && audioAttribution?.trim() ? audioUrl : null;

  // A card can be advanced mid-utterance. Without this, the previous word keeps
  // talking over the next one, which is worse than silence.
  useEffect(() => {
    const requests = requestRef;
    return () => {
      requests.current++;
      audioRef.current?.pause();
      audioRef.current = null;
      stopRecordingRef.current?.();
      stopRecordingRef.current = null;
      cancelSpeech();
    };
  }, [text, audioUrl]);

  // A new card is a new word: the previous card's failure must not disable this
  // card's button, and its credit line must not outlive it.
  useEffect(() => {
    setFailed(false);
    setSilent(false);
    setCredit(null);
  }, [text, audioUrl]);

  /**
   * The device could not speak: play a human recording of the word instead.
   * Only then, so a third party is asked only once the device has failed.
   */
  const playRecording = useCallback(
    (reason: "silent" | "failed") => {
      const request = requestRef.current;
      const giveUp = () => {
        setIsPlaying(false);
        if (reason === "failed") setFailed(true);
        else setSilent(true);
      };
      if (!isSingleWord(text)) {
        giveUp();
        return;
      }
      setIsPlaying(true);
      void findRecordings(text).then((recordings) => {
        if (request !== requestRef.current) return;
        stopRecordingRef.current?.();
        stopRecordingRef.current = playFirstRecording(recordings, {
          onPlaying: (recording) => setCredit(recording),
          onEnded: () => setIsPlaying(false),
          onExhausted: giveUp,
        });
      });
    },
    [text],
  );

  const speak = useCallback(() => {
    speakText(text, {
      lang,
      onStart: () => setIsPlaying(true),
      onEnd: () => setIsPlaying(false),
      onFailure: () => playRecording("failed"),
      onSilent: () => playRecording("silent"),
    });
  }, [text, lang, playRecording]);

  const handleClick = useCallback(
    (e: React.MouseEvent) => {
      // These buttons sit inside cards that flip on click. Without this, hearing
      // the word also turns the card over and gives away the answer.
      e.stopPropagation();
      e.preventDefault();

      if (!text.trim()) return;

      requestRef.current++;
      stopRecording();
      setSilent(false);
      setCredit(null);

      if (!playableAudioUrl) {
        speak();
        return;
      }

      try {
        const audio = new Audio(reachableStorageUrl(playableAudioUrl));
        audioRef.current = audio;
        // The credit appears only once the recording actually plays, so a dead
        // link shows synthesis and no licence line it does not owe.
        audio.onplaying = () => {
          setCredit({
            url: playableAudioUrl,
            creditUrl: audioAttribution ?? "",
            licence: audioLicence?.trim() ? audioLicence : undefined,
          });
        };
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
    [audioAttribution, audioLicence, playableAudioUrl, speak, text],
  );

  useEffect(() => {
    if (!silent) return undefined;
    const hide = setTimeout(() => setSilent(false), 6000);
    return () => clearTimeout(hide);
  }, [silent]);

  useEffect(() => {
    if (!credit) return undefined;
    const hide = setTimeout(() => setCredit(null), 8000);
    return () => clearTimeout(hide);
  }, [credit]);

  const unavailable = failed || !text.trim();
  const title =
    label ??
    (unavailable
      ? t("pronounce.unavailable", "Pronunciation unavailable")
      : t("pronounce.listen", "Listen to the pronunciation"));

  return (
    <span className="inline-flex shrink-0">
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
      {silent &&
        // Portalled to <body>: speakers sit at card edges, inside
        // overflow-hidden rows and inside the flip card's 3D transform, which
        // turns `fixed` into "relative to the card". None of those can clip it.
        createPortal(
          <span
            role="status"
            className="pointer-events-none fixed inset-x-4 bottom-24 z-50 mx-auto max-w-sm rounded-lg border border-border-subtle bg-surface-card px-3 py-2 text-left text-sm text-text shadow-lg"
          >
            {t(
              "pronounce.silent",
              "No sound? Turn up your phone's media volume, or install an English voice in its text-to-speech settings.",
            )}
          </span>,
          document.body,
        )}
      {credit &&
        createPortal(
          <span
            role="status"
            className="fixed inset-x-4 bottom-24 z-50 mx-auto max-w-sm rounded-lg border border-border-subtle bg-surface-card px-3 py-2 text-left text-xs text-text-muted shadow-lg"
          >
            {t("pronounce.recordingCredit", "Audio")}
            {": "}
            <a
              href={credit.creditUrl}
              target="_blank"
              rel="noopener noreferrer"
              className="inline-flex min-h-[44px] items-center font-medium text-primary-accent underline"
            >
              {creditLabel(credit)}
              {credit.licence ? ` · ${credit.licence}` : ""}
            </a>
          </span>,
          document.body,
        )}
    </span>
  );
};
