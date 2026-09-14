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
import { examApi, problemCode } from "../api/examApi";

export interface SpeakingRecorderProps {
  taskType: "read_aloud" | "respond";
  promptText?: string | undefined;
  referenceText?: string | undefined;
  speakingTimeSeconds: number;
  mode: "exam" | "practice";
  currentRecordingKey?: string | undefined;
  onRecordingComplete: (objectKey: string) => void;
  className?: string | undefined;
}

const NOTICE_STORAGE_KEY = "fluentra_voice_notice_seen";

function readNoticeSeen(): boolean {
  try {
    return localStorage.getItem(NOTICE_STORAGE_KEY) === "true";
  } catch {
    return false;
  }
}

function rememberNoticeSeen(): void {
  try {
    localStorage.setItem(NOTICE_STORAGE_KEY, "true");
  } catch {
    // The notice shows again next time; nothing else depends on it.
  }
}

function pickMimeType(): string {
  if (typeof MediaRecorder === "undefined") return "";
  if (MediaRecorder.isTypeSupported("audio/webm;codecs=opus")) {
    return "audio/webm;codecs=opus";
  }
  if (MediaRecorder.isTypeSupported("audio/mp4")) return "audio/mp4";
  return "";
}

export const SpeakingRecorder: React.FC<SpeakingRecorderProps> = ({
  taskType,
  promptText,
  referenceText,
  speakingTimeSeconds,
  mode,
  currentRecordingKey,
  onRecordingComplete,
  className,
}) => {
  const { t } = useTranslation();

  const [noticeSeen, setNoticeSeen] = useState(readNoticeSeen);
  const [showNotice, setShowNotice] = useState(false);
  const [isRecording, setIsRecording] = useState(false);
  const [isUploading, setIsUploading] = useState(false);
  const [secondsLeft, setSecondsLeft] = useState(speakingTimeSeconds);
  const [recordingKey, setRecordingKey] = useState<string | undefined>(
    currentRecordingKey,
  );
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

  // Exam mode records once; practice mode may record again.
  const canRecord = !isUploading && (mode === "practice" || !recordingKey);

  const upload = async (blob: Blob) => {
    setIsUploading(true);
    try {
      const intent = await examApi.getSpeakingUploadIntent(
        blob.type || "audio/webm",
      );
      await examApi.uploadSpeakingAudio(intent.upload_url, blob);
      setRecordingKey(intent.object_key);
      onRecordingComplete(intent.object_key);
    } catch (err: unknown) {
      setError(
        problemCode(err) === "SPEECH_DAILY_LIMIT_REACHED"
          ? t("exam.speaking.dailyLimitReached")
          : t("exam.speaking.uploadFailed"),
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
    if (!noticeSeen) {
      setShowNotice(true);
      return;
    }
    setError(null);
    chunksRef.current = [];
    try {
      const stream = await navigator.mediaDevices.getUserMedia({ audio: true });
      const mimeType = pickMimeType();
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
      setError(t("exam.speaking.micPermissionDenied"));
    }
  };

  const acceptNotice = () => {
    rememberNoticeSeen();
    setNoticeSeen(true);
    setShowNotice(false);
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
              ? t("exam.speaking.readAloudTitle")
              : t("exam.speaking.respondTitle")}
          </h4>
          <p className="text-xs text-text-muted">
            {taskType === "read_aloud"
              ? t("exam.speaking.readAloudDesc")
              : t("exam.speaking.respondDesc")}
          </p>
        </div>
        <Badge variant="outline">
          {t("exam.speaking.timeLimit", { seconds: speakingTimeSeconds })}
        </Badge>
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
            isRecording ? "text-danger" : "text-text",
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
            className="rounded-full px-6"
          >
            <Square className="h-5 w-5" aria-hidden="true" />
            <span>{t("exam.speaking.stop")}</span>
          </Button>
        ) : (
          <Button
            type="button"
            variant={recordingKey ? "outline" : "destructive"}
            onClick={() => void startRecording()}
            disabled={!canRecord}
            className="rounded-full px-6"
          >
            <Mic className="h-5 w-5" aria-hidden="true" />
            <span>
              {recordingKey
                ? mode === "practice"
                  ? t("exam.speaking.reRecord")
                  : t("exam.speaking.recorded")
                : t("exam.speaking.record")}
            </span>
          </Button>
        )}

        {isUploading && (
          <p className="flex items-center gap-2 text-xs font-medium text-primary">
            <UploadCloud className="h-4 w-4" aria-hidden="true" />
            {t("exam.speaking.uploading")}
          </p>
        )}
        {recordingKey && !isUploading && !isRecording && (
          <p className="flex items-center gap-1.5 text-xs font-medium text-success">
            <CheckCircle2 className="h-4 w-4" aria-hidden="true" />
            {t("exam.speaking.uploaded")}
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
                {t("exam.speaking.noticeTitle")}
              </h3>
            </div>
            <div className="space-y-2 rounded-lg border border-border-subtle bg-surface-muted/50 p-3.5 text-xs leading-relaxed text-text-muted">
              <p>{t("exam.speaking.noticeTranscription")}</p>
              <p>{t("exam.speaking.noticeRetention")}</p>
            </div>
            <div className="flex justify-end gap-2.5">
              <Button
                type="button"
                variant="outline"
                onClick={() => setShowNotice(false)}
              >
                {t("exam.speaking.noticeCancel")}
              </Button>
              <Button type="button" onClick={acceptNotice}>
                {t("exam.speaking.noticeAccept")}
              </Button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
};
