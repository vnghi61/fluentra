import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
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

import i18n, { initI18n, loadLocale } from "@/i18n";
import { ProgressPage } from "@/routes/ProgressPage";
import type { ProgressResponse } from "@/features/learning";
import type { CourseSummary } from "@/features/lesson";
import { server } from "./msw-server";

const courseA: CourseSummary = {
  id: "0199a1c2-3d4e-7f80-9abc-def012345678",
  slug: "test-course-a",
  title: "Course Alpha",
  cefr_from: "A1",
  cefr_to: "B1",
  status: "published",
  estimated_hours: 10,
};

const courseB: CourseSummary = {
  id: "0199a1c2-3d4e-7f80-9abc-def012345679",
  slug: "test-course-b",
  title: "Course Beta",
  cefr_from: "B1",
  cefr_to: "C1",
  status: "published",
  estimated_hours: 15,
};

async function renderProgress() {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  const rootRoute = createRootRoute();
  const progressRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: "/progress",
    component: () => (
      <I18nextProvider i18n={i18n}>
        <QueryClientProvider client={client}>
          <ProgressPage />
        </QueryClientProvider>
      </I18nextProvider>
    ),
  });
  const learnRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: "/learn",
    component: () => <div>Learn Destination</div>,
  });
  const router = createRouter({
    routeTree: rootRoute.addChildren([progressRoute, learnRoute]),
    history: createMemoryHistory({ initialEntries: ["/progress"] }),
  });
  await router.load();
  return render(<RouterProvider router={router} />);
}

