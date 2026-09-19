import { apiFetch } from "@/api/client";
import type { ListeningPlayResult, ListeningTranscriptResult } from "../types";

/**
 * Listening, for a lesson attempt.
 *
 * The exam feature has its own caller for the same two routes, bound to a
 * sitting: an exam plays a clip once, while it is the open section's current
 * item. A lesson attempt is the third context the route already accepts, with
 * three plays and a transcript released after grading — none of which the exam
 * caller can express, because it does not take an attempt.
 */
export const listeningApi = {
  /**
   * Records a play and returns the URL for it.
   *
   * The server counts the play, so a URL is spent when it is asked for, not
   * when the audio finishes.
   */
  async play(
    versionId: string,
    attemptId: string,
  ): Promise<ListeningPlayResult> {
    return apiFetch<ListeningPlayResult>(
      `/api/v1/listening/items/${versionId}/plays`,
      {
        method: "POST",
        body: JSON.stringify({
          context_type: "attempt",
          context_id: attemptId,
        }),
      },
    );
  },

  /**
   * The script. 403 until the attempt is graded — ADR-0025 — which is why it is
   * fetched after submitting and never before.
   */
  async getTranscript(
    versionId: string,
    attemptId: string,
  ): Promise<ListeningTranscriptResult> {
    return apiFetch<ListeningTranscriptResult>(
      `/api/v1/listening/items/${versionId}/transcript?attempt_id=${attemptId}`,
    );
  },
};
