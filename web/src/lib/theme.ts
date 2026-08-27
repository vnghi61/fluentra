/** What is painted. */
export type Theme = "light" | "dark";

/**
 * What the learner chose. `system` is not a third palette — it is the absence
 * of a choice, which is why it is stored by removing the key rather than by
 * writing "system" into it. public/theme-init.js reads the same key and falls
 * back to `prefers-color-scheme`, so the two agree by construction.
 */
export type ThemeChoice = Theme | "system";

const STORAGE_KEY = "fluentra.theme";

/**
 * The theme is applied by public/theme-init.js before React mounts, so this
 * module only has to agree with it. The storage key is the contract between the
 * two; changing it here without changing that script reintroduces the flash of
 * the wrong colours on load.
 *
 * Every function here writes to the DOM first and to storage second. A private
 * window denies localStorage, and a learner who cannot be *remembered* should
 * still be obeyed for the session they are in.
 */
export function currentTheme(): Theme {
  return document.documentElement.classList.contains("dark") ? "dark" : "light";
}

/**
 * What the OS is asking for, when the learner has asked for nothing.
 *
 * `matchMedia` is optional here rather than assumed. jsdom does not implement
 * it, and a theme helper that throws in a test environment is a theme helper
 * that throws — the settings form went red the moment it started calling this,
 * and the fault was here, not in the test. Light is the documented default.
 */
export function systemTheme(): Theme {
  const media = window.matchMedia?.("(prefers-color-scheme: dark)");
  return media?.matches ? "dark" : "light";
}

/** The stored choice, or `system` when there is none. */
export function currentChoice(): ThemeChoice {
  try {
    const stored = localStorage.getItem(STORAGE_KEY);
    if (stored === "light" || stored === "dark") return stored;
  } catch {
    // Private mode: unreadable is indistinguishable from unset, and both mean
    // the same thing here — no explicit choice.
  }
  return "system";
}

/** Paints `choice`, resolving `system` against the OS, and remembers it. */
export function applyThemeChoice(choice: ThemeChoice): Theme {
  const painted = choice === "system" ? systemTheme() : choice;
  document.documentElement.classList.toggle("dark", painted === "dark");
  try {
    if (choice === "system") localStorage.removeItem(STORAGE_KEY);
    else localStorage.setItem(STORAGE_KEY, choice);
  } catch {
    // The choice applies to this session and is not remembered.
  }
  return painted;
}

/**
 * Keeps `system` honest after the first paint.
 *
 * theme-init.js resolves the OS preference once, at load. Someone who flips
 * their laptop to dark at sunset while the tab is open has changed the answer,
 * and an app that only reads it at boot is wrong until reloaded. Returns its
 * own unsubscribe.
 */
export function watchSystemTheme(): () => void {
  const media = window.matchMedia?.("(prefers-color-scheme: dark)");
  if (!media?.addEventListener) return () => undefined;
  const onChange = () => {
    if (currentChoice() === "system") applyThemeChoice("system");
  };
  media.addEventListener("change", onChange);
  return () => media.removeEventListener("change", onChange);
}
