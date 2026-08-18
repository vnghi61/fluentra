import { afterEach, beforeEach, describe, expect, it } from "vitest";
import { http, HttpResponse } from "msw";
import { createMemoryHistory } from "@tanstack/react-router";
import { initApp } from "@/app/bootstrap";
import { router } from "@/app/router";
import { useAuthStore } from "@/stores/authStore";
import { server } from "./msw-server";

describe("Boot-time Silent Refresh & Route Gates (Task 2)", () => {
  beforeEach(() => {
    useAuthStore.getState().clearAuth();
  });

  afterEach(() => {
    useAuthStore.getState().clearAuth();
  });

  it("renders dashboard directly on valid refresh cookie with zero login screen flash", async () => {
    const renderedPaths: string[] = [];

    server.use(
      http.post("/api/v1/auth/refresh", () => {
        return HttpResponse.json({
          access_token: "valid-boot-token",
          token_type: "Bearer",
          expires_in: 900,
          user_id: "user-returning-1",
          role: "user",
        });
      }),
    );

    // Boot the app before first render
    await initApp();

    expect(useAuthStore.getState().status).toBe("authenticated");
    expect(useAuthStore.getState().accessToken).toBe("valid-boot-token");

    // Initialize router at "/"
    const history = createMemoryHistory({ initialEntries: ["/"] });
    router.update({ history });

    // Track every route transition
    router.subscribe("onResolved", (match) => {
      renderedPaths.push(match.toLocation.pathname);
    });

    await router.load();

    expect(router.state.location.pathname).toBe("/");
    expect(renderedPaths).not.toContain("/login");
  });

  it("redirects unauthenticated caller to /login when refresh fails", async () => {
    server.use(
      http.post("/api/v1/auth/refresh", () => {
        return HttpResponse.json(
          { title: "Unauthorized", status: 401, code: "TOKEN_INVALID" },
          { status: 401 },
        );
      }),
    );

    await initApp();

    expect(useAuthStore.getState().status).toBe("unauthenticated");

    const history = createMemoryHistory({ initialEntries: ["/"] });
    router.update({ history });

    await router.load();

    expect(router.state.location.pathname).toBe("/login");
  });

  it("redirects signed-in user away from /login back to dashboard", async () => {
    useAuthStore.getState().setAuthSession({
      access_token: "signed-in-token",
      token_type: "Bearer",
      expires_in: 900,
      user_id: "user-123",
      role: "user",
    });

    const history = createMemoryHistory({ initialEntries: ["/login"] });
    router.update({ history });

    await router.load();

    expect(router.state.location.pathname).toBe("/");
  });
});
