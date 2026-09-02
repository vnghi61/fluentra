export const gamificationKeys = {
  all: ["gamification"] as const,
  summary: () => [...gamificationKeys.all, "summary"] as const,
  streak: () => [...gamificationKeys.all, "streak"] as const,
  leaderboard: () => [...gamificationKeys.all, "leaderboard"] as const,
};
