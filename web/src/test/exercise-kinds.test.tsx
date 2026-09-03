import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";

import {
  ExerciseContextChoice,
  ExerciseListenType,
  ExerciseMatch,
  ExerciseReorder,
} from "@/features/learning";

/**
 * The four kinds added beside multiple choice, gap fill and flashcard.
 *
 * What is asserted here is what a learner can and cannot do — the shape of the
 * submitted payload, and whether the interface lets them build an answer at all.
 * The grading itself is the server's, and is tested in Go.
 */

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

describe("ExerciseListenType", () => {
  const props = {
    prompt: "Listen and type the word you hear:",
    audioText: "leisure",
    isSubmitted: false,
    onSubmit: noop,
    onContinue: noop,
  };

  it("does not show the word before the learner answers", () => {
    render(<ExerciseListenType {...props} ipa="/ˈleʒ.ər/" hint="Free time" />);

    // The hint is the meaning, which is fair. The spelling is the question.
    expect(screen.getByText(/Free time/)).toBeInTheDocument();
    expect(screen.queryByText("/ˈleʒ.ər/")).not.toBeInTheDocument();
  });

  it("reveals the transcription once the answer is in", () => {
    render(
      <ExerciseListenType
        {...props}
        ipa="/ˈleʒ.ər/"
        isSubmitted
        isCorrect
        expectedAnswer="leisure"
      />,
    );
    expect(screen.getByText("/ˈleʒ.ər/")).toBeInTheDocument();
  });

  it("submits what was typed", async () => {
    const onSubmit = vi.fn();
    render(<ExerciseListenType {...props} onSubmit={onSubmit} />);

    await userEvent.type(screen.getByRole("textbox"), "leisure");
    await userEvent.click(screen.getByRole("button", { name: /check/i }));

    expect(onSubmit).toHaveBeenCalledWith("leisure");
  });
});

describe("ExerciseMatch", () => {
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

  it("cannot be checked until every word is paired", async () => {
    render(<ExerciseMatch {...props} />);
    const check = screen.getByRole("button", { name: /check/i });
    expect(check).toBeDisabled();

    await userEvent.click(screen.getByRole("button", { name: /^1\s*habit$/ }));
    await userEvent.click(
      screen.getByRole("button", { name: /A regular tendency/ }),
    );
    // One of two pairs made.
    expect(check).toBeDisabled();
  });

  it("submits the pairing the learner built", async () => {
    const onSubmit = vi.fn();
    render(<ExerciseMatch {...props} onSubmit={onSubmit} />);

    await userEvent.click(screen.getByRole("button", { name: /^1\s*habit$/ }));
    await userEvent.click(
      screen.getByRole("button", { name: /A regular tendency/ }),
    );
    await userEvent.click(screen.getByRole("button", { name: /^2\s*afford$/ }));
    await userEvent.click(
      screen.getByRole("button", { name: /Have enough money/ }),
    );
    await userEvent.click(screen.getByRole("button", { name: /check/i }));

    expect(onSubmit).toHaveBeenCalledWith({
      w_habit: "d_habit",
      w_afford: "d_afford",
    });
  });

  it("lets a pair be undone", async () => {
    render(<ExerciseMatch {...props} />);

    await userEvent.click(screen.getByRole("button", { name: /^1\s*habit$/ }));
    await userEvent.click(
      screen.getByRole("button", { name: /A regular tendency/ }),
    );
    // The counter is built from several text nodes, so it is read off the
    // element rather than matched as one string.
    const counter = () =>
      screen.getByText(/Pick a word|Now pick/).textContent ?? "";
    expect(counter()).toContain("1/2");

    // Tapping the definition again breaks the pair. Without an undo the
    // learner is stuck answering the interface rather than the question.
    await userEvent.click(
      screen.getByRole("button", { name: /A regular tendency/ }),
    );
    expect(counter()).toContain("0/2");
  });

  it("reports a partial score rather than a bare failure", () => {
    render(
      <ExerciseMatch {...props} isSubmitted isCorrect={false} score={75} />,
    );
    expect(screen.getByRole("status")).toHaveTextContent("75%");
  });
});

describe("ExerciseReorder", () => {
  // A sentence with a word repeated twice: tiles are addressed by index, and
  // keying them on their text would make both "to" tiles the same tile.
  const tokens = ["to", "Remember", "to", "breathe"];
  const props = {
    prompt: "Put the words in the right order:",
    tokens,
    isSubmitted: false,
    onSubmit: noop,
    onContinue: noop,
  };

  it("cannot be checked until every word is placed", async () => {
    const onSubmit = vi.fn();
    render(<ExerciseReorder {...props} onSubmit={onSubmit} />);

    const check = screen.getByRole("button", { name: /check/i });
    expect(check).toBeDisabled();

    const bank = screen.getByRole("group", { name: /available words/i });
    for (const tile of Array.from(bank.querySelectorAll("button"))) {
      await userEvent.click(tile);
    }
    expect(check).toBeEnabled();
  });

  it("places both copies of a repeated word", async () => {
    const onSubmit = vi.fn();
    render(<ExerciseReorder {...props} onSubmit={onSubmit} />);

    const bank = screen.getByRole("group", { name: /available words/i });
    const tiles = Array.from(bank.querySelectorAll("button"));
    // Remember(1) to(0) breathe(3) — then the second "to" at index 2.
    for (const index of [1, 0, 3, 2]) {
      await userEvent.click(tiles[index] as HTMLElement);
    }
    await userEvent.click(screen.getByRole("button", { name: /check/i }));

    expect(onSubmit).toHaveBeenCalledWith("Remember to breathe to");
  });

  it("takes a word back out of the answer", async () => {
    render(<ExerciseReorder {...props} />);

    const bank = screen.getByRole("group", { name: /available words/i });
    await userEvent.click(
      bank.querySelectorAll("button")[1] as unknown as HTMLElement,
    );

    const answer = screen.getByRole("group", { name: /your sentence/i });
    expect(answer.querySelectorAll("button")).toHaveLength(1);

    await userEvent.click(
      answer.querySelectorAll("button")[0] as unknown as HTMLElement,
    );
    expect(answer.querySelectorAll("button")).toHaveLength(0);
  });
});

describe("ExerciseContextChoice", () => {
  const props = {
    prompt: "What does the word mean in this sentence?",
    sentence: "He spends his leisure time restoring old bicycles.",
    targetWord: "leisure",
    options: [
      { id: "opt_free_time", text: "Time when you are not working" },
      { id: "opt_paid_work", text: "Time you are paid to work" },
    ],
    isSubmitted: false,
    onSubmit: noop,
    onContinue: noop,
  };

  it("submits the chosen option id, not its text", async () => {
    const onSubmit = vi.fn();
    render(<ExerciseContextChoice {...props} onSubmit={onSubmit} />);

    await userEvent.click(screen.getByRole("radio", { name: /not working/i }));
    await userEvent.click(screen.getByRole("button", { name: /check/i }));

    expect(onSubmit).toHaveBeenCalledWith("opt_free_time");
  });

  it("shows the sentence with the target word emphasised", () => {
    render(<ExerciseContextChoice {...props} />);
    // The word is bold inside the sentence, so it is its own element.
    expect(screen.getByText("leisure").tagName).toBe("STRONG");
  });
});
