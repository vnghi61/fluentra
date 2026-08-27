import { useCallback, useEffect } from "react";

import { currentLocale, isLocale, setLocale, type Locale } from "@/i18n";
import {
  applyThemeChoice,
  currentChoice,
  watchSystemTheme,
  type ThemeChoice,
} from "@/lib/theme";
import {
  usePreferencesStore,
  type Preferences,
} from "@/stores/preferencesStore";
import { accountApi } from "../api/accountApi";

function isThemeChoice(value: string | null | undefined): value is ThemeChoice {
  return value === "light" || value === "dark" || value === "system";
}

export interface PreferencesSync {
  themeChoice: ThemeChoice;
  locale: Locale;
  setThemeChoice: (choice: ThemeChoice) => void;
  setLocaleChoice: (locale: Locale) => void;
}

/**
 * Makes the stored preferences the ones the learner actually sees, and keeps a
 * change in either direction in step with the other.
 *
 * Until this existed, saving the settings form wrote `theme` and `locale` to the
 * database and changed nothing on screen — `applyTheme` and `setLocale` were
 * both written and both had no caller anywhere in the app. Because nothing ever
 * wrote the storage key, `theme-init.js` always fell through to
 * `prefers-color-scheme`, and the locale always fell through to
 * `navigator.language`. There was no way, anywhere in the product, to choose
 * either one; the settings screen only looked like there was.
 *
 * Precedence: for a signed-in learner the server wins, because that is the
 * answer to "I set dark on my phone and opened my laptop". The local value is
 * the fallback for a visitor who has no row yet, and is written on every change
 * so the *next* boot paints correctly on the first frame instead of flipping
 * once the request lands.
 */
export function usePreferencesSync(signedIn: boolean): PreferencesSync {
  const preferences = usePreferencesStore((s) => s.preferences);
  const setStored = usePreferencesStore((s) => s.set);
  const clearStored = usePreferencesStore((s) => s.clear);

  // `system` is a live choice, not a snapshot: honour the OS changing while
  // the tab is open.
  useEffect(() => watchSystemTheme(), []);

  useEffect(() => {
    if (!signedIn) {
      clearStored();
      return;
    }
    let cancelled = false;
    void (async () => {
      try {
        const loaded = await accountApi.getPreferences();
        if (cancelled) return;
        setStored(loaded);
        if (isThemeChoice(loaded.theme)) applyThemeChoice(loaded.theme);
        if (isLocale(loaded.locale)) setLocale(loaded.locale);
      } catch {
        // A preferences read that fails is not a reason to block the app: the
        // locally stored choice is already painted, and it is a good answer.
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [signedIn, setStored, clearStored]);

  /** Applies locally first, then tells the server — for a signed-in learner. */
  const persist = useCallback(
    (patch: Partial<Pick<Preferences, "theme" | "locale">>) => {
      const current = usePreferencesStore.getState().preferences;
      if (!current) return; // signed out: the local choice is the whole story
      const next = { ...current, ...patch };
      setStored(next);
      void accountApi
        .replacePreferences({
          locale: next.locale,
          theme: next.theme,
          daily_goal_minutes: next.daily_goal_minutes,
          notification_channels: next.notification_channels,
          quiet_hours: next.quiet_hours ?? null,
          ai_processing_opt_out: next.ai_processing_opt_out,
        })
        .catch(() => {
          // The learner sees the change either way. Losing the round trip costs
          // them the setting on their next device, not on this one.
        });
    },
    [setStored],
  );

  const setThemeChoice = useCallback(
    (choice: ThemeChoice) => {
      applyThemeChoice(choice);
      persist({ theme: choice });
    },
    [persist],
  );

  const setLocaleChoice = useCallback(
    (locale: Locale) => {
      setLocale(locale);
      persist({ locale });
    },
    [persist],
  );

  const themeChoice: ThemeChoice = isThemeChoice(preferences?.theme)
    ? preferences.theme
    : currentChoice();
  const locale: Locale = isLocale(preferences?.locale)
    ? preferences.locale
    : currentLocale();

  return { themeChoice, locale, setThemeChoice, setLocaleChoice };
}
