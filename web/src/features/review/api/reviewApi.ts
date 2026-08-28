import { useQuery } from "@tanstack/react-query";

import { apiFetch } from "@/api/client";
import type { components } from "@/types/api";
import { reviewKeys } from "./keys";

export type ReviewGrade = components["schemas"]["ReviewGrade"];
export type ReviewCard = components["schemas"]["ReviewCard"];
export type ReviewSessionResponse =
  components["schemas"]["ReviewSessionResponse"];
export type AnswerReviewRequest = components["schemas"]["AnswerReviewRequest"];
export type AnswerReviewResponse =
  components["schemas"]["AnswerReviewResponse"];
export type DueCountResponse = components["schemas"]["DueCountResponse"];
export type ForecastResponse = components["schemas"]["ForecastResponse"];
export type ForecastItem = components["schemas"]["ForecastItem"];

export const reviewApi = {
  /** Fetch the current review session card queue */
  async getSession(): Promise<ReviewSessionResponse> {
    return apiFetch<ReviewSessionResponse>("/api/v1/reviews/session");
  },

  /** Fetch the count of due cards */
  async getDueCount(): Promise<DueCountResponse> {
    return apiFetch<DueCountResponse>("/api/v1/reviews/due-count");
  },

  /** Projected review workload for the next 30 days */
  async getForecast(): Promise<ForecastResponse> {
    return apiFetch<ForecastResponse>("/api/v1/reviews/forecast");
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

/**
 * How many cards are due right now.
 *
 * Separate from the dashboard query on purpose: /me/dashboard carries the same
 * number, but Practice is reachable without passing through the dashboard, and
 * a hub that has to load the whole dashboard to print one integer would be
 * slower and would fail for a reason that has nothing to do with reviews.
 */
export function useDueCount() {
  return useQuery({
    queryKey: reviewKeys.dueCount(),
    queryFn: () => reviewApi.getDueCount(),
  });
}

/** The next 30 days of scheduled reviews. */
export function useForecast() {
  return useQuery({
    queryKey: reviewKeys.forecast(),
    queryFn: () => reviewApi.getForecast(),
  });
}
