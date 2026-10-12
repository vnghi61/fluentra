import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { http, HttpResponse } from "msw";
import { I18nextProvider } from "react-i18next";
import { useState, type ReactElement } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import i18n, { initI18n, loadLocale } from "@/i18n";
import { CheckoutModal } from "@/features/studio/components/CheckoutModal";
import { server } from "./msw-server";

const COURSE_ID = "course-checkout";

function renderWith(ui: ReactElement) {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return render(
    <I18nextProvider i18n={i18n}>
      <QueryClientProvider client={client}>{ui}</QueryClientProvider>
    </I18nextProvider>,
  );
}

describe("CheckoutModal", () => {
  beforeEach(async () => {
    await initI18n("en");
    await loadLocale("en");
    await i18n.changeLanguage("en");
  });

  it("claims a free course and shows the success message", async () => {
    const user = userEvent.setup();
    const claim = vi.fn(() =>
      HttpResponse.json({ message: "claimed", purchase_id: "purchase-1" }),
    );
    server.use(http.post(`/api/v1/courses/${COURSE_ID}/claim`, claim));

    renderWith(
      <CheckoutModal
        courseId={COURSE_ID}
        courseTitle="Free course"
        priceVnd={0}
        isOpen
        onClose={vi.fn()}
        onSuccess={vi.fn()}
      />,
    );

    await user.click(screen.getByRole("button", { name: "Enroll for Free" }));

    expect(await screen.findByText(/Enrolled successfully!/i)).toBeInTheDocument();
    expect(claim).toHaveBeenCalledTimes(1);
  });

  it("shows the paid VietQR order and confirms payment", async () => {
    const user = userEvent.setup();
    const onClose = vi.fn();
    const onSuccess = vi.fn();
    server.use(
      http.post(`/api/v1/courses/${COURSE_ID}/purchase`, () =>
        HttpResponse.json({
          order_id: "order-1",
          reference: "FLUENTRA-ORDER-1",
          amount_vnd: 250000,
          bank_code: "VCB",
          account_number: "0123456789",
          account_holder_name: "FLUENTRA",
          qr_url: "https://example.test/qr.png",
        }),
      ),
      http.get("/api/v1/me/orders/order-1", () =>
        HttpResponse.json({
          id: "order-1",
          status: "paid",
          amount_vnd: 250000,
        }),
      ),
    );

    renderWith(
      <CheckoutModal
        courseId={COURSE_ID}
        courseTitle="Paid course"
        priceVnd={250000}
        isOpen
        onClose={onClose}
        onSuccess={onSuccess}
      />,
    );

    expect(await screen.findByText("FLUENTRA-ORDER-1")).toBeInTheDocument();
    expect(screen.getByRole("img", { name: "VietQR Transfer Code" })).toHaveAttribute(
      "src",
      "https://example.test/qr.png",
    );
    expect(await screen.findByText("Payment Verified!")).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "Start Learning Now" }));
    expect(onSuccess).toHaveBeenCalledTimes(1);
    expect(onClose).toHaveBeenCalledTimes(1);
  });

  it("closes on Escape and restores focus to the opener", async () => {
    const user = userEvent.setup();
    function CheckoutWithOpener() {
      const [isOpen, setIsOpen] = useState(false);
      return (
        <>
          <button onClick={() => setIsOpen(true)}>Open checkout</button>
          <CheckoutModal
            courseId={COURSE_ID}
            courseTitle="Free course"
            priceVnd={0}
            isOpen={isOpen}
            onClose={() => setIsOpen(false)}
            onSuccess={vi.fn()}
          />
        </>
      );
    }

    renderWith(<CheckoutWithOpener />);
    const opener = screen.getByRole("button", { name: "Open checkout" });
    await user.click(opener);
    await user.keyboard("{Escape}");

    await waitFor(() => expect(opener).toHaveFocus());
  });
});
