import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { ExampleSentences } from "@/components/ui/example-sentences";
import { PronounceButton } from "@/components/ui/pronounce-button";
import { readExampleSentences } from "@/lib/examples";

/**
 * The behaviour these cover is the reason the feature exists.
 *
 * A pronunciation control that renders but never makes a sound is exactly what
 * shipped before: the old button lived behind `ipa && audioUrl`, and no seeded
 * sense carries an `audio_url`, so every learner saw a card with a phonetic
 * transcription and no way to hear it. So the assertions are about the speaking,
 * not about the icon.
 */

const speak = vi.fn();
const cancel = vi.fn();

class FakeUtterance {
  text: string;
  lang = "";
  rate = 1;
  onend: (() => void) | null = null;
  onerror: (() => void) | null = null;

  constructor(text: string) {
    this.text = text;
  }
}

beforeEach(() => {
  speak.mockClear();
  cancel.mockClear();
  vi.stubGlobal("speechSynthesis", { speak, cancel });
  vi.stubGlobal("SpeechSynthesisUtterance", FakeUtterance);
});

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("PronounceButton", () => {
  it("speaks the word when the content carries no recorded audio", async () => {
    render(<PronounceButton text="delicious" />);

    await userEvent.click(screen.getByRole("button"));

    expect(speak).toHaveBeenCalledTimes(1);
    const utterance = speak.mock.calls[0]?.[0] as FakeUtterance;
    expect(utterance.text).toBe("delicious");
    expect(utterance.lang).toBe("en-US");
  });

  it("does not flip the card behind it", async () => {
    const onFlip = vi.fn();
    // A button, not a div with a click handler: the real card is keyboard
    // operable, and this stands in for it without needing a11y rules disabled.
    render(
      <button type="button" onClick={onFlip}>
        <PronounceButton text="delicious" />
      </button>,
    );

    // The inner speaker, not the card around it.
    const speaker = screen.getAllByRole("button").at(-1) as HTMLElement;
    await userEvent.click(speaker);

    expect(speak).toHaveBeenCalledTimes(1);
    // The click must not reach the card. Without stopPropagation, hearing the
    // word also turns the card over and gives away the answer.
    expect(onFlip).not.toHaveBeenCalled();
  });

  it("is disabled when there is nothing to say", () => {
    render(<PronounceButton text="   " />);
    expect(screen.getByRole("button")).toBeDisabled();
  });
});

describe("ExampleSentences", () => {
  const sentences = [
    {
      text: "The pasta was absolutely delicious.",
      translation: "Món mì rất ngon.",
    },
    { text: "That was the most delicious meal all year." },
    { text: "The soup smelled delicious." },
    { text: "She thanked him for the delicious bread." },
    { text: "Everything on the menu looked delicious." },
  ];

  it("shows the first two and reveals the rest on request", async () => {
    render(<ExampleSentences sentences={sentences} highlight="delicious" />);

    expect(screen.getByText(/The pasta was absolutely/)).toBeInTheDocument();
    expect(
      screen.queryByText(/Everything on the menu/),
    ).not.toBeInTheDocument();

    await userEvent.click(screen.getByRole("button", { name: /show more/i }));

    expect(screen.getByText(/Everything on the menu/)).toBeInTheDocument();
  });

  it("gives every visible sentence its own speaker", () => {
    render(<ExampleSentences sentences={sentences} initialVisible={5} />);
    // By name, not by counting every button: the block also carries the
    // translation toggle, and a bare count would silently pass or fail on it.
    expect(
      screen.getAllByRole("button", { name: /listen to this sentence/i }),
    ).toHaveLength(5);
  });

  it("renders nothing when a sense has no examples", () => {
    const { container } = render(<ExampleSentences sentences={[]} />);
    expect(container).toBeEmptyDOMElement();
  });

  it("keeps the translation hidden until it is asked for", async () => {
    // Showing both at once defeats the exercise: the eye goes to the line it
    // can read and the English becomes decoration under it.
    render(<ExampleSentences sentences={sentences} />);
    expect(screen.queryByText("Món mì rất ngon.")).not.toBeInTheDocument();

    await userEvent.click(
      screen.getByRole("button", { name: /show meaning/i }),
    );
    expect(screen.getByText("Món mì rất ngon.")).toBeInTheDocument();

    await userEvent.click(
      screen.getByRole("button", { name: /hide meaning/i }),
    );
    expect(screen.queryByText("Món mì rất ngon.")).not.toBeInTheDocument();
  });

  it("offers no translation control when the content carries none", () => {
    render(<ExampleSentences sentences={[{ text: "English only." }]} />);
    expect(
      screen.queryByRole("button", { name: /show meaning/i }),
    ).not.toBeInTheDocument();
  });
});

describe("readExampleSentences", () => {
  it("reads the object shape the seed writes, translation and all", () => {
    expect(
      readExampleSentences({
        example_sentences: [
          {
            sentence: "He reads at leisure.",
            sentence_vi: "Anh ấy đọc lúc rảnh.",
          },
        ],
      }),
    ).toEqual([
      { text: "He reads at leisure.", translation: "Anh ấy đọc lúc rảnh." },
    ]);
  });

  it("carries an audio URL when the content has one", () => {
    // The dictionary returns a link to a human recording, and it is stored as a
    // link rather than a downloaded file.
    expect(
      readExampleSentences({
        example_sentences: [
          { sentence: "A sentence.", audio_url: "https://example.test/a.mp3" },
        ],
      }),
    ).toEqual([
      { text: "A sentence.", audioUrl: "https://example.test/a.mp3" },
    ]);
  });

  it("still reads the first version of the list, which was English-only", () => {
    expect(
      readExampleSentences({
        example_sentence: "One.",
        example_sentences: ["One.", "Two.", "Three."],
      }),
    ).toEqual([{ text: "One." }, { text: "Two." }, { text: "Three." }]);
  });

  it("falls back to the single sentence on content authored before the list", () => {
    expect(readExampleSentences({ example_sentence: "Only one." })).toEqual([
      { text: "Only one." },
    ]);
  });

  it("drops entries with no sentence rather than rendering empty rows", () => {
    expect(
      readExampleSentences({
        example_sentences: ["  ", 42, { sentence_vi: "no english" }, "Kept."],
      }),
    ).toEqual([{ text: "Kept." }]);
  });

  it("yields an empty list for a body with no examples at all", () => {
    expect(readExampleSentences({ word: "delicious" })).toEqual([]);
    expect(readExampleSentences(null)).toEqual([]);
  });
});
