import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import {
  createMemoryHistory,
  createRouter,
  RouterProvider,
} from "@tanstack/react-router";
import { render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it } from "vitest";

import { TooltipProvider } from "@/components/ui/tooltip";
import i18n, { initI18n, loadLocale } from "@/i18n";
import { routeTree } from "@/app/router";

describe("standalone system-state routing", () => {
  beforeEach(async () => {
    await initI18n("en");
    await loadLocale("en");
    await i18n.changeLanguage("en");
  });

  it("renders unmatched URLs without application navigation", async () => {
    const router = createRouter({
      routeTree,
      history: createMemoryHistory({ initialEntries: ["/no-such-page"] }),
    });
    const client = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    });

    await router.load();
    render(
      <QueryClientProvider client={client}>
        <TooltipProvider delayDuration={0}>
          <RouterProvider router={router} />
        </TooltipProvider>
      </QueryClientProvider>,
    );

    expect(
      await screen.findByRole("heading", { level: 1 }),
    ).toBeInTheDocument();
    expect(screen.getByText("404")).toBeInTheDocument();
    expect(
      screen.queryByRole("navigation", { name: /Main navigation/i }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole("navigation", { name: /Mobile navigation/i }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: /More/i }),
    ).not.toBeInTheDocument();
  });

  it("renders a real public route without the 404 state", async () => {
    const router = createRouter({
      routeTree,
      history: createMemoryHistory({ initialEntries: ["/login"] }),
    });
    const client = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    });

    await router.load();
    render(
      <QueryClientProvider client={client}>
        <TooltipProvider delayDuration={0}>
          <RouterProvider router={router} />
        </TooltipProvider>
      </QueryClientProvider>,
    );

    expect(screen.queryByText("404")).not.toBeInTheDocument();
    expect(
      await screen.findByPlaceholderText("learner@example.com"),
    ).toBeInTheDocument();
  });
});
