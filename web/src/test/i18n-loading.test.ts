import { beforeEach, describe, expect, it, vi as vitest } from "vitest";

/**
 * Only the locale being read ships with the page.
 *
 * Both bundles used to be static imports in the entry chunk, so every visitor
 * downloaded both languages and read one. Splitting them means the translations
 * now arrive over a fetch, and a fetch is something that can be forgotten: the
 * failure mode is an interface rendering `dashboard.welcome` instead of a
 * sentence, which no type checker and no build step can see.
 *
 * Every test loads the module afresh. i18next is a singleton, so a locale one
 * test fetched stays registered for the next one — sharing it would let a test
 * pass on a bundle some earlier test happened to load, which is precisely the
 * bug being guarded against.
 */
async function freshI18n(): Promise<typeof import("@/i18n")> {
  vitest.resetModules();
  return import("@/i18n");
}

describe("locale bundles", () => {
  beforeEach(() => {
    localStorage.clear();
  });

  it("boots straight into Vietnamese without a second step", async () => {
    const { default: i18n, initI18n } = await freshI18n();

    await initI18n("vi");

    expect(i18n.language).toBe("vi");
    expect(i18n.t("dashboard.welcome")).toBe("Chào mừng đến với Fluentra");
  });

  it("ships one language, not both", async () => {
    const { default: i18n, initI18n, loadLocale } = await freshI18n();

    await initI18n("vi");
    expect(i18n.hasResourceBundle("en", "translation")).toBe(false);

    await loadLocale("en");
    await i18n.changeLanguage("en");

    expect(i18n.t("dashboard.welcome")).toBe("Welcome to Fluentra");
  });
});
