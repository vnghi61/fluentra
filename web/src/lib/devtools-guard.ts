/**
 * Ignores the keyboard shortcuts that open the browser's developer tools and
 * page source: F12, Ctrl/Cmd+Shift+I/J/C and Ctrl/Cmd+U (Cmd+Alt+I/J/C on a
 * Mac). Production builds only.
 *
 * This is a deterrent, not a security boundary, and nothing may rely on it.
 * The browser menu still opens DevTools, and every request the page makes can
 * be replayed with curl. What actually protects the product is on the server:
 * answer keys are redacted before a learner sees an item, grading happens
 * server-side, exam deadlines are server clocks and paid content sits behind
 * the paywall check. Deliberately not done here: blocking the context menu
 * (it breaks copy, paste, spellcheck and translate) and "DevTools open"
 * detection (window-size and `debugger` tricks misfire on docked windows,
 * zoom and phones, and cost every learner CPU).
 */
export function installDevtoolsGuard(): () => void {
  const onKeyDown = (event: KeyboardEvent) => {
    const key = event.key.toLowerCase();
    const mod = event.ctrlKey || event.metaKey;
    const blocked =
      key === "f12" ||
      (mod && event.shiftKey && ["i", "j", "c"].includes(key)) ||
      (event.metaKey && event.altKey && ["i", "j", "c"].includes(key)) ||
      (mod && !event.shiftKey && key === "u");
    if (blocked) {
      event.preventDefault();
      event.stopPropagation();
    }
  };
  window.addEventListener("keydown", onKeyDown, { capture: true });
  return () =>
    window.removeEventListener("keydown", onKeyDown, { capture: true });
}
