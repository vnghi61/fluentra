import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { ExerciseMatch } from "@/features/learning";

const noop = () => {};

beforeEach(() => {
  vi.stubGlobal("speechSynthesis", { speak: vi.fn(), cancel: vi.fn() });
  vi.stubGlobal(
    "SpeechSynthesisUtterance",
    class {
      constructor(public text: string) {}
      lang = "";
      rate = 1;
      onend: (() => void) | null = null;
      onerror: (() => void) | null = null;
    },
  );
});

describe("ExerciseMatch keyboard and pairing", () => {
  const words = [
    { id: "w_habit", text: "habit" },
    { id: "w_afford", text: "afford" },
  ];
  const definitions = [
    { id: "d_afford", text: "Have enough money to pay for something" },
    { id: "d_habit", text: "A regular tendency or practice" },
  ];
  const props = {
    prompt: "Match each word with its meaning:",
    words,
    definitions,
    isSubmitted: false,
    onSubmit: noop,
    onContinue: noop,
  };

  it("ensures every item is a button reachable by Tab before any word is selected", async () => {
    render(<ExerciseMatch {...props} />);

    const word1 = screen.getByRole("button", { name: /habit/ });
    const word2 = screen.getByRole("button", { name: /afford/ });
    const def1 = screen.getByRole("button", { name: /Have enough money/ });
    const def2 = screen.getByRole("button", { name: /A regular tendency/ });

    expect(word1).toBeEnabled();
    expect(word2).toBeEnabled();
    expect(def1).toBeEnabled();
    expect(def2).toBeEnabled();

    // First tab focuses the first word
    await userEvent.tab();
    expect(word1).toHaveFocus();
  });

  it("supports keyboard-only completion and submits pairings", async () => {
    const onSubmit = vi.fn();
    render(<ExerciseMatch {...props} onSubmit={onSubmit} />);

    // Tab to word 1 ("habit")
    await userEvent.tab();
    expect(screen.getByRole("button", { name: /habit/ })).toHaveFocus();
    await userEvent.keyboard("{Enter}");

    // Word 1 is selected
    expect(screen.getByRole("button", { name: /habit/ })).toHaveAttribute(
      "aria-pressed",
      "true",
    );
    expect(screen.getByText("Selected")).toBeInTheDocument();

    // Tab past pronounce 1, word 2, pronounce 2, def 1 to def 2 ("A regular tendency")
    await userEvent.tab(); // Pronounce word 1
    await userEvent.tab(); // Word 2
    await userEvent.tab(); // Pronounce word 2
    await userEvent.tab(); // Definition 1 ("Have enough money")
    await userEvent.tab(); // Definition 2 ("A regular tendency")
    expect(
      screen.getByRole("button", { name: /A regular tendency/ }),
    ).toHaveFocus();
    await userEvent.keyboard("{Enter}");

    // Pair 1 formed: word 1 + def 2
    expect(screen.getByText(/1\/2/)).toBeInTheDocument();

    // Tab backwards to word 2 ("afford")
    await userEvent.tab({ shift: true }); // Def 1
    await userEvent.tab({ shift: true }); // Pronounce word 2
    await userEvent.tab({ shift: true }); // Word 2
    expect(screen.getByRole("button", { name: /afford/ })).toHaveFocus();
    // Use Space key to select word 2
    await userEvent.keyboard(" ");
    expect(screen.getByRole("button", { name: /afford/ })).toHaveAttribute(
      "aria-pressed",
      "true",
    );

    // Tab forward to Definition 1 ("Have enough money")
    await userEvent.tab(); // Pronounce word 2
    await userEvent.tab(); // Definition 1
    expect(
      screen.getByRole("button", { name: /Have enough money/ }),
    ).toHaveFocus();
    // Use Space key to complete pair 2
    await userEvent.keyboard(" ");

    // Both pairs are complete: 2/2
    expect(screen.getByText(/2\/2/)).toBeInTheDocument();

    // Tab forward to Check Answer button
    await userEvent.tab(); // Definition 2
    await userEvent.tab(); // Clear all button
    await userEvent.tab(); // Check Answer button
    const checkBtn = screen.getByRole("button", { name: /check/i });
    expect(checkBtn).toHaveFocus();
    expect(checkBtn).toBeEnabled();

    await userEvent.keyboard("{Enter}");

    expect(onSubmit).toHaveBeenCalledTimes(1);
    expect(onSubmit).toHaveBeenCalledWith({
      w_habit: "d_habit",
      w_afford: "d_afford",
    });
  });

  it("shows matching pair number badges on both sides with accessible names including 'pair N'", async () => {
    render(<ExerciseMatch {...props} />);

    const word1 = screen.getByRole("button", { name: /habit/ });
    const word2 = screen.getByRole("button", { name: /afford/ });
    const defHabit = screen.getByRole("button", { name: /A regular tendency/ });
    const defAfford = screen.getByRole("button", { name: /Have enough money/ });

    // Initially neither side has pair 1
    expect(word1).not.toHaveAccessibleName(/pair 1/i);
    expect(defHabit).not.toHaveAccessibleName(/pair 1/i);

    // Pair word 1 ("habit") with defHabit
    await userEvent.click(word1);
    await userEvent.click(defHabit);

    // Both sides show pair badge 1 and accessible name with "pair 1"
    expect(word1).toHaveAccessibleName(/pair 1/i);
    expect(defHabit).toHaveAccessibleName(/pair 1/i);
    expect(within(word1).getByText("1")).toBeInTheDocument();
    expect(within(defHabit).getByText("1")).toBeInTheDocument();

    // Pair word 2 ("afford") with defAfford
    await userEvent.click(word2);
    await userEvent.click(defAfford);

    // Both sides show pair badge 2 and accessible name with "pair 2"
    expect(word2).toHaveAccessibleName(/pair 2/i);
    expect(defAfford).toHaveAccessibleName(/pair 2/i);
    expect(within(word2).getByText("2")).toBeInTheDocument();
    expect(within(defAfford).getByText("2")).toBeInTheDocument();

    // Accessible name queries find both sides
    const pair1Items = screen.getAllByRole("button", { name: /pair 1/i });
    expect(pair1Items).toHaveLength(2);
    const pair2Items = screen.getAllByRole("button", { name: /pair 2/i });
    expect(pair2Items).toHaveLength(2);
  });

  it("unpairs when selecting a paired word or definition", async () => {
    render(<ExerciseMatch {...props} />);

    const word1 = screen.getByRole("button", { name: /habit/ });
    const defHabit = screen.getByRole("button", { name: /A regular tendency/ });

    // Pair word 1 and defHabit
    await userEvent.click(word1);
    await userEvent.click(defHabit);
    expect(screen.getByText(/1\/2/)).toBeInTheDocument();
    expect(word1).toHaveAccessibleName(/pair 1/i);
    expect(defHabit).toHaveAccessibleName(/pair 1/i);

    // Clicking paired word unpairs it
    await userEvent.click(word1);
    expect(screen.getByText(/0\/2/)).toBeInTheDocument();
    expect(word1).not.toHaveAccessibleName(/pair 1/i);
    expect(defHabit).not.toHaveAccessibleName(/pair 1/i);
    expect(word1).toHaveAttribute("aria-pressed", "false");
    expect(screen.queryByText("Selected")).not.toBeInTheDocument();

    // Pair again and unpair via definition
    await userEvent.click(word1);
    await userEvent.click(defHabit);
    expect(screen.getByText(/1\/2/)).toBeInTheDocument();

    await userEvent.click(defHabit);
    expect(screen.getByText(/0\/2/)).toBeInTheDocument();
    expect(word1).not.toHaveAccessibleName(/pair 1/i);
    expect(defHabit).not.toHaveAccessibleName(/pair 1/i);
    expect(screen.queryByText("Selected")).not.toBeInTheDocument();
  });

  it("clears current selection on Escape key", async () => {
    render(<ExerciseMatch {...props} />);

    const word1 = screen.getByRole("button", { name: /habit/ });

    // Select word 1
    await userEvent.click(word1);
    expect(word1).toHaveAttribute("aria-pressed", "true");
    expect(screen.getByText("Selected")).toBeInTheDocument();

    // Press Escape
    await userEvent.keyboard("{Escape}");
    expect(word1).toHaveAttribute("aria-pressed", "false");
    expect(screen.queryByText("Selected")).not.toBeInTheDocument();
  });

  it("shows no number on unpaired words and matching numbers when paired", async () => {
    render(<ExerciseMatch {...props} />);

    const word1 = screen.getByRole("button", { name: /habit/ });
    const word2 = screen.getByRole("button", { name: /afford/ });
    const defHabit = screen.getByRole("button", { name: /A regular tendency/ });
    const defAfford = screen.getByRole("button", { name: /Have enough money/ });

    // Unpaired items show placeholder (dash or question mark) and no number
    expect(within(word1).queryByText("1")).not.toBeInTheDocument();
    expect(within(word2).queryByText("2")).not.toBeInTheDocument();
    expect(within(defHabit).queryByText("1")).not.toBeInTheDocument();
    expect(within(defAfford).queryByText("2")).not.toBeInTheDocument();
    expect(within(word1).getByText("—")).toBeInTheDocument();
    expect(within(defHabit).getByText("?")).toBeInTheDocument();

    // Pair word 1 and defHabit
    await userEvent.click(word1);
    await userEvent.click(defHabit);

    // Paired items show number 1 on both sides
    expect(within(word1).getByText("1")).toBeInTheDocument();
    expect(within(defHabit).getByText("1")).toBeInTheDocument();

    // Word 2 is still unpaired and shows no number
    expect(within(word2).queryByText("2")).not.toBeInTheDocument();
    expect(within(word2).getByText("—")).toBeInTheDocument();
  });

  it("formats aria-label without index prefix when paired", async () => {
    render(<ExerciseMatch {...props} />);

    const word1 = screen.getByRole("button", { name: /habit/ });
    const defHabit = screen.getByRole("button", { name: /A regular tendency/ });

    await userEvent.click(word1);
    await userEvent.click(defHabit);

    expect(word1).toHaveAttribute("aria-label", "habit, pair 1");
    expect(defHabit).toHaveAttribute(
      "aria-label",
      "A regular tendency or practice, pair 1",
    );
  });

  it("registers no window keydown listener", () => {
    const addEventListenerSpy = vi.spyOn(window, "addEventListener");
    render(<ExerciseMatch {...props} />);
    expect(addEventListenerSpy).not.toHaveBeenCalledWith(
      "keydown",
      expect.any(Function),
    );
    addEventListenerSpy.mockRestore();
  });
});
