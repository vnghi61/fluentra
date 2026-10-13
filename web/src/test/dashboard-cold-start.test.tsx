import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, render, screen } from "@testing-library/react";
import { http, HttpResponse } from "msw";
import { I18nextProvider } from "react-i18next";
import { afterEach, beforeEach, describe, expect, it } from "vitest";
import {
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
  Outlet,
  RouterProvider,
} from "@tanstack/react-router";

import i18n, { initI18n, loadLocale } from "@/i18n";
import { DashboardPage } from "@/routes/DashboardPage";
import { useAuthStore } from "@/stores/authStore";
import { server } from "./msw-server";

/**
 * A visitor on a cold host. The boot refresh outlives its paint budget, so
 * `status` is still `idle` when "/" loads and the guard lets the dashboard
 * render; the refresh fails a minute later. The visitor used to be left on
 * the dashboard with an error card for a dashboard they never had.
 */
async function renderAt(path: string) {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  const rootRoute = createRootRoute({
    component: () => (
      <I18nextProvider i18n={i18n}>
        <QueryClientProvider client={client}>
          <Outlet />
        </QueryClientProvider>
      </I18nextProvider>
    ),
  });
  const router = createRouter({
    routeTree: rootRoute.addChildren([
      createRoute({
        getParentRoute: () => rootRoute,
        path: "/",
        component: DashboardPage,
      }),
      createRoute({
        getParentRoute: () => rootRoute,
        path: "/learn",
        component: () => <h1>Learn page</h1>,
      }),
    ]),
    history: createMemoryHistory({ initialEntries: [path] }),
  });
  await router.load();
  render(<RouterProvider router={router} />);
  return router;
}

describe("the dashboard while the session is unknown", () => {
  beforeEach(async () => {
    await initI18n("en");
    await loadLocale("en");
    await i18n.changeLanguage("en");
    useAuthStore.setState({ status: "idle", user: null });
    // Without a session every dashboard request is refused.
    server.use(
      http.get("/api/v1/*", () =>
        HttpResponse.json(
          { title: "Authentication required", status: 401 },
          { status: 401 },
        ),
      ),
    );
  });

  afterEach(() => {
    useAuthStore.setState({ status: "idle", user: null });
  });

  it("waits instead of showing an error, then sends a visitor to /learn", async () => {
    const router = await renderAt("/");

    // Idle: the requests fail, but that is a wait, not an error.
    await act(async () => {
      await new Promise((resolve) => setTimeout(resolve, 50));
    });
    expect(screen.queryByRole("alert")).toBeNull();
    expect(router.state.location.pathname).toBe("/");

    // The boot refresh fails: this is a visitor.
    act(() => {
      useAuthStore.setState({ status: "unauthenticated", user: null });
    });

    expect(
      await screen.findByRole("heading", { name: "Learn page" }),
    ).toBeTruthy();
    expect(router.state.location.pathname).toBe("/learn");
    expect(screen.queryByRole("alert")).toBeNull();
  });

  it("leaves a learner who signs out to whoever sends them to /login", async () => {
    useAuthStore.setState({
      status: "authenticated",
      user: { userId: "u1", role: "user" },
    });
    const router = await renderAt("/");

    act(() => {
      useAuthStore.setState({ status: "unauthenticated", user: null });
    });
    await act(async () => {
      await new Promise((resolve) => setTimeout(resolve, 50));
    });

    // Sending them to /learn here raced the sign-out's own navigation to
    // /login, and won: E2E journeys 7-9 landed on the catalogue.
    expect(router.state.location.pathname).toBe("/");
  });
});
