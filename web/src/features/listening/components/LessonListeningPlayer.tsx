import React, { useRef, useState } from "react";
import { AlertCircle, Headphones, Loader2, Pause, Play } from "lucide-react";
import { useTranslation } from "react-i18next";

import { ApiError } from "@/api/client";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { reachableStorageUrl } from "@/lib/storage-url";
import { cn } from "@/lib/utils";
import { listeningApi } from "../api/listeningApi";

export interface LessonListeningPlayerProps {
  /** Content version of the listening item. */
  versionId: string;
  /** The learner's in-progress attempt at it; plays are counted against this. */
  attemptId: string | null;
  title?: string | undefined;
  className?: string | undefined;
}

function formatTime(secs: number): string {
  if (!Number.isFinite(secs) || secs < 0) return "00:00";
  const m = Math.floor(secs / 60);
  const s = Math.floor(secs % 60);
  return `${m.toString().padStart(2, "0")}:${s.toString().padStart(2, "0")}`;
}

/**
 * The clip, in a lesson.
 *
 * A learning attempt gets three plays, counted by the server; the URL it hands
 * back expires shortly after the clip ends, so a play is spent when it starts.
 * Pausing and resuming inside one play does not spend another, which is why the
 * source is kept until the audio ends rather than re-fetched on every press.
 *
 * Audio is rendered offline by `cmd/tts`. An item whose clip has not been
 * rendered answers 409 AUDIO_NOT_READY, and this says so instead of showing a
 * button that does nothing.
 */
export const LessonListeningPlayer: React.FC<LessonListeningPlayerProps> = ({
  versionId,
  attemptId,
  title,
  className,
}) => {
  const { t } = useTranslation();
  const audioRef = useRef<HTMLAudioElement | null>(null);

  const [hasSource, setHasSource] = useState(false);
  const [isPlaying, setIsPlaying] = useState(false);
  const [isLoading, setIsLoading] = useState(false);
  const [currentTime, setCurrentTime] = useState(0);
  const [duration, setDuration] = useState(0);
  const [playsLeft, setPlaysLeft] = useState<number | null>(null);
  const [error, setError] = useState<string | null>(null);

  const startPlay = async () => {
    if (!attemptId) {
      // A guest, or the moment before the attempt has been started. Neither is
      // an error the learner caused, so it reads as "not yet" rather than as a
      // failure.
      setError(
        t(
          "runner.listening.needsAttempt",
          "Sign in to play the recording — plays are counted against your attempt.",
        ),
      );
      return;
    }
    setIsLoading(true);
    try {
      const res = await listeningApi.play(versionId, attemptId);
      setPlaysLeft(Math.max(0, res.plays_allowed - res.plays_used));
      const audio = audioRef.current;
      if (audio) {
        audio.src = reachableStorageUrl(res.audio_url);
        setHasSource(true);
        await audio.play();
        setIsPlaying(true);
      }
    } catch (err: unknown) {
      const code = err instanceof ApiError ? err.problem.code : undefined;
      if (code === "PLAY_LIMIT_REACHED") {
        setPlaysLeft(0);
        setError(
          t("runner.listening.limitReached", "You have used all your plays."),
        );
      } else if (code === "AUDIO_NOT_READY") {
        setError(
          t(
            "runner.listening.notReady",
            "This recording has not been produced yet. Skip it and try another exercise.",
          ),
        );
      } else {
        setError(
          t(
            "runner.listening.loadFailed",
            "The recording could not be played.",
          ),
        );
      }
    } finally {
      setIsLoading(false);
    }
  };

  const handleToggle = async () => {
    setError(null);
    const audio = audioRef.current;
    if (isPlaying && audio) {
      audio.pause();
      setIsPlaying(false);
      return;
    }
    if (hasSource && audio) {
      try {
        await audio.play();
        setIsPlaying(true);
      } catch {
        setError(
          t(
            "runner.listening.loadFailed",
            "The recording could not be played.",
          ),
        );
      }
      return;
    }
    await startPlay();
  };

  const handleEnded = () => {
    // The play is over; the next one goes back to the server.
    setIsPlaying(false);
    setHasSource(false);
    setCurrentTime(0);
  };

  const progress = duration > 0 ? (currentTime / duration) * 100 : 0;
  const disabled = isLoading || (playsLeft === 0 && !hasSource);

  return (
    <div
      className={cn(
        "space-y-4 rounded-2xl border border-border bg-surface-card p-4 shadow-sm sm:p-5",
        className,
      )}
    >
      <audio
        ref={audioRef}
        onTimeUpdate={() => setCurrentTime(audioRef.current?.currentTime ?? 0)}
        onLoadedMetadata={() => setDuration(audioRef.current?.duration ?? 0)}
        onEnded={handleEnded}
        onError={() => {
          setIsPlaying(false);
          setIsLoading(false);
        }}
      />

      <div className="flex flex-wrap items-center justify-between gap-3">
        <div className="flex items-center gap-2.5">
          <div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-lg bg-primary/10 text-primary">
            <Headphones className="h-5 w-5" aria-hidden="true" />
          </div>
          <div>
            <h4 className="text-base font-semibold text-text">
              {title || t("runner.listening.audioClip", "Audio clip")}
            </h4>
            <p className="text-xs text-text-muted">
              {t(
                "runner.listening.instructions",
                "Listen, then answer the questions below.",
              )}
            </p>
          </div>
        </div>
        {playsLeft !== null && (
          <Badge variant={playsLeft === 0 ? "danger" : "outline"}>
            {t("runner.listening.playsLeft", {
              count: playsLeft,
              defaultValue: "Plays left: {{count}}",
            })}
          </Badge>
        )}
      </div>

      <div className="flex items-center gap-4 rounded-lg bg-surface-muted/50 p-3">
        <Button
          type="button"
          onClick={() => void handleToggle()}
          disabled={disabled}
          aria-label={
            isPlaying
              ? t("runner.listening.pause", "Pause")
              : t("runner.listening.play", "Play")
          }
          className="h-12 min-h-[44px] w-12 min-w-[44px] shrink-0 rounded-full"
        >
          {isLoading ? (
            <Loader2 className="h-5 w-5 animate-spin" aria-hidden="true" />
          ) : isPlaying ? (
            <Pause className="h-5 w-5" aria-hidden="true" />
          ) : (
            <Play className="h-5 w-5" aria-hidden="true" />
          )}
        </Button>
        <div className="min-w-0 flex-1 space-y-1.5">
          <div className="h-2 w-full overflow-hidden rounded-full bg-border-subtle">
            <div
              className="h-full rounded-full bg-primary transition-all duration-150"
              style={{ width: `${progress}%` }}
            />
          </div>
          <div className="flex justify-between font-mono text-xs text-text-muted">
            <span>{formatTime(currentTime)}</span>
            <span>{formatTime(duration)}</span>
          </div>
        </div>
      </div>

      {error && (
        <div
          role="alert"
          className="flex items-center gap-2 rounded-lg bg-danger/10 p-2.5 text-xs text-danger"
        >
          <AlertCircle className="h-4 w-4 shrink-0" aria-hidden="true" />
          <span>{error}</span>
        </div>
      )}
    </div>
  );
};
