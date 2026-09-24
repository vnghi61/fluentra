import React from "react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import {
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
  RouterProvider,
} from "@tanstack/react-router";
import { fireEvent, render, screen, within } from "@testing-library/react";
import { http, HttpResponse } from "msw";
import { I18nextProvider } from "react-i18next";
import { beforeEach, describe, expect, it } from "vitest";

import i18n, { initI18n } from "@/i18n";
import { ExamHub } from "@/features/exam/components/ExamHub";
import type { ExamVersion } from "@/features/exam/api/examApi";
import { server } from "./msw-server";

const VERSION_ID = "20000000-0000-0000-0000-000000000001";
const BLUEPRINT_ID = "20000000-0000-0000-0010-000000000001";
const TEST_ID = "30000000-0000-0000-0000-000000000003";

const version: ExamVersion = {
  id: VERSION_ID,
  exam_family: "toeic_lr",
  code: "TOEIC_LR_2026",
  title: "TOEIC Listening & Reading (2026)",
  total_minutes: 120,
  scoring: { type: "raw_with_estimate" },
  source_url: "https://www.ets.org/toeic",
  verified_at: "2026-09-20",
  is_current: true,
  notes: "7 parts, 200 questions",
  distinct_tests_possible: 3,
  blueprints: [
    {
      id: BLUEPRINT_ID,
      name: "toeic_default",
      cefr_distribution: { B1: 0.5, B2: 0.5 },
      node_distribution: {},
    },
  ],
  parts: [
    {
      part_number: 1,
      section: "listening",
      kind: "photo_description",
      question_count: 100,
      group_size: 1,
    },
    {
      part_number: 5,
      section: "reading",
      kind: "mcq_gap",
      question_count: 100,
      group_size: 1,
    },
  ],
};

async function renderHub() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  const rootRoute = createRootRoute();
  const indexRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: "/",
    component: () => (
      <QueryClientProvider client={queryClient}>
        <I18nextProvider i18n={i18n}>
          <ExamHub />
        </I18nextProvider>
      </QueryClientProvider>
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

describe("exam hub", () => {
  beforeEach(async () => {
    await initI18n("en");
    server.use(
      http.get("/api/v1/exam-versions", () =>
        HttpResponse.json({ items: [version] }),
      ),
      http.get(`/api/v1/exam-versions/${VERSION_ID}/tests`, () =>
        HttpResponse.json({
          items: [
            { id: "30000000-0000-0000-0000-000000000001", number: 1, title: "Đề 1", question_count: 200, minutes: 120 },
            { id: "30000000-0000-0000-0000-000000000002", number: 2, title: "Đề 2", question_count: 200, minutes: 120 },
            { id: TEST_ID, number: 3, title: "Đề 3", question_count: 200, minutes: 120 },
          ],
        }),
      ),
    );
  });

  it("walks exam → Đề 3 → practice, Reading only, no limit", async () => {
    let started: unknown = null;
    server.use(
      http.post(`/api/v1/mock-tests/${TEST_ID}/attempts`, async ({ request }) => {
        started = await request.json();
        return HttpResponse.json(
          { id: "44444444-4444-4444-4444-444444444444", status: "in_progress" },
          { status: 201 },
        );
      }),
    );

    await renderHub();

    // Level 1 → level 2.
    fireEvent.click(await screen.findByText("TOEIC Listening & Reading (2026)"));
    const card = (await screen.findByText("Đề 3")).closest("li") as HTMLElement;
    fireEvent.click(within(card).getByRole("button", { name: "Options" }));

    // Level 3: practice, Reading only, no limit.
    fireEvent.click(within(card).getByRole("button", { name: /Practice/ }));
    fireEvent.click(within(card).getByLabelText("Reading"));
    fireEvent.click(within(card).getByLabelText("No time limit"));
    fireEvent.click(within(card).getByRole("button", { name: "Start" }));

    await screen.findByText("sitting");
    expect(started).toEqual({
      mode: "practice",
      unlimited: true,
      sections: [2],
    });
  });

  it("starts a random test from the version in one more tap", async () => {
    let composed: unknown = null;
    server.use(
      http.post("/api/v1/mock-tests", async ({ request }) => {
        composed = await request.json();
        return HttpResponse.json({ id: "99999999-0000-0000-0000-000000000001" }, { status: 201 });
      }),
      http.post(
        "/api/v1/mock-tests/99999999-0000-0000-0000-000000000001/attempts",
        () =>
          HttpResponse.json(
            { id: "44444444-4444-4444-4444-444444444444", status: "in_progress" },
            { status: 201 },
          ),
      ),
    );

    await renderHub();
    fireEvent.click(await screen.findByText("TOEIC Listening & Reading (2026)"));
    fireEvent.click(await screen.findByRole("button", { name: /Random test/ }));

    await screen.findByText("sitting");
    expect(composed).toEqual({ blueprint_id: BLUEPRINT_ID, mode: "random" });
  });
});
