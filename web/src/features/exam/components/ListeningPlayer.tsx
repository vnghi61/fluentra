import React, { useRef, useState } from "react";
import { AlertCircle, Headphones, Loader2, Pause, Play, Volume2 } from "lucide-react";
import { useTranslation } from "react-i18next";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";
import { examApi } from "../api/examApi";

export interface ListeningPlayerProps {
  versionId: string;
  contextId: string;
  title?: string | undefined;
  className?: string | undefined;
}

export const ListeningPlayer: React.FC<ListeningPlayerProps> = ({
  versionId,
  contextId,
  title,
  className,
}) => {
  const { t } = useTranslation();
  const audioRef = useRef<HTMLAudioElement | null>(null);

  const [audioUrl, setAudioUrl] = useState<string | null>(null);
  const [isPlaying, setIsPlaying] = useState<boolean>(false);
  const [isLoading, setIsLoading] = useState<boolean>(false);
  const [currentTime, setCurrentTime] = useState<number>(0);
  const [duration, setDuration] = useState<number>(0);
  const [playsRemaining, setPlaysRemaining] = useState<number | null>(null);
  const [playsAllowed, setPlaysAllowed] = useState<number>(2);
  const [errorMsg, setErrorMsg] = useState<string | null>(null);

  const formatTime = (secs: number): string => {
    if (isNaN(secs) || secs < 0) return "00:00";
    const m = Math.floor(secs / 60);
    const s = Math.floor(secs % 60);
    return `${m.toString().padStart(2, "0")}:${s.toString().padStart(2, "0")}`;
  };

  const handlePlayToggle = async () => {
    setErrorMsg(null);

    // If currently playing, allow pausing
    if (isPlaying && audioRef.current) {
      audioRef.current.pause();
      setIsPlaying(false);
      return;
    }

    // If audio already loaded and has duration, resume
    if (audioUrl && audioRef.current) {
      try {
        await audioRef.current.play();
        setIsPlaying(true);
      } catch (err: any) {
        setErrorMsg(err?.message || "Failed to play audio");
      }
      return;
    }

    // First time play -> request presigned URL and decrement play count on server
    setIsLoading(true);
    try {
      const res = await examApi.getListeningPlay(versionId, "exam", contextId);
      setAudioUrl(res.audio_url);
      setPlaysRemaining(Math.max(0, res.plays_allowed - res.plays_used));
      setPlaysAllowed(res.plays_allowed);

      if (audioRef.current) {
        audioRef.current.src = res.audio_url;
        await audioRef.current.play();
        setIsPlaying(true);
      }
    } catch (err: any) {
      const isLimit =
        err?.problem?.code === "PLAY_LIMIT_REACHED" ||
        err?.status === 403 ||
        err?.message?.includes("PLAY_LIMIT_REACHED");
      if (isLimit) {
        setPlaysRemaining(0);
        setErrorMsg(t("exam.listening.limitReached", "Maximum play limit reached for this audio track."));
      } else {
        setErrorMsg(err?.message || t("exam.listening.loadFailed", "Unable to load audio track."));
      }
    } finally {
      setIsLoading(false);
    }
  };

  const handleTimeUpdate = () => {
    if (audioRef.current) {
      setCurrentTime(audioRef.current.currentTime);
    }
  };

  const handleLoadedMetadata = () => {
    if (audioRef.current) {
      setDuration(audioRef.current.duration);
    }
  };

  const handleEnded = () => {
    setIsPlaying(false);
    setCurrentTime(0);
  };

  const progressPercent = duration > 0 ? (currentTime / duration) * 100 : 0;
  const isPlayDisabled = isLoading || (playsRemaining !== null && playsRemaining <= 0 && !audioUrl);

  return (
    <div
      className={cn(
        "rounded-xl border border-border bg-card p-4 sm:p-5 shadow-sm space-y-4",
        className,
      )}
    >
      <audio
        ref={audioRef}
        onTimeUpdate={handleTimeUpdate}
        onLoadedMetadata={handleLoadedMetadata}
        onEnded={handleEnded}
        onError={() => {
          setIsPlaying(false);
          setIsLoading(false);
        }}
      />

      {/* Header with Title & Plays Badge */}
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div className="flex items-center gap-2.5">
          <div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-lg bg-primary/10 text-primary">
            <Headphones className="h-5 w-5" />
          </div>
          <div>
            <h4 className="font-semibold text-text text-base">
              {title || t("exam.listening.audioClip", "Listening Audio Track")}
            </h4>
            <p className="text-xs text-text-muted">
              {t("exam.listening.instructions", "Listen carefully to answer the following questions.")}
            </p>
          </div>
        </div>

        <Badge
          variant={playsRemaining === 0 ? "danger" : "outline"}
          className="text-xs px-3 py-1 font-medium gap-1.5"
        >
          <Volume2 className="h-3.5 w-3.5" />
          {playsRemaining !== null
            ? t("exam.listening.playsLeft", {
                count: playsRemaining,
                total: playsAllowed,
                defaultValue: `${playsRemaining}/${playsAllowed} plays left`,
              })
            : t("exam.listening.maxPlays", { count: playsAllowed, defaultValue: `Up to ${playsAllowed} plays` })}
        </Badge>
      </div>

      {/* Player Controls Bar */}
      <div className="flex items-center gap-4 bg-surface-muted/50 rounded-lg p-3">
        <Button
          type="button"
          onClick={handlePlayToggle}
          disabled={isPlayDisabled}
          aria-label={isPlaying ? t("common.pause", "Pause") : t("common.play", "Play")}
          className={cn(
            "h-12 w-12 rounded-full shrink-0 flex items-center justify-center min-h-[44px] min-w-[44px] shadow-sm",
            isPlaying ? "bg-accent hover:bg-accent/90" : "bg-primary hover:bg-primary/90",
          )}
        >
          {isLoading ? (
            <Loader2 className="h-5 w-5 animate-spin text-white" />
          ) : isPlaying ? (
            <Pause className="h-5 w-5 text-white" />
          ) : (
            <Play className="h-5 w-5 fill-current text-white translate-x-0.5" />
          )}
        </Button>

        {/* Progress Display */}
        <div className="flex-1 min-w-0 space-y-1.5">
          <div className="h-2 w-full bg-border-subtle rounded-full overflow-hidden">
            <div
              className="h-full bg-primary transition-all duration-150 rounded-full"
              style={{ width: `${progressPercent}%` }}
            />
          </div>
          <div className="flex justify-between text-xs font-mono text-text-muted">
            <span>{formatTime(currentTime)}</span>
            <span>{formatTime(duration)}</span>
          </div>
        </div>
      </div>

      {errorMsg && (
        <div className="flex items-center gap-2 text-xs text-danger bg-danger/10 p-2.5 rounded-lg">
          <AlertCircle className="h-4 w-4 shrink-0" />
          <span>{errorMsg}</span>
        </div>
      )}
    </div>
  );
};
