import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { I18nextProvider } from "react-i18next";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { useState } from "react";

import i18n, { initI18n, loadLocale } from "@/i18n";
import { Modal, ModalBody, ModalFooter } from "@/components/ui/modal";

function Harness({ onClose }: { onClose?: () => void }) {
  const [open, setOpen] = useState(false);
  return (
    <I18nextProvider i18n={i18n}>
      <button type="button" onClick={() => setOpen(true)}>
        trigger
      </button>
      <Modal
        open={open}
        title="Hello"
        onClose={() => {
          onClose?.();
          setOpen(false);
        }}
      >
        <ModalBody data-testid="body">content</ModalBody>
        <ModalFooter>
          <button type="button">Footer action</button>
        </ModalFooter>
      </Modal>
    </I18nextProvider>
  );
}

describe("Modal", () => {
  beforeEach(async () => {
    await initI18n("en");
    await loadLocale("en");
    await i18n.changeLanguage("en");
    document.documentElement.style.overflow = "";
  });

  it("portals the dialog to document.body", async () => {
    const user = userEvent.setup();
    render(<Harness />);
    await user.click(screen.getByText("trigger"));
    const dialog = screen.getByRole("dialog");
    expect(dialog.parentElement).toBe(document.body);
    expect(dialog).toHaveAttribute("data-scroll-lock", "managed");
  });

  it("renders a scrolling body and a footer", async () => {
    const user = userEvent.setup();
    render(<Harness />);
    await user.click(screen.getByText("trigger"));
    expect(screen.getByTestId("body").className).toContain("overflow-y-auto");
    expect(screen.getByText("Footer action")).toBeInTheDocument();
  });

  it("closes on Escape", async () => {
    const user = userEvent.setup();
    const onClose = vi.fn();
    render(<Harness onClose={onClose} />);
    await user.click(screen.getByText("trigger"));
    await user.keyboard("{Escape}");
    expect(onClose).toHaveBeenCalledTimes(1);
    expect(screen.queryByRole("dialog")).toBeNull();
  });

  it("locks and restores documentElement overflow", async () => {
    const user = userEvent.setup();
    document.documentElement.style.overflow = "scroll";
    const { unmount } = render(<Harness />);
    await user.click(screen.getByText("trigger"));
    expect(document.documentElement.style.overflow).toBe("hidden");
    await user.keyboard("{Escape}");
    expect(document.documentElement.style.overflow).toBe("scroll");

    await user.click(screen.getByText("trigger"));
    expect(document.documentElement.style.overflow).toBe("hidden");
    unmount();
    expect(document.documentElement.style.overflow).toBe("scroll");
  });

  it("returns focus to the trigger after close", async () => {
    const user = userEvent.setup();
    render(<Harness />);
    const trigger = screen.getByText("trigger");
    await user.click(trigger);
    expect(screen.getByRole("dialog")).toContainElement(
      document.activeElement as HTMLElement,
    );
    await user.keyboard("{Escape}");
    expect(trigger).toHaveFocus();
  });
});
