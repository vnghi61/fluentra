import type { ReviewGrade } from "../api/reviewApi";

/** The number keys that grade a revealed card, in the order the buttons show. */
export const GRADE_SHORTCUTS: Record<string, ReviewGrade> = {
  "1": "again",
  "2": "hard",
  "3": "good",
  "4": "easy",
};

/**
 * Whether a key press belongs to something other than the review session.
 *
 * The session listens on `window`, so without this a digit typed into the
 * report dialog's note would grade the card behind it, Ctrl+1 (switch tab)
 * would grade "again", and Space on the focused pronounce button would flip the
 * card instead of speaking the word. A focused control keeps its own keys.
 */
export function isForeignKeyEvent(event: KeyboardEvent): boolean {
  if (event.defaultPrevented || event.repeat) return true;
  if (event.ctrlKey || event.metaKey || event.altKey) return true;

  const target = event.target;
  if (!(target instanceof Element)) return false;
  if (
    target instanceof HTMLInputElement ||
    target instanceof HTMLTextAreaElement ||
    target instanceof HTMLSelectElement ||
    (target instanceof HTMLElement && target.isContentEditable)
  ) {
    return true;
  }
  return target.closest('[role="dialog"], [role="alertdialog"]') !== null;
}

/**
 * Whether the focused element answers Space/Enter itself.
 *
 * Only the reveal keys need this: a digit means nothing to a button, so the
 * grade shortcuts still work while a grade button has focus.
 */
export function isActivationTarget(event: KeyboardEvent): boolean {
  const target = event.target;
  if (!(target instanceof Element)) return false;
  return (
    target.closest(
      'button, a[href], summary, [role="button"], [role="link"]',
    ) !== null
  );
}
