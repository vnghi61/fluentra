import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useState } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import i18n, { initI18n, loadLocale } from "@/i18n";
import { ExerciseSentenceTransform } from "@/features/learning/components/Runner/ExerciseSentenceTransform";
import { RunnerHeader } from "@/features/learning/components/Runner/RunnerHeader";
import { ReportDialog } from "@/features/learning/components/Runner/ReportDialog";
import { ExitDialog } from "@/features/learning/components/Runner/ExitDialog";

beforeEach(async () => {
  await initI18n("en");
  await loadLocale("en");
  await i18n.changeLanguage("en");
});

describe("runner answer interface", () => {
  it("keeps the answer editable without hints and sends trimmed text", async () => {
    const user = userEvent.setup();
    const onSubmit = vi.fn();
    render(
      <ExerciseSentenceTransform
        prompt="Rewrite the sentence using Although."
        expectedAnswer="Although he was tired, he finished."
        isSubmitted={false}
        onSubmit={onSubmit}
        onContinue={vi.fn()}
      />,
    );
    const input = screen.getByRole("textbox", {
      name: "Your rewritten sentence",
    });
    expect(input.tagName).toBe("TEXTAREA");
    expect(
      screen.queryByText("Although he was tired, he finished."),
    ).toBeNull();
    expect(screen.queryByText(/hint/i)).toBeNull();
    expect(screen.getByRole("button", { name: "Check Answer" })).toBeDisabled();
    await user.type(input, "  Although he was tired, he finished.  ");
    await user.click(screen.getByRole("button", { name: "Check Answer" }));
    expect(onSubmit).toHaveBeenCalledWith(
      "Although he was tired, he finished.",
    );
  });

  it("reveals the correction only after submitting", () => {
    render(
      <ExerciseSentenceTransform
        prompt="Rewrite the sentence."
        expectedAnswer="Although he was tired, he finished."
        isSubmitted
        isCorrect={false}
        onSubmit={vi.fn()}
        onContinue={vi.fn()}
      />,
    );
    expect(
      screen.getByText("Although he was tired, he finished."),
    ).toBeInTheDocument();
    expect(screen.getByRole("textbox")).toBeDisabled();
  });

  it("makes reporting reachable from the header before answering", async () => {
    const onReport = vi.fn();
    render(
      <RunnerHeader
        lessonTitle="Tenses"
        currentStep={2}
        totalSteps={5}
        onExit={vi.fn()}
        onReport={onReport}
      />,
    );
    await userEvent
      .setup()
      .click(screen.getByRole("button", { name: "Report issue" }));
    expect(onReport).toHaveBeenCalledOnce();
    expect(screen.getByRole("progressbar")).toHaveAttribute(
      "aria-valuenow",
      "40",
    );
  });

  it("portals the report form and closes it with Escape", async () => {
    const onClose = vi.fn();
    const { container } = render(
      <ReportDialog
        isOpen
        contentVersionId="content-version"
        onClose={onClose}
      />,
    );
    const dialog = await screen.findByRole("dialog", {
      name: "Report an issue",
    });
    expect(container.contains(dialog)).toBe(false);
    expect(
      within(dialog).getByRole("textbox", {
        name: "Additional details (optional)",
      }),
    ).toBeInTheDocument();
    await userEvent.setup().keyboard("{Escape}");
    expect(onClose).toHaveBeenCalledOnce();
  });

  it("focuses Keep Learning, traps focus, and restores the exit trigger on Escape", async () => {
    const user = userEvent.setup();
    const onConfirm = vi.fn();
    function RunnerWithExit() {
      const [open, setOpen] = useState(false);
      return (
        <>
          <button onClick={() => setOpen(true)}>Open exit confirmation</button>
          <ExitDialog
            isOpen={open}
            onCancel={() => setOpen(false)}
            onConfirm={onConfirm}
          />
        </>
      );
    }
    const { container } = render(<RunnerWithExit />);
    const trigger = screen.getByRole("button", {
      name: "Open exit confirmation",
    });
    await user.click(trigger);
    const dialog = screen.getByRole("alertdialog", { name: "Exit Lesson?" });
    expect(container.contains(dialog)).toBe(false);
    expect(dialog).toHaveAccessibleDescription(
      /haven't submitted will be lost/,
    );
    expect(screen.getByRole("button", { name: "Keep Learning" })).toHaveFocus();
    await user.tab();
    expect(screen.getByRole("button", { name: "Exit Lesson" })).toHaveFocus();
    await user.tab();
    expect(screen.getByRole("button", { name: "Keep Learning" })).toHaveFocus();
    await user.keyboard("{Escape}");
    expect(screen.queryByRole("alertdialog")).toBeNull();
    expect(trigger).toHaveFocus();
    expect(onConfirm).not.toHaveBeenCalled();
    await user.click(trigger);
    await user.click(screen.getByRole("button", { name: "Keep Learning" }));
    expect(screen.queryByRole("alertdialog")).toBeNull();
    expect(onConfirm).not.toHaveBeenCalled();
  });

  it("only exits the lesson on explicit confirmation", async () => {
    const onConfirm = vi.fn();
    render(<ExitDialog isOpen onCancel={vi.fn()} onConfirm={onConfirm} />);
    await userEvent
      .setup()
      .click(screen.getByRole("button", { name: "Exit Lesson" }));
    expect(onConfirm).toHaveBeenCalledOnce();
  });
});
