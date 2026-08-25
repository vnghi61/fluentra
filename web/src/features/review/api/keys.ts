export const reviewKeys = {
  all: ["review"] as const,
  session: () => [...reviewKeys.all, "session"] as const,
  dueCount: () => [...reviewKeys.all, "due-count"] as const,
};
