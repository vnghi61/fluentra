import { useQuery } from "@tanstack/react-query";

import { ApiError, apiFetch } from "@/api/client";
import type { components } from "@/types/api";
import { learningKeys } from "./keys";

export type DashboardResponse = components["schemas"]["DashboardResponse"];
export type NextActivity = components["schemas"]["NextActivity"];
export type SkillMastery = components["schemas"]["SkillMastery"];
export type CourseProgress = components["schemas"]["CourseProgress"];
export type ProgressResponse = components["schemas"]["ProgressResponse"];
export type StartAttemptResult = components["schemas"]["StartAttemptResult"];
export type SubmitAttemptRequest =
  components["schemas"]["SubmitAttemptRequest"];
export type SubmitAttemptResult = components["schemas"]["SubmitAttemptResult"];
export type AttemptDetail = components["schemas"]["AttemptDetail"];
export type Enrollment = components["schemas"]["Enrollment"];
export type PreviewGradeResult = components["schemas"]["PreviewGradeResult"];
export type ItemReportReason = components["schemas"]["ItemReportReason"];
export type CreateItemReportRequest =
  components["schemas"]["CreateItemReportRequest"];
export type ItemReport = components["schemas"]["ItemReport"];
export type DailyPracticeSet = components["schemas"]["DailyPracticeSet"];

export const learningApi = {
  /** Fetch the learner's current dashboard state */
  async getDashboard(): Promise<DashboardResponse> {
    return apiFetch<DashboardResponse>("/api/v1/me/dashboard");
  },

  /** Fetch the learner's overall progress */
  async getProgress(): Promise<ProgressResponse> {
    return apiFetch<ProgressResponse>("/api/v1/me/progress");
  },

  /**
   * Enrol the learner in a course.
   *
   * P8.4 made `StartAttempt` enforce enrolment, so a learner who reaches a
   * lesson without one is refused at the first activity with 403 NOT_ENROLLED
   * and the runner shows "Could not start this activity". Nothing in the SPA
   * called this endpoint, which made every lesson unreachable for every real
   * learner.
   *
   * Idempotent from the caller's side: a second enrolment answers 409, and 409
   * means the learner is enrolled, which is the state this function promises.
   */
  async enrollCourse(courseId: string): Promise<void> {
    try {
      await apiFetch<Enrollment>(`/api/v1/courses/${courseId}/enroll`, {
        method: "POST",
      });
    } catch (err) {
      if (err instanceof ApiError && err.problem.status === 409) return;
      throw err;
    }
  },

  /** Start an activity attempt */
  async startAttempt(activityId: string): Promise<StartAttemptResult> {
    return apiFetch<StartAttemptResult>(
      `/api/v1/activities/${activityId}/attempts`,
      {
        method: "POST",
      },
    );
  },

  /** Submit an attempt with an explicit Idempotency-Key */
  async submitAttempt(
    attemptId: string,
    data: SubmitAttemptRequest,
    idempotencyKey: string,
  ): Promise<SubmitAttemptResult> {
    return apiFetch<SubmitAttemptResult>(
      `/api/v1/attempts/${attemptId}/submit`,
      {
        method: "POST",
        headers: {
          "Idempotency-Key": idempotencyKey,
        },
        body: JSON.stringify(data),
      },
    );
  },

  /**
   * Grade one response and keep nothing.
   *
   * What a visitor with no account submits to. There is no attempt to start
   * first and no Idempotency-Key to send, because there is nothing to make
   * idempotent — replaying it changes no state on the server.
   *
   * A signed-in learner must not come here: their answers belong in the attempt
   * flow, which is what produces their progress and their review cards.
   */
  async gradePreview(
    activityId: string,
    data: SubmitAttemptRequest,
  ): Promise<PreviewGradeResult> {
    return apiFetch<PreviewGradeResult>(
      `/api/v1/activities/${activityId}/grade`,
      {
        method: "POST",
        body: JSON.stringify(data),
      },
    );
  },

  /** Read an attempt detail */
  async getAttempt(attemptId: string): Promise<AttemptDetail> {
    return apiFetch<AttemptDetail>(`/api/v1/attempts/${attemptId}`);
  },

  /** Report an issue with a content version */
  async reportContentVersion(
    versionId: string,
    req: CreateItemReportRequest,
  ): Promise<ItemReport> {
    return apiFetch<ItemReport>(
      `/api/v1/content/versions/${versionId}/reports`,
      {
        method: "POST",
        body: JSON.stringify(req),
      },
    );
  },

  /** Fetch the caller's daily practice set for today */
  async getDailyPracticeSet(level?: string): Promise<DailyPracticeSet> {
    const query = level ? `?level=${encodeURIComponent(level)}` : "";
    return apiFetch<DailyPracticeSet>(`/api/v1/practice/daily${query}`);
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

/** React Query hook for daily practice set */
export function useDailyPracticeSet(level?: string, enabled = true) {
  return useQuery({
    queryKey: learningKeys.dailySet(level),
    queryFn: () => learningApi.getDailyPracticeSet(level),
    enabled,
    staleTime: 5 * 60 * 1000,
  });
}
