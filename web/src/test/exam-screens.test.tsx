import React from "react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import {
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
  RouterProvider,
} from "@tanstack/react-router";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { http, HttpResponse } from "msw";
import { I18nextProvider } from "react-i18next";
import { beforeEach, describe, expect, it, vi } from "vitest";

import i18n, { initI18n } from "@/i18n";
import { ExamList } from "@/features/exam/components/ExamList";
import { ExamReport } from "@/features/exam/components/ExamReport";
import { ExamSittingRunner } from "@/features/exam/components/ExamSittingRunner";
import type {
  ExamAttempt,
  ExamTemplate,
  ScoreReport,
} from "@/features/exam/types";
import { server } from "./msw-server";

const EXAM_ID = "10000000-0000-0000-0000-0000000000b1";
const SITTING_ID = "44444444-4444-4444-4444-444444444444";
const LISTENING_ID = "55555555-5555-5555-5555-555555555501";
const READING_ID = "55555555-5555-5555-5555-555555555502";

const exam: ExamTemplate = {
  id: EXAM_ID,
  slug: "mock-toeic-b1",
  title_en: "TOEIC Mock Exam (B1)",
  title_vi: "Bài thi thử TOEIC (B1)",
  level: "B1",
  format: "mock_toeic",
  total_minutes: 75,
};

const attempt: ExamAttempt = {
  id: SITTING_ID,
  exam_id: EXAM_ID,
  exam_title: exam.title_en,
  mode: "exam",
  chosen_duration_minutes: 75,
  started_at: "2026-09-14T09:00:00Z",
  deadline_at: "2026-09-14T10:15:00Z",
  remaining_seconds: 4500,
  current_section: 1,
  section_remaining_seconds: 1200,
  status: "in_progress",
  server_time: "2026-09-14T09:00:00Z",
  section_activities: [
    {
      section_position: 1,
      skill: "listening",
      activities: [
        {
          id: LISTENING_ID,
          kind: "listening_comprehension",
          content_version_id: "66666666-6666-6666-6666-666666666601",
          weight: 1,
          config: {
            title: "Airport announcement",
            questions: [
              {
                id: "q1",
                prompt: "Where is the flight going?",
                options: [
                  { id: "A", text: "Tokyo" },
                  { id: "B", text: "Seoul" },
                ],
              },
            ],
          },
        },
      ],
    },
    {
      section_position: 2,
      skill: "reading",
      activities: [
        {
          id: READING_ID,
          kind: "reading_comprehension",
          content_version_id: "66666666-6666-6666-6666-666666666602",
          weight: 1,
          config: {
            passage: "The library is closed on Friday.",
            questions: [],
          },
        },
      ],
    },
  ],
};

const report: ScoreReport = {
  attempt_id: SITTING_ID,
  mode: "exam",
  status: "partial",
  overall_score: 82.5,
  overall_band: "B2",
  disclaimer: "Not an official TOEIC score. Pronunciation not assessed.",
  per_section: [
    {
      position: 1,
      skill: "listening",
      status: "scored",
      score: 85,
      max_score: 100,
      items: [
        {
          activity_id: LISTENING_ID,
          content_version_id: "66666666-6666-6666-6666-666666666601",
          kind: "listening_comprehension",
          status: "graded",
          score: 4,
          max_score: 5,
          item_results: [
            { id: "q1", correct: true },
            { id: "q2", correct: false },
          ],
        },
      ],
    },
    {
      position: 4,
      skill: "speaking",
      status: "not_scored",
      max_score: 100,
      items: [
        {
          activity_id: "55555555-5555-5555-5555-555555555504",
          content_version_id: "66666666-6666-6666-6666-666666666604",
          kind: "speaking_task",
          status: "failed",
          score: 0,
          max_score: 0,
        },
      ],
    },
  ],
  integrity_signals: [{ kind: "tab_hidden", count: 2 }],
};

