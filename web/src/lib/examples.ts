/**
 * One example sentence and, where the content carries it, its translation.
 *
 * The shape mirrors the server's `ExampleSentence` schema — `sentence`,
 * `sentence_vi`, `audio_url` — because that is what the dictionary API, the word
 * sense rows and the flashcard bodies all speak.
 */
export interface ExampleSentence {
  text: string;
  /** The learner's own language. Absent on content authored before it existed. */
  translation?: string;
  /** A recorded pronunciation of the sentence, if one is ever attached. */
  audioUrl?: string;
}

function trimmed(value: unknown): string | undefined {
  return typeof value === "string" && value.trim() !== ""
    ? value.trim()
    : undefined;
}

/**
 * Reads the example sentences out of an authored body or activity config.
 *
 * Three shapes are accepted because three exist, and a learner should never
 * lose their examples to a content version that predates a key:
 *
 *  1. `example_sentences: [{sentence, sentence_vi}]` — what the seed writes now,
 *     and the same objects `domain.ExampleSentence` defines.
 *  2. `example_sentences: ["…"]` — the first version of the list, English only.
 *  3. `example_sentence: "…"` — the single string that came before the list, and
 *     still the field the published OpenAPI examples show.
 *
 * Doing this once, here, is what keeps the fallback out of four rendering
 * components.
 */
export function readExampleSentences(
  source: Record<string, unknown> | null | undefined,
): ExampleSentence[] {
  if (!source) return [];

  const list = source["example_sentences"];
  if (Array.isArray(list)) {
    const sentences = list
      .map((entry): ExampleSentence | null => {
        // Shape 2: a bare string.
        const asString = trimmed(entry);
        if (asString) return { text: asString };

        // Shape 1: the object.
        if (entry === null || typeof entry !== "object") return null;
        const row = entry as Record<string, unknown>;
        const text = trimmed(row["sentence"]) ?? trimmed(row["text"]);
        if (!text) return null;

        const translation = trimmed(row["sentence_vi"]);
        const audioUrl = trimmed(row["audio_url"]);
        return {
          text,
          ...(translation !== undefined && { translation }),
          ...(audioUrl !== undefined && { audioUrl }),
        };
      })
      .filter((entry): entry is ExampleSentence => entry !== null);

    if (sentences.length > 0) return sentences;
  }

  // Shape 3.
  const single = trimmed(source["example_sentence"]);
  return single ? [{ text: single }] : [];
}

/**
 * Picks the example sentences one view of a flashcard shows.
 *
 * Always shuffled, never only when the sense is full. The enrichment job fills a
 * word to fifteen over several nights, so gating the shuffle on reaching fifteen
 * meant a word with five — which is every word for its first week, and every word
 * the model cannot extend — showed the same five in the same order for ever. That
 * is the thing this function exists to stop.
 *
 * A word with three or fewer keeps all of them; there is nothing to choose, and
 * shuffling three lines that are always all on screen only makes the card jitter.
 */
export function shuffleExamplesForReview(
  sentences: ExampleSentence[],
  limit = 3,
): ExampleSentence[] {
  if (sentences.length <= limit) return sentences;

  // Fisher-Yates over a copy: the caller's array belongs to the card content and
  // is read again on the next render.
  const copy = [...sentences];
  for (let i = copy.length - 1; i > 0; i--) {
    const j = Math.floor(Math.random() * (i + 1));
    const itemI = copy[i];
    const itemJ = copy[j];
    // Both indices are in range by construction; the guard is what
    // `noUncheckedIndexedAccess` wants to see rather than a real branch.
    if (itemI !== undefined && itemJ !== undefined) {
      copy[i] = itemJ;
      copy[j] = itemI;
    }
  }
  return copy.slice(0, limit);
}
