import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { apiFetch } from "@/api/client";
import type {
  CompleteSectionResult,
  ExamAttempt,
  ExamAttemptListResponse,
  ExamTemplate,
  ListeningPlayResult,
  SaveAnswersRequest,
  SaveAnswersResult,
  ScoreReport,
  SpeakingUploadIntentResult,
  StartSittingRequest,
  SubmitExamResult,
} from "../types";
import { examKeys } from "./keys";

export const examApi = {
  /** Fetch available exam templates */
  async listExams(): Promise<ExamTemplate[]> {
    return apiFetch<ExamTemplate[]>("/api/v1/exams");
  },

  /** Start a new exam sitting */
  async startSitting(
    examId: string,
    req: StartSittingRequest,
  ): Promise<ExamAttempt> {
    return apiFetch<ExamAttempt>(`/api/v1/exams/${examId}/attempts`, {
      method: "POST",
      body: JSON.stringify(req),
    });
  },

  /** List authenticated user's exam attempts with pagination */
  async listAttempts(
    page = 1,
    pageSize = 10,
  ): Promise<ExamAttemptListResponse> {
    return apiFetch<ExamAttemptListResponse>(
      `/api/v1/exam-attempts?page=${page}&page_size=${pageSize}`,
    );
  },

  /** Get an exam sitting by ID */
  async getAttempt(id: string): Promise<ExamAttempt> {
    return apiFetch<ExamAttempt>(`/api/v1/exam-attempts/${id}`);
  },

  /** Autosave draft answers and record integrity events */
  async saveAnswers(
    id: string,
    req: SaveAnswersRequest,
  ): Promise<SaveAnswersResult> {
    return apiFetch<SaveAnswersResult>(`/api/v1/exam-attempts/${id}/answers`, {
      method: "PUT",
      body: JSON.stringify(req),
    });
  },

  /** Mark a section complete and advance to next */
  async completeSection(
    id: string,
    sectionNum: number,
  ): Promise<CompleteSectionResult> {
    return apiFetch<CompleteSectionResult>(
      `/api/v1/exam-attempts/${id}/sections/${sectionNum}/complete`,
      {
        method: "POST",
      },
    );
  },

  /** Submit an exam sitting */
  async submitExam(id: string): Promise<SubmitExamResult> {
    return apiFetch<SubmitExamResult>(`/api/v1/exam-attempts/${id}/submit`, {
      method: "POST",
    });
  },

  /** Get the score report for an exam attempt */
  async getReport(id: string): Promise<ScoreReport> {
    return apiFetch<ScoreReport>(`/api/v1/exam-attempts/${id}/report`);
  },

  /** Record a listening play and receive presigned audio URL */
  async getListeningPlay(
    versionId: string,
    contextType = "exam",
    contextId = "",
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

  /** Request presigned upload URL for speaking recording */
  async getSpeakingUploadIntent(
    contentType: string,
    fileSize: number,
  ): Promise<SpeakingUploadIntentResult> {
    return apiFetch<SpeakingUploadIntentResult>(
      "/api/v1/speaking/upload-intent",
      {
        method: "POST",
        body: JSON.stringify({
          content_type: contentType,
          file_size: fileSize,
        }),
      },
    );
  },

  /** Upload audio blob directly to presigned storage URL */
  async uploadSpeakingAudio(uploadUrl: string, blob: Blob): Promise<void> {
    const res = await fetch(uploadUrl, {
      method: "PUT",
      body: blob,
      headers: {
        "Content-Type": blob.type || "audio/webm",
      },
    });
    if (!res.ok) {
      throw new Error(`Failed to upload recording: HTTP ${res.status}`);
    }
  },

  /** Delete a speaking recording (GDPR / right to be forgotten) */
  async deleteSpeakingRecording(attemptId: string): Promise<void> {
    return apiFetch<void>(`/api/v1/speaking/attempts/${attemptId}/recording`, {
      method: "DELETE",
    });
  },
};

/** React Query hook to list available exams */
export function useExams(enabled = true) {
  return useQuery({
    queryKey: examKeys.lists(),
    queryFn: () => examApi.listExams(),
    enabled,
  });
}

/** React Query hook to list user's attempts */
export function useUserExamAttempts(page = 1, pageSize = 10, enabled = true) {
  return useQuery({
    queryKey: examKeys.attempts(page, pageSize),
    queryFn: () => examApi.listAttempts(page, pageSize),
    enabled,
  });
}

/** React Query hook to get attempt details */
export function useExamAttempt(id: string, enabled = true) {
  return useQuery({
    queryKey: examKeys.attempt(id),
    queryFn: () => examApi.getAttempt(id),
    enabled: enabled && !!id,
    refetchInterval: (query) => {
      const data = query.state.data;
      if (data && data.status === "in_progress") {
        return 30_000; // sync state every 30s
      }
      return false;
    },
  });
}

/** React Query hook to get score report */
export function useExamReport(id: string, enabled = true) {
  return useQuery({
    queryKey: examKeys.report(id),
    queryFn: () => examApi.getReport(id),
    enabled: enabled && !!id,
    refetchInterval: (query) => {
      const data = query.state.data;
      if (data && data.status === "pending") {
        return 3_000; // poll while async grading is settling
      }
      return false;
    },
  });
}

/** React Query mutation to submit an exam */
export function useSubmitExam() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => examApi.submitExam(id),
    onSuccess: (_, id) => {
      void queryClient.invalidateQueries({ queryKey: examKeys.attempt(id) });
      void queryClient.invalidateQueries({ queryKey: examKeys.report(id) });
      void queryClient.invalidateQueries({ queryKey: examKeys.all });
    },
  });
}
