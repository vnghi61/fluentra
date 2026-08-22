import { readFileSync } from "node:fs";
import { join } from "node:path";
import { createElement } from "react";
import { render } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { Badge, type BadgeVariant } from "@/components/ui/badge";
import { Button, type ButtonProps } from "@/components/ui/button";

const indexCss = readFileSync(
  join(process.cwd(), "src", "index.css"),
  "utf8",
).replace(/\/\*[\s\S]*?\*\//g, "");

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

describe("UI Primitives Contrast Verification (P6.2)", () => {
  const theme = customProperties(block(indexCss, "@theme"));
  const light = customProperties(block(indexCss, ":root"));
  const dark = customProperties(block(indexCss, ".dark"));

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

  // Table-driven tests for button variants across themes (WCAG AA text >= 4.5:1)
  const buttonVariants = [
    {
      variant: "primary",
      fgToken: "color-primary-fg",
      bgToken: "color-primary",
      description: "primary button text on primary fill",
    },
    {
      variant: "primary (hover)",
      fgToken: "color-primary-fg",
      bgToken: "color-primary-hover",
      description: "primary button text on primary hover fill",
    },
    {
      variant: "secondary",
      fgToken: "color-text",
      bgToken: "color-surface-muted",
      description: "secondary button text on muted surface",
    },
    {
      variant: "outline",
      fgToken: "color-text",
      bgToken: "color-surface",
      description: "outline button text on page surface",
    },
    {
      variant: "outline (on card)",
      fgToken: "color-text",
      bgToken: "color-surface-card",
      description: "outline button text on card surface",
    },
    {
      variant: "ghost",
      fgToken: "color-primary-accent",
      bgToken: "color-surface",
      description: "ghost button text on page surface",
    },
    {
      variant: "ghost (hover)",
      fgToken: "color-primary-accent",
      bgToken: "color-surface-muted",
      description: "ghost button text on muted hover surface",
    },
    {
      variant: "ghost (on card)",
      fgToken: "color-primary-accent",
      bgToken: "color-surface-card",
      description: "ghost button text on card surface",
    },
    {
      variant: "destructive",
      fgToken: "color-primary-fg",
      bgToken: "color-danger-fill",
      description: "destructive button text on danger fill",
    },
    {
      variant: "destructive (hover)",
      fgToken: "color-primary-fg",
      bgToken: "color-danger-fill-hover",
      description: "destructive button text on danger hover fill",
    },
  ] as const;

  describe("Button text contrast across variants and themes (WCAG AA >= 4.5:1)", () => {
    for (const { variant, fgToken, bgToken, description } of buttonVariants) {
      for (const [mode, vars] of modes) {
        it(`${mode} mode: ${variant} (${description}) achieves >= 4.5:1`, () => {
          const fg = resolve(fgToken, vars);
          const bg = resolve(bgToken, vars);
          const ratio = contrast(fg, bg);
          expect(
            ratio,
            `${mode} mode: contrast of --${fgToken} (${fg}) on --${bgToken} (${bg}) is ${ratio.toFixed(2)}:1, needs >= 4.5:1`,
          ).toBeGreaterThanOrEqual(4.5);
        });
      }
    }
  });

  describe("Primary accent vs primary fill contrast trap", () => {
    it("confirms --color-primary as text fails WCAG in dark mode, while --color-primary-accent passes", () => {
      const darkSurface = resolve("color-surface", dark);
      const darkPrimaryFill = resolve("color-primary", dark);
      const darkPrimaryAccent = resolve("color-primary-accent", dark);

      const fillRatio = contrast(darkPrimaryFill, darkSurface);
      const accentRatio = contrast(darkPrimaryAccent, darkSurface);

      // Primary fill (#2563eb) is only 3.90:1 on dark surface (#020617) — fails WCAG AA text
      expect(fillRatio).toBeLessThan(4.5);
      // Primary accent (#60a5fa) is 9.87:1 on dark surface (#020617) — comfortably passes
      expect(accentRatio).toBeGreaterThanOrEqual(4.5);
    });
  });

  describe("Focus rings and non-text visual contrast (>= 3:1)", () => {
    const surfaces = [
      ["page surface", "color-surface"],
      ["muted surface", "color-surface-muted"],
      ["card surface", "color-surface-card"],
    ] as const;

    for (const [surfaceName, bgToken] of surfaces) {
      for (const [mode, vars] of modes) {
        it(`${mode} mode: focus ring (--color-primary) on ${surfaceName} is >= 3:1`, () => {
          const ring = resolve("color-primary", vars);
          const bg = resolve(bgToken, vars);
          const ratio = contrast(ring, bg);
          expect(
            ratio,
            `${mode} mode: focus ring contrast on ${surfaceName} is ${ratio.toFixed(2)}:1, needs >= 3:1`,
          ).toBeGreaterThanOrEqual(3.0);
        });
      }
    }
  });

  describe("Borders (non-text visual contrast >= 3:1)", () => {
    const surfaces = [
      ["page surface", "color-surface"],
      ["muted surface", "color-surface-muted"],
      ["card surface", "color-surface-card"],
    ] as const;

    for (const [surfaceName, bgToken] of surfaces) {
      for (const [mode, vars] of modes) {
        it(`${mode} mode: border (--color-border) on ${surfaceName} is >= 3:1`, () => {
          const border = resolve("color-border", vars);
          const bg = resolve(bgToken, vars);
          const ratio = contrast(border, bg);
          expect(
            ratio,
            `${mode} mode: border contrast on ${surfaceName} is ${ratio.toFixed(2)}:1, needs >= 3:1`,
          ).toBeGreaterThanOrEqual(3.0);
        });
      }
    }
  });

  describe("Button variants reference the verified tokens (component-level)", () => {
    // Guards against the token table drifting from what <Button> actually
    // renders: if a variant class is swapped to a token outside the table
    // (or back to a hardcoded color), the rendered class no longer matches the
    // token pair the contrast assertions above resolved.
    const variantTokenClasses: Record<
      ButtonProps["variant"] & string,
      readonly string[]
    > = {
      primary: ["bg-primary", "text-primary-fg"],
      secondary: ["bg-surface-muted", "text-text", "border-border"],
      outline: ["border-border", "text-text"],
      ghost: ["text-primary-accent"],
      destructive: ["bg-danger-fill", "text-primary-fg"],
    };

    for (const [variant, tokens] of Object.entries(variantTokenClasses)) {
      for (const token of tokens) {
        it(`renders ${variant} with the ${token} class`, () => {
          const { container } = render(
            createElement(
              Button,
              { variant: variant as ButtonProps["variant"] },
              "confirm",
            ),
          );
          const button = container.querySelector("button");
          expect(
            button,
            `${variant} variant renders a <button>`,
          ).not.toBeNull();
          // A full-class match, not a substring: `bg-danger-fill` is a prefix of
          // `hover:bg-danger-fill-hover`, so a naive toContain would pass a
          // broken base fill. Split on whitespace and compare whole classes.
          const classes = new Set(button!.className.split(/\s+/));
          expect(
            classes.has(token),
            `${variant} variant must keep class ${token}, got: ${button!.className}`,
          ).toBe(true);
        });
      }
    }
  });

  describe("Form elements text contrast (WCAG AA >= 4.5:1)", () => {
    const formChecks = [
      {
        name: "Input text on input surface",
        fgToken: "color-text",
        bgToken: "color-surface",
      },
      {
        name: "Input placeholder on input surface",
        fgToken: "color-text-muted",
        bgToken: "color-surface",
      },
      {
        name: "Form label text on surface",
        fgToken: "color-text",
        bgToken: "color-surface",
      },
      {
        name: "Form description text on surface",
        fgToken: "color-text-muted",
        bgToken: "color-surface",
      },
      {
        name: "Checkbox label on surface",
        fgToken: "color-text",
        bgToken: "color-surface",
      },
      {
        name: "Checkbox checked checkmark on primary fill",
        fgToken: "color-primary-fg",
        bgToken: "color-primary",
      },
      {
        name: "Card text on card surface",
        fgToken: "color-text",
        bgToken: "color-surface-card",
      },
      {
        name: "Card description on card surface",
        fgToken: "color-text-muted",
        bgToken: "color-surface-card",
      },
    ] as const;

    for (const { name, fgToken, bgToken } of formChecks) {
      for (const [mode, vars] of modes) {
        it(`${mode} mode: ${name} is >= 4.5:1`, () => {
          const fg = resolve(fgToken, vars);
          const bg = resolve(bgToken, vars);
          const ratio = contrast(fg, bg);
          expect(
            ratio,
            `${mode} mode: contrast of --${fgToken} on --${bgToken} is ${ratio.toFixed(2)}:1, needs >= 4.5:1`,
          ).toBeGreaterThanOrEqual(4.5);
        });
      }
    }
  });

  describe("Border tokens distinction (P6.3 Trap 2)", () => {
    it("distinguishes control border from subtle card boundary in both themes", () => {
      for (const [mode, vars] of modes) {
        const borderControl = resolve("color-border", vars);
        const borderSubtle = resolve("color-border-subtle", vars);
        expect(
          borderControl,
          `${mode} mode: control border must differ from subtle hairline border`,
        ).not.toBe(borderSubtle);
        // Subtle must be strictly weaker (closer to surface) than control border
        for (const bgToken of [
          "color-surface",
          "color-surface-card",
          "color-surface-muted",
        ]) {
          const bg = resolve(bgToken, vars);
          const controlRatio = contrast(borderControl, bg);
          const subtleRatio = contrast(borderSubtle, bg);
          expect(
            subtleRatio,
            `${mode} mode: subtle border on ${bgToken} must be weaker than control border (${subtleRatio.toFixed(2)} < ${controlRatio.toFixed(2)})`,
          ).toBeLessThan(controlRatio);
        }
      }
    });
  });

  describe("Badge text contrast across variants and themes (WCAG AA >= 4.5:1)", () => {
    // Badge now uses transparent bg (no 10% tint) + accent text so contrast is
    // measured fg on the underlying surface. The previous primary/10 tint gave
    // 4.49:1 (just under AA) because the tinted bg inches toward the fg hue.
    const badgeVariants = [
      {
        variant: "primary",
        fgToken: "color-primary-accent",
        bgToken: "color-surface-card",
      },
      {
        variant: "primary",
        fgToken: "color-primary-accent",
        bgToken: "color-surface",
      },
      {
        variant: "secondary",
        fgToken: "color-text-muted",
        bgToken: "color-surface-muted",
      },
      {
        variant: "outline",
        fgToken: "color-text",
        bgToken: "color-surface-card",
      },
      { variant: "outline", fgToken: "color-text", bgToken: "color-surface" },
      {
        variant: "success",
        fgToken: "color-success-accent",
        bgToken: "color-surface-card",
      },
      {
        variant: "success",
        fgToken: "color-success-accent",
        bgToken: "color-surface",
      },
      {
        variant: "warning",
        fgToken: "color-warning-accent",
        bgToken: "color-surface-card",
      },
      {
        variant: "warning",
        fgToken: "color-warning-accent",
        bgToken: "color-surface",
      },
      {
        variant: "danger",
        fgToken: "color-danger-accent",
        bgToken: "color-surface-card",
      },
      {
        variant: "danger",
        fgToken: "color-danger-accent",
        bgToken: "color-surface",
      },
    ] as const;

    for (const { variant, fgToken, bgToken } of badgeVariants) {
      for (const [mode, vars] of modes) {
        it(`${mode} mode: ${variant} badge on ${bgToken.replace("color-", "")} achieves >= 4.5:1`, () => {
          const fg = resolve(fgToken, vars);
          const bg = resolve(bgToken, vars);
          const ratio = contrast(fg, bg);
          expect(
            ratio,
            `${mode} mode: contrast of --${fgToken} (${fg}) on --${bgToken} (${bg}) is ${ratio.toFixed(2)}:1, needs >= 4.5:1`,
          ).toBeGreaterThanOrEqual(4.5);
        });
      }
    }
  });

  describe("Badge variants reference the verified tokens (component-level)", () => {
    const badgeVariantClasses: Record<BadgeVariant, readonly string[]> = {
      primary: ["text-primary-accent", "border-primary/20"],
      secondary: ["bg-surface-muted", "text-text-muted"],
      outline: ["text-text", "border-border-subtle"],
      success: ["text-success-accent", "border-success/20"],
      warning: ["text-warning-accent", "border-warning/20"],
      danger: ["text-danger-accent", "border-danger/20"],
    };

    const entries = Object.entries(badgeVariantClasses) as [
      BadgeVariant,
      readonly string[],
    ][];

    for (const [variant, tokens] of entries) {
      for (const token of tokens) {
        it(`renders ${variant} with the ${token} class`, () => {
          const { container } = render(
            createElement(Badge, { variant }, variant.toUpperCase()),
          );
          const badge = container.querySelector("span");
          expect(badge, `${variant} variant renders a <span>`).not.toBeNull();
          const classes = new Set(badge!.className.split(/\s+/));
          expect(
            classes.has(token),
            `${variant} variant must keep class ${token}, got: ${badge!.className}`,
          ).toBe(true);
        });
      }
    }
  });
});
