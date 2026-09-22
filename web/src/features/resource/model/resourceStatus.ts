import type { BadgeVariant } from "@/components/ui/badge";

/**
 * How a resource's state reads on screen. One place, because the list and the
 * detail page must agree: a resource that says "Ready" in the list and
 * "Processing" on its own page is two answers to one question.
 */
export type ResourceStatusTone = BadgeVariant;

export function statusTone(status: string): ResourceStatusTone {
  switch (status) {
    case "validated":
      return "success";
    case "rejected":
      return "danger";
    case "failed":
      return "warning";
    case "pending":
    case "uploaded":
      return "secondary";
    default:
      return "outline";
  }
}

/** The i18n key for a status, defaulted in the translation files. */
export function statusLabelKey(status: string): string {
  switch (status) {
    case "pending":
      return "resources.statusPending";
    case "uploaded":
      return "resources.statusUploaded";
    case "validated":
      return "resources.statusValidated";
    case "rejected":
      return "resources.statusRejected";
    case "failed":
      return "resources.statusFailed";
    default:
      return "resources.statusUnknown";
  }
}

/** The i18n key for a file family, used on the list's kind line. */
export function familyLabelKey(mime: string): string {
  const base = mime.split(";")[0]?.trim().toLowerCase() ?? "";
  if (base === "application/pdf") return "resources.kindPdf";
  if (base.startsWith("image/")) return "resources.kindImage";
  if (base.startsWith("audio/")) return "resources.kindAudio";
  if (base.startsWith("video/")) return "resources.kindVideo";
  if (base.includes("word") || base.includes("presentation")) {
    return "resources.kindDocument";
  }
  return "resources.kindFile";
}

/**
 * A byte count a learner can read.
 *
 * Deliberately not `Intl.NumberFormat`'s byte units: its output is locale-
 * dependent in ways the layout is not ("2,4 MB" beside "2.4 MB"), and the
 * quota bar needs the same string in both languages.
 */
export function formatBytes(bytes: number | null | undefined): string {
  if (!bytes || bytes <= 0) return "0 B";
  const units = ["B", "KB", "MB", "GB"];
  const index = Math.min(
    Math.floor(Math.log(bytes) / Math.log(1024)),
    units.length - 1,
  );
  const value = bytes / 1024 ** index;
  const rounded = value >= 10 || index === 0 ? Math.round(value) : value.toFixed(1);
  return `${rounded} ${units[index]}`;
}
