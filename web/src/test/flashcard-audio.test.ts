import { describe, expect, it } from "vitest";

import { flashcardContent } from "@/features/review/model/flashcard";
import type { ReviewCard } from "@/features/review";

/**
 * The recording and its credit are one unit.
 *
 * The dictionary's recordings are mostly CC BY-SA, and a licence that requires
 * attribution is not satisfied by playing the file. A body that names a file
 * but not the page crediting it must therefore reach the card with no audio at
 * all — synthesis is the honest fallback, an uncredited recording is a breach.
 */
function card(body: Record<string, unknown>): ReviewCard {
  return {
    id: "0199a1c2-3d4e-7f80-9abc-def012345678",
    user_id: "0199a1c2-3d4e-7f80-9abc-def012345679",
    content_version_id: "0199a1c2-3d4e-7f80-9abc-def01234567a",
    skill: "vocabulary",
    stability: 1,
    difficulty: 5,
    due_at: "2026-09-22T10:00:00Z",
    reps: 0,
    lapses: 0,
    state: "new",
    content: { kind: "vocab_flashcard", body },
  } as ReviewCard;
}

describe("flashcardContent audio", () => {
  it("carries a credited recording through to the card", () => {
    const content = flashcardContent(
      card({
        word: "eat",
        definition: "Put food into the mouth.",
        audio_url: "https://api.dictionaryapi.dev/media/pronunciations/en/eat-us.mp3",
        audio_attribution: "https://commons.wikimedia.org/wiki/File:En-us-eat.ogg",
        audio_licence: "BY-SA 3.0",
      }),
    );

    expect(content?.audioUrl).toBe(
      "https://api.dictionaryapi.dev/media/pronunciations/en/eat-us.mp3",
    );
    expect(content?.audioAttribution).toBe(
      "https://commons.wikimedia.org/wiki/File:En-us-eat.ogg",
    );
    expect(content?.audioLicence).toBe("BY-SA 3.0");
  });

  it("drops a recording whose credit is missing", () => {
    const content = flashcardContent(
      card({
        word: "eat",
        definition: "Put food into the mouth.",
        audio_url: "https://api.dictionaryapi.dev/media/pronunciations/en/eat-us.mp3",
      }),
    );

    expect(content?.audioUrl).toBeUndefined();
  });
});