async function renderWithProviders(ui: React.ReactElement) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  const rootRoute = createRootRoute();
  const route = createRoute({
    getParentRoute: () => rootRoute,
    path: "/",
    component: () => (
      <QueryClientProvider client={queryClient}>
        <I18nextProvider i18n={i18n}>{ui}</I18nextProvider>
      </QueryClientProvider>
    ),
  });
  const router = createRouter({
    routeTree: rootRoute.addChildren([route]),
    history: createMemoryHistory({ initialEntries: ["/"] }),
  });
  await router.load();
  return render(<RouterProvider router={router} />);
}

describe("exam screens", () => {
  beforeEach(async () => {
    await initI18n("en");
    server.use(
      http.get("/api/v1/exams", () => HttpResponse.json([exam])),
      http.get("/api/v1/exam-attempts", () =>
        HttpResponse.json({
          items: [],
          total: 0,
          sittings_today: 1,
          daily_limit: 5,
        }),
      ),
    );
  });

  it("lists the exam at the learner's practice level with today's sittings left", async () => {
    await renderWithProviders(<ExamList userPracticeLevel="B1" />);

    expect(await screen.findByText("TOEIC Mock Exam (B1)")).toBeInTheDocument();
    expect(
      await screen.findByText("4 of 5 sittings left today"),
    ).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /B1/ })).toHaveAttribute(
      "aria-pressed",
      "true",
    );
    expect(screen.getByText("Exam mode · 75 min")).toBeInTheDocument();
  });

  it("shows an empty pool as not available yet, not as an error", async () => {
    server.use(
      http.post(`/api/v1/exams/${EXAM_ID}/attempts`, () =>
        HttpResponse.json(
          {
            type: "about:blank",
            title: "Not Found",
            status: 404,
            code: "EXAM_POOL_EMPTY",
          },
          {
            status: 404,
            headers: { "Content-Type": "application/problem+json" },
          },
        ),
      ),
    );
    await renderWithProviders(<ExamList userPracticeLevel="B1" />);

    fireEvent.click(await screen.findByText("Exam mode · 75 min"));

    expect(
      await screen.findByText("Exams are not available at this level yet"),
    ).toBeInTheDocument();
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
  });

  it("reports a failed section as not scored, never as zero, with the disclaimers", async () => {
    await renderWithProviders(<ExamReport report={report} />);

    expect(await screen.findByText("83")).toBeInTheDocument();
    expect(screen.getByText("B2")).toBeInTheDocument();
    expect(
      screen.getByText("Not an official TOEIC score."),
    ).toBeInTheDocument();
    expect(screen.getByText(/Pronunciation not assessed/)).toBeInTheDocument();
    expect(
      screen.getByText(/two sittings hold different items/),
    ).toBeInTheDocument();
    expect(screen.getAllByText("Not scored").length).toBeGreaterThan(0);
    expect(screen.getByText("1 of 2 questions correct")).toBeInTheDocument();
    expect(screen.getByText("Tab hidden: 2")).toBeInTheDocument();
  });

  it("reviews each question: what was chosen, the right answer and why", async () => {
    const reviewed: ScoreReport = {
      ...report,
      status: "ready",
      integrity_signals: [],
      per_section: [
        {
          position: 2,
          skill: "reading",
          status: "scored",
          score: 50,
          max_score: 100,
          items: [
            {
              activity_id: READING_ID,
              content_version_id: "66666666-6666-6666-6666-666666666602",
              kind: "reading_comprehension",
              status: "graded",
              score: 50,
              max_score: 100,
              item_results: [
                { id: "q1", correct: false, correct_answer: "A" },
                { id: "q2", correct: true, correct_answer: "B" },
              ],
              response: { answers: { q1: "B", q2: "B" } },
              content: {
                passage: "The library is closed on Friday.",
                questions: [
                  {
                    id: "q1",
                    prompt: "When is the library closed?",
                    options: [
                      { id: "A", text: "Friday" },
                      { id: "B", text: "Monday" },
                    ],
                    correct_option_id: "A",
                    explanation: {
                      explanation_en:
                        "The passage says it is closed on Friday.",
                      explanation_vi: "Đoạn văn nói thư viện đóng cửa thứ Sáu.",
                    },
                  },
                  {
                    id: "q2",
                    prompt: "What is closed?",
                    options: [
                      { id: "A", text: "The bank" },
                      { id: "B", text: "The library" },
                    ],
                    correct_option_id: "B",
                  },
                ],
              },
            },
          ],
        },
      ],
    };
    await renderWithProviders(<ExamReport report={reviewed} />);

    expect(await screen.findByText("Answer sheet")).toBeInTheDocument();
    expect(screen.getByText("When is the library closed?")).toBeInTheDocument();
    expect(screen.getByText("B · Monday")).toBeInTheDocument();
    expect(screen.getAllByText("A · Friday").length).toBeGreaterThan(0);
    expect(
      screen.getByText("The passage says it is closed on Friday."),
    ).toBeInTheDocument();
    // Question 1 is on the answer sheet and in its question type's row.
    const links = screen.getAllByRole("link", { name: "1" });
    expect(links).toHaveLength(2);
    for (const link of links) {
      expect(link).toHaveAttribute("href", "#review-q-1");
    }
    expect(screen.getByText("Results by question type")).toBeInTheDocument();
    expect(screen.getByText("#Reading comprehension")).toBeInTheDocument();
  });

  it("starts a practice sitting of only the sections chosen", async () => {
    let body: unknown;
    server.use(
      http.post(`/api/v1/exams/${EXAM_ID}/attempts`, async ({ request }) => {
        body = await request.json();
        return HttpResponse.json(attempt, { status: 201 });
      }),
    );
    await renderWithProviders(<ExamList userPracticeLevel="B1" />);

    fireEvent.click(await screen.findByText("Practice mode"));
    fireEvent.click(
      screen.getByRole("checkbox", { name: /Section 1 · Listening/ }),
    );
    fireEvent.click(
      screen.getByRole("checkbox", { name: /Section 3 · Writing/ }),
    );
    fireEvent.click(screen.getByRole("button", { name: "Start" }));

    await waitFor(() =>
      expect(body).toEqual({
        mode: "practice",
        chosen_duration_minutes: 60,
        sections: [2, 4],
      }),
    );
  });

  it("numbers every question and jumps to one in another practice section", async () => {
    // A practice sitting has no section clock.
    const { section_remaining_seconds: _unused, ...rest } = attempt;
    await renderWithProviders(
      <ExamSittingRunner
        attempt={{ ...rest, mode: "practice" }}
        onSubmitted={vi.fn()}
      />,
    );

    expect(
      await screen.findByText("1. Where is the flight going?"),
    ).toBeInTheDocument();
    expect(screen.getByText("0 of 2 answered")).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Go to question 2" }));

    expect(
      await screen.findByText("The library is closed on Friday."),
    ).toBeInTheDocument();
  });

  it("saves only the open section's answers and moves on through the server", async () => {
    const saved = vi.fn();
    server.use(
      http.put(
        `/api/v1/exam-attempts/${SITTING_ID}/answers`,
        async ({ request }) => {
          saved(await request.json());
          return HttpResponse.json({
            saved: true,
            remaining_seconds: 4400,
            current_section: 1,
            section_remaining_seconds: 1100,
          });
        },
      ),
      http.post(`/api/v1/exam-attempts/${SITTING_ID}/sections/1/complete`, () =>
        HttpResponse.json({
          current_section: 2,
          remaining_seconds: 4400,
          section_remaining_seconds: 1500,
          submitted: false,
        }),
      ),
    );
    await renderWithProviders(
      <ExamSittingRunner attempt={attempt} onSubmitted={() => undefined} />,
    );

    fireEvent.click(await screen.findByRole("button", { name: /Tokyo/ }));
    fireEvent.click(screen.getByRole("button", { name: "Next section" }));

    await waitFor(() => expect(saved).toHaveBeenCalled());
    expect(saved.mock.calls[0]?.[0]).toEqual({
      answers: { [LISTENING_ID]: { answers: { q1: "A" } } },
      integrity_events: [],
      section_number: 1,
    });
    expect(
      await screen.findByText("The library is closed on Friday."),
    ).toBeInTheDocument();
  });
});
