import { useState } from "react";
import { describe, expect, it, vi } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import {
  Dialog,
  DialogContent,
  DialogTitle,
} from "@/components/ui/dialog";
import {
  SheetDialog,
  SheetTitle,
} from "@/components/ui/sheet";

function ControlledDialog({ unmountOnClose = false }: { unmountOnClose?: boolean }) {
  const [open, setOpen] = useState(false);
  const opener = <button onClick={() => setOpen(true)}>Open dialog</button>;

  if (unmountOnClose && !open) return opener;

  return (
    <>
      {opener}
      <Dialog open={open} onOpenChange={setOpen}>
        <DialogContent>
          <DialogTitle>Dialog title</DialogTitle>
          <button onClick={() => setOpen(false)}>Close dialog</button>
        </DialogContent>
      </Dialog>
    </>
  );
}

function ControlledSheet() {
  const [open, setOpen] = useState(false);
  return (
    <>
      <button onClick={() => setOpen(true)}>Open sheet</button>
      <SheetDialog open={open} onOpenChange={setOpen} title="Sheet title">
        <SheetTitle className="sr-only">Sheet title</SheetTitle>
      </SheetDialog>
    </>
  );
}

describe("dialog focus restoration", () => {
  it("restores focus after a controlled dialog closes", async () => {
    const user = userEvent.setup();
    render(<ControlledDialog />);
    const opener = screen.getByRole("button", { name: "Open dialog" });

    await user.click(opener);
    await user.keyboard("{Escape}");

    await waitFor(() => expect(opener).toHaveFocus());
  });

  it("restores focus after a sheet closes", async () => {
    const user = userEvent.setup();
    render(<ControlledSheet />);
    const opener = screen.getByRole("button", { name: "Open sheet" });

    await user.click(opener);
    await user.keyboard("{Escape}");

    await waitFor(() => expect(opener).toHaveFocus());
  });

  it("restores focus when the dialog subtree unmounts", async () => {
    const user = userEvent.setup();
    render(<ControlledDialog unmountOnClose />);
    const opener = screen.getByRole("button", { name: "Open dialog" });

    await user.click(opener);
    await user.click(screen.getByRole("button", { name: "Close dialog" }));

    await waitFor(() => expect(opener).toHaveFocus());
  });

  it("preserves caller autofocus handlers and preventDefault control", async () => {
    const user = userEvent.setup();
    const onOpenAutoFocus = vi.fn();
    const onCloseAutoFocus = vi.fn((event: Event) => event.preventDefault());
    function Example() {
      const [open, setOpen] = useState(false);
      return (
        <>
          <button onClick={() => setOpen(true)}>Open custom dialog</button>
          <Dialog open={open} onOpenChange={setOpen}>
            <DialogContent
              onOpenAutoFocus={onOpenAutoFocus}
              onCloseAutoFocus={onCloseAutoFocus}
            >
              <DialogTitle>Custom dialog</DialogTitle>
            </DialogContent>
          </Dialog>
        </>
      );
    }
    render(<Example />);
    const opener = screen.getByRole("button", { name: "Open custom dialog" });

    await user.click(opener);
    await user.keyboard("{Escape}");

    await waitFor(() => expect(onOpenAutoFocus).toHaveBeenCalled());
    expect(onCloseAutoFocus).toHaveBeenCalled();
    expect(opener).not.toHaveFocus();
  });
});
