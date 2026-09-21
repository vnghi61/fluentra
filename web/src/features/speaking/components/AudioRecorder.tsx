import React, { useEffect, useRef, useState } from "react";
import {
  AlertCircle,
  CheckCircle2,
  Mic,
  ShieldCheck,
  Square,
  UploadCloud,
} from "lucide-react";
import { useTranslation } from "react-i18next";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";
import { problemCode, speakingApi } from "../api/speakingApi";

export interface AudioRecorderProps {
  taskType: "read_aloud" | "respond";
  promptText?: string | undefined;
  referenceText?: string | undefined;
  speakingTimeSeconds: number;
  mode: "exam" | "practice";
  currentRecordingKey?: string | undefined;
  onRecordingComplete: (objectKey: string) => void;
  onUpload?: ((blob: Blob) => Promise<string>) | undefined;
  disabled?: boolean | undefined;
  dailyRecordingsUsed?: number | undefined;
  dailyRecordingsLimit?: number | undefined;
  className?: string | undefined;
}

/**
 * Consent is read from and written to the server (BR-SPEAKING-03).
 *
 * It used to be a localStorage flag. That asked the learner once per browser and
 * kept no record at all: cleared site data forgot it, another device never knew
 * it, and nobody could say when it was given. Voice is biometric-adjacent
 * personal data, so the record outlives the browser profile — and the server
 * refuses a presigned upload URL without it, which is what makes the rule a rule
 * rather than a screen.
 */

export function pickSupportedMimeType(): string {
  if (typeof MediaRecorder === "undefined") return "";
  if (MediaRecorder.isTypeSupported("audio/webm;codecs=opus")) {
    return "audio/webm;codecs=opus";
  }
  if (MediaRecorder.isTypeSupported("audio/mp4")) return "audio/mp4";
  return "";
}

