/**
 * A human recording of a word, for when the device cannot speak it.
 *
 * Speech synthesis is the device's: on some phones it is simply unreachable
 * (a Xiaomi whose OS stops Chrome binding "Speech Services by Google" stays
 * silent with the volume up and an English voice installed). A recording plays
 * through a plain <audio> element and needs none of that. It is only a
 * fallback — the content's own `audio_url` comes first and synthesis second —
 * so a third party is asked only after the device has already failed.
 *
 * Two sources, in order:
 *  1. api.dictionaryapi.dev, which returns Wikimedia Commons recordings with
 *     their licence and source page. It is sometimes down (522s and timeouts).
 *  2. Wikimedia Commons directly, by its naming convention `En-us-<word>.ogg`.
 *     Most of the dictionary's files are these, so this keeps working when the
 *     dictionary does not.
 *
 * The recordings are mostly CC BY-SA, which is not satisfied by playing the
 * file: every recording carries the page that credits it.
 */

export interface Recording {
  url: string;
  /** The page that credits the recording's author and licence. */
  creditUrl: string;
  /** A short licence label, when the source names one. */
  licence?: string | undefined;
}

/**
 * What to call the source that must be credited.
 *
 * Wikimedia Commons is named rather than shown as a hostname because that is
 * where these files live and what a learner recognises; anything else is
 * labelled by its host, which is still an honest credit and better than
 * claiming Commons for a file that is not there.
 */
export function creditLabel(recording: Recording): string {
  try {
    const host = new URL(recording.creditUrl).hostname.replace(/^www\./, "");
    if (host === "commons.wikimedia.org" || host.endsWith(".wikimedia.org")) {
      return "Wikimedia Commons";
    }
    return host;
  } catch {
    return "Wikimedia Commons";
  }
}

const DICTIONARY_URL = "https://api.dictionaryapi.dev/api/v2/entries/en/";
const COMMONS = "https://commons.wikimedia.org/wiki/";
const LOOKUP_TIMEOUT_MS = 4000;

/** Only single words have recordings; a sentence never will. */
const SINGLE_WORD = /^[a-z][a-z'-]*$/i;

export function isSingleWord(text: string): boolean {
  return SINGLE_WORD.test(text.trim());
}

interface DictionaryPhonetic {
  audio?: string;
  sourceUrl?: string;
  license?: { name?: string };
}

/** US first, then UK, then anything else: the course teaches American English. */
function accentRank(url: string): number {
  if (/-us\.(mp3|ogg)$/i.test(url)) return 0;
  if (/-uk\.(mp3|ogg)$/i.test(url)) return 1;
  return 2;
}

async function dictionaryRecordings(word: string): Promise<Recording[]> {
  const controller = new AbortController();
  const timer = setTimeout(() => controller.abort(), LOOKUP_TIMEOUT_MS);
  try {
    const response = await fetch(DICTIONARY_URL + encodeURIComponent(word), {
      signal: controller.signal,
    });
    if (!response.ok) return [];
    const entries = (await response.json()) as {
      phonetics?: DictionaryPhonetic[];
    }[];
    const recordings: Recording[] = [];
    for (const entry of Array.isArray(entries) ? entries : []) {
      for (const phonetic of entry.phonetics ?? []) {
        const url = phonetic.audio?.trim();
        if (!url?.startsWith("https://")) continue;
        recordings.push({
          url,
          // A recording with no source page cannot be credited, so it is
          // credited to Commons, where this API's files come from.
          creditUrl: phonetic.sourceUrl ?? `${COMMONS}Commons:Licensing`,
          licence: phonetic.license?.name,
        });
      }
    }
    return recordings.sort((a, b) => accentRank(a.url) - accentRank(b.url));
  } catch {
    return [];
  } finally {
    clearTimeout(timer);
  }
}

function commonsRecordings(word: string): Recording[] {
  const lemma = word.toLowerCase();
  return ["En-us", "En-uk"].map((prefix) => {
    const file = `${prefix}-${lemma}.ogg`;
    return {
      url: `${COMMONS}Special:FilePath/${encodeURIComponent(file)}`,
      creditUrl: `${COMMONS}File:${encodeURIComponent(file)}`,
    };
  });
}

const found = new Map<string, Promise<Recording[]>>();

/** Candidate recordings for a word, best first. Looked up once per word. */
export function findRecordings(word: string): Promise<Recording[]> {
  const key = word.trim().toLowerCase();
  let pending = found.get(key);
  if (!pending) {
    pending = dictionaryRecordings(key).then((fromDictionary) => {
      const seen = new Set<string>();
      return [...fromDictionary, ...commonsRecordings(key)].filter((r) => {
        if (seen.has(r.url)) return false;
        seen.add(r.url);
        return true;
      });
    });
    found.set(key, pending);
  }
  return pending;
}

export interface PlayHandlers {
  onPlaying: (recording: Recording) => void;
  onEnded: () => void;
  /** Every candidate failed to load or play. */
  onExhausted: () => void;
}

/**
 * Plays the first recording that loads, moving to the next on any error.
 * Returns a stop function; after it is called no handler fires.
 */
export function playFirstRecording(
  recordings: Recording[],
  handlers: PlayHandlers,
): () => void {
  let stopped = false;
  let audio: HTMLAudioElement | null = null;

  const tryAt = (index: number) => {
    if (stopped) return;
    const recording = recordings[index];
    if (!recording) {
      handlers.onExhausted();
      return;
    }
    const next = () => {
      if (audio === candidate) tryAt(index + 1);
    };
    const candidate = new Audio(recording.url);
    audio = candidate;
    candidate.onplaying = () => {
      if (!stopped && audio === candidate) handlers.onPlaying(recording);
    };
    candidate.onended = () => {
      if (!stopped && audio === candidate) handlers.onEnded();
    };
    candidate.onerror = next;
    candidate.play().catch(next);
  };

  tryAt(0);
  return () => {
    stopped = true;
    audio?.pause();
    audio = null;
  };
}
