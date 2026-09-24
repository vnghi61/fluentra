import { useQuery } from "@tanstack/react-query";

import { ApiError, apiFetch } from "@/api/client";
import { reachableStorageUrl } from "@/lib/storage-url";
import type { components } from "@/types/api";
import type {
  CompleteSectionResult,
  ExamAttempt,
  ExamAttemptListResponse,
  ExamTemplate,
  ListeningPlayResult,
  SaveAnswersRequest,
  SaveAnswersResult,
  ScoreReport,
  SpeakingFeedback,
  SpeakingUploadIntentResult,
  StartSittingRequest,
  SubmitExamResult,
} from "../types";
import { examKeys } from "./keys";

/** The mock-test surface, typed from the spec rather than by hand. */
export type ExamVersionListResponse =
  components["schemas"]["ExamVersionListResponse"];
export type ExamVersion = components["schemas"]["ExamVersion"];
export type MockTest = components["schemas"]["MockTest"];
export type ComposeMockTestRequest =
  components["schemas"]["ComposeMockTestRequest"];
export type FixedTestSummary = components["schemas"]["FixedTestSummary"];
export type FixedTestListResponse =
  components["schemas"]["FixedTestListResponse"];

/** The problem code of a failed request, if the server sent one. */
export function problemCode(err: unknown): string | undefined {
  return err instanceof ApiError ? err.problem.code : undefined;
}

