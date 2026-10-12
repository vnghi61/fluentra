// One card and one inline-alert look for every settings screen, so the five
// panels stop drifting apart (they used five slightly different paddings).
export const settingsCard =
  "rounded-2xl border border-border-subtle bg-surface-card p-4 shadow-[var(--shadow-card)] sm:p-6";

export const settingsCardTitle =
  "flex items-center gap-2 text-base font-semibold text-text";

export const settingsCardDesc = "text-sm text-text-muted";

export const settingsAlert = (type: "success" | "error"): string =>
  `flex items-start gap-2.5 rounded-xl border p-3 text-sm ${
    type === "success"
      ? "border-success/30 bg-success/10 text-success-accent"
      : "border-danger/30 bg-danger/10 text-danger-accent"
  }`;

/** Dates and times in the interface language, not the browser's default. */
export const formatDateTime = (iso: string, locale: string): string =>
  new Date(iso).toLocaleString(locale, {
    day: "2-digit",
    month: "2-digit",
    year: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  });

export const formatDate = (iso: string, locale: string): string =>
  new Date(iso).toLocaleDateString(locale, {
    day: "2-digit",
    month: "2-digit",
    year: "numeric",
  });

export const isMobileLabel = (label: string | null | undefined): boolean => {
  const l = label?.toLowerCase() ?? "";
  return (
    l.includes("mobile") ||
    l.includes("android") ||
    l.includes("iphone") ||
    l.includes("ipad")
  );
};

const RELATIVE_STEPS: readonly [Intl.RelativeTimeFormatUnit, number][] = [
  ["second", 60],
  ["minute", 60],
  ["hour", 24],
  ["day", 7],
  ["week", 4.35],
  ["month", 12],
  ["year", Number.POSITIVE_INFINITY],
];

/** "2 hours ago" in the interface language; empty for an unreadable date. */
export const formatRelativeTime = (
  iso: string,
  locale: string,
  now: number = Date.now(),
): string => {
  const then = Date.parse(iso);
  if (Number.isNaN(then)) return "";
  let value = (then - now) / 1000;
  const rtf = new Intl.RelativeTimeFormat(locale, { numeric: "auto" });
  for (const [unit, size] of RELATIVE_STEPS) {
    if (Math.abs(value) < size) return rtf.format(Math.round(value), unit);
    value /= size;
  }
  return "";
};

/**
 * The server's coarse device label is "<browser> on <os>" ("Chrome on
 * Windows"). Split it so the card can show each part; anything else is
 * shown as it came.
 */
export const splitDeviceLabel = (
  label: string | null | undefined,
): { browser: string; os: string } | null => {
  const match = /^(.+?) on (.+)$/.exec(label?.trim() ?? "");
  if (!match?.[1] || !match[2]) return null;
  return { browser: match[1], os: match[2] };
};
