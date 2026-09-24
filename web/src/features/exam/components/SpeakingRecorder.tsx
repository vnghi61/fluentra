import React, { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";

import {
  AudioRecorder,
  type AudioRecorderProps,
} from "@/features/speaking/components/AudioRecorder";
import { Button } from "@/components/ui/button";
import { examApi } from "../api/examApi";

export type SpeakingRecorderProps = AudioRecorderProps & {
  /**
   * Seconds to prepare before recording starts — IELTS Speaking Part 2's minute
   * (WO 22 D22-24). Zero or absent means record straight away.
   */
  preparationSeconds?: number | undefined;
};

/**
 * The exam's speaking task. A part with preparation time shows a countdown
 * first; the learner can start early, and recording begins on its own when the
 * minute runs out.
 */
export const SpeakingRecorder: React.FC<SpeakingRecorderProps> = ({
  preparationSeconds = 0,
  ...props
}) => {
  const { t } = useTranslation();
  const [started, setStarted] = useState(preparationSeconds <= 0);
  const [left, setLeft] = useState(preparationSeconds);

  useEffect(() => {
    if (started || left <= 0) return;
    const id = setInterval(() => setLeft((seconds) => seconds - 1), 1000);
    return () => clearInterval(id);
  }, [started, left]);

  // Preparing while there is time left and the learner has not started early.
  // Derived, not stored, so reaching zero needs no state change in the effect.
  const preparing = !started && left > 0;

  const handleUpload = async (blob: Blob): Promise<string> => {
    const intent = await examApi.getSpeakingUploadIntent(
      blob.type || "audio/webm",
    );
    await examApi.uploadSpeakingAudio(intent.upload_url, blob);
    return intent.object_key;
  };

  if (preparing) {
    return (
      <div className="space-y-3 rounded-2xl border border-border bg-surface-card p-5 text-center sm:p-7">
        <p className="text-sm font-medium text-text">
          {t("exam.speaking.prepare", "Take a moment to prepare.")}
        </p>
        <p className="font-mono text-4xl font-extrabold text-primary-accent">
          {Math.max(left, 0)}s
        </p>
        <Button onClick={() => setStarted(true)} className="mx-auto">
          {t("exam.speaking.startNow", "Start now")}
        </Button>
      </div>
    );
  }

  return <AudioRecorder {...props} onUpload={props.onUpload ?? handleUpload} />;
};

export default SpeakingRecorder;
