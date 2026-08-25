export const learningKeys = {
  all: ["learning"] as const,
  dashboard: () => [...learningKeys.all, "dashboard"] as const,
  progress: () => [...learningKeys.all, "progress"] as const,
};
