import { readdirSync, readFileSync } from "node:fs";
import { join } from "node:path";

import { describe, expect, it } from "vitest";

import en from "@/i18n/en.json";
import vi from "@/i18n/vi.json";

/**
 * i18next's second argument to `t()` is a default, not a fallback for a missing
 * translation: `t("admin.foo", "Save")` returns "Save" in every locale until
 * `admin.foo` exists, so a Vietnamese learner reads English and nothing warns.
 *
 * Four admin screens shipped that way. These tests are the reason a fifth
 * cannot: every key a component asks for must exist in both locales, and the
 * two locales must carry the same keys as each other.
 */

const SOURCE_ROOT = join(process.cwd(), "src");

function sourceFiles(dir: string): string[] {
  const found: string[] = [];
  for (const entry of readdirSync(dir, { withFileTypes: true })) {
    const full = join(dir, entry.name);
    if (entry.isDirectory()) {
      found.push(...sourceFiles(full));
    } else if (/\.tsx?$/.test(entry.name)) {
      found.push(full);
    }
  }
  return found;
}

function flatten(value: unknown, prefix = ""): string[] {
  if (typeof value !== "object" || value === null) return [prefix];
  return Object.entries(value).flatMap(([key, child]) =>
    flatten(child, prefix ? `${prefix}.${key}` : key),
  );
}

function resolve(bundle: unknown, key: string): unknown {
  return key
    .split(".")
    .reduce<unknown>(
      (node, part) =>
        typeof node === "object" && node !== null
          ? (node as Record<string, unknown>)[part]
          : undefined,
      bundle,
    );
}

/**
 * Literal `t("some.key"` calls. A key built at runtime cannot be checked
 * statically and is deliberately out of scope; the point is to catch the
 * ordinary case, which is every case in this codebase today.
 */
const T_CALL = /\bt\(\s*"([a-zA-Z][\w.]*)"/g;

function usedKeys(): Map<string, Set<string>> {
  const uses = new Map<string, Set<string>>();
  for (const file of sourceFiles(SOURCE_ROOT)) {
    // The test files themselves may name keys that do not exist, on purpose.
    if (file.includes(`${join("src", "test")}`)) continue;
    const source = readFileSync(file, "utf8");
    for (const match of source.matchAll(T_CALL)) {
      const key = match[1];
      if (key === undefined) continue;
      const where = uses.get(key) ?? new Set<string>();
      where.add(file.slice(SOURCE_ROOT.length + 1));
      uses.set(key, where);
    }
  }
  return uses;
}

describe("translation keys", () => {
  it("every key a component asks for exists in English", () => {
    const missing = [...usedKeys().entries()]
      .filter(([key]) => resolve(en, key) === undefined)
      .map(([key, files]) => `${key} (${[...files].join(", ")})`);

    expect(missing).toEqual([]);
  });

  it("every key a component asks for exists in Vietnamese", () => {
    // Separate from the English case on purpose. A key present in en.json and
    // absent from vi.json is the failure that renders English to the learners
    // this product is for, and it should name itself rather than hide behind a
    // combined assertion.
    const missing = [...usedKeys().entries()]
      .filter(([key]) => resolve(vi, key) === undefined)
      .map(([key, files]) => `${key} (${[...files].join(", ")})`);

    expect(missing).toEqual([]);
  });

  it("the two locales carry exactly the same keys", () => {
    const enKeys = flatten(en).sort();
    const viKeys = flatten(vi).sort();

    expect(enKeys.filter((k) => !viKeys.includes(k))).toEqual([]);
    expect(viKeys.filter((k) => !enKeys.includes(k))).toEqual([]);
  });
});

// Not asserted here: that no call site passes an inline English default. It is
// tempting, because a default is what let these keys go missing for so long
// without anyone noticing. But once the three tests above hold, a default is
// unreachable — i18next prefers the resource, and the resource is now required
// to exist in both locales. Banning the second argument would be a style rule
// over thirty existing files, and it would not catch anything these do not.
