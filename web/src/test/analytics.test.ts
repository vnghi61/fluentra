import { afterEach, describe, expect, it } from "vitest";
import {
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
  type AnyRouter,
} from "@tanstack/react-router";

import { analyticsMeasurementId, startAnalytics } from "@/app/analytics";

const GA_ID = "G-TEST12345";

function makeRouter(initial: string): AnyRouter {
  const rootRoute = createRootRoute();
  const routeTree = rootRoute.addChildren([
    createRoute({ getParentRoute: () => rootRoute, path: "/" }),
    createRoute({ getParentRoute: () => rootRoute, path: "/forgot-password" }),
    createRoute({ getParentRoute: () => rootRoute, path: "/exams/$attemptId" }),
  ]);
  return createRouter({
    routeTree,
    history: createMemoryHistory({ initialEntries: [initial] }),
  });
}

function dataLayer(): unknown[][] {
  const layer =
    (window as { dataLayer?: ArrayLike<unknown>[] }).dataLayer ?? [];
  return layer.map((entry) => Array.from(entry));
}

function pageViews(): string[] {
  return dataLayer()
    .filter((args) => args[0] === "event" && args[1] === "page_view")
    .map((args) => (args[2] as { page_location: string }).page_location);
}

function gaScripts(): HTMLScriptElement[] {
  return Array.from(
    document.querySelectorAll<HTMLScriptElement>(
      'script[src^="https://www.googletagmanager.com/gtag/js"]',
    ),
  );
}

const origin = window.location.origin;
let stop: (() => void) | undefined;

afterEach(() => {
  stop?.();
  stop = undefined;
  delete (window as { dataLayer?: unknown }).dataLayer;
  for (const script of gaScripts()) script.remove();
});

describe("analyticsMeasurementId", () => {
  it("returns null without an id, so no script is ever loaded", () => {
    expect(analyticsMeasurementId({ PROD: true })).toBeNull();
    expect(
      analyticsMeasurementId({ PROD: true, VITE_GA_MEASUREMENT_ID: "" }),
    ).toBeNull();
    expect(
      analyticsMeasurementId({ PROD: true, VITE_GA_MEASUREMENT_ID: "  " }),
    ).toBeNull();
    expect(gaScripts()).toHaveLength(0);
  });

  it("returns null outside production builds", () => {
    expect(
      analyticsMeasurementId({ PROD: false, VITE_GA_MEASUREMENT_ID: GA_ID }),
    ).toBeNull();
  });

  it("rejects a value that is not a GA4 measurement id", () => {
    expect(
      analyticsMeasurementId({
        PROD: true,
        VITE_GA_MEASUREMENT_ID: 'G-1"><script>',
      }),
    ).toBeNull();
    expect(
      analyticsMeasurementId({
        PROD: true,
        VITE_GA_MEASUREMENT_ID: "UA-1234-1",
      }),
    ).toBeNull();
  });

  it("returns the trimmed id in a production build", () => {
    expect(
      analyticsMeasurementId({
        PROD: true,
        VITE_GA_MEASUREMENT_ID: ` ${GA_ID} `,
      }),
    ).toBe(GA_ID);
  });
});

describe("startAnalytics", () => {
  it("loads gtag.js once with automatic page views and ads signals off", async () => {
    const router = makeRouter("/");
    await router.load();
    stop = startAnalytics(router, GA_ID);

    const scripts = gaScripts();
    expect(scripts).toHaveLength(1);
    expect(scripts[0]?.src).toBe(
      `https://www.googletagmanager.com/gtag/js?id=${GA_ID}`,
    );
    expect(scripts[0]?.async).toBe(true);

    const config = dataLayer().find((args) => args[0] === "config");
    expect(config?.[1]).toBe(GA_ID);
    expect(config?.[2]).toMatchObject({
      send_page_view: false,
      allow_google_signals: false,
      allow_ad_personalization_signals: false,
    });
    const consent = dataLayer().find((args) => args[0] === "consent");
    expect(consent?.[2]).toMatchObject({
      ad_storage: "denied",
      ad_user_data: "denied",
    });
  });

  it("sends one page_view per route change, as the route template without the query", async () => {
    const router = makeRouter("/forgot-password?token=reset-secret");
    await router.load();
    stop = startAnalytics(router, GA_ID);
    expect(pageViews()).toEqual([`${origin}/forgot-password`]);

    await router.navigate({
      to: "/exams/$attemptId",
      params: { attemptId: "attempt-42" },
    });
    expect(pageViews()).toEqual([
      `${origin}/forgot-password`,
      `${origin}/exams/$attemptId`,
    ]);

    await router.navigate({
      to: "/exams/$attemptId",
      params: { attemptId: "attempt-43" },
    });
    expect(pageViews()).toHaveLength(3);

    await router.navigate({ to: "/", search: { tab: "x" } });
    await router.navigate({ to: "/", search: { tab: "y" } });
    expect(pageViews()).toHaveLength(4);
    expect(pageViews().at(-1)).toBe(`${origin}/`);

    const sent = JSON.stringify(dataLayer());
    expect(sent).not.toContain("reset-secret");
    expect(sent).not.toContain("attempt-42");
    expect(sent).not.toContain("token");
  });

  it("reports an unknown path as /404 rather than what was typed", async () => {
    const router = makeRouter("/");
    await router.load();
    stop = startAnalytics(router, GA_ID);

    await router.navigate({ to: "/learner@example.com" });
    expect(pageViews().at(-1)).toBe(`${origin}/404`);
    expect(JSON.stringify(dataLayer())).not.toContain("learner@example.com");
  });

  it("counts the first page once when started before the router resolves", async () => {
    const router = makeRouter("/exams/attempt-7");
    stop = startAnalytics(router, GA_ID);
    await router.load();

    expect(pageViews()).toEqual([`${origin}/exams/$attemptId`]);
  });

  it("stops tracking once stopped", async () => {
    const router = makeRouter("/");
    await router.load();
    stop = startAnalytics(router, GA_ID);
    stop();

    await router.navigate({ to: "/forgot-password" });
    expect(pageViews()).toHaveLength(1);
  });
});