export const AudioRecorder: React.FC<AudioRecorderProps> = ({
  taskType,
  promptText,
  referenceText,
  speakingTimeSeconds,
  mode,
  currentRecordingKey,
  onRecordingComplete,
  onUpload,
  disabled = false,
  dailyRecordingsUsed,
  dailyRecordingsLimit,
  className,
}) => {
  const { t } = useTranslation();

  // Undefined until the read lands. Nothing offers to record before then: the
  // upload would be refused, and asking again after a "yes" is worse than
  // waiting a moment.
  const [noticeSeen, setNoticeSeen] = useState<boolean | undefined>(undefined);
  const [showNotice, setShowNotice] = useState(false);
  const [isRecording, setIsRecording] = useState(false);
  const [isUploading, setIsUploading] = useState(false);
  const [secondsLeft, setSecondsLeft] = useState(speakingTimeSeconds);
  const [prevPropKey, setPrevPropKey] = useState<string | undefined>(
    currentRecordingKey,
  );
  const [recordingKey, setRecordingKey] = useState<string | undefined>(
    currentRecordingKey,
  );

  // One read per mount. The answer is the server's, so a learner who consented
  // on their phone is not asked again on their laptop.
  useEffect(() => {
    let alive = true;
    speakingApi
      .getConsent()
      .then((consent) => {
        if (alive) setNoticeSeen(consent.consented);
      })
      .catch(() => {
        // A failed read is not evidence of consent: ask again rather than
        // letting the upload be refused with no explanation.
        if (alive) setNoticeSeen(false);
      });
    return () => {
      alive = false;
    };
  }, []);

  if (currentRecordingKey !== prevPropKey) {
    setPrevPropKey(currentRecordingKey);
    setRecordingKey(currentRecordingKey);
  }
  const [error, setError] = useState<string | null>(null);

  const recorderRef = useRef<MediaRecorder | null>(null);
  const chunksRef = useRef<Blob[]>([]);
  const timerRef = useRef<ReturnType<typeof setInterval> | null>(null);

  useEffect(() => {
    return () => {
      if (timerRef.current) clearInterval(timerRef.current);
      const recorder = recorderRef.current;
      if (recorder && recorder.state !== "inactive") recorder.stop();
    };
  }, []);

  const canRecord =
    !disabled && !isUploading && (mode === "practice" || !recordingKey);

  const upload = async (blob: Blob) => {
    setIsUploading(true);
    setError(null);
    try {
      let key = "";
      if (onUpload) {
        key = await onUpload(blob);
      } else {
        const intent = await speakingApi.getUploadIntent(
          blob.type || "audio/webm",
        );
        await speakingApi.uploadAudio(intent.upload_url, blob);
        key = intent.object_key;
      }
      setRecordingKey(key);
      onRecordingComplete(key);
    } catch (err: unknown) {
      setError(
        problemCode(err) === "SPEECH_DAILY_LIMIT_REACHED"
          ? t(
              "speaking.dailyLimitReached",
              "You have reached your daily speaking recordings limit.",
            )
          : t(
              "speaking.uploadFailed",
              "Failed to upload audio recording. Please try again.",
            ),
      );
    } finally {
      setIsUploading(false);
    }
  };

  const stopRecording = () => {
    if (timerRef.current) {
      clearInterval(timerRef.current);
      timerRef.current = null;
    }
    const recorder = recorderRef.current;
    if (recorder && recorder.state !== "inactive") recorder.stop();
    setIsRecording(false);
  };

  const startRecording = async () => {
    // Still reading. Showing the consent dialog here would ask a learner who has
    // already agreed, so wait for the answer instead.
    if (noticeSeen === undefined) return;
    if (!noticeSeen) {
      setShowNotice(true);
      return;
    }
    setError(null);
    chunksRef.current = [];

    if (!window.isSecureContext || !navigator.mediaDevices?.getUserMedia) {
      setError(
        t(
          "speaking.micNeedsHttps",
          "Microphone access requires a secure connection (HTTPS).",
        ),
      );
      return;
    }
    if (typeof MediaRecorder === "undefined") {
      setError(
        t(
          "speaking.recordingUnsupported",
          "Audio recording is not supported on this device/browser.",
        ),
      );
      return;
    }

    try {
      const stream = await navigator.mediaDevices.getUserMedia({ audio: true });
      const mimeType = pickSupportedMimeType();
      const recorder = new MediaRecorder(
        stream,
        mimeType ? { mimeType } : undefined,
      );
      recorderRef.current = recorder;
      recorder.ondataavailable = (e) => {
        if (e.data.size > 0) chunksRef.current.push(e.data);
      };
      recorder.onstop = () => {
        stream.getTracks().forEach((track) => track.stop());
        const blob = new Blob(chunksRef.current, {
          type: recorder.mimeType || "audio/webm",
        });
        void upload(blob);
      };
      recorder.start(250);
      setIsRecording(true);
      setSecondsLeft(speakingTimeSeconds);
      timerRef.current = setInterval(() => {
        setSecondsLeft((prev) => {
          if (prev <= 1) {
            stopRecording();
            return 0;
          }
          return prev - 1;
        });
      }, 1000);
    } catch {
      setError(
        t(
          "speaking.micPermissionDenied",
          "Microphone access was denied. Please allow microphone permissions in your browser.",
        ),
      );
    }
  };

  const acceptNotice = () => {
    // Optimistic on screen, recorded on the server. A failed write leaves the
    // flag unset, so the next upload attempt is refused and the notice returns —
    // which is the right way round for a consent record.
    setNoticeSeen(true);
    setShowNotice(false);
    void speakingApi.recordConsent().catch(() => {
      setNoticeSeen(false);
    });
  };

  return (
    <div
      className={cn(
        "space-y-4 rounded-xl border border-border bg-card p-5 shadow-sm",
        className,
      )}
    >
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div>
          <h4 className="text-base font-semibold text-text">
            {taskType === "read_aloud"
              ? t("speaking.readAloudTitle", "Read Aloud")
              : t("speaking.respondTitle", "Spoken Response")}
          </h4>
          <p className="text-xs text-text-muted">
            {taskType === "read_aloud"
              ? t(
                  "speaking.readAloudDesc",
                  "Read the text aloud clearly and naturally into your microphone.",
                )
              : t(
                  "speaking.respondDesc",
                  "Speak your response clearly answering the prompt.",
                )}
          </p>
        </div>
        <div className="flex items-center gap-2">
          {typeof dailyRecordingsLimit === "number" &&
            dailyRecordingsLimit > 0 && (
              <Badge variant="secondary" className="text-xs">
                {t("speaking.quotaBadge", "{{used}} / {{limit}} takes today", {
                  used: dailyRecordingsUsed ?? 0,
                  limit: dailyRecordingsLimit,
                })}
              </Badge>
            )}
          <Badge variant="outline">
            {t("speaking.timeLimit", "{{seconds}}s", {
              seconds: speakingTimeSeconds,
            })}
          </Badge>
        </div>
      </div>

      {taskType === "read_aloud" && referenceText && (
        <div className="rounded-lg border border-border-subtle bg-surface-muted p-4">
          <p className="text-sm font-medium leading-relaxed text-text sm:text-base">
            {referenceText}
          </p>
        </div>
      )}
      {promptText && (
        <p className="text-sm font-medium text-text-muted">{promptText}</p>
      )}

      {error && (
        <div
          role="alert"
          className="flex items-center gap-2 rounded-lg bg-danger/10 p-3 text-xs text-danger"
        >
          <AlertCircle className="h-4 w-4 shrink-0" aria-hidden="true" />
          <span>{error}</span>
        </div>
      )}

      <div className="flex flex-col items-center justify-center space-y-4 rounded-xl border border-border-subtle bg-surface-muted/40 p-6">
        <div
          className={cn(
            "font-mono text-3xl font-bold tracking-tight sm:text-4xl",
            isRecording ? "text-danger animate-pulse" : "text-text",
          )}
          aria-live="polite"
        >
          {`00:${secondsLeft.toString().padStart(2, "0")}`}
        </div>

        {isRecording ? (
          <Button
            type="button"
            variant="destructive"
            onClick={stopRecording}
            className="rounded-full px-6 gap-2"
          >
            <Square className="h-5 w-5" aria-hidden="true" />
            <span>{t("speaking.stop", "Stop Recording")}</span>
          </Button>
        ) : (
          <Button
            type="button"
            variant={recordingKey ? "outline" : "destructive"}
            onClick={() => void startRecording()}
            disabled={!canRecord}
            className="rounded-full px-6 gap-2"
          >
            <Mic className="h-5 w-5" aria-hidden="true" />
            <span>
              {recordingKey
                ? mode === "practice"
                  ? t("speaking.reRecord", "Record Again")
                  : t("speaking.recorded", "Recorded")
                : t("speaking.record", "Start Recording")}
            </span>
          </Button>
        )}

        {isUploading && (
          <p className="flex items-center gap-2 text-xs font-medium text-primary">
            <UploadCloud
              className="h-4 w-4 animate-bounce"
              aria-hidden="true"
            />
            {t("speaking.uploading", "Uploading recording...")}
          </p>
        )}
        {recordingKey && !isUploading && !isRecording && (
          <p className="flex items-center gap-1.5 text-xs font-medium text-success">
            <CheckCircle2 className="h-4 w-4" aria-hidden="true" />
            {t("speaking.uploaded", "Recording ready")}
          </p>
        )}
      </div>

      {showNotice && (
        <div
          role="dialog"
          aria-modal="true"
          aria-labelledby="voice-notice-title"
          className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 p-4"
        >
          <div className="w-full max-w-md space-y-4 rounded-2xl border border-border bg-card p-6 shadow-xl">
            <div className="flex items-center gap-3">
              <div className="flex h-11 w-11 shrink-0 items-center justify-center rounded-xl bg-primary/10 text-primary">
                <ShieldCheck className="h-6 w-6" aria-hidden="true" />
              </div>
              <h3
                id="voice-notice-title"
                className="text-base font-semibold text-text"
              >
                {t("speaking.noticeTitle", "Voice Recording Consent")}
              </h3>
            </div>
            <div className="space-y-2 rounded-lg border border-border-subtle bg-surface-muted/50 p-3.5 text-xs leading-relaxed text-text-muted">
              <p>
                {t(
                  "speaking.noticeTranscription",
                  "Your voice will be recorded and transcribed for automated grading and educational feedback.",
                )}
              </p>
              <p>
                {t(
                  "speaking.noticeRetention",
                  "Audio recordings are securely stored and automatically purged after 90 days. Scores and feedback remain in your history.",
                )}
              </p>
            </div>
            <div className="flex justify-end gap-2.5">
              <Button
                type="button"
                variant="outline"
                onClick={() => setShowNotice(false)}
              >
                {t("speaking.noticeCancel", "Cancel")}
              </Button>
              <Button type="button" onClick={acceptNotice}>
                {t("speaking.noticeAccept", "I Consent & Continue")}
              </Button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
};