export const examApi = {
  async listExams(): Promise<ExamTemplate[]> {
    return apiFetch<ExamTemplate[]>("/api/v1/exams");
  },

  async startSitting(
    examId: string,
    req: StartSittingRequest,
  ): Promise<ExamAttempt> {
    return apiFetch<ExamAttempt>(`/api/v1/exams/${examId}/attempts`, {
      method: "POST",
      body: JSON.stringify(req),
    });
  },

  async listAttempts(limit = 10, offset = 0): Promise<ExamAttemptListResponse> {
    return apiFetch<ExamAttemptListResponse>(
      `/api/v1/exam-attempts?limit=${limit}&offset=${offset}`,
    );
  },

  async getAttempt(id: string): Promise<ExamAttempt> {
    return apiFetch<ExamAttempt>(`/api/v1/exam-attempts/${id}`);
  },

  async saveAnswers(
    id: string,
    req: SaveAnswersRequest,
  ): Promise<SaveAnswersResult> {
    return apiFetch<SaveAnswersResult>(`/api/v1/exam-attempts/${id}/answers`, {
      method: "PUT",
      body: JSON.stringify(req),
    });
  },

  async completeSection(
    id: string,
    sectionNum: number,
  ): Promise<CompleteSectionResult> {
    return apiFetch<CompleteSectionResult>(
      `/api/v1/exam-attempts/${id}/sections/${sectionNum}/complete`,
      { method: "POST" },
    );
  },

  async submitExam(id: string): Promise<SubmitExamResult> {
    return apiFetch<SubmitExamResult>(`/api/v1/exam-attempts/${id}/submit`, {
      method: "POST",
    });
  },

  async getReport(id: string): Promise<ScoreReport> {
    return apiFetch<ScoreReport>(`/api/v1/exam-attempts/${id}/report`);
  },

  /**
   * Record a play of a clip in a sitting or a placement session and receive a
   * short-lived audio URL. The server checks the context is the caller's.
   */
  async playListening(
    versionId: string,
    contextId: string,
    contextType: "exam" | "placement" = "exam",
  ): Promise<ListeningPlayResult> {
    return apiFetch<ListeningPlayResult>(
      `/api/v1/listening/items/${versionId}/plays`,
      {
        method: "POST",
        body: JSON.stringify({
          context_type: contextType,
          context_id: contextId,
        }),
      },
    );
  },

  async getSpeakingUploadIntent(
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

  /** Upload a recording straight to storage with the presigned URL. */
  async uploadSpeakingAudio(uploadUrl: string, blob: Blob): Promise<void> {
    const res = await fetch(reachableStorageUrl(uploadUrl), {
      method: "PUT",
      body: blob,
      headers: { "Content-Type": blob.type || "audio/webm" },
    });
    if (!res.ok) {
      throw new Error(`Recording upload failed: HTTP ${res.status}`);
    }
  },

  async getSpeakingFeedback(attemptId: string): Promise<SpeakingFeedback> {
    return apiFetch<SpeakingFeedback>(
      `/api/v1/speaking/attempts/${attemptId}/feedback`,
    );
  },

  async deleteSpeakingRecording(attemptId: string): Promise<void> {
    return apiFetch<void>(`/api/v1/speaking/attempts/${attemptId}/recording`, {
      method: "DELETE",
    });
  },

  /** Current exam versions with their blueprints and how many tests exist. */
  async listExamVersions(): Promise<ExamVersionListResponse> {
    return apiFetch<ExamVersionListResponse>("/api/v1/exam-versions");
  },

  /**
   * Compose a mock test from the bank. A bank that cannot fill a part answers
   * 409 with the part that is short in the problem's detail.
   */
  async composeMockTest(req: ComposeMockTestRequest): Promise<MockTest> {
    return apiFetch<MockTest>("/api/v1/mock-tests", {
      method: "POST",
      body: JSON.stringify(req),
    });
  },

  /**
   * The numbered fixed tests of one exam version, in order, each with the
   * caller's latest attempt (WO 22 Stage J).
   */
  async listExamVersionTests(
    versionId: string,
  ): Promise<FixedTestListResponse> {
    return apiFetch<FixedTestListResponse>(
      `/api/v1/exam-versions/${versionId}/tests`,
    );
  },

  /**
   * Start or retake a composed test; a retake replays the same composition. The
   * body is the same one a sitting start accepts, so a mock test can be sat in
   * exam or practice mode (WO 22 Stage K).
   */
  async startMockTestAttempt(
    mockTestId: string,
    req: StartSittingRequest = { mode: "exam" },
  ): Promise<ExamAttempt> {
    return apiFetch<ExamAttempt>(`/api/v1/mock-tests/${mockTestId}/attempts`, {
      method: "POST",
      body: JSON.stringify(req),
    });
  },
};

export function useExams(enabled = true) {
  return useQuery({
    queryKey: examKeys.lists(),
    queryFn: () => examApi.listExams(),
    enabled,
  });
}

/** The verified exam versions a mock test can be composed from. */
export function useExamVersions(enabled = true) {
  return useQuery({
    queryKey: examKeys.versions(),
    queryFn: () => examApi.listExamVersions(),
    enabled,
  });
}

/** The numbered fixed tests of one exam version, in order. */
export function useExamVersionTests(versionId: string, enabled = true) {
  return useQuery({
    queryKey: examKeys.versionTests(versionId),
    queryFn: () => examApi.listExamVersionTests(versionId),
    enabled: enabled && !!versionId,
  });
}

export function useUserExamAttempts(limit = 10, offset = 0, enabled = true) {
  return useQuery({
    queryKey: examKeys.attempts(limit, offset),
    queryFn: () => examApi.listAttempts(limit, offset),
    enabled,
  });
}

export function useExamAttempt(id: string, enabled = true) {
  return useQuery({
    queryKey: examKeys.attempt(id),
    queryFn: () => examApi.getAttempt(id),
    enabled: enabled && !!id,
  });
}

export function useExamReport(id: string, enabled = true) {
  return useQuery({
    queryKey: examKeys.report(id),
    queryFn: () => examApi.getReport(id),
    enabled: enabled && !!id,
    refetchInterval: (query) =>
      query.state.data?.status === "pending" ? 5_000 : false,
  });
}

export function useSpeakingFeedback(attemptId: string, enabled = true) {
  return useQuery({
    queryKey: examKeys.speakingFeedback(attemptId),
    queryFn: () => examApi.getSpeakingFeedback(attemptId),
    enabled: enabled && !!attemptId,
  });
}
