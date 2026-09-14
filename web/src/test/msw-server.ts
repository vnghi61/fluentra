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
  http.get("/api/v1/me/placement", () => {
    return HttpResponse.json({
      result: null,
      active_session: null,
      retake_available_at: null,
      invite_available: false,
    });
  }),
  http.get("/api/v1/me/weekly-plan", () => {
    return HttpResponse.json({
      week_start: "2026-09-14",
      minutes_goal: 90,
      items: [],
      progress: { minutes: 0, items_done: 0, items_total: 0 },
    });
  }),
  http.get("/api/v1/me/path", () => {
    return HttpResponse.json({
      level: "A2",
      level_source: "default",
      courses: [],
    });
  }),
];

export const server = setupServer(...handlers);
