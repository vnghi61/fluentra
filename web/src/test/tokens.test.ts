import { readFileSync } from "node:fs";
import { join } from "node:path";
import { describe, expect, it } from "vitest";

// `css: false` in vitest.config means the browser never applies index.css, so
// computed styles read only what a test itself wrote — a self-fulfilling check.
// To test the real design tokens we read the source file from disk (tests run
// with `web/` as the working directory) and assert on the actual declared
// values, including the WCAG AA contrast of text on surface.
const indexCss = readFileSync(join(process.cwd(), "src", "index.css"), "utf8")
  // Comments explain the values and sometimes name a token; stripping them
  // first keeps prose out of the declaration parser.
  .replace(/\/\*[\s\S]*?\*\//g, "");

/** Extract the body of the first block whose opening line starts with `selector`. */
function block(source: string, selector: string): string {
  const lines = source.split("\n");
  const start = lines.findIndex((l) => l.trimStart().startsWith(selector));
  if (start === -1) return "";
  const open = lines.findIndex((_, i) => {
    const line = lines[i];
    return i >= start && line !== undefined && line.includes("{");
  });
  const end = lines.findIndex((_, i) => {
    const line = lines[i];
    return i > open && line !== undefined && line.includes("}");
  });
  return lines.slice(open, end).join("\n");
}

/** Split "--name: value" custom-property declarations out of a block. */
function customProperties(source: string): Map<string, string> {
  const map = new Map<string, string>();
  for (const match of source.matchAll(/--([a-z0-9-]+)\s*:\s*([^;]+);/gi)) {
    const name = match[1];
    const value = match[2];
    if (name !== undefined && value !== undefined) {
      map.set(name, value.trim());
    }
  }
  return map;
}

function toRgb(hex: string): [number, number, number] {
  const value = hex.replace("#", "");
  const full =
    value.length === 3
      ? value
          .split("")
          .map((c) => c + c)
          .join("")
      : value;
  const int = parseInt(full, 16);
  return [(int >> 16) & 255, (int >> 8) & 255, int & 255];
}

function luminance(hex: string): number {
  const rgb = toRgb(hex);
  const sr = rgb[0] / 255;
  const sg = rgb[1] / 255;
  const sb = rgb[2] / 255;
  const linear = (c: number) =>
    c <= 0.03928 ? c / 12.92 : Math.pow((c + 0.055) / 1.055, 2.4);
  return 0.2126 * linear(sr) + 0.7152 * linear(sg) + 0.0722 * linear(sb);
}

function contrast(a: string, b: string): number {
  const la = luminance(a);
  const lb = luminance(b);
  const hi = Math.max(la, lb);
  const lo = Math.min(la, lb);
  return (hi + 0.05) / (lo + 0.05);
}

describe("Design System Tokens (P6.1)", () => {
  const theme = customProperties(block(indexCss, "@theme"));
  const light = customProperties(block(indexCss, ":root"));
  const dark = customProperties(block(indexCss, ".dark"));

  /**
   * Resolve a `--color-*` token to a literal hex for one mode.
   *
   * A token is either a constant (`--color-primary: #2563eb`) or a redirect to
   * a per-mode variable (`--color-surface: var(--surface)`). Resolving through
   * the redirect is what makes the contrast assertions below test the value a
   * learner actually sees, rather than the string sitting in `@theme`.
   */
  function resolve(token: string, vars: Map<string, string>): string {
    const declared = theme.get(token);
    expect(declared, `@theme must declare --${token}`).toBeDefined();
    const redirect = /^var\(--([a-z0-9-]+)\)$/.exec(declared!);
    if (redirect === null) return declared!;
    const target = redirect[1]!;
    const value = vars.get(target);
    expect(
      value,
      `--${token} redirects to --${target}, which this mode does not define`,
    ).toBeDefined();
    return value!;
  }

  const modes = [
    ["light", light],
    ["dark", dark],
  ] as const;

  it("declares every semantic token in the @theme block", () => {
    for (const name of [
      "color-primary",
      "color-primary-fg",
      "color-primary-hover",
      "color-primary-accent",
      "color-surface",
      "color-surface-muted",
      "color-surface-card",
      "color-border",
      "color-text",
      "color-text-muted",
      "color-success",
      "color-warning",
      "color-danger",
    ]) {
      expect(theme.has(name), `@theme must declare --${name}`).toBe(true);
    }
  });

  it("wires surface/text/border tokens to distinct light and dark values", () => {
    for (const name of [
      "surface",
      "surface-muted",
      "surface-card",
      "border",
      "text",
      "text-muted",
      "primary-accent",
    ]) {
      expect(light.has(name), `:root must define --${name}`).toBe(true);
      expect(dark.has(name), `.dark must define --${name}`).toBe(true);
      expect(light.get(name), `--${name} must differ between modes`).not.toBe(
        dark.get(name),
      );
    }
  });

  it("routes every themed token through a per-mode variable", () => {
    // Tailwind v4 emits `--color-surface: var(--surface)` into :root, and the
    // `.dark` class sits on <html> — the same element — so the redirect
    // re-resolves per theme. A token that hardcodes a light value here would
    // still pass a "declared" check while rendering light chrome in dark mode.
    for (const token of [
      "color-surface",
      "color-surface-muted",
      "color-surface-card",
      "color-border",
      "color-text",
      "color-text-muted",
      "color-primary-accent",
    ]) {
      expect(
        theme.get(token),
        `--${token} must redirect to a per-mode variable`,
      ).toMatch(/^var\(--[a-z0-9-]+\)$/);
      expect(resolve(token, light)).not.toBe(resolve(token, dark));
    }
  });

  it("switches the page colour-scheme between light and dark", () => {
    expect(block(indexCss, ":root")).toContain("color-scheme: light");
    expect(block(indexCss, ".dark")).toContain("color-scheme: dark");
  });

  it("keeps text readable on its surface in both modes (WCAG AA >= 4.5:1)", () => {
    const pairs: Array<[string, string]> = [
      ["color-text", "color-surface"],
      ["color-text", "color-surface-card"],
      ["color-text-muted", "color-surface"],
      // The accent is what links, active nav and inline icons are painted in.
      // #2563eb reads 3.90:1 on the dark surface, which is why the accent is
      // per-mode and the fill blue is not — see the comment in index.css.
      ["color-primary-accent", "color-surface"],
      ["color-primary-accent", "color-surface-card"],
      ["color-primary-accent", "color-surface-muted"],
    ];
    for (const [fg, bg] of pairs) {
      for (const [mode, vars] of modes) {
        const ratio = contrast(resolve(fg, vars), resolve(bg, vars));
        expect(
          ratio,
          `${mode}: contrast of --${fg} on --${bg} is ${ratio.toFixed(2)}:1, needs >= 4.5:1`,
        ).toBeGreaterThanOrEqual(4.5);
      }
    }
  });

  it("keeps the primary fill legible with its own foreground in both modes", () => {
    for (const fill of ["color-primary", "color-primary-hover"]) {
      for (const [mode, vars] of modes) {
        const ratio = contrast(
          resolve("color-primary-fg", vars),
          resolve(fill, vars),
        );
        expect(
          ratio,
          `${mode}: --color-primary-fg on --${fill} is ${ratio.toFixed(2)}:1, needs >= 4.5:1`,
        ).toBeGreaterThanOrEqual(4.5);
      }
    }
  });

  it("keeps the focus ring visible against every surface (>= 3:1)", () => {
    for (const bg of ["color-surface", "color-surface-card"]) {
      for (const [mode, vars] of modes) {
        const ratio = contrast(resolve("color-primary", vars), resolve(bg, vars));
        expect(
          ratio,
          `${mode}: focus ring --color-primary on --${bg} is ${ratio.toFixed(2)}:1, needs >= 3:1`,
        ).toBeGreaterThanOrEqual(3);
      }
    }
  });
});
