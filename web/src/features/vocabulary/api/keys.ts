export const vocabularyKeys = {
  all: ["vocabulary"] as const,
  uploads: () => [...vocabularyKeys.all, "uploads"] as const,
  upload: (id: string) => [...vocabularyKeys.uploads(), id] as const,
};
