import { useQuery } from "@tanstack/react-query";

import { apiFetch } from "@/api/client";
import type { components } from "@/types/api";
import { reviewKeys } from "./keys";

export type ReviewGrade = components["schemas"]["ReviewGrade"];
export type ReviewCard = components["schemas"]["ReviewCard"];
export type ReviewSessionResponse = components["schemas"]["ReviewSessionResponse"];
export type AnswerReviewRequest = components["schemas"]["AnswerReviewRequest"];
export type AnswerReviewResponse = components["schemas"]["AnswerReviewResponse"];
export type DueCountResponse = components["schemas"]["DueCountResponse"];

export const reviewApi = {
  /** Fetch the current review session card queue */
  async getSession(): Promise<ReviewSessionResponse> {
    return apiFetch<ReviewSessionResponse>("/api/v1/reviews/session");
  },

  /** Fetch the count of due cards */
  async getDueCount(): Promise<DueCountResponse> {
    return apiFetch<DueCountResponse>("/api/v1/reviews/due-count");
  },

  /** Submit grade for a specific review card */
  async answerCard(
    cardId: string,
    grade: ReviewGrade,
    elapsedMs?: number,
  ): Promise<AnswerReviewResponse> {
    // Typed against the generated request schema rather than an object literal:
    // the grade is an enum on the wire and the keyboard map is 1-4, so a digit
    // that slipped through here would type-check and mis-grade every review.
    const body: AnswerReviewRequest = {
      grade,
      ...(elapsedMs !== undefined && { elapsed_ms: elapsedMs }),
    };
    return apiFetch<AnswerReviewResponse>(`/api/v1/reviews/${cardId}/answer`, {
      method: "POST",
      body: JSON.stringify(body),
    });
  },
};

export function useReviewSession() {
  return useQuery({
    queryKey: reviewKeys.session(),
    queryFn: () => reviewApi.getSession(),
  });
}
