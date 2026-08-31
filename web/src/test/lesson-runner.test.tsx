import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEventDefault from "@testing-library/user-event";
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
import { LessonPage } from "@/routes/LessonPage";
import type { LessonDetail } from "@/features/lesson";
import type {
  StartAttemptResult,
  SubmitAttemptResult,
} from "@/features/learning";
import { useAuthStore } from "@/stores/authStore";
import { server } from "./msw-server";

async function renderLessonRunner(
  lessonId = "0199a1c2-3d4e-7f80-9abc-def01234567a",
) {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  const rootRoute = createRootRoute();
  const lessonRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: "/learn/lesson/$lessonId",
    component: () => (
      <I18nextProvider i18n={i18n}>
        <QueryClientProvider client={client}>
          <LessonPage />
        </QueryClientProvider>
      </I18nextProvider>
    ),
  });
  const router = createRouter({
    routeTree: rootRoute.addChildren([lessonRoute]),
    history: createMemoryHistory({
      initialEntries: [`/learn/lesson/${lessonId}`],
    }),
  });
  await router.load();
  return { ...render(<RouterProvider router={router} />), client };
}

/**
 * These exercise the signed-in runner: an attempt is started, the answer goes
 * through the attempt flow, and the result is stored. A guest takes a different
 * path through the same screen — see the guest suite — so the session has to be
 * real here rather than incidental.
 */
function signIn(): void {
  useAuthStore.getState().setAuthSession({
    access_token: "valid-test-token",
    token_type: "Bearer",
    expires_in: 900,
    user_id: "user-123",
    role: "user",
  });
}

