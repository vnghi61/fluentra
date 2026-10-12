export const vocabularyKeys = {
  all: ["vocabulary"] as const,
  uploads: () => [...vocabularyKeys.all, "uploads"] as const,
  // Under uploads(), so the invalidation a submit does reaches every word list.
  uploadItems: (filter: {
    uploadId?: string | undefined;
    q?: string | undefined;
    statuses?: readonly string[] | undefined;
  }) => [...vocabularyKeys.uploads(), "items", filter] as const,
  decks: () => [...vocabularyKeys.all, "decks"] as const,
};
