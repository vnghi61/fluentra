import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import {
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
  RouterProvider,
} from "@tanstack/react-router";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { http, HttpResponse } from "msw";
import { I18nextProvider } from "react-i18next";
import { beforeEach, describe, expect, it } from "vitest";

import i18n, { initI18n } from "@/i18n";
import { MockTestComposer } from "@/features/exam/components/MockTestComposer";

import { server } from "./msw-server";

const version = {
  id: "20000000-0000-0000-0000-000000000002",
  exam_family: "vstep",
  code: "VSTEP_3_5",
  title: "VSTEP 3-5",
  total_minutes: 100,
  scoring: { type: "raw" },
  source_url: "https://example.test/vstep",
  verified_at: "2026-09-20",
  is_current: true,
  notes: "",
  distinct_tests_possible: 2,
  blueprints: [
    {
      id: "30000000-0000-0000-0010-000000000002",
      name: "vstep_default",
      cefr_distribution: { B1: 1 },
      node_distribution: {},
    },
  ],
  parts: [
    {
      part_number: 1,
      section: "listening",
      kind: "listening_comprehension",
      question_count: 35,
      group_size: 1,
    },
  ],
};

const mockTest = {
  id: "40000000-0000-0000-0000-000000000001",
  blueprint_id: version.blueprints[0]?.id ?? "",
  mode: "random",
  seed: 123,
  composition: [
    {
      part_id: "50000000-0000-0000-0000-000000000001",
      activity_ids: ["60000000-0000-0000-0000-000000000001"],
    },
  ],
  owner_id: "70000000-0000-0000-0000-000000000001",
  created_at: "2026-09-22T12:00:00Z",
};

async function renderComposer() {
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
          <MockTestComposer />
        </QueryClientProvider>
      </I18nextProvider>
    ),
  });
  const sittingRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: "/exams/$attemptId",
    component: () => <div>sitting</div>,
  });
  const router = createRouter({
    routeTree: rootRoute.addChildren([indexRoute, sittingRoute]),
    history: createMemoryHistory({ initialEntries: ["/"] }),
  });
  await router.load();
  return render(<RouterProvider router={router} />);
}

describe("MockTestComposer", () => {
  let composed: Record<string, unknown>[] = [];
  let started: string[] = [];

  beforeEach(async () => {
    composed = [];
    started = [];
    await initI18n("en");
    await i18n.changeLanguage("en");

    server.use(
      http.get("/api/v1/exam-versions", () =>
        HttpResponse.json({ items: [version] }),
      ),
      http.post("/api/v1/mock-tests", async ({ request }) => {
        composed.push((await request.json()) as Record<string, unknown>);
        return HttpResponse.json(mockTest, { status: 201 });
      }),
      http.post("/api/v1/mock-tests/:id/attempts", ({ params }) => {
        started.push(String(params.id));
        return HttpResponse.json({
          id: "80000000-0000-0000-0000-000000000001",
          exam_id: "90000000-0000-0000-0000-000000000001",
          exam_slug: "vstep-3-5",
          exam_title: "VSTEP 3-5",
          level: "B1",
          mode: "exam",
          chosen_duration_minutes: 100,
          unlimited: false,
          started_at: "2026-09-22T12:00:00Z",
          deadline_at: "2026-09-22T13:40:00Z",
          remaining_seconds: 6000,
          current_section: 1,
          status: "in_progress",
          server_time: "2026-09-22T12:00:00Z",
          section_activities: [],
        });
      }),
    );
  });

  it("shows the version and the coverage number it carries", async () => {
    await renderComposer();

    expect(await screen.findByText("VSTEP 3-5")).toBeInTheDocument();
    expect(screen.getByText(/2 distinct tests available/)).toBeInTheDocument();
  });

  it("composes with the chosen blueprint and mode, then sits the same test", async () => {
    await renderComposer();
    await screen.findByText("VSTEP 3-5");

    await userEvent.click(
      screen.getByRole("button", { name: /Compose the test/i }),
    );

    await waitFor(() => expect(composed).toHaveLength(1));
    expect(composed[0]).toEqual({
      blueprint_id: version.blueprints[0]?.id ?? "",
      mode: "random",
    });
    expect(await screen.findByText(/Your test is ready/)).toBeInTheDocument();

    await userEvent.click(
      screen.getByRole("button", { name: /Sit this test/i }),
    );
    await waitFor(() => expect(started).toEqual([mockTest.id]));
  });

  it("names the short part when the bank cannot fill the test", async () => {
    server.use(
      http.post("/api/v1/mock-tests", () =>
        HttpResponse.json(
          {
            type: "https://fluentra.dev/errors/INSUFFICIENT_ITEMS",
            title: "Conflict",
            status: 409,
            detail:
              "insufficient published questions to compose part listening-1: needed 35, available 0",
            code: "INSUFFICIENT_ITEMS",
          },
          { status: 409 },
        ),
      ),
    );
    await renderComposer();
    await screen.findByText("VSTEP 3-5");

    await userEvent.click(
      screen.getByRole("button", { name: /Compose the test/i }),
    );

    expect(
      await screen.findByText(/part listening-1.*needed 35, available 0/),
    ).toBeInTheDocument();
  });
});
