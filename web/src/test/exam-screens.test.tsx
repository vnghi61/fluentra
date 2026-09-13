import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, waitFor } from "@testing-library/react";
import { http, HttpResponse } from "msw";
import { I18nextProvider } from "react-i18next";
import { beforeEach, describe, expect, it } from "vitest";

import React from "react";
import i18n, { initI18n } from "@/i18n";
import { ExamList } from "@/features/exam/components/ExamList";
import { ExamReport } from "@/features/exam/components/ExamReport";
import { ExamSittingRunner } from "@/features/exam/components/ExamSittingRunner";
import type { ExamAttempt, ExamTemplate, ScoreReport } from "@/features/exam/types";
import { server } from "./msw-server";

const mockExam: ExamTemplate = {
  id: "33333333-3333-3333-3333-333333333333",
  slug: "b1-standard-exam",
  title_en: "B1 Standard Mock Exam",
  title_vi: "Đề thi thử chuẩn B1",
  description_en: "Full 4-skill mock examination at B1 level.",
  description_vi: "Kỳ thi thử 4 kỹ năng chuẩn CEFR B1.",
  level: "B1",
  format: "standard",
  total_minutes: 75,
  sections: [
    {
      id: "sec-1",
      exam_id: "33333333-3333-3333-3333-333333333333",
      position: 1,
      skill: "listening",
      exam_duration_minutes: 20,
      item_count: 3,
      item_kinds: ["listening_comprehension"],
    },
    {
      id: "sec-2",
      exam_id: "33333333-3333-3333-3333-333333333333",
      position: 2,
      skill: "reading",
      exam_duration_minutes: 25,
      item_count: 2,
      item_kinds: ["reading_comprehension"],
    },
  ],
};

const mockAttempt: ExamAttempt = {
  id: "44444444-4444-4444-4444-444444444444",
  exam_id: mockExam.id,
  exam_title: mockExam.title_en,
  mode: "exam",
  chosen_duration_minutes: 75,
  started_at: new Date().toISOString(),
  deadline_at: new Date(Date.now() + 75 * 60 * 1000).toISOString(),
  remaining_seconds: 4500,
  current_section: 1,
  status: "in_progress",
  server_time: new Date().toISOString(),
  section_activities: [
    {
      section_position: 1,
      skill: "listening",
      activities: [
        {
          id: "act-1",
          kind: "listening_comprehension",
          content_version_id: "55555555-5555-5555-5555-555555555555",
          weight: 1,
          config: {
            title: "Airport Announcement",
            questions: [
              {
                id: "q1",
                prompt: "What is the destination?",
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
          id: "act-2",
          kind: "reading_comprehension",
          content_version_id: "66666666-6666-6666-6666-666666666666",
          weight: 1,
          config: {
            title: "City Library Notice",
            passage: "The city library will be closed this Friday for maintenance.",
            questions: [
              {
                id: "q2",
                prompt: "Why is the library closed?",
                options: [
                  { id: "A", text: "Maintenance" },
                  { id: "B", text: "Holiday" },
                ],
              },
            ],
          },
        },
      ],
    },
    {
      section_position: 3,
      skill: "writing",
      activities: [
        {
          id: "act-3",
          kind: "writing_prompt",
          content_version_id: "77777777-7777-7777-7777-777777777777",
          weight: 1,
          config: {
            topic: "Urban Transport",
            prompt: "Write about the benefits of public transportation.",
            min_words: 100,
          },
        },
      ],
    },
    {
      section_position: 4,
      skill: "speaking",
      activities: [
        {
          id: "act-4",
          kind: "speaking_task",
          content_version_id: "88888888-8888-8888-8888-888888888888",
          weight: 1,
          config: {
            task_type: "read_aloud",
            prompt: "Read the text aloud clearly.",
            reference_text: "Welcome to the conference. Please turn off your mobile devices.",
            speaking_time_seconds: 45,
          },
        },
      ],
    },
  ],
};

const mockReport: ScoreReport = {
  attempt_id: mockAttempt.id,
  status: "ready",
  overall_score: 82.5,
  overall_band: "B1",
  disclaimer: "Not an official TOEIC score. Pronunciation not assessed.",
  per_section: [
    {
      skill: "listening",
      score: 85,
      max_score: 100,
      band: "B1",
      status: "scored",
      item_results: [
        {
          id: "q1",
          prompt: "What is the destination?",
          correct: true,
          explanation: { explanation_en: "The destination is Tokyo." },
        },
      ],
    },
    {
      skill: "reading",
      score: 80,
      max_score: 100,
      band: "B1",
      status: "scored",
    },
    {
      skill: "writing",
      score: 85,
      max_score: 100,
      band: "B1",
      status: "scored",
    },
    {
      skill: "speaking",
      score: 80,
      max_score: 100,
      band: "B1",
      status: "scored",
    },
  ],
  integrity_signals: [
    { kind: "tab_hidden", count: 1 },
  ],
};

import {
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
  RouterProvider,
} from "@tanstack/react-router";

async function renderWithProviders(ui: React.ReactElement) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });

  const rootRoute = createRootRoute();
  const testRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: "/",
    component: () => (
      <QueryClientProvider client={queryClient}>
        <I18nextProvider i18n={i18n}>
          <React.Suspense fallback={<div>Loading...</div>}>
            {ui}
          </React.Suspense>
        </I18nextProvider>
      </QueryClientProvider>
    ),
  });

  const router = createRouter({
    routeTree: rootRoute.addChildren([testRoute]),
    history: createMemoryHistory({ initialEntries: ["/"] }),
  });
  await router.load();
  return render(<RouterProvider router={router} />);
}