describe("LessonPage Runner (P10.3)", () => {
  const mockLesson: LessonDetail = {
    id: "0199a1c2-3d4e-7f80-9abc-def01234567a",
    unit_id: "0199a1c2-3d4e-7f80-9abc-def012345679",
    position: 1,
    title: "Academic Word List - Topic 1",
    skill_focus: "vocabulary",
    estimated_minutes: 15,
    status: "published",
    activities: [
      {
        id: "act-1",
        lesson_id: "0199a1c2-3d4e-7f80-9abc-def01234567a",
        position: 1,
        kind: "vocab_multiple_choice",
        content_version_id: "0199a1c2-3d4e-7f80-9abc-def01234567b",
        weight: 1,
        config: {
          prompt: "What is the meaning of 'meticulous'?",
          options: [
            { id: "opt-1", text: "Showing great attention to detail" },
            { id: "opt-2", text: "Careless and fast" },
            { id: "opt-3", text: "Extremely angry" },
            { id: "opt-4", text: "Quiet and hesitant" },
          ],
          // No correct_option_id. The server redacts it out of the lesson, so a
          // fixture that still carried it would let the runner go back to
          // reading the answer from the body and nothing here would notice.
        } as unknown as Record<string, never>,
      },
      {
        id: "act-2",
        lesson_id: "0199a1c2-3d4e-7f80-9abc-def01234567a",
        position: 2,
        kind: "vocab_gap_fill",
        content_version_id: "0199a1c2-3d4e-7f80-9abc-def01234567c",
        weight: 1,
        config: {
          prompt: "Complete the sentence with the target word",
          sentence_before: "She was very",
          sentence_after: "about recording all data.",
          expected_answer: "meticulous",
        } as unknown as Record<string, never>,
      },
      {
        id: "act-3",
        lesson_id: "0199a1c2-3d4e-7f80-9abc-def01234567a",
        position: 3,
        kind: "vocab_flashcard",
        content_version_id: "0199a1c2-3d4e-7f80-9abc-def01234567d",
        weight: 1,
        config: {
          prompt: "Vocabulary Card",
          target_word: "meticulous",
          ipa: "/məˈtɪkjələs/",
          definition:
            "Showing great attention to detail; very careful and precise.",
          example_sentence:
            "He kept meticulous accounts of the laboratory tests.",
        } as unknown as Record<string, never>,
      },
    ],
  };

  let capturedSubmitHeaders: Headers[] = [];

  beforeEach(async () => {
    signIn();
    initI18n("en");
    await i18n.changeLanguage("en");
    capturedSubmitHeaders = [];

    server.use(
      http.get("/api/v1/lessons/0199a1c2-3d4e-7f80-9abc-def01234567a", () =>
        HttpResponse.json(mockLesson),
      ),
      http.post("/api/v1/activities/:id/attempts", ({ params }) => {
        const idStr = String(params.id ?? "1");
        const res: StartAttemptResult = {
          attempt_id: `att-${idStr}`,
          activity_id: idStr,
          status: "in_progress",
          started_at: "2026-08-24T09:00:00Z",
        };
        return HttpResponse.json(res);
      }),
      http.post("/api/v1/attempts/:id/submit", ({ request, params }) => {
        capturedSubmitHeaders.push(new Headers(request.headers));
        const idStr = String(params.id ?? "1");
        const res: SubmitAttemptResult = {
          attempt_id: idStr,
          status: "graded",
          correct: true,
          score: 100,
          max_score: 100,
          feedback: "Correct! Well done.",
        };
        return HttpResponse.json(res);
      }),
    );
  });

  it("completes full 3-step lesson using keyboard and mouse", async () => {
    const user = userEventDefault.setup();
    await renderLessonRunner();

    // 1. Multiple choice step
    expect(
      await screen.findByText("What is the meaning of 'meticulous'?"),
    ).toBeInTheDocument();
    expect(screen.getByText("Activity 1 of 3")).toBeInTheDocument();

    // Select option 1
    const opt1 = screen.getByText("Showing great attention to detail");
    await user.click(opt1);

    // Submit
    const checkBtn = screen.getByRole("button", { name: /Check Answer/i });
    await user.click(checkBtn);

    // Feedback shown
    expect(
      (await screen.findAllByText("Correct! Well done.")).length,
    ).toBeGreaterThanOrEqual(1);

    // Continue to Step 2
    const contBtn1 = screen.getByRole("button", { name: /Continue/i });
    await user.click(contBtn1);

    // 2. Gap fill step
    expect(
      await screen.findByText("Complete the sentence with the target word"),
    ).toBeInTheDocument();
    expect(screen.getByText("Activity 2 of 3")).toBeInTheDocument();

    const input = screen.getByPlaceholderText("Type your answer...");
    await user.type(input, "meticulous");

    const checkBtn2 = screen.getByRole("button", { name: /Check Answer/i });
    await user.click(checkBtn2);

    expect(
      (await screen.findAllByText("Correct! Well done.")).length,
    ).toBeGreaterThanOrEqual(1);

    const contBtn2 = screen.getByRole("button", { name: /Continue/i });
    await user.click(contBtn2);

    // 3. Flashcard step
    expect(await screen.findByText("Vocabulary Card")).toBeInTheDocument();
    expect(screen.getByText("Activity 3 of 3")).toBeInTheDocument();

    const flipBtn = screen.getByRole("button", { name: /Flip Card/i });
    await user.click(flipBtn);

    expect(
      await screen.findByText(
        "Showing great attention to detail; very careful and precise.",
      ),
    ).toBeInTheDocument();

    // The flashcard is graded on the learner's own recall verdict now, so it
    // submits an attempt like the other two instead of advancing silently and
    // leaving the one the runner opened stuck in `in_progress`.
    const knewItBtn = screen.getByRole("button", { name: /I knew it/i });
    await user.click(knewItBtn);

    const contBtn3 = await screen.findByRole("button", { name: /Continue/i });
    await user.click(contBtn3);

    // 4. Completion screen
    expect(await screen.findByText("Lesson Completed!")).toBeInTheDocument();
    expect(screen.getByText("100%")).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: /Back to Syllabus/i }),
    ).toBeInTheDocument();
  });

  it("sends and verifies Idempotency-Key header on attempt submissions", async () => {
    const user = userEventDefault.setup();
    await renderLessonRunner();

    expect(
      await screen.findByText("What is the meaning of 'meticulous'?"),
    ).toBeInTheDocument();
    await user.click(screen.getByText("Showing great attention to detail"));
    await user.click(screen.getByRole("button", { name: /Check Answer/i }));

    await waitFor(() => {
      expect(capturedSubmitHeaders.length).toBe(1);
    });

    const key = capturedSubmitHeaders[0]?.get("Idempotency-Key");
    expect(key).toBeTruthy();
    expect(key?.length).toBeGreaterThan(10);
  });

  // P10 §5: the backend grades one attempt per key, so the guarantee is only real
  // if the client sends one key per submission and reuses it on every retry of
  // that same answer. A key regenerated inside the retry handler turns a retry
  // into a second submission, which the backend correctly refuses — and the
  // learner sees an error for a thing that worked.
  it("reuses the same Idempotency-Key when a failed submission is retried", async () => {
    const user = userEventDefault.setup();

    let attempts = 0;
    server.use(
      http.post("/api/v1/attempts/:id/submit", ({ request, params }) => {
        capturedSubmitHeaders.push(new Headers(request.headers));
        attempts += 1;
        if (attempts === 1) {
          return HttpResponse.json({ title: "network" }, { status: 500 });
        }
        const res: SubmitAttemptResult = {
          attempt_id: String(params.id ?? "1"),
          status: "graded",
          correct: true,
          score: 100,
          max_score: 100,
          feedback: "Correct! Well done.",
        };
        return HttpResponse.json(res);
      }),
    );

    await renderLessonRunner();
    expect(
      await screen.findByText("What is the meaning of 'meticulous'?"),
    ).toBeInTheDocument();
    await user.click(screen.getByText("Showing great attention to detail"));
    await user.click(screen.getByRole("button", { name: /Check Answer/i }));

    // The answer survives the failure and a retry is offered.
    const retry = await screen.findByRole("button", { name: /Retry/i });
    await user.click(retry);

    await waitFor(() => {
      expect(capturedSubmitHeaders.length).toBe(2);
    });

    const first = capturedSubmitHeaders[0]?.get("Idempotency-Key");
    const second = capturedSubmitHeaders[1]?.get("Idempotency-Key");
    expect(first).toBeTruthy();
    expect(second).toBe(first);
  });

  // The two-tab acceptance from P10 §5, as far as jsdom can carry it: one lesson
  // open twice, both submitting the same answer. Each tab holds its own key, so
  // the server sees two distinct submissions — which is exactly why the backend
  // claim is conditional and why both tabs must render the same graded result
  // rather than one of them showing a conflict.
  it("renders the same graded result in two tabs on one lesson", async () => {
    const user = userEventDefault.setup();

    const graded: SubmitAttemptResult = {
      attempt_id: "att-act-1",
      status: "graded",
      correct: true,
      score: 100,
      max_score: 100,
      feedback: "Correct! Well done.",
    };

    let graderRuns = 0;
    server.use(
      http.post("/api/v1/attempts/:id/submit", ({ request }) => {
        capturedSubmitHeaders.push(new Headers(request.headers));
        // The conditional claim in SQL: the first submission grades, every later
        // one returns the stored result rather than grading again.
        graderRuns += 1;
        return HttpResponse.json(graded);
      }),
    );

    // Both runners live in one jsdom document, so each tab's assertions are
    // scoped to its own container; an unscoped query would find the other tab.
    const tabOne = within((await renderLessonRunner()).container);
    expect(
      await tabOne.findByText("What is the meaning of 'meticulous'?"),
    ).toBeInTheDocument();
    await user.click(tabOne.getByText("Showing great attention to detail"));
    await user.click(tabOne.getByRole("button", { name: /Check Answer/i }));
    expect(
      (await tabOne.findAllByText("Correct! Well done.")).length,
    ).toBeGreaterThan(0);

    const tabTwo = within((await renderLessonRunner()).container);
    expect(
      await tabTwo.findByText("What is the meaning of 'meticulous'?"),
    ).toBeInTheDocument();
    await user.click(tabTwo.getByText("Showing great attention to detail"));
    await user.click(tabTwo.getByRole("button", { name: /Check Answer/i }));
    expect(
      (await tabTwo.findAllByText("Correct! Well done.")).length,
    ).toBeGreaterThan(0);

    // Both tabs show the same verdict, and neither surfaced a conflict.
    expect(graderRuns).toBe(2);
    expect(capturedSubmitHeaders.length).toBe(2);
    expect(tabOne.queryByRole("alert")).not.toBeInTheDocument();
    expect(tabTwo.queryByRole("alert")).not.toBeInTheDocument();
  });

  // The runner supports three exercise kinds. A fourth, or a config missing what
  // its exercise needs, is skippable and says so — it used to be filled in with a
  // hard-coded question, which the learner then answered instead of the real one.
  it("says an activity cannot be shown rather than substituting content", async () => {
    server.use(
      http.get("/api/v1/lessons/0199a1c2-3d4e-7f80-9abc-def01234567a", () =>
        HttpResponse.json({
          ...mockLesson,
          activities: [
            {
              ...mockLesson.activities[0],
              kind: "vocab_gap_fill",
              config: { prompt: "Complete it" } as unknown as Record<
                string,
                never
              >,
            },
          ],
        }),
      ),
    );

    await renderLessonRunner();

    expect(
      await screen.findByText("This activity cannot be shown"),
    ).toBeInTheDocument();
    expect(screen.queryByText("She was very")).not.toBeInTheDocument();
  });

  it("handles exit dialog confirmation flow", async () => {
    const user = userEventDefault.setup();
    await renderLessonRunner();

    expect(
      await screen.findByText("What is the meaning of 'meticulous'?"),
    ).toBeInTheDocument();

    // Click exit header button
    await user.click(screen.getByLabelText("Exit"));

    // Modal dialog pops up
    expect(await screen.findByRole("dialog")).toBeInTheDocument();
    expect(screen.getByText("Exit Lesson?")).toBeInTheDocument();

    // Clicking 'Keep Learning' cancels
    await user.click(screen.getByRole("button", { name: /Keep Learning/i }));
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  });
});

