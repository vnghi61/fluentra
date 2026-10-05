import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, within } from "@testing-library/react";
import userEventDefault from "@testing-library/user-event";
import { http, HttpResponse } from "msw";
import { I18nextProvider } from "react-i18next";
import { afterAll, beforeEach, describe, expect, it } from "vitest";
import {
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
  RouterProvider,
} from "@tanstack/react-router";

import i18n, { initI18n, loadLocale } from "@/i18n";
import { DashboardPage } from "@/routes/DashboardPage";
import { useAuthStore } from "@/stores/authStore";
import type { DashboardResponse } from "@/features/learning";
import type { components } from "@/types/api";
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
    await initI18n("en");
    await loadLocale("en");
    await i18n.changeLanguage("en");
    useAuthStore.setState({
      status: "authenticated",
      user: {
        userId: "0199a1c2-3d4e-7f80-9abc-def012345678",
        role: "user",
      },
    });
    server.use(
      http.get("/api/v1/me/learning-profile", () =>
        HttpResponse.json({
          target_exam: "none",
          motivations: [],
          created_at: "2026-10-01T00:00:00Z",
          updated_at: "2026-10-01T00:00:00Z",
        } satisfies components["schemas"]["LearningProfile"]),
      ),
      http.get("/api/v1/me/progress", () =>
        HttpResponse.json({
          courses: [],
          skills: [],
        } satisfies components["schemas"]["ProgressResponse"]),
      ),
      http.get("/api/v1/me/foundation/next", () =>
        HttpResponse.json(
          { title: "No published foundation topic", status: 404 },
          { status: 404 },
        ),
      ),
    );
  });

  afterAll(async () => {
    await loadLocale("en");
    await i18n.changeLanguage("en");
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

    // Greeting title
    expect(
      await screen.findByRole("heading", {
        level: 1,
        name: /Good (morning|afternoon|evening)!/,
      }),
    ).toBeInTheDocument();

    // 1. Continue learning hero in not_started state
    expect(screen.getByText("Start Your English Journey")).toBeInTheDocument();
    expect(
      within(
        screen.getByRole("region", { name: "Continue Learning" }),
      ).getByRole("link", { name: /Explore Syllabus/i }),
    ).toHaveAttribute("href", "/learn");

    // 2. Reviews due in empty state (0 cards due)
    expect(
      screen.getByText("All caught up! No cards due right now."),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("link", { name: /View vocabulary/i }),
    ).toHaveAttribute("href", "/practice/my-words");

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
    expect(screen.getByText("5 min")).toBeInTheDocument();
    expect(
      screen.getByRole("link", { name: /Continue learning/i }),
    ).toHaveAttribute(
      "href",
      `/learn/lesson/${inProgressData.next_activity!.lesson_id}`,
    );

    // Reviews due renders count and start CTA
    expect(screen.getByText("12 words to review")).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: /Review now/i }),
    ).toBeInTheDocument();

    // Skill mastery renders progress bars
    const skillsRegion = within(
      screen.getByRole("region", { name: "Skill Progress" }),
    );
    expect(skillsRegion.getByText("Vocabulary")).toBeInTheDocument();
    expect(skillsRegion.getByText("B1")).toBeInTheDocument();
    expect(skillsRegion.getByText("85%")).toBeInTheDocument();

    expect(skillsRegion.getByText("Grammar")).toBeInTheDocument();
    expect(skillsRegion.getByText("A2")).toBeInTheDocument();
    expect(skillsRegion.getByText("65%")).toBeInTheDocument();
    expect(
      skillsRegion.getByRole("link", { name: /View details/i }),
    ).toHaveAttribute("href", "/progress");
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
      screen.getByRole("link", { name: /Review Lessons/i }),
    ).toHaveAttribute("href", "/learn");
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
    // Only the locale being read is bundled, so the Vietnamese
    // translations have to be fetched before the switch. `setLocale` does
    // this in the app; calling `changeLanguage` alone renders raw keys.
    await loadLocale("vi");
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
      await screen.findByRole("heading", {
        level: 1,
        name: /Chào buổi (sáng|chiều|tối)!/,
      }),
    ).toBeInTheDocument();
    expect(
      screen.getByText("Bắt đầu hành trình tiếng Anh"),
    ).toBeInTheDocument();
    expect(
      within(screen.getByRole("region", { name: "Tiếp tục học" })).getByRole(
        "link",
        { name: /Khám phá giáo trình/i },
      ),
    ).toHaveAttribute("href", "/learn");
    expect(screen.getByRole("link", { name: /Xem từ vựng/i })).toHaveAttribute(
      "href",
      "/practice/my-words",
    );
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
    expect(await screen.findByText("60 / 50 XP")).toBeInTheDocument();
    expect(
      screen.getByRole("progressbar", { name: "Daily XP Goal" }),
    ).toHaveAttribute("aria-valuenow", "100");
    expect(screen.getByText("Level 5")).toBeInTheDocument();
    expect(screen.getByText("260 XP to next")).toBeInTheDocument();
    expect(screen.getByText("7 days")).toBeInTheDocument();
    expect(screen.getByText("0 / 1")).toBeInTheDocument();
    expect(
      screen.getByRole("link", { name: /Leaderboard →/i }),
    ).toHaveAttribute("href", "/progress");
  });

  it("keeps the weekly plan compact while allowing every lesson to be reached", async () => {
    const plan: components["schemas"]["WeeklyPlan"] = {
      week_start: "2026-09-14",
      minutes_goal: 90,
      progress: { minutes: 30, items_done: 2, items_total: 7 },
      items: Array.from({ length: 7 }, (_, index) => ({
        kind: "lesson",
        lesson_id: `0199a1c2-3d4e-7f80-9abc-def01234560${index}`,
        title: `Weekly lesson ${index + 1}`,
        minutes: 10,
        done: index < 2,
      })),
    };
    server.use(
      http.get("/api/v1/me/dashboard", () =>
        HttpResponse.json({
          state: "not_started",
          due_reviews_count: 0,
          skill_mastery: [],
        } satisfies DashboardResponse),
      ),
      http.get("/api/v1/me/weekly-plan", () => HttpResponse.json(plan)),
    );
    const user = userEventDefault.setup();
    await renderDashboard();
    const aside = within(
      await screen.findByRole("complementary", { name: "This week's plan" }),
    );
    const viewAll = await aside.findByRole("button", { name: "View all" });
    expect(aside.getAllByRole("link", { name: /^Weekly lesson/ })).toHaveLength(
      5,
    );
    expect(viewAll).toHaveAttribute("aria-expanded", "false");
    expect(aside.getByRole("progressbar")).toHaveAttribute(
      "aria-valuenow",
      "33",
    );
    await user.click(viewAll);
    expect(aside.getAllByRole("link", { name: /^Weekly lesson/ })).toHaveLength(
      7,
    );
    expect(
      aside.getByRole("link", { name: "Weekly lesson 7" }),
    ).toHaveAttribute("href", `/learn/lesson/${plan.items[6]!.lesson_id}`);
    const showLess = aside.getByRole("button", { name: "Show less" });
    expect(showLess).toHaveAttribute("aria-expanded", "true");
    await user.click(showLess);
    expect(aside.getAllByRole("link", { name: /^Weekly lesson/ })).toHaveLength(
      5,
    );
  });

  it("renders not_started state with recommended lesson CTA when next_activity exists", async () => {
    const notStartedWithLesson: DashboardResponse = {
      state: "not_started",
      next_activity: {
        activity_id: "0199a1c2-3d4e-7f80-9abc-def01234567b",
        lesson_id: "0199a1c2-3d4e-7f80-9abc-def01234567a",
        unit_id: "0199a1c2-3d4e-7f80-9abc-def012345679",
        course_id: "0199a1c2-3d4e-7f80-9abc-def012345678",
        title: "Alphabet & Sounds",
        kind: "vocab_multiple_choice",
        skill: "vocabulary",
      },
      due_reviews_count: 0,
      skill_mastery: [],
    };

    server.use(
      http.get("/api/v1/me/dashboard", () =>
        HttpResponse.json(notStartedWithLesson),
      ),
    );

    await renderDashboard();

    const hero = within(
      await screen.findByRole("region", { name: "Continue Learning" }),
    );
    expect(hero.getByText("Start Your English Journey")).toBeInTheDocument();
    expect(hero.getByText("Alphabet & Sounds")).toBeInTheDocument();
    const cta = hero.getByRole("link", { name: /Start First Lesson/i });
    expect(cta).toHaveAttribute(
      "href",
      `/learn/lesson/${notStartedWithLesson.next_activity!.lesson_id}`,
    );
  });

  it("renders hero, today progress, daily practice and reviews in DOM order", async () => {
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
      due_reviews_count: 5,
      skill_mastery: [],
    };

    server.use(
      http.get("/api/v1/me/dashboard", () => HttpResponse.json(data)),
      http.get("/api/v1/me/gamification", () =>
        HttpResponse.json({
          total_xp: 100,
          level: 1,
          level_start_xp: 0,
          next_level_xp: 200,
          xp_today: 10,
          daily_goal_xp: 50,
          streak: { current: 1, longest: 1, freezes_available: 0 },
          badges: [],
          quests: [],
          league: "bronze",
        }),
      ),
    );

    await renderDashboard();

    const hero = await screen.findByRole("region", {
      name: "Continue Learning",
    });
    const gamificationSection = await screen.findByRole("region", {
      name: "Learning Motivation & Progress",
    });
    const practiceSection = screen.getByRole("region", {
      name: "Today's Practice Set",
    });
    const reviewsSection = screen.getByRole("region", { name: "Reviews Due" });

    expect(
      hero.compareDocumentPosition(gamificationSection) &
        Node.DOCUMENT_POSITION_FOLLOWING,
    ).toBeTruthy();
    expect(
      gamificationSection.compareDocumentPosition(practiceSection) &
        Node.DOCUMENT_POSITION_FOLLOWING,
    ).toBeTruthy();
    expect(
      practiceSection.compareDocumentPosition(reviewsSection) &
        Node.DOCUMENT_POSITION_FOLLOWING,
    ).toBeTruthy();
  });

  it("allows expanding the mobile weekly-plan summary to reveal all items", async () => {
    const plan: components["schemas"]["WeeklyPlan"] = {
      week_start: "2026-09-14",
      minutes_goal: 90,
      progress: { minutes: 45, items_done: 3, items_total: 6 },
      items: Array.from({ length: 6 }, (_, index) => ({
        kind: "lesson",
        lesson_id: `0199a1c2-3d4e-7f80-9abc-def01234560${index}`,
        title: `Mobile weekly item ${index + 1}`,
        minutes: 15,
        done: index < 3,
      })),
    };
    server.use(
      http.get("/api/v1/me/dashboard", () =>
        HttpResponse.json({
          state: "not_started",
          due_reviews_count: 0,
          skill_mastery: [],
        } satisfies DashboardResponse),
      ),
      http.get("/api/v1/me/weekly-plan", () => HttpResponse.json(plan)),
    );
    const user = userEventDefault.setup();
    await renderDashboard();

    const aside = within(
      await screen.findByRole("complementary", { name: "This week's plan" }),
    );

    expect(await aside.findByText("45 of 90 minutes")).toBeInTheDocument();
    expect(await aside.findByText("3 of 6 done")).toBeInTheDocument();

    const toggleBtn = aside.getByRole("button", {
      name: /This week's plan/i,
    });
    expect(toggleBtn).toHaveAttribute("aria-expanded", "false");

    await user.click(toggleBtn);
    expect(toggleBtn).toHaveAttribute("aria-expanded", "true");

    expect(
      aside.getAllByRole("link", { name: /^Mobile weekly item/ }),
    ).toHaveLength(6);
    expect(
      aside.getByRole("link", { name: "Mobile weekly item 6" }),
    ).toHaveAttribute("href", `/learn/lesson/${plan.items[5]!.lesson_id}`);

    await user.click(toggleBtn);
    expect(toggleBtn).toHaveAttribute("aria-expanded", "false");
  });
});
