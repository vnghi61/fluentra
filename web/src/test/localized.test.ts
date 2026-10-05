import { describe, expect, it } from "vitest";

import { pickLocalized, prefersVietnamese } from "@/lib/localized";

describe("prefersVietnamese", () => {
  it('prefersVietnamese("VI") true, ("en-US") false, (undefined) false', () => {
    expect(prefersVietnamese("VI")).toBe(true);
    expect(prefersVietnamese("en-US")).toBe(false);
    expect(prefersVietnamese(undefined)).toBe(false);
  });

  it("handles other variations and null", () => {
    expect(prefersVietnamese("vi")).toBe(true);
    expect(prefersVietnamese("vi-VN")).toBe(true);
    expect(prefersVietnamese(null)).toBe(false);
  });
});

describe("pickLocalized", () => {
  it("vi picks vi", () => {
    expect(pickLocalized("English feedback", "Phản hồi tiếng Việt", "vi")).toBe(
      "Phản hồi tiếng Việt",
    );
  });

  it("en picks en", () => {
    expect(pickLocalized("English feedback", "Phản hồi tiếng Việt", "en")).toBe(
      "English feedback",
    );
  });

  it('"vi-VN" counts as vi', () => {
    expect(
      pickLocalized("English feedback", "Phản hồi tiếng Việt", "vi-VN"),
    ).toBe("Phản hồi tiếng Việt");
  });

  it("vi empty falls back to en", () => {
    expect(pickLocalized("English feedback", "", "vi")).toBe(
      "English feedback",
    );
    expect(pickLocalized("English feedback", null, "vi")).toBe(
      "English feedback",
    );
    expect(pickLocalized("English feedback", undefined, "vi")).toBe(
      "English feedback",
    );
  });

  it("en empty falls back to vi", () => {
    expect(pickLocalized("", "Phản hồi tiếng Việt", "en")).toBe(
      "Phản hồi tiếng Việt",
    );
    expect(pickLocalized(null, "Phản hồi tiếng Việt", "en")).toBe(
      "Phản hồi tiếng Việt",
    );
    expect(pickLocalized(undefined, "Phản hồi tiếng Việt", "en")).toBe(
      "Phản hồi tiếng Việt",
    );
  });

  it("whitespace-only counts as empty", () => {
    expect(pickLocalized("English feedback", "   ", "vi")).toBe(
      "English feedback",
    );
    expect(pickLocalized("   ", "Phản hồi tiếng Việt", "en")).toBe(
      "Phản hồi tiếng Việt",
    );
    expect(pickLocalized("   ", "   ", "vi")).toBe("");
  });

  it('both empty/null -> ""', () => {
    expect(pickLocalized("", "", "vi")).toBe("");
    expect(pickLocalized(null, null, "vi")).toBe("");
    expect(pickLocalized(undefined, undefined, "en")).toBe("");
    expect(pickLocalized("", null, "vi")).toBe("");
    expect(pickLocalized(null, "", "en")).toBe("");
  });

  it("undefined language -> en", () => {
    expect(
      pickLocalized("English feedback", "Phản hồi tiếng Việt", undefined),
    ).toBe("English feedback");
    expect(pickLocalized("", "Phản hồi tiếng Việt", undefined)).toBe(
      "Phản hồi tiếng Việt",
    );
  });
});
