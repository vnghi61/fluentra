import { render, screen, fireEvent } from "@testing-library/react";
import { I18nextProvider } from "react-i18next";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import {
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
  RouterProvider,
} from "@tanstack/react-router";
import { beforeEach, describe, expect, it } from "vitest";
import i18n, { initI18n } from "@/i18n";
import { ExamReport } from "@/features/exam/components/ExamReport";
import type { ScoreReport } from "@/features/exam/types";

const report: ScoreReport = {
  attempt_id: "report-test",
  mode: "exam",
  status: "ready",
  overall_score: 50,
  overall_band: "B1",
  per_section: [
    {
      position: 1,
      skill: "reading",
      status: "scored",
      score: 50,
      max_score: 100,
      items: [
        {
          activity_id: "reading-activity",
          content_version_id: "content-version",
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
                explanation: {
                  explanation_en: "The passage says it is closed on Friday.",
                },
              },
              {
                id: "q2",
                prompt: "What is closed?",
                options: [
                  { id: "A", text: "The bank" },
                  { id: "B", text: "The library" },
                ],
              },
            ],
          },
        },
      ],
    },
  ],
  integrity_signals: [],
  disclaimer: "",
};

async function renderReport(fixture: ScoreReport = report) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  const root = createRootRoute();
  const route = createRoute({
    getParentRoute: () => root,
    path: "/",
    component: () => (
      <QueryClientProvider client={queryClient}>
        <I18nextProvider i18n={i18n}>
          <ExamReport report={fixture} />
        </I18nextProvider>
      </QueryClientProvider>
    ),
  });
  const router = createRouter({
    routeTree: root.addChildren([route]),
    history: createMemoryHistory({ initialEntries: ["/"] }),
  });
  await router.load();
  return render(<RouterProvider router={router} />);
}

describe("exam report question review", () => {
  beforeEach(async () => {
    await initI18n("en");
  });

  it("filters compact rows and expands only the selected question", async () => {
    await renderReport();
    expect(
      screen.queryByText("When is the library closed?"),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByText("The passage says it is closed on Friday."),
    ).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: /Wrong/ }));
    expect(screen.getByText("Question 1")).toBeInTheDocument();
    expect(screen.queryByText("Question 2")).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: /Correct/ }));
    expect(screen.getByText("Question 2")).toBeInTheDocument();
    expect(screen.queryByText("Question 1")).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: /All/ }));
    fireEvent.click(screen.getAllByRole("button", { name: "View" })[0]!);
    expect(screen.getByText("When is the library closed?")).toBeInTheDocument();
    expect(
      screen.getByText("The passage says it is closed on Friday."),
    ).toBeInTheDocument();
    expect(screen.queryByText("What is closed?")).not.toBeInTheDocument();
  });

  it("answer-sheet navigation resets the filter and expands its item", async () => {
    await renderReport();
    fireEvent.click(screen.getByRole("button", { name: /Wrong/ }));
    fireEvent.click(screen.getAllByRole("button", { name: "2" })[0]!);
    expect(screen.getByRole("button", { name: /All/ })).toHaveAttribute(
      "aria-pressed",
      "true",
    );
    expect(screen.getByText("What is closed?")).toBeInTheDocument();
  });

  it("labels other whole-item outcomes as skipped", async () => {
    const withWriting: ScoreReport = {
      ...report,
      per_section: report.per_section.map((section) => ({
        ...section,
        items: [
          ...section.items,
          {
            activity_id: "writing-activity",
            content_version_id: "writing-version",
            kind: "writing_prompt",
            status: "graded",
            score: 1,
            max_score: 2,
            response: { text_answer: "My response" },
          },
        ],
      })),
    };
    await renderReport(withWriting);
    fireEvent.click(screen.getByRole("button", { name: /Skipped/ }));
    const row = screen.getByText("Question 3").parentElement;
    expect(row).toHaveTextContent("Skipped");
  });
});
