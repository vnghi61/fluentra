import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { http, HttpResponse } from "msw";
import { I18nextProvider } from "react-i18next";
import { beforeEach, describe, expect, it } from "vitest";
import i18n, { initI18n } from "@/i18n";
import { AdminPayoutsList } from "@/features/admin/components/AdminPayoutsList";
import { server } from "./msw-server";

describe("Admin payout confirmation", () => {
  let fulfilled = false;
  beforeEach(async () => {
    fulfilled = false;
    await initI18n("en");
    server.use(
      http.get("/api/v1/admin/billing/payouts", () =>
        HttpResponse.json({
          items: [
            {
              id: "payout-1",
              creator_id: "creator-1",
              amount_vnd: 125000,
              status: "pending",
              created_at: "2026-09-20T10:00:00Z",
            },
          ],
          total: 1,
          limit: 50,
          offset: 0,
        }),
      ),
      http.post("/api/v1/admin/billing/payouts/:id/fulfill", () => {
        fulfilled = true;
        return HttpResponse.json({
          id: "payout-1",
          creator_id: "creator-1",
          amount_vnd: 125000,
          status: "sent",
          created_at: "2026-09-20T10:00:00Z",
        });
      }),
    );
  });
  it("waits for explicit confirmation before the fulfill request", async () => {
    const user = userEvent.setup();
    const client = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    });
    render(
      <I18nextProvider i18n={i18n}>
        <QueryClientProvider client={client}>
          <AdminPayoutsList />
        </QueryClientProvider>
      </I18nextProvider>,
    );
    await user.click(
      (await screen.findAllByRole("button", { name: "Fulfill" }))[0]!,
    );
    await user.type(
      screen.getByLabelText("Bank Transaction Reference"),
      "BANK-REF-123",
    );
    await user.click(
      screen.getByRole("button", { name: "Confirm Fulfillment" }),
    );
    expect(await screen.findByRole("alertdialog")).toHaveTextContent(
      "creator-1",
    );
    expect(fulfilled).toBe(false);
    await user.click(
      screen.getAllByRole("button", { name: "Confirm Fulfillment" }).at(-1)!,
    );
    expect(fulfilled).toBe(true);
  });
});
