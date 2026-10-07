import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { I18nextProvider } from "react-i18next";
import { beforeEach, describe, expect, it } from "vitest";
import { useState } from "react";
import i18n, { initI18n } from "@/i18n";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog";

function Harness({ pending = false }: { pending?: boolean }) {
  const [open, setOpen] = useState(false);
  return (
    <I18nextProvider i18n={i18n}>
      <Dialog
        open={open}
        onOpenChange={(next) => {
          if (!pending || next) setOpen(next);
        }}
      >
        <DialogTrigger asChild>
          <button type="button">Open</button>
        </DialogTrigger>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Dialog title</DialogTitle>
          </DialogHeader>
          <button type="button">First</button>
          <button type="button">Last</button>
        </DialogContent>
      </Dialog>
    </I18nextProvider>
  );
}

describe("shadcn Dialog", () => {
  beforeEach(async () => {
    await initI18n("en");
  });

  it("traps focus inside and returns focus after close", async () => {
    const user = userEvent.setup();
    render(<Harness />);
    const trigger = screen.getByRole("button", { name: "Open" });
    await user.click(trigger);
    const last = screen.getByRole("button", { name: "Last" });
    last.focus();
    await user.tab();
    expect(screen.getByRole("button", { name: "Close" })).toHaveFocus();
    await user.keyboard("{Escape}");
    await waitFor(() => expect(trigger).toHaveFocus());
  });

  it("closes on Escape and scroll-locks while open", async () => {
    const user = userEvent.setup();
    render(<Harness />);
    await user.click(screen.getByRole("button", { name: "Open" }));
    expect(document.body).toHaveAttribute("data-scroll-locked", "1");
    await user.keyboard("{Escape}");
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    await waitFor(() =>
      expect(document.body).not.toHaveAttribute("data-scroll-locked"),
    );
  });

  it("blocks closing while pending and keeps the close target at least 44px", async () => {
    const user = userEvent.setup();
    render(<Harness pending />);
    await user.click(screen.getByRole("button", { name: "Open" }));
    const close = screen.getByRole("button", { name: "Close" });
    expect(close.className).toContain("h-11");
    expect(close.className).toContain("w-11");
    await user.keyboard("{Escape}");
    expect(screen.getByRole("dialog")).toBeInTheDocument();
    await user.click(close);
    expect(screen.getByRole("dialog")).toBeInTheDocument();
  });
});
