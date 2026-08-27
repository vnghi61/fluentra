import i18n from "i18next";
import { initReactI18next } from "react-i18next";

import en from "./en.json";
import vi from "./vi.json";

/**
 * The two locales the product ships. `vi` is not a courtesy translation: the
 * learners are Vietnamese speakers learning English, so it is the language the
 * interface is read in while the content is in English.
 */
export const SUPPORTED_LOCALES = ["en", "vi"] as const;
export type Locale = (typeof SUPPORTED_LOCALES)[number];

export const DEFAULT_LOCALE: Locale = "en";

const STORAGE_KEY = "fluentra.locale";

export function isLocale(value: string | null | undefined): value is Locale {
  return (
    value !== null &&
    value !== undefined &&
    SUPPORTED_LOCALES.includes(value as Locale)
  );
}

/** Stored choice first, then the browser's preference, then English. */
export function detectLocale(): Locale {
  try {
    const stored = localStorage.getItem(STORAGE_KEY);
    if (isLocale(stored)) return stored;
  } catch {
    // Private mode denies localStorage; fall through to the browser language.
  }
  const browser = navigator.language.split("-")[0];
  return isLocale(browser) ? browser : DEFAULT_LOCALE;
}

/** The locale i18next is actually running in, not the one on <html lang>. */
export function currentLocale(): Locale {
  return isLocale(i18n.language) ? i18n.language : DEFAULT_LOCALE;
}

export function setLocale(locale: Locale): void {
  try {
    localStorage.setItem(STORAGE_KEY, locale);
  } catch {
    // Not being able to remember the choice is not a reason to refuse it.
  }
  // Remember first, switch second, and only once i18next can switch. Calling
  // changeLanguage before init throws from inside i18next
  // (`hasLanguageSomeTranslations` of undefined), which a caller would surface
  // as "failed to save" for a save that in fact succeeded. The stored value is
  // not lost either way: initI18n reads it through detectLocale.
  if (i18n.isInitialized) void i18n.changeLanguage(locale);
}

export function initI18n(locale: Locale = detectLocale()): typeof i18n {
  if (!i18n.isInitialized) {
    void i18n.use(initReactI18next).init({
      resources: {
        en: { translation: en },
        vi: { translation: vi },
      },
      lng: locale,
      fallbackLng: DEFAULT_LOCALE,
      interpolation: { escapeValue: false },
      // Missing keys must be loud in development and harmless in production.
      returnEmptyString: false,
    });
  }
  return i18n;
}

export default i18n;
