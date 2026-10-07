import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { http, HttpResponse } from "msw";
import { I18nextProvider } from "react-i18next";
import { beforeEach, describe, expect, it } from "vitest";

import i18n, { initI18n, loadLocale } from "@/i18n";
import { AdminReportedContentList } from "@/features/admin/components/AdminReportedContentList";

import { server } from "./msw-server";

const sampleReportedItem = {
  content_version_id: "0199a1c2-3d4e-7f80-9abc-def01234567a",
  item_id: "0199a1c2-3d4e-7f80-9abc-def01234567b",
  slug: "grammar-past-perfect-1",
  kind: "grammar_exercise",
  cefr_level: "B1",
  item_status: "published",
  report_count: 3,
  last_reported_at: "2026-09-20T10:00:00Z",
};

const sampleContentDetail = {
  item_id: "0199a1c2-3d4e-7f80-9abc-def01234567b",
  slug: "grammar-past-perfect-1",
  kind: "grammar_exercise",
  status: "published",
  created_at: "2026-09-10T08:00:00Z",
  updated_at: "2026-09-20T10:00:00Z",
  versions: [
    {
      id: "0199a1c2-3d4e-7f80-9abc-def01234567a",
      version_number: 1,
      status: "published",
      cefr_level: "B1",
      body: { prompt: "Test prompt" },
      tags: [],
      created_at: "2026-09-10T08:00:00Z",
    },
  ],
};

function renderReportedList() {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return render(
    <I18nextProvider i18n={i18n}>
      <QueryClientProvider client={client}>
        <AdminReportedContentList />
      </QueryClientProvider>
    </I18nextProvider>,
  );
}

describe("AdminReportedContentList", () => {
  beforeEach(async () => {
    await initI18n("en");
    await i18n.changeLanguage("en");

    server.use(
      http.get("/api/v1/admin/content/reports", () =>
        HttpResponse.json({
          items: [sampleReportedItem],
          total: 1,
          limit: 15,
          offset: 0,
        }),
      ),
      http.get("/api/v1/admin/content/:id", () =>
        HttpResponse.json(sampleContentDetail),
      ),
      http.get("/api/v1/me/permissions", () =>
        HttpResponse.json({
          permissions: [
            "content.read.published",
            "content.edit",
            "content.review",
            "content.publish",
          ],
        }),
      ),
    );
  });

  it("renders mobile cards below md with reason/summary first, status badge, secondary ID, and action button", async () => {
    renderReportedList();

    const cardList = await screen.findByTestId("reported-cards");
    expect(cardList).toBeInTheDocument();

    const card = within(cardList).getByTestId("reported-card");
    expect(card).toBeInTheDocument();

    // Reason/summary first
    expect(card).toHaveTextContent(sampleReportedItem.slug);

    // Text status badge (translated)
    expect(card).toHaveTextContent("Published");

    // Secondary ID
    expect(
      within(card).getAllByTitle(sampleReportedItem.content_version_id),
    ).toHaveLength(2);

    // Report count
    expect(card).toHaveTextContent("3 reports");

    // Action button
    const actionBtn = within(card).getByRole("button", {
      name: /Review item/i,
    });
    expect(actionBtn).toBeInTheDocument();
  });

  it("clicking review action button opens content detail modal", async () => {
    renderReportedList();

    const cardList = await screen.findByTestId("reported-cards");
    const card = within(cardList).getByTestId("reported-card");
    const actionBtn = within(card).getByRole("button", {
      name: /Review item/i,
    });

    await userEvent.click(actionBtn);

    // Modal dialog opens
    const dialog = await screen.findByRole("dialog");
    expect(dialog).toBeInTheDocument();
    expect(
      within(dialog).getByText(/grammar-past-perfect-1/i),
    ).toBeInTheDocument();
  });

  it("shows card list without filter button since no filter controls exist on this queue", async () => {
    renderReportedList();

    await screen.findByTestId("reported-cards");

    // No filter button rendered
    expect(
      screen.queryByRole("button", { name: /^Filters/i }),
    ).not.toBeInTheDocument();
    // No filter chips rendered
    expect(
      screen.queryByTestId("applied-filter-chips"),
    ).not.toBeInTheDocument();
  });

  it("shows loaded result count in role='status' saying 'loaded' when more pages exist and no total invented", async () => {
    // 1. Initial state (1 item, total = 1 -> no more pages)
    renderReportedList();
    await screen.findByTestId("reported-cards");

    const statusLine = screen.getByRole("status");
    expect(statusLine).toHaveTextContent("1 result");
    expect(statusLine).not.toHaveTextContent(/loaded/i);

    // 2. When more pages exist (e.g. total 50, limit 15, 1 item loaded)
    server.use(
      http.get("/api/v1/admin/content/reports", () =>
        HttpResponse.json({
          items: [sampleReportedItem],
          total: 50,
          limit: 15,
          offset: 0,
        }),
      ),
    );

    // Trigger refetch via Refresh button
    const refreshBtn = screen.getByRole("button", { name: /Refresh/i });
    await userEvent.click(refreshBtn);

    await waitFor(() => {
      const updatedStatus = screen.getByRole("status");
      expect(updatedStatus).toHaveTextContent("1 loaded");
      // Invents no total (e.g. does not say "50")
      expect(updatedStatus).not.toHaveTextContent("50");
    });
  });

  it("preserves desktop table with identical data columns and review action", async () => {
    renderReportedList();

    const table = await screen.findByRole("table");
    expect(table).toBeInTheDocument();

    expect(
      within(table).getByText(sampleReportedItem.slug),
    ).toBeInTheDocument();
    expect(
      within(table).getAllByTitle(sampleReportedItem.content_version_id),
    ).toHaveLength(2);
    expect(within(table).getByText("Published")).toBeInTheDocument();
    expect(
      within(table).getByRole("button", { name: /Review item/i }),
    ).toBeInTheDocument();
  });

  it("translates item_status in Vietnamese locale and falls back to raw status for unknown values", async () => {
    await loadLocale("vi");
    await i18n.changeLanguage("vi");

    const customReportedItem = {
      ...sampleReportedItem,
      content_version_id: "0199a1c2-3d4e-7f80-9abc-def01234567c",
      item_status: "custom_unknown_status",
    };

    server.use(
      http.get("/api/v1/admin/content/reports", () =>
        HttpResponse.json({
          items: [sampleReportedItem, customReportedItem],
          total: 2,
          limit: 15,
          offset: 0,
        }),
      ),
    );

    renderReportedList();

    const table = await screen.findByRole("table");
    expect(table).toBeInTheDocument();

    // Standard status translated to Vietnamese
    expect(within(table).getByText("Đã xuất bản")).toBeInTheDocument();
    // Unknown status falls back to raw string
    expect(
      within(table).getByText("custom_unknown_status"),
    ).toBeInTheDocument();
  });
});
