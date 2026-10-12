import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, waitFor } from "@testing-library/react";
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
    expect(dialog).toHaveAccessibleDescription(
      "Add the bank account where you want to receive your earnings.",
    );
    expect(screen.getByLabelText("Account Number")).toHaveAttribute(
      "inputmode",
      "numeric",
    );
    expect(
      screen.getByLabelText("Account Holder Name"),
    ).toHaveAccessibleDescription(
      "Unaccented uppercase letters matching your bank account.",
    );

    await user.click(screen.getByRole("combobox"));
    await user.keyboard("vcb");
    expect(screen.getByRole("listbox").parentElement).toHaveClass("z-[70]");
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

  it("selecting a bank with Enter does not submit the account form", async () => {
    const user = userEvent.setup();
    const save = vi.fn(() => HttpResponse.json({}, { status: 422 }));
    server.use(http.post("/api/v1/studio/creator/payout-account", save));
    renderWith(<PayoutAccountModal isOpen onClose={vi.fn()} />);
    await user.type(screen.getByLabelText("Account Number"), "00012345");
    await user.type(
      screen.getByLabelText("Account Holder Name"),
      "NGUYEN VAN A",
    );
    await user.click(screen.getByRole("combobox"));
    expect(
      screen.getByRole("textbox", { name: "Search bank by name or code..." }),
    ).toHaveFocus();
    await user.keyboard("vcb{Enter}");
    expect(screen.getByRole("combobox")).toHaveTextContent(/Vietcombank/);
    expect(save).not.toHaveBeenCalled();
    expect(screen.queryByRole("alert")).toBeNull();
  });

  it("locks the form and dismissal while saving, then permits retry after an error", async () => {
    const user = userEvent.setup();
    const onClose = vi.fn();
    let finish!: () => void;
    const pending = new Promise<void>((resolve) => {
      finish = resolve;
    });
    const save = vi.fn(async () => {
      await pending;
      return HttpResponse.json({ title: "Unable to save" }, { status: 422 });
    });
    server.use(http.post("/api/v1/studio/creator/payout-account", save));
    renderWith(<PayoutAccountModal isOpen onClose={onClose} />);
    await user.type(screen.getByLabelText("Account Number"), "00012345");
    await user.type(
      screen.getByLabelText("Account Holder Name"),
      "NGUYEN VAN A",
    );
    await user.click(screen.getByRole("button", { name: "Save Account" }));
    try {
      await waitFor(() => expect(save).toHaveBeenCalledOnce());
      expect(screen.getByRole("combobox")).toBeDisabled();
      expect(screen.getByLabelText("Account Number")).toBeDisabled();
      expect(screen.getByLabelText("Account Holder Name")).toBeDisabled();
      expect(screen.getByRole("button", { name: "Cancel" })).toBeDisabled();
      expect(screen.queryByRole("button", { name: "Close" })).toBeNull();
      await user.keyboard("{Escape}");
      expect(onClose).not.toHaveBeenCalled();
    } finally {
      finish();
    }
    expect(await screen.findByRole("alert")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Save Account" })).toBeEnabled();
    expect(screen.getByLabelText("Account Number")).toHaveValue("00012345");
    await user.keyboard("{Escape}");
    expect(onClose).toHaveBeenCalledOnce();
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
