import React from "react";
import {
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
  RouterProvider,
} from "@tanstack/react-router";
import { render, screen } from "@testing-library/react";
import { I18nextProvider } from "react-i18next";
import { beforeAll, describe, expect, it } from "vitest";

import i18n, { initI18n, loadLocale } from "@/i18n";
import { MinimalHeader } from "@/components/layout/MinimalHeader";
import { OrbitBrandMark } from "@/components/layout/OrbitBrandMark";
import { ServerWakingState } from "@/components/layout/ServerWakingState";
import { SystemStatePage } from "@/components/layout/SystemStatePage";

function renderWithI18n(ui: React.ReactNode) {
  return render(<I18nextProvider i18n={i18n}>{ui}</I18nextProvider>);
}

beforeAll(async () => {
  await initI18n("en");
  await loadLocale("en");
  await i18n.changeLanguage("en");
});

describe("system state presentation", () => {
  it("renders the 404 state and hides the code from other states", () => {
    const { rerender } = renderWithI18n(
      <SystemStatePage variant="server-unavailable" />,
    );
    expect(screen.queryByText("404")).not.toBeInTheDocument();
    expect(screen.getByRole("heading", { level: 1 })).toHaveTextContent(
      "Cannot connect to the server",
    );
    expect(
      screen.getByText(
        "Fluentra cannot reach the server right now. Check your connection or try again in a few minutes.",
      ),
    ).toBeInTheDocument();
    rerender(
      <I18nextProvider i18n={i18n}>
        <SystemStatePage variant="404" />
      </I18nextProvider>,
    );
    expect(screen.getByText("404")).toBeInTheDocument();
    rerender(
      <I18nextProvider i18n={i18n}>
        <SystemStatePage variant="500" />
      </I18nextProvider>,
    );
    expect(screen.getByRole("heading", { level: 1 })).toHaveTextContent(
      "Something went wrong",
    );
    expect(
      screen.getByText(
        "The server ran into a problem handling the request. Please try again later.",
      ),
    ).toBeInTheDocument();
  });

  it("exposes orbit motion state", () => {
    const { rerender } = renderWithI18n(
      <OrbitBrandMark size="md" animated={false} />,
    );
    expect(screen.getByTestId("orbit-brand-mark")).toHaveAttribute(
      "data-animated",
      "false",
    );
    expect(
      screen
        .getByTestId("orbit-brand-mark")
        .querySelector('[class*="animate-orbit"]'),
    ).toBeNull();
    rerender(<OrbitBrandMark size="md" animated />);
    expect(
      screen
        .getByTestId("orbit-brand-mark")
        .querySelector(".motion-safe\\:animate-orbit-slow"),
    ).toBeTruthy();
  });

  it("renders waking status and minimal home link", async () => {
    const rootRoute = createRootRoute({
      component: () => (
        <I18nextProvider i18n={i18n}>
          <ServerWakingState />
          <MinimalHeader />
        </I18nextProvider>
      ),
    });
    const pageRoute = createRoute({
      getParentRoute: () => rootRoute,
      path: "/",
    });
    const router = createRouter({
      routeTree: rootRoute.addChildren([pageRoute]),
      history: createMemoryHistory({ initialEntries: ["/"] }),
    });
    await router.load();
    render(<RouterProvider router={router} />);
    const status = screen.getByRole("status");
    expect(status).toHaveAttribute("aria-busy", "true");
    expect(status).toHaveTextContent("Starting the server");
    expect(screen.getByRole("link", { name: "Fluentra" })).toHaveAttribute(
      "href",
      "/",
    );
  });
});
