import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { apiFetch } from "@/api/client";
import type { components } from "@/types/api";

import { vocabularyKeys } from "./keys";

export type VocabUpload = components["schemas"]["VocabUpload"];
export type VocabUploadItem = components["schemas"]["VocabUploadItem"];
export type VocabUploadList = components["schemas"]["VocabUploadList"];

export const uploadApi = {
  /** Submit a paste of vocabulary. Returns before anything is checked. */
  async submit(text: string): Promise<VocabUpload> {
    return apiFetch<VocabUpload>("/api/v1/me/vocabulary/uploads", {
      method: "POST",
      body: JSON.stringify({ text }),
    });
  },

  async list(): Promise<VocabUploadList> {
    return apiFetch<VocabUploadList>("/api/v1/me/vocabulary/uploads");
  },

  async get(id: string): Promise<VocabUpload> {
    return apiFetch<VocabUpload>(`/api/v1/me/vocabulary/uploads/${id}`);
  },
};

/**
 * A learner's uploads.
 *
 * Refetched on a timer while anything is still pending, because the checking
 * happens in an hourly job and the screen would otherwise show "12 waiting"
 * until the page was reloaded by hand. The timer stops once nothing is pending,
 * so a finished list is not polled for ever.
 */
export function useUploads(enabled: boolean) {
  return useQuery({
    queryKey: vocabularyKeys.uploads(),
    queryFn: () => uploadApi.list(),
    enabled,
    refetchInterval: (query) => {
      const items = query.state.data?.items ?? [];
      const waiting = items.some((upload) => (upload.pending_count ?? 0) > 0);
      return waiting ? 30_000 : false;
    },
  });
}

export function useUpload(id: string | undefined) {
  return useQuery({
    queryKey: vocabularyKeys.upload(id ?? "__none__"),
    queryFn: () =>
      id ? uploadApi.get(id) : Promise.reject(new Error("No upload id")),
    enabled: Boolean(id),
  });
}

/** Submits a paste and refreshes the list it belongs to. */
export function useSubmitUpload() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (text: string) => uploadApi.submit(text),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: vocabularyKeys.uploads() });
    },
  });
}
