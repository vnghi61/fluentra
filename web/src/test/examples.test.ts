import { describe, expect, it } from "vitest";

import { shuffleExamplesForReview, type ExampleSentence } from "@/lib/examples";

function examples(count: number): ExampleSentence[] {
  return Array.from({ length: count }, (_, i) => ({
    text: `Sentence ${i + 1}`,
    translation: `Câu ${i + 1}`,
  }));
}

/** How many distinct three-sentence hands `count` sentences produce over `runs`. */
function distinctHands(count: number, runs = 40): number {
  const seen = new Set<string>();
  for (let i = 0; i < runs; i++) {
    seen.add(
      shuffleExamplesForReview(examples(count))
        .map((s) => s.text)
        .join("|"),
    );
  }
  return seen.size;
}

describe("shuffleExamplesForReview", () => {
  it("shows at most three, whatever the word has", () => {
    expect(shuffleExamplesForReview(examples(15))).toHaveLength(3);
    expect(shuffleExamplesForReview(examples(8))).toHaveLength(3);
    expect(shuffleExamplesForReview(examples(4))).toHaveLength(3);
  });

  it("varies the hand for a word that is not yet full", () => {
    // The enrichment job fills a word to fifteen over several nights, so five is
    // what every word has for its first week — and what a word the model cannot
    // extend has for ever. The first version only shuffled at fifteen, which
    // meant the case that needed variety most never got any.
    //
    // Five choose three is ten ordered-sample-free combinations and sixty
    // ordered ones; forty draws landing on a single hand is not chance.
    expect(distinctHands(5)).toBeGreaterThan(1);
    expect(distinctHands(15)).toBeGreaterThan(1);
  });

  it("keeps every sentence when there are three or fewer", () => {
    // Nothing to choose between, and reordering lines that are all on screen
    // anyway only makes the card jitter between flips.
    const three = examples(3);
    expect(shuffleExamplesForReview(three)).toEqual(three);
    expect(shuffleExamplesForReview([])).toEqual([]);
  });

  it("returns sentences from the word, and does not repeat one", () => {
    const source = examples(15);
    const hand = shuffleExamplesForReview(source);
    const texts = source.map((s) => s.text);

    for (const item of hand) expect(texts).toContain(item.text);
    expect(new Set(hand.map((s) => s.text)).size).toBe(hand.length);
  });

  it("does not disturb the caller's array", () => {
    // The array belongs to the card content and is read again on the next
    // render; an in-place shuffle would reorder it under the component.
    const source = examples(15);
    const before = source.map((s) => s.text);
    shuffleExamplesForReview(source);
    expect(source.map((s) => s.text)).toEqual(before);
  });
});
