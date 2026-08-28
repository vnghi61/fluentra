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
import { PracticePage } from "@/routes/PracticePage";
import { server } from "./msw-server";

async function renderPractice() {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  const rootRoute = createRootRoute();
  const practiceRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: "/practice",
    component: () => (
      <I18nextProvider i18n={i18n}>
        <QueryClientProvider client={client}>
          <PracticePage />
        </QueryClientProvider>
      </I18nextProvider>
    ),
  });
  // The session route must exist for the link to resolve; it renders nothing,
  // because what is under test is that the door is there, not what is behind it.
  const reviewRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: "/practice/review",
    component: () => <div>review session</div>,
  });
  const learnRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: "/learn",
    component: () => <div>learn</div>,
  });
  const router = createRouter({
    routeTree: rootRoute.addChildren([practiceRoute, reviewRoute, learnRoute]),
    history: createMemoryHistory({ initialEntries: ["/practice"] }),
  });
  await router.load();
  return render(<RouterProvider router={router} />);
}

const forecast = {
  days: [
    { date: "2026-08-28", due_count: 4 },
    { date: "2026-08-29", due_count: 0 },
    { date: "2026-08-30", due_count: 11 },
  ],
};

describe("PracticePage hub", () => {
  beforeEach(() => {
    initI18n("en");
  });

  /**
   * The regression this page exists for.
   *
   * /practice/review shipped in WP9 with a full FSRS session behind it and no
   * link to it anywhere in the app — the sidebar's Practice entry and both
   * dashboard buttons pointed at a placeholder. The E2E journey did not catch
   * it because it reaches the session with page.goto, so no test in the repo
   * ever traversed the path a learner has to take.
   */
  it("offers a link into the review session when cards are due", async () => {
    server.use(
      http.get("/api/v1/reviews/due-count", () =>
        HttpResponse.json({ due_count: 12 }),
      ),
      http.get("/api/v1/reviews/forecast", () => HttpResponse.json(forecast)),
    );

    await renderPractice();

    expect(await screen.findByText("12")).toBeInTheDocument();
    const start = screen.getByRole("link", { name: /Start review/i });
    expect(start).toHaveAttribute("href", "/practice/review");
  });

  it("says the queue is clear, and points at a lesson, when nothing is due", async () => {
    server.use(
      http.get("/api/v1/reviews/due-count", () =>
        HttpResponse.json({ due_count: 0 }),
      ),
      http.get("/api/v1/reviews/forecast", () => HttpResponse.json(forecast)),
    );

    await renderPractice();

    expect(
      await screen.findByText("Nothing due right now"),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("link", { name: /Go to a lesson/i }),
    ).toHaveAttribute("href", "/learn");
    expect(
      screen.queryByRole("link", { name: /Start review/i }),
    ).not.toBeInTheDocument();
  });

  it("draws the forecast the server sent, not a placeholder", async () => {
    server.use(
      http.get("/api/v1/reviews/due-count", () =>
        HttpResponse.json({ due_count: 4 }),
      ),
      http.get("/api/v1/reviews/forecast", () => HttpResponse.json(forecast)),
    );

    await renderPractice();

    expect(await screen.findByText("The week ahead")).toBeInTheDocument();
    expect(screen.getByText("11")).toBeInTheDocument();
  });

  /**
   * A forecast that fails must not take the page down with it: the queue is the
   * reason a learner opened Practice, and it loaded.
   */
  it("still shows the queue when the forecast fails", async () => {
    server.use(
      http.get("/api/v1/reviews/due-count", () =>
        HttpResponse.json({ due_count: 7 }),
      ),
      http.get(
        "/api/v1/reviews/forecast",
        () => new HttpResponse(null, { status: 500 }),
      ),
    );

    await renderPractice();

    expect(
      await screen.findByRole("link", { name: /Start review/i }),
    ).toBeInTheDocument();
    expect(screen.queryByText("The week ahead")).not.toBeInTheDocument();
  });
});
