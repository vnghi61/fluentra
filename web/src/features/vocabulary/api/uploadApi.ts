import { useEffect, useRef } from "react";
import {
  useInfiniteQuery,
  useMutation,
  useQuery,
  useQueryClient,
  type InfiniteData,
} from "@tanstack/react-query";

import { apiFetch } from "@/api/client";
import type { components } from "@/types/api";

import { vocabularyKeys } from "./keys";

export type VocabUpload = components["schemas"]["VocabUpload"];
export type VocabUploadItem = components["schemas"]["VocabUploadItem"];
export type VocabUploadList = components["schemas"]["VocabUploadList"];
export type VocabUploadItemPage = components["schemas"]["VocabUploadItemPage"];
export type UploadItemStatus = VocabUploadItem["status"];
export type Deck = components["schemas"]["Deck"];
export type DeckListResponse = components["schemas"]["DeckListResponse"];

/** Narrows a word list: one upload, some statuses, a substring. */
export interface UploadItemFilter {
  uploadId?: string | undefined;
  q?: string | undefined;
  statuses?: readonly UploadItemStatus[] | undefined;
}

export const deckApi = {
  async list(): Promise<DeckListResponse> {
    return apiFetch<DeckListResponse>("/api/v1/vocabulary/decks");
  },
};

/** One page of a list: 20 words is what a phone shows without feeling long. */
export const UPLOAD_PAGE_SIZE = 20;

export const uploadApi = {
  /**
   * Submit a paste of vocabulary. Known words come back already added; new
   * ones come back pending and are checked by a job within seconds.
   */
  async submit(text: string): Promise<VocabUpload> {
    return apiFetch<VocabUpload>("/api/v1/me/vocabulary/uploads", {
      method: "POST",
      body: JSON.stringify({ text }),
    });
  },

  async list(cursor?: string): Promise<VocabUploadList> {
    const params = new URLSearchParams({ limit: String(UPLOAD_PAGE_SIZE) });
    if (cursor) params.set("cursor", cursor);
    return apiFetch<VocabUploadList>(`/api/v1/me/vocabulary/uploads?${params}`);
  },

  async items(
    filter: UploadItemFilter,
    cursor?: string,
  ): Promise<VocabUploadItemPage> {
    const params = new URLSearchParams({ limit: String(UPLOAD_PAGE_SIZE) });
    if (filter.uploadId) params.set("upload_id", filter.uploadId);
    if (filter.q) params.set("q", filter.q);
    if (filter.statuses?.length)
      params.set("status", filter.statuses.join(","));
    if (cursor) params.set("cursor", cursor);
    return apiFetch<VocabUploadItemPage>(
      `/api/v1/me/vocabulary/upload-items?${params}`,
    );
  },
};

/**
 * How often the list asks while words are being checked, and when it gives up.
 *
 * New words are checked by a job that starts as the paste is stored, so answers
 * arrive in seconds; three seconds feels immediate without being a load test of
 * one's own account.
 *
 * It stops when nothing has moved for a minute, not after a fixed number of
 * polls: a hundred new words can take longer than a minute and are still
 * visibly progressing, while a word that never finishes -- worker stopped, no
 * provider configured -- should not be asked about for ever. A new paste
 * resets it, and so does a reload.
 */
const UPLOAD_POLL_MS = 3_000;
const UPLOAD_STALL_MS = 60_000;

function pendingIn(data: InfiniteData<VocabUploadList> | undefined): number {
  let pending = 0;
  for (const page of data?.pages ?? []) {
    for (const upload of page.items) pending += upload.pending_count ?? 0;
  }
  return pending;
}

/**
 * A learner's uploads, newest first, a page at a time.
 *
 * Refetched on a timer while anything is still pending, so the screen does not
 * sit on "12 being checked" until somebody reloads it by hand. When the last
 * word settles, the decks are refreshed too: their word counts just changed.
 */
export function useUploads(enabled: boolean) {
  const queryClient = useQueryClient();
  const progress = useRef({ pending: -1, changedAt: 0 });

  const query = useInfiniteQuery({
    queryKey: vocabularyKeys.uploads(),
    queryFn: ({ pageParam }) => uploadApi.list(pageParam),
    initialPageParam: undefined as string | undefined,
    getNextPageParam: (last) =>
      last.page?.has_more ? last.page.next_cursor : undefined,
    enabled,
    refetchInterval: (q) => {
      const pending = pendingIn(q.state.data);
      if (pending === 0) return false;
      if (pending !== progress.current.pending) {
        progress.current = { pending, changedAt: q.state.dataUpdatedAt };
      }
      const stalledFor = q.state.dataUpdatedAt - progress.current.changedAt;
      return stalledFor > UPLOAD_STALL_MS ? false : UPLOAD_POLL_MS;
    },
  });

  const pending = pendingIn(query.data);
  const previous = useRef(pending);
  useEffect(() => {
    if (previous.current > 0 && pending === 0) {
      void queryClient.invalidateQueries({ queryKey: vocabularyKeys.decks() });
    }
    previous.current = pending;
  }, [pending, queryClient]);

  return query;
}

/**
 * The learner's words, filtered and a page at a time.
 *
 * Never served stale: a row reopened after its words were checked must show
 * the checked words, not the five-minute-old "being checked" it last saw.
 */
export function useUploadItems(filter: UploadItemFilter, enabled = true) {
  return useInfiniteQuery({
    queryKey: vocabularyKeys.uploadItems(filter),
    queryFn: ({ pageParam }) => uploadApi.items(filter, pageParam),
    initialPageParam: undefined as string | undefined,
    getNextPageParam: (last) =>
      last.page.has_more ? last.page.next_cursor : undefined,
    enabled,
    staleTime: 0,
  });
}

export function useDecks(enabled: boolean) {
  return useQuery({
    queryKey: vocabularyKeys.decks(),
    queryFn: () => deckApi.list(),
    enabled,
  });
}

/** Submits a paste and refreshes the lists it belongs to. */
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
