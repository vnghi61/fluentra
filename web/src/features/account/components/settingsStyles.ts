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
