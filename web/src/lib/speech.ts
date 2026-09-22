/**
 * Browser speech synthesis, in one place.
 *
 * This lived inside PronounceButton, which is a button with an icon. The
 * read-aloud diff needs the same thing without the button: the learner taps a
 * word they got wrong and hears it. Copying the logic across would have copied
 * the awkward parts with it — the Android quirks below are not obvious, and the
 * second copy is the one that would have missed the next fix.
 */

/**
 * Errors that say this utterance was cut off, not that speech is unavailable.
 * Chrome on Android reports the utterance it just queued as `interrupted` when
 * `cancel()` ran a moment before; treating that as failure disabled the button
 * on the first tap, which is why pronunciation "did not work" on phones.
 */
const TRANSIENT_ERRORS = new Set(["interrupted", "canceled", "not-allowed"]);

/** Whether this browser can synthesise speech at all. */
export function canSynthesise(): boolean {
  return (
    typeof window !== "undefined" &&
    "speechSynthesis" in window &&
    typeof window.SpeechSynthesisUtterance === "function"
  );
}

/**
 * An installed voice for the language. Android and iOS often have no voice
 * picked for a bare `lang`, and speak nothing (or report an error) unless one
 * is set. Voices load asynchronously, so an empty list just means "default".
 */
export function voiceFor(lang: string): SpeechSynthesisVoice | undefined {
  const voices = window.speechSynthesis.getVoices?.() ?? [];
  const wanted = lang.toLowerCase().replace("_", "-");
  const base = wanted.split("-")[0] ?? wanted;
  const normalise = (voice: SpeechSynthesisVoice) =>
    voice.lang.toLowerCase().replace("_", "-");
  return (
    voices.find((voice) => normalise(voice) === wanted) ??
    voices.find((voice) => normalise(voice).startsWith(`${base}-`))
  );
}

export interface SpeakOptions {
  lang?: string;
  onStart?: () => void;
  onEnd?: () => void;
  /** Called only for a failure worth reporting, never for an interruption. */
  onFailure?: () => void;
  /**
   * Called when the engine accepted the utterance but never started speaking,
   * even without a pinned voice. On Android this is the device, not the page:
   * media volume at zero, or no English voice installed for the TTS engine.
   */
  onSilent?: () => void;
}

/**
 * How long an utterance may sit without its `start` event before it is treated
 * as dropped. A single word has finished well inside this on every engine that
 * works, so waiting longer only delays the retry.
 */
export const START_TIMEOUT_MS = 2000;

/**
 * Utterances being spoken, held so Chrome cannot garbage-collect one mid-speech
 * — which loses its events and, on Android, sometimes its audio.
 */
const liveUtterances = new Set<SpeechSynthesisUtterance>();

/**
 * Bumped by every new request and every cancel. A retry scheduled for an older
 * request sees the change and gives up, so a card advanced mid-wait does not
 * hear the previous word two seconds later.
 */
let generation = 0;

/**
 * Speaks `text`, or reports failure.
 *
 * Chrome on Android can accept an utterance and then say nothing, with no
 * `error` event: the pinned voice is not installed, or belongs to another
 * engine. The icon pulsed and the phone stayed silent. So the first attempt is
 * watched for its `start` event; without one it is retried with no pinned
 * voice, letting the engine pick its own English, and if that is silent too
 * the caller is told, rather than left pulsing.
 *
 * Returns false when synthesis is unavailable or threw, so a caller can show
 * the absence rather than appearing to do nothing.
 */
export function speakText(text: string, options: SpeakOptions = {}): boolean {
  const { lang = "en-US", onStart, onEnd, onFailure, onSilent } = options;

  if (!text.trim() || !canSynthesise()) {
    onFailure?.();
    return false;
  }

  const synth = window.speechSynthesis;
  const request = ++generation;
  let current: SpeechSynthesisUtterance | null = null;
  let started = false;
  let settled = false;
  let watchdog: ReturnType<typeof setTimeout> | undefined;

  const settle = () => {
    settled = true;
    if (watchdog) clearTimeout(watchdog);
    onEnd?.();
  };

  const attempt = (pinVoice: boolean) => {
    const utterance = new SpeechSynthesisUtterance(text);
    utterance.lang = lang;
    const voice = pinVoice ? voiceFor(lang) : undefined;
    if (voice) utterance.voice = voice;
    // Slower than speech, because the point is to be copied.
    utterance.rate = 0.9;
    current = utterance;
    liveUtterances.add(utterance);
    const release = () => liveUtterances.delete(utterance);

    utterance.onstart = () => {
      if (utterance === current) started = true;
    };
    utterance.onend = () => {
      release();
      if (utterance === current && !settled) settle();
    };
    utterance.onerror = (event?: SpeechSynthesisErrorEvent) => {
      release();
      // An attempt this function replaced reports `interrupted`; it is not the
      // outcome of the tap.
      if (utterance !== current || settled) return;
      settle();
      // Only a missing voice or engine is worth reporting; an interrupted
      // utterance plays again on the next tap.
      if (!event?.error || !TRANSIENT_ERRORS.has(event.error)) {
        onFailure?.();
      }
    };

    synth.speak(utterance);

    watchdog = setTimeout(() => {
      if (started || settled || utterance !== current) return;
      if (request !== generation) {
        // Superseded or cancelled: end quietly, never speak the old text.
        settle();
        return;
      }
      if (pinVoice && voice) {
        synth.cancel();
        attempt(false);
        return;
      }
      synth.cancel();
      settle();
      onSilent?.();
    }, START_TIMEOUT_MS);
  };

  try {
    // Cancel only what is actually queued: queued utterances otherwise pile up
    // on repeated taps, but an unconditional cancel() right before speak() is
    // what makes Chrome on Android drop the new utterance.
    if (synth.speaking || synth.pending) synth.cancel();
    // Chrome on Android can leave synthesis paused after the tab was hidden,
    // and a paused synthesiser queues silently forever.
    synth.resume?.();

    onStart?.();
    attempt(true);
    return true;
  } catch {
    if (!settled) settle();
    onFailure?.();
    return false;
  }
}

/** Stops anything currently being spoken. */
export function cancelSpeech(): void {
  generation++;
  if (canSynthesise()) window.speechSynthesis.cancel();
}
