export const learningKeys = {
  all: ["learning"] as const,
  dashboard: () => [...learningKeys.all, "dashboard"] as const,
  progress: () => [...learningKeys.all, "progress"] as const,
  dailySet: (level?: string) =>
    [...learningKeys.all, "dailySet", level ?? "default"] as const,
};
