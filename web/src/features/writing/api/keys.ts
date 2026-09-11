export const writingKeys = {
  all: ["writing"] as const,
  submissions: (page: number, pageSize: number) =>
    ["writing", "submissions", page, pageSize] as const,
  feedback: (attemptId: string) => ["writing", "feedback", attemptId] as const,
};
