import React, { useEffect, useRef, useState } from "react";
import {
  AlertCircle,
  CheckCircle2,
  Mic,
  RotateCcw,
  ShieldCheck,
  Square,
  UploadCloud,
  Volume2,
} from "lucide-react";
import { useTranslation } from "react-i18next";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";
import { examApi } from "../api/examApi";

export interface SpeakingRecorderProps {
  taskType: "read_aloud" | "respond";
  promptText: string;
  referenceText?: string | undefined;
  speakingTimeSeconds?: number | undefined;
  mode?: "exam" | "practice" | undefined;
  currentRecordingKey?: string | undefined;
  onRecordingComplete: (key: string) => void;
  className?: string | undefined;
}

const CONSENT_STORAGE_KEY = "fluentra_voice_consent_accepted";

export const SpeakingRecorder: React.FC<SpeakingRecorderProps> = ({
  taskType,
  promptText,
  referenceText,
  speakingTimeSeconds = 45,
  mode = "exam",
  currentRecordingKey,
  onRecordingComplete,
  className,
}) => {
  const { t } = useTranslation();

  const [hasConsent, setHasConsent] = useState<boolean>(() => {
    return localStorage.getItem(CONSENT_STORAGE_KEY) === "true";
  });
  const [showConsentModal, setShowConsentModal] = useState<boolean>(false);

  const [isRecording, setIsRecording] = useState<boolean>(false);
  const [isUploading, setIsUploading] = useState<boolean>(false);
  const [secondsRemaining, setSecondsRemaining] = useState<number>(speakingTimeSeconds);
  const [recordingKey, setRecordingKey] = useState<string | null>(
    currentRecordingKey || null,
  );
  const [audioUrl, setAudioUrl] = useState<string | null>(null);
  const [permissionError, setPermissionError] = useState<string | null>(null);
  const [hasTaken, setHasTaken] = useState<boolean>(!!currentRecordingKey);

  const mediaRecorderRef = useRef<MediaRecorder | null>(null);
  const audioChunksRef = useRef<Blob[]>([]);
  const timerIntervalRef = useRef<NodeJS.Timeout | null>(null);

  // Clean up recording timer and object URLs
  useEffect(() => {
    return () => {
      if (timerIntervalRef.current) {
        clearInterval(timerIntervalRef.current);
      }
      if (audioUrl) {
        URL.revokeObjectURL(audioUrl);
      }
    };
  }, [audioUrl]);

  const acceptConsent = () => {
    localStorage.setItem(CONSENT_STORAGE_KEY, "true");
    setHasConsent(true);
    setShowConsentModal(false);
  };

  const startRecording = async () => {
    if (!hasConsent) {
      setShowConsentModal(true);
      return;
    }

    setPermissionError(null);
    audioChunksRef.current = [];

    try {
      const stream = await navigator.mediaDevices.getUserMedia({ audio: true });
      let mimeType = "audio/webm;codecs=opus";
      if (!MediaRecorder.isTypeSupported(mimeType)) {
        mimeType = MediaRecorder.isTypeSupported("audio/mp4")
          ? "audio/mp4"
          : "";
      }

      const options = mimeType ? { mimeType } : undefined;
      const mediaRecorder = new MediaRecorder(stream, options);
      mediaRecorderRef.current = mediaRecorder;

      mediaRecorder.ondataavailable = (e) => {
        if (e.data && e.data.size > 0) {
          audioChunksRef.current.push(e.data);
        }
      };

      mediaRecorder.onstop = async () => {
        stream.getTracks().forEach((track) => track.stop());
        const blob = new Blob(audioChunksRef.current, {
          type: mediaRecorder.mimeType || "audio/webm",
        });
        const url = URL.createObjectURL(blob);
        setAudioUrl(url);
        setHasTaken(true);

        // Upload to S3/MinIO
        await uploadRecording(blob);
      };

      mediaRecorder.start(250); // Slice every 250ms
      setIsRecording(true);
      setSecondsRemaining(speakingTimeSeconds);

      timerIntervalRef.current = setInterval(() => {
        setSecondsRemaining((prev) => {
          if (prev <= 1) {
            stopRecording();
            return 0;
          }
          return prev - 1;
        });
      }, 1000);
    } catch (err: any) {
      setPermissionError(
        t(
          "exam.speaking.micPermissionDenied",
          "Microphone access denied. Please allow microphone permissions in your browser settings.",
        ),
      );
    }
  };

  const stopRecording = () => {
    if (timerIntervalRef.current) {
      clearInterval(timerIntervalRef.current);
      timerIntervalRef.current = null;
    }
    if (mediaRecorderRef.current && mediaRecorderRef.current.state !== "inactive") {
      mediaRecorderRef.current.stop();
    }
    setIsRecording(false);
  };

  const uploadRecording = async (blob: Blob) => {
    setIsUploading(true);
    try {
      const contentType = blob.type || "audio/webm";
      const intent = await examApi.getSpeakingUploadIntent(
        contentType,
        blob.size,
      );
      await examApi.uploadSpeakingAudio(intent.upload_url, blob);
      setRecordingKey(intent.key);
      onRecordingComplete(intent.key);
    } catch (err: any) {
      setPermissionError(
        t(
          "exam.speaking.uploadFailed",
          "Failed to upload voice recording. Please retry.",
        ),
      );
    } finally {
      setIsUploading(false);
    }
  };

  const handleResetRecording = () => {
    if (mode === "exam" && hasTaken) return;
    setAudioUrl(null);
    setRecordingKey(null);
    setSecondsRemaining(speakingTimeSeconds);
    setPermissionError(null);
    setHasTaken(false);
  };

  return (
    <div className={cn("space-y-4 rounded-xl border border-border bg-card p-5 shadow-sm", className)}>
      {/* Header Info */}
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div>
          <h4 className="text-base font-semibold text-text">
            {taskType === "read_aloud"
              ? t("exam.speaking.readAloudTitle", "Read Aloud Task")
              : t("exam.speaking.respondTitle", "Spoken Response Task")}
          </h4>
          <p className="text-xs text-text-muted">
            {taskType === "read_aloud"
              ? t("exam.speaking.readAloudDesc", "Read the text clearly at a natural speaking pace.")
              : t("exam.speaking.respondDesc", "Speak your answer clearly within the time limit.")}
          </p>
        </div>

        <Badge variant="outline" className="text-xs font-medium px-3 py-1 gap-1.5">
          <Volume2 className="h-3.5 w-3.5" />
          {t("exam.speaking.timeLimit", { seconds: speakingTimeSeconds, defaultValue: `${speakingTimeSeconds}s limit` })}
        </Badge>
      </div>

      {/* Task Content / Reference Text */}
      {taskType === "read_aloud" && referenceText && (
        <div className="rounded-lg bg-surface-muted p-4 border border-border-subtle">
          <p className="text-sm sm:text-base font-medium text-text leading-relaxed select-none">
            {referenceText}
          </p>
        </div>
      )}

      {promptText && (
        <div className="text-sm text-text-muted font-medium">
          {promptText}
        </div>
      )}

      {/* Permission error banner */}
      {permissionError && (
        <div className="flex items-center gap-2 rounded-lg bg-danger/10 p-3 text-xs text-danger">
          <AlertCircle className="h-4 w-4 shrink-0" />
          <span>{permissionError}</span>
        </div>
      )}

      {/* Recorder Controls Area */}
      <div className="flex flex-col items-center justify-center rounded-xl bg-surface-muted/40 border border-border-subtle p-6 space-y-4">
        {/* Timer / Waveform display */}
        <div className="flex flex-col items-center gap-2">
          <div
            className={cn(
              "font-mono text-3xl sm:text-4xl font-bold tracking-tight",
              isRecording ? "text-danger animate-pulse" : "text-text",
            )}
          >
            00:{secondsRemaining.toString().padStart(2, "0")}
          </div>

          {isRecording ? (
            <div className="flex items-center gap-1.5 h-6">
              {[40, 75, 50, 90, 60, 100, 70, 45].map((h, idx) => (
                <div
                  key={idx}
                  className="w-1 bg-danger rounded-full animate-pulse"
                  style={{
                    height: `${h}%`,
                    animationDelay: `${idx * 120}ms`,
                  }}
                />
              ))}
            </div>
          ) : (
            <p className="text-xs text-text-muted">
              {recordingKey
                ? t("exam.speaking.recordedSuccess", "Audio response recorded and saved.")
                : t("exam.speaking.readyToRecord", "Press Record when ready to begin speaking.")}
            </p>
          )}
        </div>

        {/* Action Buttons */}
        <div className="flex items-center gap-3">
          {!isRecording ? (
            <Button
              type="button"
              onClick={startRecording}
              disabled={isUploading || (mode === "exam" && hasTaken && !!recordingKey)}
              aria-label={t("exam.speaking.startRecording", "Start Recording")}
              className={cn(
                "h-12 px-6 rounded-full font-semibold min-h-[44px] min-w-[44px] gap-2 shadow-sm",
                recordingKey
                  ? "bg-surface-muted text-text hover:bg-surface-muted/80"
                  : "bg-danger text-white hover:bg-danger/90",
              )}
            >
              <Mic className="h-5 w-5" />
              <span>
                {recordingKey
                  ? mode === "practice"
                    ? t("exam.speaking.reRecord", "Re-record")
                    : t("exam.speaking.recorded", "Recorded")
                  : t("exam.speaking.record", "Record")}
              </span>
            </Button>
          ) : (
            <Button
              type="button"
              onClick={stopRecording}
              aria-label={t("exam.speaking.stopRecording", "Stop Recording")}
              className="h-12 px-6 rounded-full bg-danger text-white hover:bg-danger/90 font-semibold min-h-[44px] min-w-[44px] gap-2 shadow-sm"
            >
              <Square className="h-5 w-5 fill-current" />
              <span>{t("exam.speaking.stop", "Stop")}</span>
            </Button>
          )}

          {/* Practice mode reset button */}
          {mode === "practice" && recordingKey && !isRecording && (
            <Button
              type="button"
              variant="outline"
              onClick={handleResetRecording}
              className="h-12 px-4 rounded-full min-h-[44px] gap-2 text-xs"
            >
              <RotateCcw className="h-4 w-4" />
              <span>{t("common.reset", "Reset")}</span>
            </Button>
          )}
        </div>

        {/* Upload status indicator */}
        {isUploading && (
          <div className="flex items-center gap-2 text-xs text-primary font-medium animate-pulse">
            <UploadCloud className="h-4 w-4" />
            <span>{t("exam.speaking.uploading", "Uploading audio recording...")}</span>
          </div>
        )}

        {recordingKey && !isUploading && (
          <div className="flex items-center gap-1.5 text-xs text-success font-medium">
            <CheckCircle2 className="h-4 w-4" />
            <span>{t("exam.speaking.uploaded", "Audio successfully saved.")}</span>
          </div>
        )}
      </div>

      {/* Voice Privacy Consent Modal */}
      {showConsentModal && (
        <div
          role="dialog"
          aria-modal="true"
          className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm p-4 animate-in fade-in duration-200"
        >
          <div className="max-w-md w-full rounded-2xl bg-card border border-border p-6 shadow-xl space-y-4">
            <div className="flex items-center gap-3">
              <div className="flex h-11 w-11 shrink-0 items-center justify-center rounded-xl bg-primary/10 text-primary">
                <ShieldCheck className="h-6 w-6" />
              </div>
              <div>
                <h3 className="font-semibold text-text text-base">
                  {t("exam.speaking.privacyTitle", "Voice Recording Notice")}
                </h3>
                <p className="text-xs text-text-muted">
                  {t("exam.speaking.privacySubtitle", "How your audio is processed and stored")}
                </p>
              </div>
            </div>

            <div className="text-xs text-text-muted space-y-2 leading-relaxed bg-surface-muted/50 p-3.5 rounded-lg border border-border-subtle">
              <p>
                {t(
                  "exam.speaking.privacyConsentBody1",
                  "Before speaking, please note that your audio recording is sent to an automated transcription service to generate a text transcript for scoring.",
                )}
              </p>
              <p>
                {t(
                  "exam.speaking.privacyConsentBody2",
                  "Audio files are retained securely for a maximum of 90 days, after which they are permanently deleted. You can delete your recording at any time from your score report or settings. Deleting your account immediately erases all voice recordings.",
                )}
              </p>
            </div>

            <div className="flex justify-end gap-2.5 pt-2">
              <Button
                type="button"
                variant="outline"
                onClick={() => setShowConsentModal(false)}
                className="min-h-[44px]"
              >
                {t("common.cancel", "Cancel")}
              </Button>
              <Button
                type="button"
                onClick={acceptConsent}
                className="bg-primary text-white hover:bg-primary/90 min-h-[44px]"
              >
                {t("exam.speaking.iUnderstand", "I understand & consent")}
              </Button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
};
