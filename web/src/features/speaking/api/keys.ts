export const speakingKeys = {
  all: ["speaking"] as const,
  submissions: (page: number, pageSize: number) =>
    ["speaking", "submissions", page, pageSize] as const,
  feedback: (attemptId: string) => ["speaking", "feedback", attemptId] as const,
};
