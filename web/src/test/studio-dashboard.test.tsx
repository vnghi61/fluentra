import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
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

import i18n, { initI18n, loadLocale } from "@/i18n";
import { CreatorDashboard } from "@/features/studio/components/CreatorDashboard";
import type { CreatorEarningsSummary } from "@/features/studio/api/studioApi";
import { server } from "./msw-server";

const baseEarnings: CreatorEarningsSummary = {
  available_balance_vnd: 1250000,
  lifetime_earnings_vnd: 3000000,
  pending_payout_vnd: 0,
  total_paid_out_vnd: 1750000,
  payout_threshold_vnd: 500000,
  can_request_payout: true,
  payout_account_configured: false,
  recent_ledger: [],
};

async function renderDashboard() {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  const rootRoute = createRootRoute();
  const studioRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: "/studio",
    component: () => (
      <I18nextProvider i18n={i18n}>
        <QueryClientProvider client={client}>
          <CreatorDashboard />
        </QueryClientProvider>
      </I18nextProvider>
    ),
  });
  const router = createRouter({
    routeTree: rootRoute.addChildren([studioRoute]),
    history: createMemoryHistory({ initialEntries: ["/studio"] }),
  });
  await router.load();
  return render(<RouterProvider router={router} />);
}

function mockEarnings(earnings: CreatorEarningsSummary) {
  server.use(
    http.get("/api/v1/me/studio/earnings", () => HttpResponse.json(earnings)),
  );
}

describe("CreatorDashboard", () => {
  beforeEach(async () => {
    await initI18n("en");
    await loadLocale("en");
    await i18n.changeLanguage("en");
    server.use(
      http.get("/api/v1/studio/creator/profile", () =>
        HttpResponse.json({ message: "not found" }, { status: 404 }),
      ),
      http.get("/api/v1/studio/courses", () =>
        HttpResponse.json({ items: [], total: 0 }),
      ),
    );
  });

  it("renders the four finance figures and the bank notice when no account is configured", async () => {
    mockEarnings(baseEarnings);
    await renderDashboard();

    expect(
      await screen.findByTestId("studio-available-balance"),
    ).toHaveTextContent("1.250.000 ₫");
    expect(screen.getByTestId("studio-lifetime-earnings")).toHaveTextContent(
      "3.000.000 ₫",
    );
    expect(screen.getByTestId("studio-total-paid-out")).toBeInTheDocument();
    expect(screen.getByTestId("studio-pending-payout")).toBeInTheDocument();
    expect(screen.getByTestId("studio-bank-notice")).toBeInTheDocument();
    // No fabricated trend figure.
    expect(screen.queryByText(/\+0%/)).not.toBeInTheDocument();
  });

  it("hides the bank notice once a payout account is configured", async () => {
    mockEarnings({ ...baseEarnings, payout_account_configured: true });
    await renderDashboard();

    await screen.findByTestId("studio-available-balance");
    expect(screen.queryByTestId("studio-bank-notice")).not.toBeInTheDocument();
  });

  it("does not flash the bank notice while earnings are loading", async () => {
    server.use(
      http.get("/api/v1/me/studio/earnings", () => new Promise(() => {})),
    );
    await renderDashboard();

    expect(
      await screen.findByTestId("studio-finance-skeleton"),
    ).toBeInTheDocument();
    expect(screen.queryByTestId("studio-bank-notice")).not.toBeInTheDocument();
  });

  it("shows compact empty states for courses and the ledger", async () => {
    mockEarnings(baseEarnings);
    await renderDashboard();

    expect(
      await screen.findByText(
        "Earnings will appear here once learners enrol in your courses.",
      ),
    ).toBeInTheDocument();
    expect(screen.getByText("No transactions yet")).toBeInTheDocument();
    expect(await screen.findByTestId("studio-course-count")).toHaveTextContent(
      "0",
    );
    expect(
      screen.getByRole("button", { name: /Create First Course/i }),
    ).toBeInTheDocument();
  });

  it("keeps course management when earnings fail and retries that section only", async () => {
    let calls = 0;
    server.use(
      http.get("/api/v1/me/studio/earnings", () => {
        calls += 1;
        return calls === 1
          ? HttpResponse.json({ message: "boom" }, { status: 500 })
          : HttpResponse.json(baseEarnings);
      }),
    );
    await renderDashboard();

    const alert = await screen.findByRole("alert");
    expect(alert).toHaveTextContent("Couldn't load earnings.");
    expect(
      screen.getByRole("button", { name: /Create First Course/i }),
    ).toBeInTheDocument();

    await userEvent.click(screen.getByRole("button", { name: "Try again" }));
    await waitFor(() =>
      expect(
        screen.getByTestId("studio-available-balance"),
      ).toBeInTheDocument(),
    );
  });
});
