import type { ReviewCard } from "../api/reviewApi";

/**
 * The fields a vocabulary flashcard renders, read out of a card's authored body.
 *
 * This is not a hand-written response type. `ReviewCardContent.body` is
 * `{[key: string]: unknown}` in the spec on purpose — the body's shape belongs to
 * whichever skill module authored the version, and the OpenAPI document does not
 * describe every kind. Narrowing it is therefore the client's job, and doing it
 * here in one place, defensively, is what keeps the rendering components free of
 * casts.
 *
 * Nothing is defaulted. A body missing the word or the definition is a content
 * fault, and the screen says so rather than filling the hole — the first version
 * of this screen substituted a hard-coded "meticulous", and every card in every
 * learner's queue displayed it.
 */
export interface FlashcardContent {
  /** Required: a body without a word has no front, and yields null instead. */
  word: string;
  /** Required: a body without a definition has no back, and yields null instead. */
  definition: string;
  pos?: string;
  ipa?: string;
  audioUrl?: string;
  definitionVi?: string;
  exampleSentence?: string;
}

function str(body: Record<string, unknown>, key: string): string | undefined {
  const value = body[key];
  return typeof value === "string" && value.trim() !== "" ? value : undefined;
}

/** Reads the renderable fields out of a card, or null when the card has none. */
export function flashcardContent(card: ReviewCard): FlashcardContent | null {
  const content = card.content;
  if (!content) return null;

  const body = content.body;
  const word = str(body, "word");
  const definition = str(body, "definition");

  // A card missing either face is not a flashcard. Returning null sends the
  // screen to its explicit "content unavailable" state, which is the honest
  // rendering of an unauthored or half-authored version.
  if (!word || !definition) return null;

  const pos = str(body, "pos");
  const ipa = str(body, "ipa");
  const audioUrl = str(body, "audio_url");
  const definitionVi = str(body, "definition_vi");
  const exampleSentence = str(body, "example_sentence");

  return {
    word,
    definition,
    ...(pos !== undefined && { pos }),
    ...(ipa !== undefined && { ipa }),
    ...(audioUrl !== undefined && { audioUrl }),
    ...(definitionVi !== undefined && { definitionVi }),
    ...(exampleSentence !== undefined && { exampleSentence }),
  };
}
