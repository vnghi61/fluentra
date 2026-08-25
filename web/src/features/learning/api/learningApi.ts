import { useQuery } from "@tanstack/react-query";

import { apiFetch } from "@/api/client";
import type { components } from "@/types/api";
import { learningKeys } from "./keys";

export type DashboardResponse = components["schemas"]["DashboardResponse"];
export type NextActivity = components["schemas"]["NextActivity"];
export type SkillMastery = components["schemas"]["SkillMastery"];
export type CourseProgress = components["schemas"]["CourseProgress"];
export type ProgressResponse = components["schemas"]["ProgressResponse"];
export type StartAttemptResult = components["schemas"]["StartAttemptResult"];
export type SubmitAttemptRequest = components["schemas"]["SubmitAttemptRequest"];
export type SubmitAttemptResult = components["schemas"]["SubmitAttemptResult"];
export type AttemptDetail = components["schemas"]["AttemptDetail"];

export const learningApi = {
  /** Fetch the learner's current dashboard state */
  async getDashboard(): Promise<DashboardResponse> {
    return apiFetch<DashboardResponse>("/api/v1/me/dashboard");
  },

  /** Fetch the learner's overall progress */
  async getProgress(): Promise<ProgressResponse> {
    return apiFetch<ProgressResponse>("/api/v1/me/progress");
  },

  /** Start an activity attempt */
  async startAttempt(activityId: string): Promise<StartAttemptResult> {
    return apiFetch<StartAttemptResult>(`/api/v1/activities/${activityId}/attempts`, {
      method: "POST",
    });
  },

  /** Submit an attempt with an explicit Idempotency-Key */
  async submitAttempt(
    attemptId: string,
    data: SubmitAttemptRequest,
    idempotencyKey: string,
  ): Promise<SubmitAttemptResult> {
    return apiFetch<SubmitAttemptResult>(`/api/v1/attempts/${attemptId}/submit`, {
      method: "POST",
      headers: {
        "Idempotency-Key": idempotencyKey,
      },
      body: JSON.stringify(data),
    });
  },

  /** Read an attempt detail */
  async getAttempt(attemptId: string): Promise<AttemptDetail> {
    return apiFetch<AttemptDetail>(`/api/v1/attempts/${attemptId}`);
  },
};

/** React Query hook for the learner dashboard */
export function useDashboard() {
  return useQuery({
    queryKey: learningKeys.dashboard(),
    queryFn: () => learningApi.getDashboard(),
  });
}

/** React Query hook for learner progress */
export function useProgress() {
  return useQuery({
    queryKey: learningKeys.progress(),
    queryFn: () => learningApi.getProgress(),
  });
}
