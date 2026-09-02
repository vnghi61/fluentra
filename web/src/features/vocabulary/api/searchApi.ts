import { useEffect, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { apiFetch } from "@/api/client";
import type { components } from "@/types/api";
import { vocabularyKeys } from "./keys";

export type WordSummary = components["schemas"]["WordSummary"];
export type WordSearchResponse = components["schemas"]["WordSearchResponse"];
export type WordDetail = components["schemas"]["WordDetail"];
export type AddWordToDeckRequest =
  components["schemas"]["AddWordToDeckRequest"];
export type DeckItem = components["schemas"]["DeckItem"];

export const vocabularySearchKeys = {
  all: ["vocabulary", "search"] as const,
  query: (q: string) => [...vocabularySearchKeys.all, q] as const,
  word: (lemma: string) => ["vocabulary", "word", lemma] as const,
};

export const searchApi = {
  /** Search dictionary for words matching prefix `q` */
  async searchWords(q: string, limit = 10): Promise<WordSearchResponse> {
    if (!q.trim()) {
      return { results: [], total: 0 };
    }
    const params = new URLSearchParams({
      q: q.trim(),
      limit: String(limit),
    });
    return apiFetch<WordSearchResponse>(
      `/api/v1/vocabulary/search?${params.toString()}`,
    );
  },

  /** Fetch full word detail including its senses */
  async getWordDetail(lemma: string): Promise<WordDetail> {
    return apiFetch<WordDetail>(
      `/api/v1/vocabulary/words/${encodeURIComponent(lemma)}`,
    );
  },

  /** Add an existing dictionary sense to a deck (0 LLM model calls) */
  async addWordToDeck(deckId: string, senseId: string): Promise<DeckItem> {
    return apiFetch<DeckItem>(`/api/v1/vocabulary/decks/${deckId}/words`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        word_sense_id: senseId,
      } satisfies AddWordToDeckRequest),
    });
  },
};

/**
 * How long typing has to pause before the dictionary is asked.
 *
 * Long enough to swallow the middle of a word, short enough that a learner who
 * has stopped typing does not notice waiting.
 */
const SEARCH_DEBOUNCE_MS = 250;

/** Holds a value still until it has stopped changing for `delayMs`. */
function useDebouncedValue<T>(value: T, delayMs: number): T {
  const [settled, setSettled] = useState(value);

  useEffect(() => {
    const timer = setTimeout(() => setSettled(value), delayMs);
    return () => clearTimeout(timer);
  }, [value, delayMs]);

  return settled;
}

/**
 * Query hook for debounced dictionary word search.
 *
 * The debounce is real now. This hook described itself as debounced while
 * querying on every keystroke, so typing "leisure" asked the server seven
 * questions and threw six answers away -- on a free dyno that is six cold
 * requests deep in the path of somebody trying to type a word.
 */
export function useSearchVocabulary(query: string, enabled = true) {
  const settled = useDebouncedValue(query.trim(), SEARCH_DEBOUNCE_MS);
  return useQuery({
    queryKey: vocabularySearchKeys.query(settled),
    queryFn: () => searchApi.searchWords(settled),
    enabled: enabled && settled.length >= 1,
    staleTime: 5 * 60 * 1000,
  });
}

/** Hook to fetch word detail */
export function useWordDetail(lemma: string, enabled = true) {
  const trimmed = lemma.trim();
  return useQuery({
    queryKey: vocabularySearchKeys.word(trimmed),
    queryFn: () => searchApi.getWordDetail(trimmed),
    enabled: enabled && trimmed.length > 0,
    staleTime: 5 * 60 * 1000,
  });
}

/** Mutation hook to add an existing word sense to deck */
export function useAddWordToDeck() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ deckId, senseId }: { deckId: string; senseId: string }) =>
      searchApi.addWordToDeck(deckId, senseId),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: vocabularyKeys.all });
    },
  });
}
