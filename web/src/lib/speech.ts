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
}

/**
 * Speaks `text`, or reports failure.
 *
 * Returns false when synthesis is unavailable or threw, so a caller can show
 * the absence rather than appearing to do nothing.
 */
export function speakText(text: string, options: SpeakOptions = {}): boolean {
  const { lang = "en-US", onStart, onEnd, onFailure } = options;

  if (!text.trim() || !canSynthesise()) {
    onFailure?.();
    return false;
  }

  try {
    const synth = window.speechSynthesis;
    // Cancel only what is actually queued: queued utterances otherwise pile up
    // on repeated taps, but an unconditional cancel() right before speak() is
    // what makes Chrome on Android drop the new utterance.
    if (synth.speaking || synth.pending) synth.cancel();
    // Chrome on Android can leave synthesis paused after the tab was hidden,
    // and a paused synthesiser queues silently forever.
    synth.resume?.();

    const utterance = new SpeechSynthesisUtterance(text);
    utterance.lang = lang;
    const voice = voiceFor(lang);
    if (voice) utterance.voice = voice;
    // Slower than speech, because the point is to be copied.
    utterance.rate = 0.9;
    utterance.onend = () => onEnd?.();
    utterance.onerror = (event?: SpeechSynthesisErrorEvent) => {
      onEnd?.();
      // Only a missing voice or engine is worth reporting; an interrupted
      // utterance plays again on the next tap.
      if (!event?.error || !TRANSIENT_ERRORS.has(event.error)) {
        onFailure?.();
      }
    };

    onStart?.();
    synth.speak(utterance);
    return true;
  } catch {
    onEnd?.();
    onFailure?.();
    return false;
  }
}

/** Stops anything currently being spoken. */
export function cancelSpeech(): void {
  if (canSynthesise()) window.speechSynthesis.cancel();
}