/**
 * The bug this covers looked exactly like data loss.
 *
 * Grading writes progress on the server — the activity, the lesson, the course
 * rollup — and schedules review cards. Nothing invalidated the caches those
 * screens read, so a learner who answered everything and pressed back saw the
 * same untouched course they had left. The work was saved; only the reading of
 * it was stale, which is the worst version of the bug, because it is
 * indistinguishable from the answer never having been recorded.
 */
describe("LessonPage progress freshness", () => {
  beforeEach(async () => {
    signIn();
    initI18n("en");
    await i18n.changeLanguage("en");

    server.use(
      http.get("/api/v1/lessons/0199a1c2-3d4e-7f80-9abc-def01234567a", () =>
        HttpResponse.json({
          id: "0199a1c2-3d4e-7f80-9abc-def01234567a",
          unit_id: "0199a1c2-3d4e-7f80-9abc-def012345679",
          position: 1,
          title: "Academic Word List - Topic 1",
          skill_focus: "vocabulary",
          estimated_minutes: 15,
          status: "published",
          next_lesson_id: "0199a1c2-3d4e-7f80-9abc-def0123456ff",
          activities: [
            {
              id: "act-1",
              lesson_id: "0199a1c2-3d4e-7f80-9abc-def01234567a",
              position: 1,
              kind: "vocab_multiple_choice",
              content_version_id: "0199a1c2-3d4e-7f80-9abc-def01234567b",
              weight: 1,
              config: {
                prompt: "What is the meaning of 'meticulous'?",
                options: [
                  { id: "opt-1", text: "Showing great attention to detail" },
                  { id: "opt-2", text: "Careless and fast" },
                ],
              },
            },
          ],
        }),
      ),
      http.post("/api/v1/activities/:id/attempts", ({ params }) =>
        HttpResponse.json({
          attempt_id: `att-${String(params.id ?? "1")}`,
          activity_id: String(params.id ?? "1"),
          status: "in_progress",
          started_at: "2026-08-24T09:00:00Z",
        }),
      ),
      http.post("/api/v1/attempts/:id/submit", ({ params }) =>
        HttpResponse.json({
          attempt_id: String(params.id ?? "1"),
          status: "graded",
          correct: true,
          score: 100,
          max_score: 100,
          feedback: "Correct! Well done.",
        }),
      ),
    );
  });

  it("marks the course and dashboard caches stale as soon as an answer is graded", async () => {
    const user = userEventDefault.setup();
    const { client } = await renderLessonRunner();

    // Stand in for a course page the learner has already visited: a cached,
    // fresh entry that the old code left untouched.
    const courseKey = ["lesson", "courses"];
    const dashboardKey = ["learning", "dashboard"];
    client.setQueryData(courseKey, { items: [] });
    client.setQueryData(dashboardKey, { items: [] });
    expect(client.getQueryState(courseKey)?.isInvalidated).toBe(false);

    await user.click(
      await screen.findByRole("radio", {
        name: /showing great attention to detail/i,
      }),
    );
    await user.click(screen.getByRole("button", { name: /check answer/i }));

    // Not on finishing the lesson — on every graded answer. A learner who
    // leaves half-way has still made progress the course screen must show.
    await waitFor(() => {
      expect(client.getQueryState(courseKey)?.isInvalidated).toBe(true);
      expect(client.getQueryState(dashboardKey)?.isInvalidated).toBe(true);
    });
  });

  it("offers the next lesson once the last activity is done", async () => {
    const user = userEventDefault.setup();
    await renderLessonRunner();

    await user.click(
      await screen.findByRole("radio", {
        name: /showing great attention to detail/i,
      }),
    );
    await user.click(screen.getByRole("button", { name: /check answer/i }));
    await user.click(await screen.findByRole("button", { name: /continue/i }));

    // Continuing is the primary action: sending a warmed-up learner back to a
    // syllabus to hunt for the next lesson is where study sessions end.
    const next = await screen.findByRole("link", { name: /next lesson/i });
    expect(next).toHaveAttribute(
      "href",
      "/learn/lesson/0199a1c2-3d4e-7f80-9abc-def0123456ff",
    );
  });
});
