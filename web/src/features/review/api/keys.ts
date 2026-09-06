export const reviewKeys = {
  all: ["review"] as const,
  session: (deckId?: string) =>
    deckId
      ? ([...reviewKeys.all, "session", deckId] as const)
      : ([...reviewKeys.all, "session"] as const),
  dueCount: () => [...reviewKeys.all, "due-count"] as const,
  forecast: () => [...reviewKeys.all, "forecast"] as const,
};
