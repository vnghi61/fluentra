import { afterEach, describe, expect, it, vi } from "vitest";

import { detectLocale } from "@/i18n";

describe("the interface language before the learner chooses one", () => {
  afterEach(() => {
    localStorage.clear();
    vi.restoreAllMocks();
  });

  it("is Vietnamese, even in a browser set to English", () => {
    localStorage.clear();
    vi.spyOn(navigator, "language", "get").mockReturnValue("en-US");

    expect(detectLocale()).toBe("vi");
  });

  it("is the stored choice once there is one", () => {
    localStorage.setItem("fluentra.locale", "en");

    expect(detectLocale()).toBe("en");
  });
});
