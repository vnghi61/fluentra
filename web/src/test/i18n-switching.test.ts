import { describe, expect, it, vi as vitest } from "vitest";

import i18n, { initI18n, setLocale } from "@/i18n";

/**
 * Switching language after the app has booted.
 *
 * This lives in its own file rather than beside the other bundle tests because
 * i18next is a process-wide singleton that survives `vi.resetModules()`: a
 * locale any earlier test fetched is still registered, so a test asserting that
 * `setLocale` fetches one would pass on someone else's fetch. Vitest isolates
 * files, and file isolation is the only boundary that actually holds here.
 *
 * What it guards: `setLocale` must load before it switches. Calling i18next's
 * `changeLanguage` on its own compiles, type-checks, and renders raw keys.
 */
describe("switching locale after boot", () => {
  it("fetches the bundle it switches to", async () => {
    localStorage.clear();
    await initI18n("en");
    expect(i18n.hasResourceBundle("vi", "translation")).toBe(false);

    setLocale("vi");

    // setLocale is deliberately not awaitable — it is called from click
    // handlers — so wait for the switch it schedules rather than for a promise.
    await vitest.waitFor(() => {
      expect(i18n.language).toBe("vi");
    });

    expect(i18n.t("dashboard.welcome")).toBe("Chào mừng đến với Fluentra");
    expect(localStorage.getItem("fluentra.locale")).toBe("vi");
  });
});
