export const examKeys = {
  all: ["exams"] as const,
  lists: () => [...examKeys.all, "list"] as const,
  attempts: (page = 1, pageSize = 10) =>
    [...examKeys.all, "attempts", page, pageSize] as const,
  attempt: (id: string) => [...examKeys.all, "attempt", id] as const,
  report: (id: string) => [...examKeys.all, "report", id] as const,
};
