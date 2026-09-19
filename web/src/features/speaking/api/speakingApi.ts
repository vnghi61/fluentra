import { useQuery } from "@tanstack/react-query";

import { ApiError, apiFetch } from "@/api/client";
import { reachableStorageUrl } from "@/lib/storage-url";
import type {
  SpeakingFeedback,
  SpeakingSubmissionList,
  SpeakingUploadIntentResult,
} from "../types";
import { speakingKeys } from "./keys";

/** The problem code of a failed request, if the server sent one. */
export function problemCode(err: unknown): string | undefined {
  return err instanceof ApiError ? err.problem.code : undefined;
}

export interface SpeakingConsent {
  consented: boolean;
  consented_at?: string | null;
}

export const speakingApi = {
  /**
   * Whether this learner has agreed to be recorded (BR-SPEAKING-03).
   *
   * The answer lives on the server, not in localStorage: a consent record that
   * disappears when site data is cleared, and that never existed on the
   * learner's other devices, cannot say who agreed or when — which is the only
   * thing a consent record is kept for.
   */
  async getConsent(): Promise<SpeakingConsent> {
    return apiFetch<SpeakingConsent>("/api/v1/speaking/consent");
  },

  /** Record this learner's consent, with the timestamp the rule requires. */
  async recordConsent(): Promise<SpeakingConsent> {
    return apiFetch<SpeakingConsent>("/api/v1/speaking/consent", {
      method: "POST",
    });
  },

  /** Request a presigned PUT URL and object key for browser recording upload. */
  async getUploadIntent(
    contentType: string,
  ): Promise<SpeakingUploadIntentResult> {
    return apiFetch<SpeakingUploadIntentResult>(
      "/api/v1/speaking/upload-intent",
      {
        method: "POST",
        body: JSON.stringify({ content_type: contentType }),
      },
    );
  },

  /** Upload audio blob directly to storage via presigned PUT URL. */
  async uploadAudio(uploadUrl: string, blob: Blob): Promise<void> {
    const res = await fetch(reachableStorageUrl(uploadUrl), {
      method: "PUT",
      body: blob,
      headers: { "Content-Type": blob.type || "audio/webm" },
    });
    if (!res.ok) {
      throw new Error(`Audio upload failed with status ${res.status}`);
    }
  },

  /** Fetch evaluation feedback for a graded speaking attempt. */
  async getFeedback(attemptId: string): Promise<SpeakingFeedback> {
    return apiFetch<SpeakingFeedback>(
      `/api/v1/speaking/attempts/${attemptId}/feedback`,
    );
  },

  /** List user's speaking submissions with pagination. */
  async listSubmissions(
    page = 1,
    pageSize = 10,
  ): Promise<SpeakingSubmissionList> {
    return apiFetch<SpeakingSubmissionList>(
      `/api/v1/speaking/submissions?page=${page}&page_size=${pageSize}`,
    );
  },

  /** Delete audio recording object while preserving feedback and scores. */
  async deleteRecording(attemptId: string): Promise<void> {
    return apiFetch<void>(`/api/v1/speaking/attempts/${attemptId}/recording`, {
      method: "DELETE",
    });
  },
};

/** React Query hook to list speaking submissions */
export function useSpeakingSubmissions(
  page = 1,
  pageSize = 10,
  enabled = true,
) {
  return useQuery({
    queryKey: speakingKeys.submissions(page, pageSize),
    queryFn: () => speakingApi.listSubmissions(page, pageSize),
    enabled,
  });
}

/** React Query hook to fetch speaking feedback */
export function useSpeakingFeedback(attemptId: string | null, enabled = true) {
  return useQuery({
    queryKey: speakingKeys.feedback(attemptId ?? ""),
    queryFn: () => speakingApi.getFeedback(attemptId!),
    enabled: enabled && !!attemptId,
  });
}
