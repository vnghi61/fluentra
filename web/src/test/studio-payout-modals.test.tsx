import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { http, HttpResponse } from "msw";
import { I18nextProvider } from "react-i18next";
import { beforeEach, describe, expect, it, vi } from "vitest";

import i18n, { initI18n, loadLocale } from "@/i18n";
import { PayoutAccountModal } from "@/features/studio/components/PayoutAccountModal";
import { RequestPayoutModal } from "@/features/studio/components/RequestPayoutModal";
import { server } from "./msw-server";

function renderWith(ui: React.ReactElement) {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return render(
    <I18nextProvider i18n={i18n}>
      <QueryClientProvider client={client}>{ui}</QueryClientProvider>
    </I18nextProvider>,
  );
}

describe("payout modals", () => {
  beforeEach(async () => {
    await initI18n("en");
    await loadLocale("en");
    await i18n.changeLanguage("en");
    server.use(
      http.get("/api/v1/studio/creator/payout-account", () =>
        HttpResponse.json({ message: "not found" }, { status: 404 }),
      ),
      http.get("/api/v1/me/studio/earnings", () =>
        HttpResponse.json({
          available_balance_vnd: 1250000,
          lifetime_earnings_vnd: 3000000,
          pending_payout_vnd: 0,
          total_paid_out_vnd: 1750000,
          payout_threshold_vnd: 500000,
          can_request_payout: true,
          payout_account_configured: true,
          payout_bank_code: "MB",
          payout_masked_account: "****1234",
          recent_ledger: [],
        }),
      ),
    );
  });

  it("PayoutAccountModal portals, selects a bank, and Escape closes only the listbox", async () => {
    const user = userEvent.setup();
    const onClose = vi.fn();
    renderWith(<PayoutAccountModal isOpen onClose={onClose} />);

    const dialog = screen.getByRole("dialog");
    expect(dialog.parentElement).toBe(document.body);
    expect(screen.getByRole("button", { name: "Save Account" })).toBeVisible();
    expect(screen.getByRole("button", { name: "Cancel" })).toBeVisible();

    await user.click(screen.getByRole("combobox"));
    await user.keyboard("vcb");
    await user.click(
      await screen.findByRole("option", { name: /Vietcombank/ }),
    );
    expect(screen.getByRole("combobox")).toHaveTextContent(/Vietcombank/);

    await user.click(screen.getByRole("combobox"));
    await user.keyboard("{Escape}");
    expect(onClose).not.toHaveBeenCalled();
    expect(screen.queryByRole("listbox")).toBeNull();
    expect(screen.getByRole("dialog")).toBeInTheDocument();
  });

  it("RequestPayoutModal exposes Confirm Payout and closes on Escape", async () => {
    const user = userEvent.setup();
    const onClose = vi.fn();
    renderWith(<RequestPayoutModal isOpen onClose={onClose} />);

    expect(screen.getByRole("dialog").parentElement).toBe(document.body);
    expect(
      screen.getByRole("button", { name: "Confirm Payout" }),
    ).toBeInTheDocument();
    await user.keyboard("{Escape}");
    expect(onClose).toHaveBeenCalledTimes(1);
  });
});
