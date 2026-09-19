import React from "react";

import {
  AudioRecorder,
  type AudioRecorderProps,
} from "@/features/speaking/components/AudioRecorder";
import { examApi } from "../api/examApi";

export type SpeakingRecorderProps = AudioRecorderProps;

export const SpeakingRecorder: React.FC<SpeakingRecorderProps> = (props) => {
  const handleUpload = async (blob: Blob): Promise<string> => {
    const intent = await examApi.getSpeakingUploadIntent(
      blob.type || "audio/webm",
    );
    await examApi.uploadSpeakingAudio(intent.upload_url, blob);
    return intent.object_key;
  };

  return <AudioRecorder {...props} onUpload={props.onUpload ?? handleUpload} />;
};

export default SpeakingRecorder;
