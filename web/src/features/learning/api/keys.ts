export const learningKeys = {
  all: ["learning"] as const,
  dashboard: () => [...learningKeys.all, "dashboard"] as const,
  progress: () => [...learningKeys.all, "progress"] as const,
  dailySet: (level?: string) =>
    [...learningKeys.all, "dailySet", level ?? "default"] as const,
  placement: () => [...learningKeys.all, "placement"] as const,
  placementSession: (id: string) =>
    [...learningKeys.all, "placementSession", id] as const,
  startingPath: () => [...learningKeys.all, "startingPath"] as const,
  weeklyPlan: () => [...learningKeys.all, "weeklyPlan"] as const,
};
