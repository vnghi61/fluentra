export type Theme = "light" | "dark";

const STORAGE_KEY = "fluentra.theme";

/**
 * The theme is applied by an inline script in index.html before React mounts,
 * so this module only has to agree with it. The storage key is the contract
 * between the two; changing it here without changing index.html reintroduces
 * the flash of the wrong colours on load.
 */
export function currentTheme(): Theme {
  return document.documentElement.classList.contains("dark") ? "dark" : "light";
}

export function applyTheme(theme: Theme): void {
  document.documentElement.classList.toggle("dark", theme === "dark");
  try {
    localStorage.setItem(STORAGE_KEY, theme);
  } catch {
    // Private mode: the choice applies to this session and is not remembered.
  }
}

export function toggleTheme(): Theme {
  const next: Theme = currentTheme() === "dark" ? "light" : "dark";
  applyTheme(next);
  return next;
}
