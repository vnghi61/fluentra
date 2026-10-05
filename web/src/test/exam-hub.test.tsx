import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import {
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
  Link,
  RouterProvider,
} from "@tanstack/react-router";
import { fireEvent, render, screen, within } from "@testing-library/react";
import { http, HttpResponse } from "msw";
import { I18nextProvider } from "react-i18next";
import { beforeEach, describe, expect, it } from "vitest";

import i18n, { initI18n } from "@/i18n";
import { ExamHub } from "@/features/exam/components/ExamHub";
import type { ExamVersion } from "@/features/exam/api/examApi";
import type { ExamAttemptListResponse } from "@/features/exam/types";
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
  fixed_test_count: 2,
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
    defaultOptions: { queries: { retry: false, staleTime: 5 * 60 * 1000 } },
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
    component: () => (
      <div>
        sitting<Link to="/">Back to hub</Link>
      </div>
    ),
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
      http.get("/api/v1/exam-attempts", () =>
        HttpResponse.json({
          items: [],
          total: 0,
          sittings_today: 0,
          daily_limit: 3,
        }),
      ),
      http.get("/api/v1/exam-versions", () =>
        HttpResponse.json({ items: [version] }),
      ),
      http.get(`/api/v1/exam-versions/${VERSION_ID}/tests`, () =>
        HttpResponse.json({
          items: [
            {
              id: "30000000-0000-0000-0000-000000000001",
              number: 1,
              title: "Đề 1",
              question_count: 200,
              minutes: 120,
            },
            {
              id: "30000000-0000-0000-0000-000000000002",
              number: 2,
              title: "Đề 2",
              question_count: 200,
              minutes: 120,
            },
            {
              id: TEST_ID,
              number: 3,
              title: "Đề 3",
              question_count: 200,
              minutes: 120,
            },
          ],
        }),
      ),
    );
  });

  it("shows how many tests exist and the learner's best score on the card", async () => {
    server.use(
      http.get("/api/v1/exam-versions", () =>
        HttpResponse.json({ items: [{ ...version, best_score: 78.4 }] }),
      ),
    );
    await renderHub();

    expect(await screen.findByText("2 tests")).toBeInTheDocument();
    expect(screen.getByText("Best 78%")).toBeInTheDocument();
  });

  it("keeps a link to the unfinished sitting across fixed and custom tests", async () => {
    const attempts: ExamAttemptListResponse = {
      items: [
        {
          id: "44444444-4444-4444-4444-444444444444",
          exam_id: VERSION_ID,
          exam_title: version.title,
          mode: "practice",
          chosen_duration_minutes: 60,
          unlimited: false,
          started_at: "2026-09-30T14:03:32Z",
          deadline_at: "2026-09-30T15:03:32Z",
          remaining_seconds: 3600,
          current_section: 1,
          status: "in_progress",
          server_time: "2026-09-30T14:03:32Z",
          section_activities: [],
        },
      ],
      total: 1,
      sittings_today: 1,
      daily_limit: 3,
    };
    server.use(
      http.get("/api/v1/exam-attempts", () => HttpResponse.json(attempts)),
    );
    await renderHub();
    const resume = await screen.findByRole("link", { name: /Continue/ });
    expect(resume).toHaveAttribute(
      "href",
      "/exams/44444444-4444-4444-4444-444444444444",
    );
    fireEvent.click(await screen.findByText(version.title));
    expect(screen.getByRole("link", { name: /Continue/ })).toBeInTheDocument();
    fireEvent.click(
      screen.getByRole("button", { name: /Build a custom test/ }),
    );
    expect(screen.getByRole("link", { name: /Continue/ })).toBeInTheDocument();
    fireEvent.click(screen.getByRole("link", { name: /Continue/ }));
    expect(await screen.findByText("sitting")).toBeInTheDocument();
  });

  it("explains an attempt-in-progress refusal instead of displaying the raw server detail", async () => {
    server.use(
      http.post(`/api/v1/mock-tests/${TEST_ID}/attempts`, () =>
        HttpResponse.json(
          {
            status: 409,
            code: "ATTEMPT_IN_PROGRESS",
            title: "Conflict",
            detail: "raw backend detail",
          },
          { status: 409 },
        ),
      ),
    );
    await renderHub();
    fireEvent.click(await screen.findByText(version.title));
    const card = (await screen.findByText("Đề 3")).closest("li") as HTMLElement;
    fireEvent.click(within(card).getByRole("button", { name: "Choose mode" }));
    fireEvent.click(within(card).getByRole("button", { name: "Start" }));
    expect(await screen.findByRole("alert")).toHaveTextContent(
      "Continue and submit it before starting another test",
    );
    expect(screen.queryByText("raw backend detail")).not.toBeInTheDocument();
  });

  it("walks exam → Đề 3 → practice, Reading only, no limit", async () => {
    let started: unknown = null;
    server.use(
      http.post(
        `/api/v1/mock-tests/${TEST_ID}/attempts`,
        async ({ request }) => {
          started = await request.json();
          return HttpResponse.json(
            {
              id: "44444444-4444-4444-4444-444444444444",
              status: "in_progress",
            },
            { status: 201 },
          );
        },
      ),
    );

    await renderHub();

    // Level 1 → level 2.
    fireEvent.click(
      await screen.findByText("TOEIC Listening & Reading (2026)"),
    );
    const card = (await screen.findByText("Đề 3")).closest("li") as HTMLElement;
    fireEvent.click(within(card).getByRole("button", { name: "Choose mode" }));

    // Level 3: practice, Reading only, no limit.
    fireEvent.click(within(card).getByRole("button", { name: /Practice/ }));
    fireEvent.click(within(card).getByLabelText("Reading"));
    fireEvent.click(
      within(card).getByRole("button", { name: "No time limit" }),
    );
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
        return HttpResponse.json(
          { id: "99999999-0000-0000-0000-000000000001" },
          { status: 201 },
        );
      }),
      http.post(
        "/api/v1/mock-tests/99999999-0000-0000-0000-000000000001/attempts",
        () =>
          HttpResponse.json(
            {
              id: "44444444-4444-4444-4444-444444444444",
              status: "in_progress",
            },
            { status: 201 },
          ),
      ),
    );

    await renderHub();
    fireEvent.click(
      await screen.findByText("TOEIC Listening & Reading (2026)"),
    );
    fireEvent.click(await screen.findByRole("button", { name: /Random test/ }));

    await screen.findByText("sitting");
    expect(composed).toEqual({ blueprint_id: BLUEPRINT_ID, mode: "random" });
  });

  it("only expands one test mode picker at a time", async () => {
    await renderHub();
    fireEvent.click(await screen.findByText(version.title));
    const first = (await screen.findByText("Đề 1")).closest(
      "li",
    ) as HTMLElement;
    const second = (await screen.findByText("Đề 2")).closest(
      "li",
    ) as HTMLElement;
    fireEvent.click(within(first).getByRole("button", { name: "Choose mode" }));
    expect(
      within(first).getByRole("button", { name: "Start" }),
    ).toBeInTheDocument();
    fireEvent.click(
      within(second).getByRole("button", { name: "Choose mode" }),
    );
    expect(
      within(first).queryByRole("button", { name: "Start" }),
    ).not.toBeInTheDocument();
    expect(
      within(second).getByRole("button", { name: "Start" }),
    ).toBeInTheDocument();
  });

  it("switches between custom duration and no limit without retaining the input", async () => {
    await renderHub();
    fireEvent.click(await screen.findByText(version.title));
    const card = (await screen.findByText("Đề 3")).closest("li") as HTMLElement;
    fireEvent.click(within(card).getByRole("button", { name: "Choose mode" }));
    fireEvent.click(within(card).getByRole("button", { name: "Practice" }));
    fireEvent.click(within(card).getByRole("button", { name: "Custom" }));
    expect(
      within(card).getByRole("spinbutton", { name: "Duration (minutes)" }),
    ).toBeInTheDocument();
    fireEvent.click(
      within(card).getByRole("button", { name: "No time limit" }),
    );
    expect(within(card).queryByRole("spinbutton")).not.toBeInTheDocument();
  });

  it("starts timed practice using the selected duration preset", async () => {
    let started: unknown = null;
    server.use(
      http.post(
        `/api/v1/mock-tests/${TEST_ID}/attempts`,
        async ({ request }) => {
          started = await request.json();
          return HttpResponse.json(
            {
              id: "44444444-4444-4444-4444-444444444444",
              status: "in_progress",
            },
            { status: 201 },
          );
        },
      ),
    );
    await renderHub();
    fireEvent.click(await screen.findByText(version.title));
    const card = (await screen.findByText("Đề 3")).closest("li") as HTMLElement;
    fireEvent.click(within(card).getByRole("button", { name: "Choose mode" }));
    fireEvent.click(within(card).getByRole("button", { name: "Practice" }));
    fireEvent.click(within(card).getByRole("button", { name: "90 min" }));
    fireEvent.click(within(card).getByRole("button", { name: "Start" }));
    await screen.findByText("sitting");
    expect(started).toEqual({
      mode: "practice",
      unlimited: false,
      chosen_duration_minutes: 90,
    });
  });

  it("refreshes the still-fresh attempt list after returning from a newly started sitting", async () => {
    let open = false;
    const attemptId = "44444444-4444-4444-4444-444444444444";
    server.use(
      http.get("/api/v1/exam-attempts", () =>
        HttpResponse.json({
          items: open
            ? [
                {
                  id: attemptId,
                  status: "in_progress",
                  exam_title: version.title,
                },
              ]
            : [],
          total: open ? 1 : 0,
          sittings_today: open ? 1 : 0,
          daily_limit: 3,
        }),
      ),
      http.post(`/api/v1/mock-tests/${TEST_ID}/attempts`, () => {
        open = true;
        return HttpResponse.json({ id: attemptId }, { status: 201 });
      }),
    );
    await renderHub();
    fireEvent.click(await screen.findByText(version.title));
    const card = (await screen.findByText("Đề 3")).closest("li") as HTMLElement;
    fireEvent.click(within(card).getByRole("button", { name: "Choose mode" }));
    fireEvent.click(within(card).getByRole("button", { name: "Start" }));
    fireEvent.click(await screen.findByRole("link", { name: "Back to hub" }));
    expect(
      await screen.findByRole("link", { name: /Continue/ }),
    ).toHaveAttribute("href", `/exams/${attemptId}`);
  });

  it("keeps the resume link visible outside the sheet on mobile", async () => {
    const attempts: ExamAttemptListResponse = {
      items: [
        {
          id: "44444444-4444-4444-4444-444444444444",
          exam_id: VERSION_ID,
          exam_title: version.title,
          mode: "practice",
          chosen_duration_minutes: 60,
          unlimited: false,
          started_at: "2026-09-30T14:03:32Z",
          deadline_at: "2026-09-30T15:03:32Z",
          remaining_seconds: 3600,
          current_section: 1,
          status: "in_progress",
          server_time: "2026-09-30T14:03:32Z",
          section_activities: [],
        },
      ],
      total: 1,
      sittings_today: 1,
      daily_limit: 3,
    };
    server.use(
      http.get("/api/v1/exam-attempts", () => HttpResponse.json(attempts)),
    );
    await renderHub();
    const resume = await screen.findByRole("link", { name: /Continue/ });
    expect(resume).toBeInTheDocument();
    expect(resume).toHaveAttribute(
      "href",
      "/exams/44444444-4444-4444-4444-444444444444",
    );

    const practiceOptionsBtn = screen.getByRole("button", {
      name: /Practice options/i,
    });
    expect(practiceOptionsBtn).toBeInTheDocument();
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();

    fireEvent.click(practiceOptionsBtn);
    expect(await screen.findByRole("dialog")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /Continue/ })).toBeInTheDocument();
  });

  it("below md, opens practice options sheet and reaches an option", async () => {
    await renderHub();

    const trigger = await screen.findByRole("button", {
      name: /Practice options/i,
    });
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();

    fireEvent.click(trigger);
    const dialog = await screen.findByRole("dialog");
    expect(dialog).toBeInTheDocument();

    // Reaches custom test builder inside sheet
    const customOption = within(dialog).getByRole("button", {
      name: /Build a custom test/i,
    });
    fireEvent.click(customOption);
    expect(
      within(dialog).getByRole("button", { name: /Back to the tests/i }),
    ).toBeInTheDocument();

    // Back to test list and reaches version choice inside sheet
    fireEvent.click(
      within(dialog).getByRole("button", { name: /Back to the tests/i }),
    );
    const versionOption = within(dialog).getByText(version.title);
    fireEvent.click(versionOption);
    expect(
      await within(dialog).findByRole("button", { name: /Random test/i }),
    ).toBeInTheDocument();
  });
});
