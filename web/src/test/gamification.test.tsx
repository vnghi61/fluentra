import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { http, HttpResponse } from "msw";
import React from "react";
import { I18nextProvider } from "react-i18next";
import { beforeEach, describe, expect, it } from "vitest";

import {
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
  RouterProvider,
} from "@tanstack/react-router";

import {
  BadgesWidget,
  GamificationSummarySection,
  LeaderboardWidget,
  QuestsWidget,
  StreakWidget,
  TodayProgressCard,
  XPProgressBar,
} from "@/features/gamification";
import type { GamificationSummary } from "@/features/gamification";
import i18n, { initI18n, loadLocale } from "@/i18n";
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

async function renderWithRouter(ui: React.ReactElement) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  const rootRoute = createRootRoute();
  const indexRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: "/",
    component: () => (
      <I18nextProvider i18n={i18n}>
        <QueryClientProvider client={queryClient}>{ui}</QueryClientProvider>
      </I18nextProvider>
    ),
  });
  const router = createRouter({
    routeTree: rootRoute.addChildren([indexRoute]),
    history: createMemoryHistory({ initialEntries: ["/"] }),
  });
  await router.load();
  return render(<RouterProvider router={router} />);
}

describe("Gamification Feature Slice (WP14)", () => {
  beforeEach(async () => {
    await initI18n("en");
    await loadLocale("en");
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

  it("LeaderboardWidget shows the standings to a learner who has not joined", async () => {
    const user = userEvent.setup();
    let optInCalled = false;

    server.use(
      // 200 with opted_in: false, not a 403. Reading the board never required
      // being in it; the endpoint used to say otherwise, which asked a learner
      // to publish their name and weekly XP to find out what they were joining.
      http.get("/api/v1/leaderboard", () =>
        HttpResponse.json({
          opted_in: false,
          entries: [
            {
              rank: 1,
              user_id: "11111111-1111-1111-1111-111111111111",
              display_name: "Someone Else",
              xp: 910,
              is_self: false,
            },
          ],
        }),
      ),
      http.put("/api/v1/me/leaderboard-opt-in", () => {
        optInCalled = true;
        return new HttpResponse(null, { status: 204 });
      }),
    );

    renderWithProviders(<LeaderboardWidget currentLeague="silver" />);

    // The board itself, not a pitch where the board should be.
    expect(await screen.findByText("Someone Else")).toBeInTheDocument();

    // And the offer to join it, alongside rather than instead.
    const joinButton = screen.getByRole("button", {
      name: /Join SILVER League/i,
    });
    await user.click(joinButton);
    await waitFor(() => expect(optInCalled).toBe(true));
  });

  it("LeaderboardWidget does not offer to join a league the learner is in", async () => {
    server.use(
      http.get("/api/v1/leaderboard", () =>
        HttpResponse.json({
          opted_in: true,
          entries: [
            {
              rank: 1,
              user_id: "11111111-1111-1111-1111-111111111111",
              display_name: "Someone Else",
              xp: 910,
              is_self: false,
            },
          ],
        }),
      ),
    );

    renderWithProviders(<LeaderboardWidget currentLeague="silver" />);

    expect(await screen.findByText("Someone Else")).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: /Join SILVER League/i }),
    ).not.toBeInTheDocument();
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

  it("QuestsWidget renders one compact line of text without card header or badge when empty", () => {
    renderWithProviders(<QuestsWidget quests={[]} />);

    expect(
      screen.getByText("No active quests right now. Check back tomorrow!"),
    ).toBeInTheDocument();
    expect(screen.queryByText("Active Quests")).not.toBeInTheDocument();
    expect(screen.queryByText(/active$/)).not.toBeInTheDocument();
  });

  it("GamificationSummarySection keeps truthful stats and the freeze action without duplicated widgets", async () => {
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

    // 1. Daily goal stat
    expect(screen.getByText("Daily Goal")).toBeInTheDocument();
    expect(screen.getByText("10 / 50 XP")).toBeInTheDocument();

    // 2. Day streak stat
    expect(screen.getByText("Day Streak")).toBeInTheDocument();
    expect(screen.getAllByText("Day Streak")).toHaveLength(1);
    expect(screen.getByText("1 days")).toBeInTheDocument();

    // 3. Level & XP stat
    expect(screen.getAllByText("Level 1").length).toBeGreaterThanOrEqual(1);
    expect(screen.getByText("50 / 100 XP")).toBeInTheDocument();
    expect(
      screen.getByRole("progressbar", { name: "Progress towards Level 2" }),
    ).toHaveAttribute("aria-valuenow", "50");
    expect(screen.getByText("50 XP to Level 2")).toBeInTheDocument();

    // Exactly one league mention (inside LeaderboardWidget, separate stat card removed)
    expect(screen.getAllByText(/bronze/i)).toHaveLength(1);

    // Empty quest state rendered as one compact line, no Card header
    expect(
      screen.getByText("No active quests right now. Check back tomorrow!"),
    ).toBeInTheDocument();
    expect(screen.queryByText("Active Quests")).not.toBeInTheDocument();

    let freezeCalls = 0;
    server.use(
      http.post("/api/v1/me/streak/freeze", () => {
        freezeCalls++;
        return HttpResponse.json({ ...summary.streak, freezes_available: 1 });
      }),
    );
    await userEvent
      .setup()
      .click(screen.getByRole("button", { name: "Use a streak freeze" }));
    expect(await screen.findByRole("status")).toHaveTextContent(
      "Your streak is protected today.",
    );
    expect(freezeCalls).toBe(1);
  });

  it("GamificationSummarySection still renders standings toggle and omits league name when summary.league is absent", () => {
    // The contract requires league; an old or partial payload may still omit it.
    const summaryWithoutLeague: Omit<GamificationSummary, "league"> = {
      total_xp: 50,
      level: 1,
      level_start_xp: 0,
      next_level_xp: 100,
      xp_today: 10,
      daily_goal_xp: 50,
      streak: {
        current: 1,
        longest: 1,
        freezes_available: 0,
        hours_remaining: 12,
      },
      badges: [],
      quests: [],
    };

    renderWithProviders(
      <GamificationSummarySection
        summary={summaryWithoutLeague as GamificationSummary}
      />,
    );

    // Standings toggle remains reachable whether or not league is set
    expect(
      screen.getByText("Standings and league participation"),
    ).toBeInTheDocument();

    // No 'Bronze' fallback or fabricated league name
    expect(screen.queryByText(/bronze/i)).not.toBeInTheDocument();
  });

  it("LeaderboardWidget renders without default league and allows opt-in when currentLeague is undefined", async () => {
    server.use(
      http.get("/api/v1/leaderboard", () =>
        HttpResponse.json({
          opted_in: false,
          entries: [],
        }),
      ),
    );

    renderWithProviders(<LeaderboardWidget />);

    expect(
      await screen.findByRole("button", { name: /^Join League$/i }),
    ).toBeInTheDocument();
    expect(screen.queryByText(/bronze/i)).not.toBeInTheDocument();
  });

  it("TodayProgressCard renders daily XP, streak, level, and leaderboard link", async () => {
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

    await renderWithRouter(<TodayProgressCard summary={summary} />);

    expect(screen.getByText("10 / 50 XP")).toBeInTheDocument();
    expect(
      screen.getByRole("progressbar", { name: "Daily XP Goal" }),
    ).toHaveAttribute("aria-valuenow", "20");
    expect(screen.getByText("20%")).toBeInTheDocument();
    expect(screen.getByText("1 days")).toBeInTheDocument();
    expect(screen.getByText("Level 1")).toBeInTheDocument();
    expect(screen.getByText("50 XP to next")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Leaderboard →" })).toHaveAttribute(
      "href",
      "/progress",
    );
  });

  it("TodayProgressCard shows start-streak text when streak.current is 0", async () => {
    const summary: GamificationSummary = {
      total_xp: 50,
      level: 1,
      level_start_xp: 0,
      next_level_xp: 100,
      xp_today: 10,
      daily_goal_xp: 50,
      streak: {
        current: 0,
        longest: 1,
        freezes_available: 0,
        hours_remaining: 0,
      },
      badges: [],
      quests: [],
      league: "bronze",
    };

    await renderWithRouter(<TodayProgressCard summary={summary} />);

    expect(screen.getByText("Start your streak today")).toBeInTheDocument();
  });

  it("TodayProgressCard renders quests count when quests are present", async () => {
    const summaryWithQuests: GamificationSummary = {
      total_xp: 120,
      level: 2,
      level_start_xp: 100,
      next_level_xp: 200,
      xp_today: 30,
      daily_goal_xp: 50,
      streak: {
        current: 3,
        longest: 5,
        freezes_available: 1,
        hours_remaining: 10,
      },
      badges: [],
      quests: [
        {
          code: "daily_practice",
          name: "Daily Practice",
          description: "Complete three activities today.",
          progress: { complete_activities: 3 },
          steps: { complete_activities: 3 },
          reward_xp: 30,
          expires_on: "2026-03-02",
        },
        {
          code: "review_cards",
          name: "Review Cards",
          description: "Review ten words.",
          progress: { reviews: 5 },
          steps: { reviews: 10 },
          reward_xp: 20,
          expires_on: "2026-03-02",
        },
      ],
      league: "silver",
    };

    await renderWithRouter(<TodayProgressCard summary={summaryWithQuests} />);

    expect(screen.getByText("30 / 50 XP")).toBeInTheDocument();
    expect(
      screen.getByRole("progressbar", { name: "Daily XP Goal" }),
    ).toHaveAttribute("aria-valuenow", "60");
    expect(screen.getByText("3 days")).toBeInTheDocument();
    expect(screen.getByText("Level 2")).toBeInTheDocument();
    expect(screen.getByText("80 XP to next")).toBeInTheDocument();
    expect(screen.getByText("Quests")).toBeInTheDocument();
    expect(screen.getByText("1 / 2")).toBeInTheDocument();
  });
});
