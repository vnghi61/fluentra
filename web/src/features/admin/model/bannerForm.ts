import { z } from "zod";

import {
  BANNER_KINDS,
  BANNER_LIMITS,
  BANNER_TONES,
  isSafeBannerUrl,
  type Banner,
  type BannerRequest,
} from "@/features/banners";

/**
 * The banner create/edit form. Messages are i18n keys under
 * `adminBanners.err`; FormMessage translates them where they are shown. The
 * server re-checks every rule (BannerRequest in the OpenAPI spec).
 */
export const bannerFormSchema = z
  .object({
    kind: z.enum(BANNER_KINDS),
    title: z
      .string()
      .trim()
      .min(1, "adminBanners.err.titleRequired")
      .max(BANNER_LIMITS.title, "adminBanners.err.titleTooLong"),
    body: z
      .string()
      .trim()
      .max(BANNER_LIMITS.body, "adminBanners.err.bodyTooLong"),
    ctaLabel: z
      .string()
      .trim()
      .max(BANNER_LIMITS.ctaLabel, "adminBanners.err.ctaLabelTooLong"),
    ctaUrl: z.string().trim(),
    tone: z.enum(BANNER_TONES),
    priority: z
      .number({ invalid_type_error: "adminBanners.err.priorityRange" })
      .int("adminBanners.err.priorityRange")
      .min(0, "adminBanners.err.priorityRange")
      .max(BANNER_LIMITS.priorityMax, "adminBanners.err.priorityRange"),
    isActive: z.boolean(),
    dismissible: z.boolean(),
    /** `datetime-local` values in the administrator's own time zone. */
    startsAt: z.string(),
    endsAt: z.string(),
    /** A processed image's key from the upload, or "" for none. */
    imageKey: z.string(),
    /** A signed URL for the form's preview only; never sent. */
    imageUrl: z.string(),
    imageAlt: z
      .string()
      .trim()
      .max(BANNER_LIMITS.imageAlt, "adminBanners.err.imageAltTooLong"),
  })
  .superRefine((v, ctx) => {
    if (v.ctaUrl && !isSafeBannerUrl(v.ctaUrl)) {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        path: ["ctaUrl"],
        message: "adminBanners.err.urlUnsafe",
      });
    }
    if (v.kind === "slide") {
      // A slide is its picture; its link stands alone, with no button text.
      if (!v.imageKey) {
        ctx.addIssue({
          code: z.ZodIssueCode.custom,
          path: ["imageKey"],
          message: "adminBanners.err.slideImageRequired",
        });
      }
      return;
    }
    if (v.ctaUrl && !v.ctaLabel) {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        path: ["ctaLabel"],
        message: "adminBanners.err.ctaPair",
      });
    }
    if (v.ctaLabel && !v.ctaUrl) {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        path: ["ctaUrl"],
        message: "adminBanners.err.ctaPair",
      });
    }
    const start = parseLocal(v.startsAt);
    const end = parseLocal(v.endsAt);
    if (start !== null && end !== null && end <= start) {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        path: ["endsAt"],
        message: "adminBanners.err.windowOrder",
      });
    }
  });

export type BannerFormValues = z.infer<typeof bannerFormSchema>;

export const emptyBannerForm: BannerFormValues = {
  kind: "announcement",
  title: "",
  body: "",
  ctaLabel: "",
  ctaUrl: "",
  tone: "info",
  priority: 0,
  isActive: false,
  dismissible: true,
  startsAt: "",
  endsAt: "",
  imageKey: "",
  imageUrl: "",
  imageAlt: "",
};

function parseLocal(value: string): number | null {
  if (!value) return null;
  const time = new Date(value).getTime();
  return Number.isNaN(time) ? null : time;
}

function pad(n: number): string {
  return String(n).padStart(2, "0");
}

/** An ISO instant as a `datetime-local` value in the browser's time zone. */
export function isoToLocalInput(iso: string | null | undefined): string {
  if (!iso) return "";
  const date = new Date(iso);
  if (Number.isNaN(date.getTime())) return "";
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}T${pad(date.getHours())}:${pad(date.getMinutes())}`;
}

function localInputToIso(value: string): string | null {
  const time = parseLocal(value);
  return time === null ? null : new Date(time).toISOString();
}

export function bannerToForm(banner: Banner): BannerFormValues {
  return {
    kind: banner.kind,
    title: banner.title,
    body: banner.body,
    ctaLabel: banner.cta_label ?? "",
    ctaUrl: banner.cta_url ?? "",
    tone: banner.tone,
    priority: banner.priority,
    isActive: banner.is_active,
    dismissible: banner.dismissible,
    startsAt: isoToLocalInput(banner.starts_at),
    endsAt: isoToLocalInput(banner.ends_at),
    imageKey: banner.image_key ?? "",
    imageUrl: banner.image_url ?? "",
    imageAlt: banner.image_alt ?? "",
  };
}

export function formToBannerRequest(values: BannerFormValues): BannerRequest {
  const label = values.ctaLabel.trim();
  const url = values.ctaUrl.trim();
  const slide = values.kind === "slide";
  return {
    kind: values.kind,
    title: values.title.trim(),
    // A slide carries no text, no button and no dismissal; the server clears
    // them too, but sending them empty keeps the request honest.
    body: slide ? "" : values.body.trim(),
    cta_label: slide ? null : label && url ? label : null,
    cta_url: slide ? url || null : label && url ? url : null,
    tone: values.tone,
    priority: values.priority,
    is_active: values.isActive,
    dismissible: slide ? false : values.dismissible,
    starts_at: localInputToIso(values.startsAt),
    ends_at: localInputToIso(values.endsAt),
    image_key: values.imageKey || null,
    image_alt: values.imageKey ? values.imageAlt.trim() : "",
  };
}

/** Server field names (snake_case) mapped onto the form's. */
export const SERVER_FIELD_TO_FORM: Record<string, keyof BannerFormValues> = {
  title: "title",
  body: "body",
  cta_label: "ctaLabel",
  cta_url: "ctaUrl",
  tone: "tone",
  priority: "priority",
  starts_at: "startsAt",
  ends_at: "endsAt",
  kind: "kind",
  image_key: "imageKey",
  image_alt: "imageAlt",
};
