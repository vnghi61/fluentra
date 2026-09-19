import { useQuery } from "@tanstack/react-query";

import { ApiError, apiFetch } from "@/api/client";
import type { components } from "@/types/api";
import { learningKeys } from "./keys";

export type PlacementOverview = components["schemas"]["PlacementOverview"];
export type PlacementSession = components["schemas"]["PlacementSession"];
export type PlacementItem = components["schemas"]["PlacementItem"];
export type PlacementProductiveItem =
  components["schemas"]["PlacementProductiveItem"];
export type PlacementResult = components["schemas"]["PlacementResult"];
export type PlacementResponse =
  components["schemas"]["PlacementAnswerRequest"]["response"];
export type StartingPath = components["schemas"]["StartingPath"];
export type WeeklyPlan = components["schemas"]["WeeklyPlan"];
export type WeeklyPlanItem = components["schemas"]["WeeklyPlanItem"];

/** The problem code of a failed placement request, if the server sent one. */
export function placementProblemCode(err: unknown): string | undefined {
  return err instanceof ApiError ? err.problem.code : undefined;
}

const PLACEMENT = "/api/v1/me/placement";

export const placementApi = {
  /** The current result, a session in progress, and whether to invite. */
  async getOverview(): Promise<PlacementOverview> {
    return apiFetch<PlacementOverview>(PLACEMENT);
  },

  async start(): Promise<PlacementSession> {
    return apiFetch<PlacementSession>(PLACEMENT, { method: "POST" });
  },

  async getSession(id: string): Promise<PlacementSession> {
    return apiFetch<PlacementSession>(`${PLACEMENT}/sessions/${id}`);
  },

  /**
   * Answers the current item. The same key sends the same answer again after a
   * dropped connection without grading it twice.
   */
  async answer(
    sessionId: string,
    activityId: string,
    response: PlacementResponse,
    idempotencyKey: string,
  ): Promise<PlacementSession> {
    return apiFetch<PlacementSession>(
      `${PLACEMENT}/sessions/${sessionId}/answers`,
      {
        method: "POST",
        headers: { "Idempotency-Key": idempotencyKey },
        body: JSON.stringify({ activity_id: activityId, response }),
      },
    );
  },

  /** Starts the writing and speaking part, or skips it. */
  async productive(
    sessionId: string,
    skip: boolean,
  ): Promise<PlacementSession> {
    return apiFetch<PlacementSession>(
      `${PLACEMENT}/sessions/${sessionId}/productive`,
      { method: "POST", body: JSON.stringify({ skip }) },
    );
  },

  async getStartingPath(): Promise<StartingPath> {
    return apiFetch<StartingPath>("/api/v1/me/path");
  },

  async getWeeklyPlan(): Promise<WeeklyPlan> {
    return apiFetch<WeeklyPlan>("/api/v1/me/weekly-plan");
  },
};

export function usePlacementOverview(enabled = true) {
  return useQuery({
    queryKey: learningKeys.placement(),
    queryFn: () => placementApi.getOverview(),
    enabled,
  });
}

export function usePlacementSession(id: string | null) {
  return useQuery({
    queryKey: learningKeys.placementSession(id ?? ""),
    queryFn: () => placementApi.getSession(id ?? ""),
    enabled: id !== null,
  });
}

export function useStartingPath(enabled = true) {
  return useQuery({
    queryKey: learningKeys.startingPath(),
    queryFn: () => placementApi.getStartingPath(),
    enabled,
  });
}

export function useWeeklyPlan(enabled = true) {
  return useQuery({
    queryKey: learningKeys.weeklyPlan(),
    queryFn: () => placementApi.getWeeklyPlan(),
    enabled,
  });
}
