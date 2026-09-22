import { act, fireEvent, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { ExampleSentences } from "@/components/ui/example-sentences";
import { PronounceButton } from "@/components/ui/pronounce-button";
import { readExampleSentences } from "@/lib/examples";
import { START_TIMEOUT_MS } from "@/lib/speech";

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
  voice: unknown = undefined;
  onstart: (() => void) | null = null;
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

  it("stays usable when a phone reports the utterance as interrupted", async () => {
    // Chrome on Android fires `interrupted` on an utterance queued just after a
    // cancel. That used to grey the button out for good on the first tap.
    render(<PronounceButton text="delicious" />);
    await userEvent.click(screen.getByRole("button"));

    const utterance = speak.mock.calls[0]?.[0] as FakeUtterance;
    act(() => {
      (utterance.onerror as ((event: { error: string }) => void) | null)?.({
        error: "interrupted",
      });
    });

    expect(screen.getByRole("button")).toBeEnabled();
    await userEvent.click(screen.getByRole("button"));
    expect(speak).toHaveBeenCalledTimes(2);
  });

  describe("when the engine takes the utterance and never speaks", () => {
    // Chrome on Android: the pinned voice is not installed, so the utterance is
    // dropped with no error. The icon pulsed and the phone stayed silent.
    const englishVoice = { lang: "en-US", name: "English" };

    beforeEach(() => {
      vi.useFakeTimers();
      vi.stubGlobal("speechSynthesis", {
        speak,
        cancel,
        resume: vi.fn(),
        getVoices: () => [englishVoice],
        speaking: false,
        pending: false,
      });
    });

    afterEach(() => {
      vi.useRealTimers();
    });

    it("retries without the pinned voice, then says what to check", () => {
      // A phrase, so no recording is looked up and the hint is the last resort.
      render(<PronounceButton text="a delicious meal" />);
      fireEvent.click(screen.getByRole("button"));

      expect(speak).toHaveBeenCalledTimes(1);
      const first = speak.mock.calls[0]?.[0] as FakeUtterance & {
        voice?: unknown;
      };
      expect(first.voice).toBe(englishVoice);

      act(() => {
        vi.advanceTimersByTime(START_TIMEOUT_MS);
      });
      expect(speak).toHaveBeenCalledTimes(2);
      const retry = speak.mock.calls[1]?.[0] as FakeUtterance & {
        voice?: unknown;
      };
      expect(retry.voice).toBeUndefined();
      expect(retry.lang).toBe("en-US");
      expect(screen.queryByRole("status")).not.toBeInTheDocument();

      act(() => {
        vi.advanceTimersByTime(START_TIMEOUT_MS);
      });
      expect(screen.getByRole("status")).toHaveTextContent(/media volume/i);
      // Still usable: the fix is on the phone, and the next tap should try again.
      expect(screen.getByRole("button")).toBeEnabled();
    });

    it("does not speak the previous card's word after the card changes", () => {
      const { rerender } = render(<PronounceButton text="delicious" />);
      fireEvent.click(screen.getByRole("button"));
      rerender(<PronounceButton text="habit" />);

      act(() => {
        vi.advanceTimersByTime(START_TIMEOUT_MS * 2);
      });
      expect(speak).toHaveBeenCalledTimes(1);
      expect(screen.queryByRole("status")).not.toBeInTheDocument();
    });

    it("leaves speech alone once the engine has started", () => {
      render(<PronounceButton text="delicious" />);
      fireEvent.click(screen.getByRole("button"));
      const first = speak.mock.calls[0]?.[0] as FakeUtterance & {
        onstart: (() => void) | null;
      };
      act(() => {
        first.onstart?.();
        vi.advanceTimersByTime(START_TIMEOUT_MS * 2);
      });
      expect(speak).toHaveBeenCalledTimes(1);
      expect(screen.queryByRole("status")).not.toBeInTheDocument();
    });
  });

  describe("when the device cannot speak, a recording plays instead", () => {
    class FakeAudio {
      static instances: FakeAudio[] = [];
      static fail = false;
      src: string;
      onplaying: (() => void) | null = null;
      onended: (() => void) | null = null;
      onerror: (() => void) | null = null;
      constructor(src: string) {
        this.src = src;
        FakeAudio.instances.push(this);
      }
      play() {
        return FakeAudio.fail
          ? Promise.reject(new Error("unsupported"))
          : Promise.resolve();
      }
      pause() {}
    }

    const fetchMock = vi.fn();

    beforeEach(() => {
      vi.useFakeTimers();
      FakeAudio.instances = [];
      FakeAudio.fail = false;
      fetchMock.mockReset();
      vi.stubGlobal("Audio", FakeAudio);
      vi.stubGlobal("fetch", fetchMock);
      // A device whose engine takes every utterance and never speaks.
      vi.stubGlobal("speechSynthesis", {
        speak,
        cancel,
        resume: vi.fn(),
        getVoices: () => [],
        speaking: false,
        pending: false,
      });
    });

    afterEach(() => {
      vi.useRealTimers();
    });

    const waitOutTheDevice = async () => {
      await act(async () => {
        await vi.advanceTimersByTimeAsync(START_TIMEOUT_MS * 2);
      });
    };

    it("plays the dictionary's US recording and credits it", async () => {
      fetchMock.mockResolvedValue({
        ok: true,
        json: () =>
          Promise.resolve([
            {
              phonetics: [
                {
                  audio: "https://api.example/eat-uk.mp3",
                  sourceUrl: "https://commons.example/uk",
                  license: { name: "BY 3.0 US" },
                },
                {
                  audio: "https://api.example/eat-us.mp3",
                  sourceUrl: "https://commons.example/us",
                  license: { name: "BY-SA 3.0" },
                },
              ],
            },
          ]),
      });
      render(<PronounceButton text="eat" />);
      fireEvent.click(screen.getByRole("button"));
      await waitOutTheDevice();

      expect(FakeAudio.instances[0]?.src).toBe(
        "https://api.example/eat-us.mp3",
      );
      act(() => FakeAudio.instances[0]?.onplaying?.());

      const credit = screen.getByRole("link", { name: /Wikimedia Commons/ });
      expect(credit).toHaveAttribute("href", "https://commons.example/us");
      expect(credit).toHaveTextContent("BY-SA 3.0");
      expect(screen.queryByText(/media volume/i)).not.toBeInTheDocument();
    });

    it("goes straight to Wikimedia Commons when the dictionary is down", async () => {
      fetchMock.mockRejectedValue(new Error("522"));
      render(<PronounceButton text="work" />);
      fireEvent.click(screen.getByRole("button"));
      await waitOutTheDevice();

      expect(FakeAudio.instances[0]?.src).toContain(
        "commons.wikimedia.org/wiki/Special:FilePath/En-us-work.ogg",
      );
    });

    it("says what to check when no source plays", async () => {
      fetchMock.mockResolvedValue({
        ok: false,
        json: () => Promise.resolve([]),
      });
      FakeAudio.fail = true;
      render(<PronounceButton text="habit" />);
      fireEvent.click(screen.getByRole("button"));
      await waitOutTheDevice();

      // Both Commons candidates were tried before giving up.
      expect(FakeAudio.instances).toHaveLength(2);
      expect(screen.getByRole("status")).toHaveTextContent(/media volume/i);
      expect(screen.getByRole("button")).toBeEnabled();
    });

    it("does not look up a recording for a sentence", async () => {
      render(<PronounceButton text="The soup smelled delicious." />);
      fireEvent.click(screen.getByRole("button"));
      await waitOutTheDevice();

      expect(fetchMock).not.toHaveBeenCalled();
      expect(FakeAudio.instances).toHaveLength(0);
      expect(screen.getByRole("status")).toHaveTextContent(/media volume/i);
    });
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
