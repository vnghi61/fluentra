import { setupServer } from "msw/node";
import { http, HttpResponse } from "msw";

// Default MSW API handlers matching OpenAPI contracts
export const handlers = [
  http.get("/api/v1/ping", () => {
    return HttpResponse.json({
      status: "ok",
      timestamp: new Date().toISOString(),
    });
  }),
  http.get("/api/v1/me/gamification", () => {
    return HttpResponse.json({
      total_xp: 0,
      level: 1,
      level_start_xp: 0,
      next_level_xp: 100,
      xp_today: 0,
      daily_goal_xp: 50,
      streak: {
        current: 0,
        longest: 0,
        freezes_available: 2,
        hours_remaining: 24,
      },
      badges: [],
      quests: [],
      league: "bronze",
    });
  }),
  http.get("/api/v1/leaderboard", () => {
    return HttpResponse.json({
      entries: [],
    });
  }),
];

export const server = setupServer(...handlers);