describe("Exam Frontend Screens", () => {
  beforeEach(async () => {
    await initI18n("en");
    server.use(
      http.get("/api/v1/exams", () => HttpResponse.json([mockExam])),
      http.get("/api/v1/exam-attempts", () =>
        HttpResponse.json({ attempts: [], total: 0 }),
      ),
    );
  });

  it("ExamList renders CEFR level switcher and available exam cards", async () => {
    await renderWithProviders(<ExamList userPracticeLevel="B1" />);

    // Check header and tabs
    expect(await screen.findByText("Standardized Exam Simulation")).toBeInTheDocument();
    expect(await screen.findByText("B1 Standard Mock Exam")).toBeInTheDocument();

    // Check daily sittings tracker
    expect(screen.getByText("5 / 5")).toBeInTheDocument();

    // Check mode buttons
    expect(screen.getByText("Exam Mode (75 min)")).toBeInTheDocument();
    expect(screen.getByText("Practice Mode (Custom)")).toBeInTheDocument();
  });

  it("ExamList shows empty pool state when level has no exams", async () => {
    server.use(http.get("/api/v1/exams", () => HttpResponse.json([])));
    await renderWithProviders(<ExamList userPracticeLevel="A2" />);

    await waitFor(() => {
      expect(
        screen.getByText("Exams not available yet at this level"),
      ).toBeInTheDocument();
    });
  });

  it("ExamReport displays overall score and official TOEIC disclaimers", async () => {
    await renderWithProviders(<ExamReport report={mockReport} />);

    // Score & CEFR band
    expect(await screen.findByText("83")).toBeInTheDocument(); // Math.round(82.5)
    expect(screen.getByText("B1")).toBeInTheDocument();

    // Disclaimers (§8 requirement)
    expect(screen.getByText("Not an official TOEIC score.")).toBeInTheDocument();
    expect(
      screen.getByText(/Pronunciation is not assessed/i),
    ).toBeInTheDocument();
    expect(
      screen.getByText(/uniquely drawn from our verified question pool/i),
    ).toBeInTheDocument();

    // Section performance breakdown
    expect(screen.getByText("listening")).toBeInTheDocument();
    expect(screen.getByText("reading")).toBeInTheDocument();
    expect(screen.getByText("writing")).toBeInTheDocument();
    expect(screen.getByText("speaking")).toBeInTheDocument();
  });

  it("ExamSittingRunner displays server timer and section navigation", async () => {
    await renderWithProviders(<ExamSittingRunner attempt={mockAttempt} />);

    // Header title and mode
    expect(await screen.findByText("B1 Standard Mock Exam")).toBeInTheDocument();
    expect(screen.getByText("Exam Mode")).toBeInTheDocument();

    // Section 1: Listening
    expect(screen.getAllByText("Airport Announcement").length).toBeGreaterThan(0);
    expect(screen.getByText("Tokyo")).toBeInTheDocument();
    expect(screen.getByText("Seoul")).toBeInTheDocument();

    // Action button
    expect(screen.getByText("Next Section")).toBeInTheDocument();
  });
});
