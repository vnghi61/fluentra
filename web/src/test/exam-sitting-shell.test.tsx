import {
  fireEvent,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
import { render } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import {
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
  RouterProvider,
} from "@tanstack/react-router";
import { I18nextProvider } from "react-i18next";
import { beforeEach, describe, expect, it } from "vitest";
import i18n, { initI18n } from "@/i18n";
import { ExamSittingRunner } from "@/features/exam/components/ExamSittingRunner";
import type { ExamAttempt } from "@/features/exam/types";

const attempt: ExamAttempt = {
  id: "sitting-test",
  exam_id: "exam-test",
  exam_title: "Shell test",
  mode: "exam",
  chosen_duration_minutes: 75,
  unlimited: false,
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
          id: "activity-one",
          kind: "listening_comprehension",
          content_version_id: "content-one",
          weight: 1,
          config: {
            title: "Audio",
            questions: [
              {
                id: "q1",
                prompt: "First?",
                options: [{ id: "A", text: "Tokyo" }],
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
          id: "activity-two",
          kind: "reading_comprehension",
          content_version_id: "content-two",
          weight: 1,
          config: {
            passage: "Passage",
            questions: [
              {
                id: "q2",
                prompt: "Second?",
                options: [{ id: "B", text: "Seoul" }],
              },
            ],
          },
        },
      ],
    },
  ],
};

async function renderWithProviders(ui: React.ReactElement) {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  const root = createRootRoute();
  const route = createRoute({
    getParentRoute: () => root,
    path: "/",
    component: () => (
      <QueryClientProvider client={client}>
        <I18nextProvider i18n={i18n}>{ui}</I18nextProvider>
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

describe("exam sitting shell", () => {
  beforeEach(async () => initI18n("en"));

  it("opens and closes the mobile question list when navigating", async () => {
    await renderWithProviders(
      <ExamSittingRunner attempt={attempt} onSubmitted={() => {}} />,
    );
    fireEvent.click(screen.getByRole("button", { name: "Questions" }));
    const dialog = screen.getByRole("dialog");
    expect(
      within(dialog).getByRole("button", { name: "Go to question 1" }),
    ).toBeInTheDocument();
    fireEvent.click(
      within(dialog).getByRole("button", { name: "Go to question 1" }),
    );
    await waitFor(() => {
      expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    });
  });

  it("filters answered and unanswered questions", async () => {
    await renderWithProviders(
      <ExamSittingRunner
        attempt={{ ...attempt, mode: "practice" }}
        onSubmitted={() => {}}
      />,
    );
    fireEvent.click(screen.getByRole("button", { name: "ATokyo" }));
    const nav = screen.getByRole("complementary");
    fireEvent.click(within(nav).getByRole("button", { name: "Unanswered" }));
    expect(
      within(nav).queryByRole("button", { name: "Go to question 1" }),
    ).not.toBeInTheDocument();
    expect(
      within(nav).getByRole("button", { name: "Go to question 2" }),
    ).toBeInTheDocument();
    fireEvent.click(
      within(nav).getByRole("button", { name: "Go to question 2" }),
    );
    expect(
      within(nav).getByRole("button", { name: "Go to question 2" }),
    ).toHaveAttribute("aria-current", "true");
  });

  it("shows the remaining-question count in submit confirmation", async () => {
    await renderWithProviders(
      <ExamSittingRunner attempt={attempt} onSubmitted={() => {}} />,
    );
    fireEvent.click(screen.getByRole("button", { name: "Submit" }));
    expect(
      screen.getByText("You have 2 unanswered questions"),
    ).toBeInTheDocument();
  });

  it("uses warning and danger timer styles at the thresholds", async () => {
    const nearWarning = { ...attempt, remaining_seconds: 299 };
    const { unmount } = await renderWithProviders(
      <ExamSittingRunner attempt={nearWarning} onSubmitted={() => {}} />,
    );
    expect(screen.getByRole("timer")).toHaveClass("text-warning-accent");
    unmount();
    await renderWithProviders(
      <ExamSittingRunner
        attempt={{ ...attempt, remaining_seconds: 59 }}
        onSubmitted={() => {}}
      />,
    );
    expect(screen.getByRole("timer")).toHaveClass("text-danger-accent");
  });
});
import React from "react";
