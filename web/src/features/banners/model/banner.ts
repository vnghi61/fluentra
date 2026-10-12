import type { components } from "@/types/api";

export type Banner = components["schemas"]["Banner"];
export type ActiveBanner = components["schemas"]["ActiveBanner"];
export type BannerTone = components["schemas"]["BannerTone"];
export type BannerKind = components["schemas"]["BannerKind"];
export type BannerRequest = components["schemas"]["BannerRequest"];
export type BannerList = components["schemas"]["BannerList"];
export type ActiveBannerList = components["schemas"]["ActiveBannerList"];
export type BannerImageUploadIntent =
  components["schemas"]["BannerImageUploadIntent"];
export type BannerImage = components["schemas"]["BannerImage"];

export const BANNER_KINDS = [
  "announcement",
  "slide",
] as const satisfies readonly BannerKind[];

/**
 * The image size the admin form recommends for a slide: the carousel frame is
 * 3:1 (4:1 on wide screens) and crops anything else (object-cover).
 */
export const SLIDE_RECOMMENDED_SIZE = "1500×500";

export const BANNER_TONES = [
  "info",
  "success",
  "warning",
  "promo",
] as const satisfies readonly BannerTone[];

/** Server limits (api/openapi/components/admin.yaml BannerRequest). */
export const BANNER_LIMITS = {
  title: 120,
  body: 500,
  ctaLabel: 40,
  ctaUrl: 2048,
  priorityMax: 1000,
  imageAlt: 160,
} as const;

/**
 * What an administrator may upload as a banner image (the server's
 * BannerImageUploadIntentRequest). The server re-checks the bytes, scales the
 * image to fit 1600x900 and re-encodes it as JPEG; this only spares a doomed
 * upload.
 */
export const BANNER_IMAGE = {
  types: ["image/jpeg", "image/png", "image/webp"] as const,
  accept: "image/jpeg,image/png,image/webp",
  maxBytes: 5 * 1024 * 1024,
} as const;

export type BannerImageType = (typeof BANNER_IMAGE.types)[number];

export function isBannerImageType(type: string): type is BannerImageType {
  return (BANNER_IMAGE.types as readonly string[]).includes(type);
}

/**
 * Whether a call-to-action link is one the dashboard may follow. The server
 * applies the same rule and is the authority; this only lets the form say so
 * before a round trip. An internal path starts with "/" but not "//" or "/\",
 * which browsers resolve as protocol-relative; anything else must be https.
 */
export function isSafeBannerUrl(raw: string): boolean {
  if (raw === "" || raw.length > BANNER_LIMITS.ctaUrl) return false;
  // Whitespace, control characters and backslashes are never part of a link
  // we accept; they are how "java\nscript:" slips past naive checks.
  // eslint-disable-next-line no-control-regex
  if (/[\s\u0000-\u001f\u007f\\]/.test(raw)) return false;
  if (raw.startsWith("/")) return !raw.startsWith("//");
  if (!raw.startsWith("https://")) return false;
  try {
    const url = new URL(raw);
    return (
      url.protocol === "https:" &&
      url.hostname !== "" &&
      url.username === "" &&
      url.password === ""
    );
  } catch {
    return false;
  }
}

/** An internal path is followed with the router; anything else opens a tab. */
export function isInternalBannerUrl(raw: string): boolean {
  return raw.startsWith("/") && !raw.startsWith("//");
}

export type BannerStatus = "active" | "scheduled" | "expired" | "off";

/** Where a banner stands at `now`, from its switch and its window. */
export function bannerStatus(
  banner: Pick<Banner, "is_active" | "starts_at" | "ends_at">,
  now: number,
): BannerStatus {
  if (!banner.is_active) return "off";
  if (banner.starts_at && Date.parse(banner.starts_at) > now) {
    return "scheduled";
  }
  if (banner.ends_at && Date.parse(banner.ends_at) <= now) return "expired";
  return "active";
}

const DISMISSED_PREFIX = "fluentra.banners.dismissed.";
/** Dismissed ids kept per user; older ones fall off so the list stays small. */
const DISMISSED_MAX = 50;

/**
 * Banner ids this user has dismissed on this device. Storage can be missing
 * or refuse (private mode, quota, a disabled policy); a failure reads as
 * "nothing dismissed" rather than as an error the dashboard has to show.
 */
export function readDismissedBanners(userId: string): string[] {
  try {
    const raw = window.localStorage.getItem(DISMISSED_PREFIX + userId);
    if (!raw) return [];
    const parsed: unknown = JSON.parse(raw);
    return Array.isArray(parsed)
      ? parsed.filter((id): id is string => typeof id === "string")
      : [];
  } catch {
    return [];
  }
}

/** Remembers a dismissal. Returns the new list even when storage refuses. */
export function rememberDismissedBanner(
  userId: string,
  bannerId: string,
): string[] {
  const next = [
    ...readDismissedBanners(userId).filter((id) => id !== bannerId),
    bannerId,
  ].slice(-DISMISSED_MAX);
  try {
    window.localStorage.setItem(
      DISMISSED_PREFIX + userId,
      JSON.stringify(next),
    );
  } catch {
    // Storage refused: the banner stays hidden for this visit only.
  }
  return next;
}

const SEEN_PREFIX = "fluentra.banners.seen.";

/**
 * Announcements this user has already been shown in this browser session. A
 * non-dismissible announcement comes back on the next visit, but not every
 * time the learner returns to the dashboard within one. Storage can be
 * missing or refuse; that reads as "nothing seen".
 */
export function readSessionSeenBanners(userId: string): string[] {
  try {
    const raw = window.sessionStorage.getItem(SEEN_PREFIX + userId);
    if (!raw) return [];
    const parsed: unknown = JSON.parse(raw);
    return Array.isArray(parsed)
      ? parsed.filter((id): id is string => typeof id === "string")
      : [];
  } catch {
    return [];
  }
}

/** Remembers that an announcement was shown in this session. */
export function rememberSessionSeenBanner(
  userId: string,
  bannerId: string,
): string[] {
  const next = [
    ...readSessionSeenBanners(userId).filter((id) => id !== bannerId),
    bannerId,
  ].slice(-DISMISSED_MAX);
  try {
    window.sessionStorage.setItem(SEEN_PREFIX + userId, JSON.stringify(next));
  } catch {
    // Storage refused: it may show once more on this visit's next load.
  }
  return next;
}
