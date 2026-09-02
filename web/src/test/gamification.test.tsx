import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { http, HttpResponse } from "msw";
import React from "react";
import { I18nextProvider } from "react-i18next";
import { beforeEach, describe, expect, it } from "vitest";

import {
  BadgesWidget,
  GamificationSummarySection,
  LeaderboardWidget,
  QuestsWidget,
  StreakWidget,
  XPProgressBar,
} from "@/features/gamification";
import type { GamificationSummary } from "@/features/gamification";
import i18n, { initI18n } from "@/i18n";
import { server } from "./msw-server";

function renderWithProviders(ui: React.ReactElement) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return render(
    <I18nextProvider i18n={i18n}>
      <QueryClientProvider client={queryClient}>{ui}</QueryClientProvider>
    </I18nextProvider>,
  );
}

describe("Gamification Feature Slice (WP14)", () => {
  beforeEach(async () => {
    initI18n("en");
    await i18n.changeLanguage("en");
  });

  it("XPProgressBar renders level, total XP, and daily goal progress", () => {
    const summary: GamificationSummary = {
      total_xp: 1240,
      level: 5,
      level_start_xp: 1000,
      next_level_xp: 1500,
      xp_today: 60,
      daily_goal_xp: 50,
      streak: {
        current: 5,
        longest: 12,
        freezes_available: 2,
        hours_remaining: 8,
      },
      badges: [],
      quests: [],
      league: "silver",
    };

    renderWithProviders(<XPProgressBar summary={summary} />);

    expect(screen.getAllByText("Level 5").length).toBeGreaterThanOrEqual(1);
    expect(screen.getByText("1240 XP")).toBeInTheDocument();
    expect(screen.getByText("240 / 500 XP to Level 6")).toBeInTheDocument();
    expect(screen.getByText("60/50 XP today")).toBeInTheDocument();
    expect(screen.getByText(/Achieved/i)).toBeInTheDocument();
  });

  it("StreakWidget renders streak count, longest streak, and hours remaining", () => {
    const streak = {
      current: 7,
      longest: 30,
      freezes_available: 2,
      hours_remaining: 5,
    };

    renderWithProviders(<StreakWidget streak={streak} />);

    expect(screen.getByText("Day Streak")).toBeInTheDocument();
    expect(screen.getByText("7")).toBeInTheDocument();
    expect(screen.getByText("Best: 30 days")).toBeInTheDocument();
    expect(screen.getByText("5h left today")).toBeInTheDocument();
    expect(screen.getByText("2 freeze")).toBeInTheDocument();
  });

  it("StreakWidget allows using a streak freeze", async () => {
    const user = userEvent.setup();
    let freezeCalled = false;

    server.use(
      http.post("/api/v1/me/streak/freeze", () => {
        freezeCalled = true;
        return HttpResponse.json({
          current: 7,
          longest: 30,
          freezes_available: 1,
          hours_remaining: 0,
        });
      }),
    );

    const streak = {
      current: 7,
      longest: 30,
      freezes_available: 2,
      hours_remaining: 5,
    };

    renderWithProviders(<StreakWidget streak={streak} />);

    const useButton = screen.getByRole("button", { name: /Use/i });
    expect(useButton).toBeInTheDocument();

    await user.click(useButton);
    await waitFor(() => expect(freezeCalled).toBe(true));
  });

  it("LeaderboardWidget renders standings when opted in", async () => {
    server.use(
      http.get("/api/v1/leaderboard", () =>
        HttpResponse.json({
          entries: [
            {
              rank: 1,
              user_id: "0199a1c2-3d4e-7f80-9abc-def012345601",
              display_name: "Leader Player",
              xp: 500,
              is_self: false,
            },
            {
              rank: 2,
              user_id: "0199a1c2-3d4e-7f80-9abc-def012345602",
              display_name: "Second Player",
              xp: 320,
              is_self: true,
            },
          ],
        }),
      ),
    );

    renderWithProviders(<LeaderboardWidget currentLeague="gold" />);

    expect(await screen.findByText("Leader Player")).toBeInTheDocument();
    expect(screen.getByText("500 XP")).toBeInTheDocument();
    expect(screen.getByText("Second Player")).toBeInTheDocument();
    expect(screen.getByText("gold")).toBeInTheDocument();
  });

  it("LeaderboardWidget offers opt-in when learner has not opted in (403)", async () => {
    const user = userEvent.setup();
    let optInCalled = false;

    server.use(
      http.get("/api/v1/leaderboard", () =>
        HttpResponse.json(
          {
            type: "https://fluentra.dev/errors/forbidden",
            title: "Forbidden",
            status: 403,
            code: "LEADERBOARD_NOT_OPTED_IN",
            detail: "Learner has not opted in to leaderboards.",
          },
          { status: 403 },
        ),
      ),
      http.put("/api/v1/me/leaderboard-opt-in", () => {
        optInCalled = true;
        return new HttpResponse(null, { status: 204 });
      }),
    );

    renderWithProviders(<LeaderboardWidget currentLeague="silver" />);

    expect(
      await screen.findByText(/Compete with learners at your skill level/i),
    ).toBeInTheDocument();
    const joinButton = screen.getByRole("button", {
      name: /Join SILVER League/i,
    });
    expect(joinButton).toBeInTheDocument();

    await user.click(joinButton);
    await waitFor(() => expect(optInCalled).toBe(true));
  });

  it("BadgesWidget renders unlocked badges and tier styling", () => {
    const badges = [
      {
        code: "week_streak",
        name: "Seven Days",
        description: "Studied seven days in a row.",
        tier: "bronze" as const,
        earned_at: "2026-03-01T08:15:00Z",
      },
      {
        code: "level_ten",
        name: "Level Ten",
        description: "Reached level 10.",
        tier: "silver" as const,
        earned_at: "2026-03-02T10:00:00Z",
      },
    ];

    renderWithProviders(<BadgesWidget badges={badges} />);

    expect(screen.getByText("Seven Days")).toBeInTheDocument();
    expect(
      screen.getByText("Studied seven days in a row."),
    ).toBeInTheDocument();
    expect(screen.getByText("Level Ten")).toBeInTheDocument();
    expect(screen.getByText("2 unlocked")).toBeInTheDocument();
  });

  it("QuestsWidget renders active quests with progress and rewards", () => {
    const quests = [
      {
        code: "daily_practice",
        name: "Daily Practice",
        description: "Complete three activities today.",
        progress: { complete_activities: 2 },
        steps: { complete_activities: 3 },
        reward_xp: 30,
        expires_on: "2026-03-02",
      },
    ];

    renderWithProviders(<QuestsWidget quests={quests} />);

    expect(screen.getByText("Daily Practice")).toBeInTheDocument();
    expect(
      screen.getByText("Complete three activities today."),
    ).toBeInTheDocument();
    expect(screen.getByText("+30 XP")).toBeInTheDocument();
    expect(screen.getByText("2 / 3")).toBeInTheDocument();
  });

  it("GamificationSummarySection composites all widgets", () => {
    const summary: GamificationSummary = {
      total_xp: 50,
      level: 1,
      level_start_xp: 0,
      next_level_xp: 100,
      xp_today: 10,
      daily_goal_xp: 50,
      streak: {
        current: 1,
        longest: 1,
        freezes_available: 2,
        hours_remaining: 12,
      },
      badges: [],
      quests: [],
      league: "bronze",
    };

    renderWithProviders(<GamificationSummarySection summary={summary} />);

    expect(screen.getAllByText("Level 1").length).toBeGreaterThanOrEqual(1);
    expect(screen.getByText("50 XP")).toBeInTheDocument();
    expect(screen.getByText("Day Streak")).toBeInTheDocument();
  });
});
