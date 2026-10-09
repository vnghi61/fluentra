import { createMemoryHistory, createRouter } from "@tanstack/react-router";
import { afterEach, describe, expect, it } from "vitest";
import { routeTree } from "@/app/router";
import { useAuthStore } from "@/stores/authStore";

const originalAuth = useAuthStore.getState();
afterEach(() => useAuthStore.setState(originalAuth));

describe("leaderboard route", () => {
  it("resolves the real leaderboard route for an authenticated learner", async () => {
    useAuthStore.setState({ status: "authenticated" });
    const router = createRouter({
      routeTree,
      history: createMemoryHistory({ initialEntries: ["/leaderboard"] }),
    });
    await router.load();
    expect(router.state.location.pathname).toBe("/leaderboard");
    expect(
      router.state.matches.some(
        (match) =>
          match.routeId === "/leaderboard" && match.status === "success",
      ),
    ).toBe(true);
  });

  it("redirects a guest to login rather than progress", async () => {
    useAuthStore.setState({ status: "unauthenticated", user: null });
    const router = createRouter({
      routeTree,
      history: createMemoryHistory({ initialEntries: ["/leaderboard"] }),
    });
    await router.load();
    expect(router.state.location.pathname).toBe("/login");
  });
});
