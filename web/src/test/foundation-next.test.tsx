import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import {
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
  RouterProvider,
} from "@tanstack/react-router";
import { render, screen } from "@testing-library/react";
import { http, HttpResponse } from "msw";
import { I18nextProvider } from "react-i18next";
import { beforeEach, describe, expect, it } from "vitest";

import i18n, { initI18n } from "@/i18n";
import {
  FoundationNextCard,
  pathNodeState,
} from "@/features/learning/components/Foundation/FoundationNextCard";

import { server } from "./msw-server";

async function renderCard() {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  const rootRoute = createRootRoute();
  const indexRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: "/",
    component: () => (
      <I18nextProvider i18n={i18n}>
        <QueryClientProvider client={client}>
          <FoundationNextCard />
        </QueryClientProvider>
      </I18nextProvider>
    ),
  });
  const topicRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: "/foundation/topics/$code",
    component: () => <div>topic</div>,
  });
  const router = createRouter({
    routeTree: rootRoute.addChildren([indexRoute, topicRoute]),
    history: createMemoryHistory({ initialEntries: ["/"] }),
  });
  await router.load();
  return render(<RouterProvider router={router} />);
}

describe("pathNodeState", () => {
  it("marks everything after the first unmastered node as locked", () => {
    expect(pathNodeState({ mastered: true, next: false }, false)).toBe(
      "mastered",
    );
    expect(pathNodeState({ mastered: false, next: true }, false)).toBe("next");
    // The node after next waits on it.
    expect(pathNodeState({ mastered: false, next: false }, true)).toBe(
      "locked",
    );
    // Before the next node there is nothing to wait for.
    expect(pathNodeState({ mastered: false, next: false }, false)).toBe("open");
  });
});

describe("FoundationNextCard", () => {
  beforeEach(async () => {
    await initI18n("en");
    await i18n.changeLanguage("en");
  });

  it("shows the server's next node and links to its topic", async () => {
    server.use(
      http.get("/api/v1/me/foundation/next", () =>
        HttpResponse.json({
          id: "0199a1c2-3d4e-7f80-9abc-def01234567c",
          namespace: "grammar",
          code: "PRESENT_CONTINUOUS",
          label: "Present Continuous",
          cefr_level: "A1",
          attempts: 0,
          score: 0,
          mastered: false,
          next: true,
        }),
      ),
    );

    await renderCard();

    expect(await screen.findByText("Present Continuous")).toBeInTheDocument();
    expect(
      screen.getByRole("link", { name: /Start this topic/i }),
    ).toHaveAttribute("href", "/foundation/topics/PRESENT_CONTINUOUS");
  });

  it("renders nothing when no topics are published yet", async () => {
    server.use(
      http.get("/api/v1/me/foundation/next", () =>
        HttpResponse.json({ title: "Not Found", status: 404 }, { status: 404 }),
      ),
    );

    const { container } = await renderCard();

    // The request settles, and the card stays out of the dashboard.
    await new Promise((resolve) => setTimeout(resolve, 50));
    expect(container).toBeEmptyDOMElement();
  });
});