describe("ProgressPage (P10.5)", () => {
  beforeEach(async () => {
    await initI18n("en");
    await loadLocale("en");
    await i18n.changeLanguage("en");
    server.use(
      http.get("/api/v1/courses", () => HttpResponse.json({ courses: [] })),
    );
  });

  it("renders intentional empty state with 0% skills and explore link when no progress exists", async () => {
    const emptyProgress: ProgressResponse = {
      courses: [],
      skills: [],
    };

    server.use(
      http.get("/api/v1/me/progress", () => HttpResponse.json(emptyProgress)),
    );

    await renderProgress();

    expect(await screen.findByText("Learning Progress")).toBeInTheDocument();
    expect(
      screen.getByText("You haven't started any courses yet."),
    ).toBeInTheDocument();

    const exploreLink = screen.getByRole("link", { name: "Explore courses" });
    expect(exploreLink).toBeInTheDocument();
    expect(exploreLink).toHaveAttribute("href", "/learn");

    // Inactive skills render 0% and progressbars with aria-valuenow 0
    const zeroPercents = screen.getAllByText("0%");
    expect(zeroPercents.length).toBe(6);

    const progressbars = screen.getAllByRole("progressbar");
    const zeroProgressbars = progressbars.filter(
      (pb) => pb.getAttribute("aria-valuenow") === "0",
    );
    expect(zeroProgressbars.length).toBe(6);
  });

  it("renders active course progress and skill mastery percentages", async () => {
    const activeProgress: ProgressResponse = {
      courses: [
        {
          course_id: courseA.id,
          status: "in_progress",
          completed_activities: 12,
          total_activities: 40,
          percentage: 30,
          score: 88,
        },
      ],
      skills: [
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
      http.get("/api/v1/courses", () =>
        HttpResponse.json({ courses: [courseA] }),
      ),
      http.get("/api/v1/me/progress", () => HttpResponse.json(activeProgress)),
    );

    await renderProgress();

    expect(await screen.findByText("Course Alpha")).toBeInTheDocument();
    expect(screen.getByText("12/40 activities · 30%")).toBeInTheDocument();

    expect(screen.getByText("Vocabulary")).toBeInTheDocument();
    expect(screen.getByText("B1")).toBeInTheDocument();
    expect(screen.getByText("85%")).toBeInTheDocument();

    expect(screen.getByText("Grammar")).toBeInTheDocument();
    expect(screen.getByText("A2")).toBeInTheDocument();
    expect(screen.getByText("65%")).toBeInTheDocument();
  });

  it("filters courses by status (All, Completed, In progress)", async () => {
    const twoCoursesProgress: ProgressResponse = {
      courses: [
        {
          course_id: courseA.id,
          status: "in_progress",
          completed_activities: 12,
          total_activities: 40,
          percentage: 30,
        },
        {
          course_id: courseB.id,
          status: "completed",
          completed_activities: 81,
          total_activities: 81,
          percentage: 100,
        },
      ],
      skills: [],
    };

    server.use(
      http.get("/api/v1/courses", () =>
        HttpResponse.json({ courses: [courseA, courseB] }),
      ),
      http.get("/api/v1/me/progress", () =>
        HttpResponse.json(twoCoursesProgress),
      ),
    );

    await renderProgress();

    const courseList = await screen.findByRole("list", {
      name: "Your courses",
    });

    // 'All' filter (default) shows both titles
    expect(within(courseList).getByText("Course Alpha")).toBeInTheDocument();
    expect(within(courseList).getByText("Course Beta")).toBeInTheDocument();

    const filterSelect = screen.getByLabelText("Filter courses");

    // Select 'Completed'
    await userEvent.selectOptions(filterSelect, "completed");
    expect(within(courseList).getByText("Course Beta")).toBeInTheDocument();
    expect(
      within(courseList).queryByText("Course Alpha"),
    ).not.toBeInTheDocument();
    expect(within(courseList).getByText("Completed")).toBeInTheDocument();

    // Select 'In progress'
    await userEvent.selectOptions(filterSelect, "in_progress");
    expect(within(courseList).getByText("Course Alpha")).toBeInTheDocument();
    expect(
      within(courseList).queryByText("Course Beta"),
    ).not.toBeInTheDocument();
    expect(
      within(courseList).getByText("Continue learning"),
    ).toBeInTheDocument();
  });

  it("renders row link with href containing /learn?course=<slug>", async () => {
    const progressWithCourseA: ProgressResponse = {
      courses: [
        {
          course_id: courseA.id,
          status: "in_progress",
          completed_activities: 12,
          total_activities: 40,
          percentage: 30,
        },
      ],
      skills: [],
    };

    server.use(
      http.get("/api/v1/courses", () =>
        HttpResponse.json({ courses: [courseA] }),
      ),
      http.get("/api/v1/me/progress", () =>
        HttpResponse.json(progressWithCourseA),
      ),
    );

    await renderProgress();

    const courseTitleElement = await screen.findByText("Course Alpha");
    const link = courseTitleElement.closest("a");
    expect(link).not.toBeNull();
    expect(link).toHaveAttribute(
      "href",
      expect.stringContaining(`/learn?course=${courseA.slug}`),
    );
  });

  it("renders accurate KPI metrics for in-progress and completed courses", async () => {
    const twoCoursesProgress: ProgressResponse = {
      courses: [
        {
          course_id: courseA.id,
          status: "in_progress",
          completed_activities: 12,
          total_activities: 40,
          percentage: 30,
        },
        {
          course_id: courseB.id,
          status: "completed",
          completed_activities: 81,
          total_activities: 81,
          percentage: 100,
        },
      ],
      skills: [],
    };

    server.use(
      http.get("/api/v1/courses", () =>
        HttpResponse.json({ courses: [courseA, courseB] }),
      ),
      http.get("/api/v1/me/progress", () =>
        HttpResponse.json(twoCoursesProgress),
      ),
    );

    await renderProgress();

    expect(await screen.findByTestId("courses-in-progress")).toHaveTextContent(
      "1",
    );
    expect(screen.getByTestId("courses-completed")).toHaveTextContent("1");
    expect(screen.getByTestId("activities-completed")).toHaveTextContent("93");
  });

  it("shows this week's study minutes from the weekly plan, not a derived total", async () => {
    server.use(
      http.get("/api/v1/me/progress", () =>
        HttpResponse.json({
          courses: [],
          skills: [],
        } satisfies ProgressResponse),
      ),
      http.get("/api/v1/me/weekly-plan", () =>
        HttpResponse.json({
          week_start: "2026-10-05",
          minutes_goal: 90,
          items: [],
          progress: { minutes: 95, items_done: 0, items_total: 0 },
        }),
      ),
    );

    await renderProgress();

    expect(await screen.findByTestId("study-this-week")).toHaveTextContent(
      "1h 35m",
    );
    expect(screen.getByText("Study time this week")).toBeInTheDocument();
    expect(screen.getByText("Weekly goal 1h 30m")).toBeInTheDocument();
  });

  it("leaves the study-time tile out when the weekly plan cannot be read", async () => {
    server.use(
      http.get("/api/v1/me/progress", () =>
        HttpResponse.json({
          courses: [],
          skills: [],
        } satisfies ProgressResponse),
      ),
      http.get("/api/v1/me/weekly-plan", () =>
        HttpResponse.json(
          { title: "Internal Error", status: 500 },
          { status: 500 },
        ),
      ),
    );

    await renderProgress();

    expect(
      await screen.findByTestId("activities-completed"),
    ).toBeInTheDocument();
    expect(screen.queryByTestId("study-this-week")).not.toBeInTheDocument();
  });

  it("shows filter-empty message when no courses match selected filter", async () => {
    const inProgressOnly: ProgressResponse = {
      courses: [
        {
          course_id: courseA.id,
          status: "in_progress",
          completed_activities: 12,
          total_activities: 40,
          percentage: 30,
        },
      ],
      skills: [],
    };

    server.use(
      http.get("/api/v1/courses", () =>
        HttpResponse.json({ courses: [courseA] }),
      ),
      http.get("/api/v1/me/progress", () => HttpResponse.json(inProgressOnly)),
    );

    await renderProgress();

    await screen.findByText("Course Alpha");

    const filterSelect = screen.getByLabelText("Filter courses");
    await userEvent.selectOptions(filterSelect, "completed");

    expect(
      screen.getByText("No courses match this filter."),
    ).toBeInTheDocument();
  });

  // P10 §1: every number on screen is a number the API actually returned.
  // "Total Study Time" and "Words Mastered" were computed here from the activity
  // count — three minutes and four words apiece — and rendered in the same weight
  // as the real figure, which is what made them read as measurements.
  // ProgressResponse carries neither.
  it("renders no figure GET /me/progress does not return", async () => {
    server.use(
      http.get("/api/v1/me/progress", () =>
        HttpResponse.json({
          courses: [
            {
              course_id: "0199a1c2-3d4e-7f80-9abc-def012345678",
              status: "in_progress",
              completed_activities: 12,
              total_activities: 40,
              percentage: 30,
            },
          ],
          skills: [],
        } satisfies ProgressResponse),
      ),
    );
    await renderProgress();

    // The one figure the response does carry.
    expect(await screen.findByText("12")).toBeInTheDocument();

    expect(screen.queryByText(/Total Study Time/i)).not.toBeInTheDocument();
    expect(screen.queryByText(/Words Mastered/i)).not.toBeInTheDocument();
    // 12 activities x 3 minutes, and x 4 words: the two numbers that used to be
    // derived from the one above.
    expect(screen.queryByText("36 mins")).not.toBeInTheDocument();
    expect(screen.queryByText("48")).not.toBeInTheDocument();
  });

  it("does NOT render any Phase 3 gamification features", async () => {
    const activeProgress: ProgressResponse = {
      courses: [],
      skills: [],
    };

    server.use(
      http.get("/api/v1/me/progress", () => HttpResponse.json(activeProgress)),
    );

    await renderProgress();

    await screen.findByText("Learning Progress");

    expect(screen.queryByText(/streak/i)).not.toBeInTheDocument();
    expect(screen.queryByText(/0 XP/i)).not.toBeInTheDocument();
    expect(screen.queryByText(/achievements/i)).not.toBeInTheDocument();
    expect(screen.queryByText(/leaderboard/i)).not.toBeInTheDocument();
  });
});
