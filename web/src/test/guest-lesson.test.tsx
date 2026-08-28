import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, waitFor } from "@testing-library/react";
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
import { useAuthStore } from "@/stores/authStore";
import { server } from "./msw-server";

const userEvent = userEventDefault;
const lessonId = "0199a1c2-3d4e-7f80-9abc-def01234567a";

/**
 * One activity, and no answer anywhere in it — which is what the server now
 * sends. A fixture carrying `correct_option_id` would let the runner go back to
 * reading the answer out of the lesson without any test noticing.
 */
const guestLesson: LessonDetail = {
  id: lessonId,
  unit_id: "0199a1c2-3d4e-7f80-9abc-def012345679",
  position: 1,
  title: "Morning Routines & Habits",
  skill_focus: "vocabulary",
  estimated_minutes: 15,
  status: "published",
  activities: [
    {
      id: "act-1",
      lesson_id: lessonId,
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
      } as unknown as Record<string, never>,
    },
  ],
};

async function renderGuestLesson() {
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
  // The prompt links to these; they must resolve or the render throws.
  const loginRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: "/login",
    component: () => <div>login</div>,
  });
  const registerRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: "/register",
    component: () => <div>register</div>,
  });
  const router = createRouter({
    routeTree: rootRoute.addChildren([lessonRoute, loginRoute, registerRoute]),
    history: createMemoryHistory({
      initialEntries: [`/learn/lesson/${lessonId}`],
    }),
  });
  await router.load();
  return render(<RouterProvider router={router} />);
}

describe("A guest working through a lesson", () => {
  let attemptCalls = 0;
  let previewCalls = 0;

  beforeEach(async () => {
    // The state under test: no session at all.
    useAuthStore.getState().clearAuth();
    initI18n("en");
    await i18n.changeLanguage("en");

    attemptCalls = 0;
    previewCalls = 0;

    server.use(
      http.get(`/api/v1/lessons/${lessonId}`, () =>
        HttpResponse.json(guestLesson),
      ),
      http.post("/api/v1/activities/:id/attempts", () => {
        attemptCalls += 1;
        return HttpResponse.json(
          { title: "Unauthorized", status: 401, code: "UNAUTHORIZED" },
          { status: 401 },
        );
      }),
      http.post("/api/v1/activities/:id/grade", () => {
        previewCalls += 1;
        return HttpResponse.json({
          correct: true,
          score: 100,
          max_score: 100,
          feedback: "Correct! Well done.",
          correct_answer: "opt-1",
          saved: false,
        });
      }),
    );
  });

  /**
   * The whole point of the guest path: the answer is graded, and no attempt is
   * ever opened. Asserting that `attemptCalls` stayed at zero is the assertion
   * that matters — a runner that quietly started an attempt would still show
   * the right feedback here while writing rows for a person who does not exist.
   */
  it("grades through the preview route and starts no attempt", async () => {
    const user = userEvent.setup();
    await renderGuestLesson();

    await screen.findByText("What is the meaning of 'meticulous'?");

    await user.click(
      screen.getByRole("radio", {
        name: /Showing great attention to detail/i,
      }),
    );
    await user.click(screen.getByRole("button", { name: /Check Answer/i }));

    await waitFor(() => expect(previewCalls).toBe(1));
    expect(attemptCalls).toBe(0);
    // findAllByText: the runner shows the verdict in two places — beside the
    // chosen option and in the feedback panel — and a single-match query throws
    // on that rather than passing.
    expect(
      (await screen.findAllByText("Correct! Well done.")).length,
    ).toBeGreaterThan(0);
  });

  /**
   * And at the end, they are told. The wording is the contract: nothing was
   * recorded, and signing in is what changes that.
   */
  it("says the result was not saved once the lesson is finished", async () => {
    const user = userEvent.setup();
    await renderGuestLesson();

    await screen.findByText("What is the meaning of 'meticulous'?");
    await user.click(
      screen.getByRole("radio", {
        name: /Showing great attention to detail/i,
      }),
    );
    await user.click(screen.getByRole("button", { name: /Check Answer/i }));
    await user.click(await screen.findByRole("button", { name: /Continue/i }));

    const dialog = await screen.findByRole("dialog");
    expect(dialog).toHaveTextContent(/was not saved/i);
    expect(
      screen.getByRole("link", { name: /Create an account/i }),
    ).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /Sign in/i })).toBeInTheDocument();

    // Dismissible. A guest who wants to keep looking around is not trapped, and
    // the result they earned is still behind it.
    await user.click(
      screen.getByRole("button", { name: /Keep looking around/i }),
    );
    await waitFor(() =>
      expect(screen.queryByRole("dialog")).not.toBeInTheDocument(),
    );
  });
});
