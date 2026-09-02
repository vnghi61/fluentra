import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { apiFetch } from "@/api/client";
import type { components } from "@/types/api";
import { gamificationKeys } from "./keys";

export type GamificationSummary = components["schemas"]["GamificationSummary"];
export type Streak = components["schemas"]["Streak"];
export type Badge = components["schemas"]["Badge"];
export type Quest = components["schemas"]["Quest"];
export type LeaderboardResponse = components["schemas"]["LeaderboardResponse"];
export type LeaderboardEntry = components["schemas"]["LeaderboardEntry"];
export type SetDailyGoalRequest = components["schemas"]["SetDailyGoalRequest"];
export type SetLeaderboardOptInRequest =
  components["schemas"]["SetLeaderboardOptInRequest"];

export const gamificationApi = {
  /** Fetch the caller's full gamification summary (XP, level, streak, badges, quests, league) */
  async getGamificationSummary(): Promise<GamificationSummary> {
    return apiFetch<GamificationSummary>("/api/v1/me/gamification");
  },

  /** Fetch the caller's streak and freeze status */
  async getStreak(): Promise<Streak> {
    return apiFetch<Streak>("/api/v1/me/streak");
  },

  /** Spend a streak freeze for today */
  async useStreakFreeze(): Promise<Streak> {
    return apiFetch<Streak>("/api/v1/me/streak/freeze", {
      method: "POST",
    });
  },

  /** Fetch this week's leaderboard standings for the caller's league */
  async getLeaderboard(): Promise<LeaderboardResponse> {
    return apiFetch<LeaderboardResponse>("/api/v1/leaderboard");
  },

  /** Join or leave the opt-in leaderboard */
  async setLeaderboardOptIn(optIn: boolean): Promise<void> {
    await apiFetch<void>("/api/v1/me/leaderboard-opt-in", {
      method: "PUT",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        opt_in: optIn,
      } satisfies SetLeaderboardOptInRequest),
    });
  },

  /** Set daily goal XP target */
  async setDailyGoal(dailyGoalXP: number): Promise<GamificationSummary> {
    return apiFetch<GamificationSummary>("/api/v1/me/daily-goal", {
      method: "PUT",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        daily_goal_xp: dailyGoalXP,
      } satisfies SetDailyGoalRequest),
    });
  },
};

/** Hook to fetch the full gamification summary */
export function useGamificationSummary() {
  return useQuery({
    queryKey: gamificationKeys.summary(),
    queryFn: () => gamificationApi.getGamificationSummary(),
    staleTime: 60 * 1000,
  });
}

/** Hook to fetch streak details */
export function useStreak() {
  return useQuery({
    queryKey: gamificationKeys.streak(),
    queryFn: () => gamificationApi.getStreak(),
    staleTime: 60 * 1000,
  });
}

/** Hook to spend a streak freeze */
export function useUseStreakFreeze() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: () => gamificationApi.useStreakFreeze(),
    onSuccess: (updatedStreak) => {
      queryClient.setQueryData(gamificationKeys.streak(), updatedStreak);
      void queryClient.invalidateQueries({
        queryKey: gamificationKeys.summary(),
      });
    },
  });
}

/** Hook to fetch the weekly leaderboard */
export function useLeaderboard(enabled = true) {
  return useQuery({
    queryKey: gamificationKeys.leaderboard(),
    queryFn: () => gamificationApi.getLeaderboard(),
    staleTime: 60 * 1000,
    retry: (failureCount, error: unknown) => {
      // Don't retry if not opted in (403 LEADERBOARD_NOT_OPTED_IN)
      if (
        typeof error === "object" &&
        error !== null &&
        "problem" in error &&
        (error as { problem: { status: number } }).problem.status === 403
      ) {
        return false;
      }
      return failureCount < 2;
    },
    enabled,
  });
}

/** Hook to update leaderboard opt-in preference */
export function useSetLeaderboardOptIn() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (optIn: boolean) => gamificationApi.setLeaderboardOptIn(optIn),
    onSuccess: () => {
      void queryClient.invalidateQueries({
        queryKey: gamificationKeys.leaderboard(),
      });
      void queryClient.invalidateQueries({
        queryKey: gamificationKeys.summary(),
      });
    },
  });
}

/** Hook to set daily XP goal */
export function useSetDailyGoal() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (dailyGoalXP: number) =>
      gamificationApi.setDailyGoal(dailyGoalXP),
    onSuccess: (summary) => {
      queryClient.setQueryData(gamificationKeys.summary(), summary);
      void queryClient.invalidateQueries({
        queryKey: gamificationKeys.streak(),
      });
    },
  });
}
