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

  /**
   * A visitor with no session lands on the catalogue, not on a login form.
   *
   * This asserted `/login` until ADR-0025. The dashboard is still not somewhere
   * a guest can be — it is "continue where you left off", "reviews due" and
   * "your skill mastery", every one of which is a fact about a person — but the
   * answer to that is the course catalogue, which is what they came to see, and
   * not a registration form in front of the whole product.
   */
  it("sends a visitor with no session to the catalogue, not to /login", async () => {
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

    expect(router.state.location.pathname).toBe("/learn");
  });

  /**
   * The other half, and the one worth stating explicitly: opening the
   * curriculum did not open anything else. A guest asking for a screen built
   * out of their own data is still sent to sign in.
   */
  it("still sends a visitor with no session away from /progress", async () => {
    server.use(
      http.post("/api/v1/auth/refresh", () =>
        HttpResponse.json(
          { title: "Unauthorized", status: 401, code: "TOKEN_INVALID" },
          { status: 401 },
        ),
      ),
    );

    await initApp();
    expect(useAuthStore.getState().status).toBe("unauthenticated");

    for (const guarded of ["/progress", "/settings"]) {
      const history = createMemoryHistory({ initialEntries: [guarded] });
      router.update({ history });
      await router.load();
      expect(router.state.location.pathname).toBe("/login");
    }
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

describe("A slow boot refresh that fails after someone signs in", () => {
  beforeEach(() => {
    useAuthStore.setState({
      accessToken: null,
      user: null,
      status: "idle",
    });
  });

  afterEach(() => {
    useAuthStore.getState().clearAuth();
  });

  it("does not clear the session created while it was in flight", async () => {
    // The cold-host case. initApp gives the refresh 2.5s of the first paint and
    // then stops waiting, but the request keeps running -- long enough for the
    // learner to reach the login form and sign in. When it finally fails,
    // clearing unconditionally threw away the session they had just created,
    // and the sign-in appeared to do nothing until the page was reloaded.
    let failRefresh: (() => void) | undefined;
    const refreshCalled = new Promise<void>((resolveCalled) => {
      server.use(
        http.post("/api/v1/auth/refresh", async () => {
          resolveCalled();
          await new Promise<void>((resolve) => {
            failRefresh = resolve;
          });
          return new HttpResponse(null, { status: 401 });
        }),
      );
    });

    const booting = initApp();
    await refreshCalled;

    // The learner signs in while the boot refresh is still hanging.
    useAuthStore.getState().setAuthSession({
      access_token: "token-from-signing-in",
      token_type: "Bearer",
      expires_in: 900,
      user_id: "user-who-just-signed-in",
      role: "user",
    });
    expect(useAuthStore.getState().status).toBe("authenticated");

    failRefresh?.();
    await booting;
    // Give the rejected promise's catch a turn to run.
    await new Promise((resolve) => setTimeout(resolve, 0));

    expect(useAuthStore.getState().status).toBe("authenticated");
    expect(useAuthStore.getState().accessToken).toBe("token-from-signing-in");
  });

  it("still clears when nobody signed in", async () => {
    // The other half: a first-time visitor with no session must end up
    // unauthenticated, or the guards never show them a login screen.
    server.use(
      http.post(
        "/api/v1/auth/refresh",
        () => new HttpResponse(null, { status: 401 }),
      ),
    );

    await initApp();
    await new Promise((resolve) => setTimeout(resolve, 0));

    expect(useAuthStore.getState().status).toBe("unauthenticated");
  });
});
