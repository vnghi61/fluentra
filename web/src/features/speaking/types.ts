import type { components } from "@/types/api";

export type SpeakingFeedback = components["schemas"]["SpeakingFeedback"];
export type SpeakingCriterion = components["schemas"]["SpeakingCriterion"];
export type SpeakingSubmissionSummary =
  components["schemas"]["SpeakingSubmissionSummary"];
export type SpeakingSubmissionList =
  components["schemas"]["SpeakingSubmissionList"];

export interface SpeakingUploadIntentResult {
  upload_url: string;
  object_key: string;
  expires_at: string;
  daily_recordings_used: number;
  daily_recordings_limit: number;
}
