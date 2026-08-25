import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen } from "@testing-library/react";
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
import { ProgressPage } from "@/routes/ProgressPage";
import type { ProgressResponse } from "@/features/learning";
import { server } from "./msw-server";

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
  const router = createRouter({
    routeTree: rootRoute.addChildren([progressRoute]),
    history: createMemoryHistory({ initialEntries: ["/progress"] }),
  });
  await router.load();
  return render(<RouterProvider router={router} />);
}

describe("ProgressPage (P10.5)", () => {
  beforeEach(async () => {
    initI18n("en");
    await i18n.changeLanguage("en");
  });

  it("renders intentional empty state with 'not started yet' badges when no progress exists", async () => {
    const emptyProgress: ProgressResponse = {
      courses: [],
      skills: [],
    };

    server.use(
      http.get("/api/v1/me/progress", () => HttpResponse.json(emptyProgress)),
    );

    await renderProgress();

    expect(await screen.findByText("Learning Progress")).toBeInTheDocument();
    expect(screen.getByText("No course progress recorded yet.")).toBeInTheDocument();

    // Inactive skills render "Not started yet"
    const notStartedBadges = screen.getAllByText("Not started yet");
    expect(notStartedBadges.length).toBe(6);
  });

  it("renders active course progress and skill mastery percentages", async () => {
    const activeProgress: ProgressResponse = {
      courses: [
        {
          course_id: "0199a1c2-3d4e-7f80-9abc-def012345678",
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
      http.get("/api/v1/me/progress", () => HttpResponse.json(activeProgress)),
    );

    await renderProgress();

    expect(await screen.findByText("Course #1")).toBeInTheDocument();
    expect(screen.getByText("12 / 40 Activities Completed")).toBeInTheDocument();

    expect(screen.getByText("Vocabulary")).toBeInTheDocument();
    expect(screen.getByText("B1")).toBeInTheDocument();
    expect(screen.getByText("85%")).toBeInTheDocument();

    expect(screen.getByText("Grammar")).toBeInTheDocument();
    expect(screen.getByText("A2")).toBeInTheDocument();
    expect(screen.getByText("65%")).toBeInTheDocument();
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
