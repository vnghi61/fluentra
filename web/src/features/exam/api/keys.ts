export const examKeys = {
  all: ["exams"] as const,
  lists: () => [...examKeys.all, "list"] as const,
  attempts: (limit = 10, offset = 0) =>
    [...examKeys.all, "attempts", limit, offset] as const,
  attempt: (id: string) => [...examKeys.all, "attempt", id] as const,
  report: (id: string) => [...examKeys.all, "report", id] as const,
  speakingFeedback: (attemptId: string) =>
    [...examKeys.all, "speaking-feedback", attemptId] as const,
};
