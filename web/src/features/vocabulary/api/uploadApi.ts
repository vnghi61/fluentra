import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { apiFetch } from "@/api/client";
import type { components } from "@/types/api";

import { vocabularyKeys } from "./keys";

export type VocabUpload = components["schemas"]["VocabUpload"];
export type VocabUploadItem = components["schemas"]["VocabUploadItem"];
export type VocabUploadList = components["schemas"]["VocabUploadList"];
export type Deck = components["schemas"]["Deck"];
export type DeckListResponse = components["schemas"]["DeckListResponse"];

export const deckApi = {
  async list(): Promise<DeckListResponse> {
    return apiFetch<DeckListResponse>("/api/v1/vocabulary/decks");
  },
};

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
 * How long the list keeps asking, and how often.
 *
 * Submitting now enqueues the verification job in the same transaction as the
 * words, so the answer arrives in seconds rather than on the next turn of an
 * hourly sweep. Three seconds is fast enough to feel immediate and slow enough
 * not to be a load test of one's own account.
 *
 * The cap is the part that was missing. Polling ran while anything was pending
 * and nothing bounded it, so a word that never finished -- worker stopped, no
 * provider configured -- was asked about for ever. Twenty polls is a minute; if
 * a word is still pending after that, something is wrong upstream and asking a
 * two-hundredth time will not fix it. The list still shows what it knows, and a
 * reload starts a fresh minute.
 */
const UPLOAD_POLL_MS = 3_000;
const UPLOAD_POLL_LIMIT = 20;

/**
 * A learner's uploads.
 *
 * Refetched on a timer while anything is still pending, so the screen does not
 * sit on "12 waiting" until somebody reloads it by hand.
 */
export function useUploads(enabled: boolean) {
  return useQuery({
    queryKey: vocabularyKeys.uploads(),
    queryFn: () => uploadApi.list(),
    enabled,
    refetchInterval: (query) => {
      const items = query.state.data?.items ?? [];
      const waiting = items.some((upload) => (upload.pending_count ?? 0) > 0);
      if (!waiting) return false;
      // dataUpdateCount counts successful fetches, the first one included, so
      // this is "stop after UPLOAD_POLL_LIMIT answers" rather than an interval
      // that outlives the tab.
      if (query.state.dataUpdateCount > UPLOAD_POLL_LIMIT) return false;
      return UPLOAD_POLL_MS;
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

export function useDecks(enabled: boolean) {
  return useQuery({
    queryKey: vocabularyKeys.decks(),
    queryFn: () => deckApi.list(),
    enabled,
  });
}

/** Submits a paste and refreshes the list it belongs to. */
export function useSubmitUpload() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (text: string) => uploadApi.submit(text),
    onSuccess: () => {
      void queryClient.invalidateQueries({
        queryKey: vocabularyKeys.uploads(),
      });
      void queryClient.invalidateQueries({
        queryKey: vocabularyKeys.decks(),
      });
    },
  });
}
