import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen } from "@testing-library/react";
import userEventDefault from "@testing-library/user-event";
import { http, HttpResponse } from "msw";
import { I18nextProvider } from "react-i18next";
import { beforeEach, describe, expect, it } from "vitest";
import {
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
  RouterProvider,
} from "@tanstack/react-router";

import i18n, { initI18n } from "@/i18n";
import { DashboardPage } from "@/routes/DashboardPage";
import { useAuthStore } from "@/stores/authStore";
import type { DashboardResponse } from "@/features/learning";
import { server } from "./msw-server";

async function renderDashboard() {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  const rootRoute = createRootRoute();
  const indexRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: "/",
    component: () => (
      <I18nextProvider i18n={i18n}>
        <QueryClientProvider client={client}>
          <DashboardPage />
        </QueryClientProvider>
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

describe("DashboardPage (P10.1)", () => {
  beforeEach(async () => {
    initI18n("en");
    await i18n.changeLanguage("en");
    useAuthStore.setState({
      status: "authenticated",
      user: {
        userId: "0199a1c2-3d4e-7f80-9abc-def012345678",
        role: "user",
      },
    });
  });

  it("renders the 4 states: 1. not_started (new learner with no enrolment)", async () => {
    const notStartedData: DashboardResponse = {
      state: "not_started",
      due_reviews_count: 0,
      skill_mastery: [],
    };

    server.use(
      http.get("/api/v1/me/dashboard", () => HttpResponse.json(notStartedData)),
    );

    await renderDashboard();

    // Welcome title
    expect(await screen.findByText("Welcome to Fluentra")).toBeInTheDocument();

    // 1. Continue learning hero in not_started state
    expect(screen.getByText("Start Your English Journey")).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: /Explore Syllabus/i }),
    ).toBeInTheDocument();

    // 2. Reviews due in empty state (0 cards due)
    expect(
      screen.getByText(
        "Nothing due right now. New cards are scheduled as you finish lessons.",
      ),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: /Go to practice/i }),
    ).toBeInTheDocument();

    // 3. Skill progress empty state
    expect(
      screen.getByText(
        "No skill data yet. Complete lessons and exercises to build your mastery profile.",
      ),
    ).toBeInTheDocument();
  });

  it("renders the 4 states: 2. in_progress (mid-course learner with next activity and reviews due)", async () => {
    const inProgressData: DashboardResponse = {
      state: "in_progress",
      next_activity: {
        activity_id: "0199a1c2-3d4e-7f80-9abc-def01234567b",
        lesson_id: "0199a1c2-3d4e-7f80-9abc-def01234567a",
        unit_id: "0199a1c2-3d4e-7f80-9abc-def012345679",
        course_id: "0199a1c2-3d4e-7f80-9abc-def012345678",
        title: "Academic Word List - Topic 1",
        kind: "vocab_multiple_choice",
        skill: "vocabulary",
        estimated_minutes: 5,
      },
      due_reviews_count: 12,
      skill_mastery: [
        {
          skill: "vocabulary",
          level: "B1",
          confidence: 0.85,
          updated_at: "2026-08-24T09:00:00Z",
        },
        {
          skill: "grammar",
          level: "A2",
          confidence: 0.65,
          updated_at: "2026-08-24T09:00:00Z",
        },
      ],
    };

    server.use(
      http.get("/api/v1/me/dashboard", () => HttpResponse.json(inProgressData)),
    );

    await renderDashboard();

    // Hero card renders next activity
    expect(
      await screen.findByText("Academic Word List - Topic 1"),
    ).toBeInTheDocument();
    expect(screen.getByText("~5 mins")).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: /Continue Lesson/i }),
    ).toBeInTheDocument();

    // Reviews due renders count and start CTA
    expect(screen.getByText("12")).toBeInTheDocument();
    expect(screen.getByText("cards to review today")).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: /Start Review/i }),
    ).toBeInTheDocument();

    // Skill mastery renders progress bars
    expect(screen.getByText("Vocabulary")).toBeInTheDocument();
    expect(screen.getByText("B1")).toBeInTheDocument();
    expect(screen.getByText("85%")).toBeInTheDocument();

    expect(screen.getByText("Grammar")).toBeInTheDocument();
    expect(screen.getByText("A2")).toBeInTheDocument();
    expect(screen.getByText("65%")).toBeInTheDocument();
  });

  it("renders the 4 states: 3. completed (course completed learner)", async () => {
    const completedData: DashboardResponse = {
      state: "completed",
      due_reviews_count: 0,
      skill_mastery: [
        {
          skill: "vocabulary",
          level: "B2",
          confidence: 0.92,
          updated_at: "2026-08-24T09:00:00Z",
        },
      ],
    };

    server.use(
      http.get("/api/v1/me/dashboard", () => HttpResponse.json(completedData)),
    );

    await renderDashboard();

    expect(await screen.findByText("Course Completed")).toBeInTheDocument();
    expect(screen.getAllByText("All Caught Up!").length).toBeGreaterThanOrEqual(
      1,
    );
    expect(
      screen.getByText(
        /You have completed all activities in your enrolled course/i,
      ),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: /Review Lessons/i }),
    ).toBeInTheDocument();
  });

  it("renders the 4 states: 4. API error with retry functionality", async () => {
    let attempts = 0;
    server.use(
      http.get("/api/v1/me/dashboard", () => {
        attempts++;
        if (attempts === 1) {
          return HttpResponse.json(
            { title: "Internal Error", status: 500 },
            { status: 500 },
          );
        }
        return HttpResponse.json({
          state: "not_started",
          due_reviews_count: 0,
          skill_mastery: [],
        });
      }),
    );

    const user = userEventDefault.setup();
    await renderDashboard();

    // Should show error state
    expect(
      await screen.findByText("Unable to Load Dashboard"),
    ).toBeInTheDocument();
    const retryBtn = screen.getByRole("button", { name: /Try again/i });
    expect(retryBtn).toBeInTheDocument();

    // Clicking retry recovers
    await user.click(retryBtn);
    expect(
      await screen.findByText("Start Your English Journey"),
    ).toBeInTheDocument();
  });

  it("renders cleanly in Vietnamese (vi)", async () => {
    await i18n.changeLanguage("vi");
    const notStartedData: DashboardResponse = {
      state: "not_started",
      due_reviews_count: 0,
      skill_mastery: [],
    };

    server.use(
      http.get("/api/v1/me/dashboard", () => HttpResponse.json(notStartedData)),
    );

    await renderDashboard();

    expect(
      await screen.findByText("Chào mừng đến với Fluentra"),
    ).toBeInTheDocument();
    expect(
      screen.getByText("Bắt đầu hành trình tiếng Anh"),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: /Khám phá giáo trình/i }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: /Đến trang luyện tập/i }),
    ).toBeInTheDocument();
  });

  it("renders Phase 3 gamification widgets with real numbers", async () => {
    const data: DashboardResponse = {
      state: "in_progress",
      next_activity: {
        activity_id: "0199a1c2-3d4e-7f80-9abc-def01234567b",
        lesson_id: "0199a1c2-3d4e-7f80-9abc-def01234567a",
        unit_id: "0199a1c2-3d4e-7f80-9abc-def012345679",
        course_id: "0199a1c2-3d4e-7f80-9abc-def012345678",
        title: "Lesson 1",
        kind: "vocab_multiple_choice",
        skill: "vocabulary",
      },
      due_reviews_count: 0,
      skill_mastery: [],
    };

    server.use(
      http.get("/api/v1/me/dashboard", () => HttpResponse.json(data)),
      http.get("/api/v1/me/gamification", () =>
        HttpResponse.json({
          total_xp: 1240,
          level: 5,
          level_start_xp: 1000,
          next_level_xp: 1500,
          xp_today: 60,
          daily_goal_xp: 50,
          streak: {
            current: 7,
            longest: 31,
            last_active_on: "2026-03-02",
            freezes_available: 2,
            hours_remaining: 6,
          },
          badges: [
            {
              code: "week_streak",
              name: "Seven Days",
              description: "Studied seven days in a row.",
              tier: "bronze",
              earned_at: "2026-03-01T08:15:00Z",
            },
          ],
          quests: [
            {
              code: "daily_practice",
              name: "Daily Practice",
              description: "Complete three activities today.",
              progress: { complete_activities: 2 },
              steps: { complete_activities: 3 },
              reward_xp: 30,
              expires_on: "2026-03-02",
            },
          ],
          league: "silver",
        }),
      ),
      http.get("/api/v1/leaderboard", () =>
        HttpResponse.json({
          entries: [
            {
              rank: 1,
              user_id: "0199a1c2-3d4e-7f80-9abc-def012345601",
              display_name: "Top Learner",
              xp: 250,
              is_self: false,
            },
            {
              rank: 2,
              user_id: "0199a1c2-3d4e-7f80-9abc-def012345678",
              display_name: "Demo Learner",
              xp: 60,
              is_self: true,
            },
          ],
        }),
      ),
    );

    await renderDashboard();

    expect(await screen.findByText("Lesson 1")).toBeInTheDocument();

    // Assert real gamification numbers after async query resolves
    expect(await screen.findByText("1240 XP")).toBeInTheDocument();
    expect(screen.getAllByText("Level 5").length).toBeGreaterThanOrEqual(1);
    expect(screen.getByText("7")).toBeInTheDocument();
    expect(screen.getByText("Best: 31 days")).toBeInTheDocument();
    expect(screen.getByText("6h left today")).toBeInTheDocument();
    expect(screen.getByText("Daily Practice")).toBeInTheDocument();
    expect(screen.getByText("+30 XP")).toBeInTheDocument();
    expect(await screen.findByText("Top Learner")).toBeInTheDocument();
  });
});
