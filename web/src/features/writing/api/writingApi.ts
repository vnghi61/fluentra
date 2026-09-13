import { useQuery } from "@tanstack/react-query";

import { apiFetch } from "@/api/client";
import type { WritingFeedback, WritingSubmissionList } from "../types";
import { writingKeys } from "./keys";

export const writingApi = {
  /** Fetch detailed feedback for a specific writing attempt */
  async getFeedback(attemptId: string): Promise<WritingFeedback> {
    return apiFetch<WritingFeedback>(
      `/api/v1/writing/attempts/${attemptId}/feedback`,
    );
  },

  /** List user's writing submissions with pagination */
  async listSubmissions(
    page = 1,
    pageSize = 10,
  ): Promise<WritingSubmissionList> {
    return apiFetch<WritingSubmissionList>(
      `/api/v1/writing/submissions?page=${page}&page_size=${pageSize}`,
    );
  },
};

/** React Query hook to list writing submissions */
export function useWritingSubmissions(page = 1, pageSize = 10, enabled = true) {
  return useQuery({
    queryKey: writingKeys.submissions(page, pageSize),
    queryFn: () => writingApi.listSubmissions(page, pageSize),
    enabled,
  });
}

/** React Query hook to fetch writing feedback */
export function useWritingFeedback(attemptId: string | null, enabled = true) {
  return useQuery({
    queryKey: writingKeys.feedback(attemptId ?? ""),
    queryFn: () => writingApi.getFeedback(attemptId!),
    enabled: enabled && !!attemptId,
  });
}
