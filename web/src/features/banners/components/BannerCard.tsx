import * as React from "react";
import { Link } from "@tanstack/react-router";
import { useTranslation } from "react-i18next";
import {
  AlertTriangle,
  ArrowRight,
  CheckCircle2,
  ExternalLink,
  Info,
  Megaphone,
  X,
  type LucideIcon,
} from "lucide-react";

import { buttonVariants } from "@/components/ui/button";
import { reachableStorageUrl } from "@/lib/storage-url";
import { cn } from "@/lib/utils";
import {
  isInternalBannerUrl,
  isSafeBannerUrl,
  type ActiveBanner,
  type BannerTone,
} from "../model/banner";

const TONE_STYLES: Record<
  BannerTone,
  { frame: string; icon: string; Icon: LucideIcon }
> = {
  info: {
    frame: "border-primary/25 bg-primary/5",
    icon: "bg-primary/10 text-primary-accent",
    Icon: Info,
  },
  success: {
    frame: "border-success/25 bg-success/5",
    icon: "bg-success/10 text-success-accent",
    Icon: CheckCircle2,
  },
  warning: {
    frame: "border-warning/30 bg-warning/5",
    icon: "bg-warning/10 text-warning-accent",
    Icon: AlertTriangle,
  },
  promo: {
    frame: "border-mission/30 bg-mission/5",
    icon: "bg-mission/10 text-mission",
    Icon: Megaphone,
  },
};

export type BannerCardData = Pick<
  ActiveBanner,
  "title" | "body" | "cta_label" | "cta_url" | "tone" | "dismissible"
> &
  Partial<
    Pick<
      ActiveBanner,
      "image_url" | "image_alt" | "image_width" | "image_height"
    >
  >;

export interface BannerCardProps {
  banner: BannerCardData;
  /** Shown as a 44px close button when the banner is dismissible. */
  onDismiss?: (() => void) | undefined;
  /**
   * The admin form's preview: the call to action is drawn but goes nowhere,
   * and the close button is inert, so clicking around the preview never
   * navigates away from an unsaved form.
   */
  preview?: boolean | undefined;
  /** Rendered under the text, e.g. the dashboard's position dots. */
  footer?: React.ReactNode;
  className?: string | undefined;
}

/**
 * One announcement banner, exactly as a learner sees it. Every string is
 * rendered as text — the API serves plain text and nothing here interprets it.
 */
export function BannerCard({
  banner,
  onDismiss,
  preview = false,
  footer,
  className,
}: BannerCardProps): React.JSX.Element {
  const { t } = useTranslation();
  const tone = TONE_STYLES[banner.tone] ?? TONE_STYLES.info;
  const showDismiss = banner.dismissible && (preview || onDismiss);
  const ctaLabel = banner.cta_label?.trim();
  const ctaUrl = banner.cta_url?.trim();
  const hasCta = Boolean(ctaLabel && ctaUrl && isSafeBannerUrl(ctaUrl));
  const [imageFailed, setImageFailed] = React.useState<string | null>(null);
  const imageUrl =
    banner.image_url && banner.image_url !== imageFailed
      ? banner.image_url
      : null;

  // The layout follows the card's own width, not the viewport: the same
  // banner is a wide dashboard strip and a narrow preview in the admin form.
  return (
    <div className={cn("@container", className)}>
      <div
        className={cn(
          "relative overflow-hidden rounded-2xl border",
          imageUrl
            ? "flex flex-col @2xl:grid @2xl:grid-cols-[minmax(0,5fr)_minmax(0,6fr)]"
            : "flex",
          tone.frame,
        )}
      >
        {imageUrl && (
          <BannerImage
            url={imageUrl}
            alt={banner.image_alt ?? ""}
            width={banner.image_width}
            height={banner.image_height}
            onError={() => setImageFailed(imageUrl)}
          />
        )}

        <div
          className={cn(
            "flex min-w-0 flex-1 gap-3 p-4 md:gap-4 md:p-5",
            imageUrl && "@2xl:self-center",
          )}
        >
          {/* With a picture the tone icon is noise; the frame keeps the tone. */}
          {!imageUrl && (
            <span
              aria-hidden="true"
              className={cn(
                "flex h-10 w-10 shrink-0 items-center justify-center rounded-xl",
                tone.icon,
              )}
            >
              <tone.Icon className="h-5 w-5" />
            </span>
          )}

          <div
            className={cn("min-w-0 flex-1 space-y-1", showDismiss && "pr-10")}
          >
            <p className="break-words text-base font-semibold text-text">
              {banner.title}
            </p>
            {banner.body && (
              <p className="whitespace-pre-line break-words text-sm text-text-muted">
                {banner.body}
              </p>
            )}
            {hasCta && ctaLabel && ctaUrl && (
              <div className="pt-2">
                <BannerCta label={ctaLabel} url={ctaUrl} preview={preview} />
              </div>
            )}
            {footer}
          </div>
        </div>

        {showDismiss && (
          <button
            type="button"
            onClick={preview ? undefined : onDismiss}
            aria-disabled={preview || undefined}
            aria-label={t("banners.dismiss", { title: banner.title })}
            className={cn(
              "absolute right-1.5 top-1.5 flex h-11 w-11 items-center justify-center rounded-lg transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary motion-reduce:transition-none",
              // Over the picture (mobile: the image is on top) it needs its own
              // backdrop to stay legible whatever the image is.
              imageUrl
                ? "bg-black/45 text-white backdrop-blur-sm hover:bg-black/60 @2xl:bg-transparent @2xl:text-text-muted @2xl:backdrop-blur-none @2xl:hover:bg-surface-muted @2xl:hover:text-text"
                : "text-text-muted hover:bg-surface-muted hover:text-text",
            )}
          >
            <X aria-hidden="true" className="h-5 w-5" />
          </button>
        )}
      </div>
    </div>
  );
}

/**
 * The banner picture. Its recorded size sets the aspect ratio, so the card
 * has its final height before a byte arrives and nothing below it jumps.
 */
function BannerImage({
  url,
  alt,
  width,
  height,
  onError,
}: {
  url: string;
  alt: string;
  width?: number | null | undefined;
  height?: number | null | undefined;
  onError: () => void;
}): React.JSX.Element {
  const ratio = width && height ? `${width} / ${height}` : "16 / 9";
  return (
    <div
      className="relative max-h-56 w-full overflow-hidden bg-surface-muted @2xl:h-full @2xl:max-h-none @2xl:min-h-40"
      style={{ aspectRatio: ratio }}
    >
      <img
        src={reachableStorageUrl(url)}
        alt={alt}
        width={width ?? undefined}
        height={height ?? undefined}
        decoding="async"
        draggable={false}
        onError={onError}
        className="absolute inset-0 h-full w-full object-cover"
      />
    </div>
  );
}

/** A small square crop of a banner's picture, for lists. */
export function BannerThumbnail({
  url,
  className,
}: {
  url: string;
  className?: string | undefined;
}): React.JSX.Element {
  return (
    <img
      src={reachableStorageUrl(url)}
      alt=""
      loading="lazy"
      decoding="async"
      draggable={false}
      className={cn(
        "h-9 w-9 shrink-0 rounded-lg border border-border-subtle object-cover",
        className,
      )}
    />
  );
}

function BannerCta({
  label,
  url,
  preview,
}: {
  label: string;
  url: string;
  preview: boolean;
}): React.JSX.Element {
  const { t } = useTranslation();
  const classes = cn(
    buttonVariants({ variant: "outline", size: "sm" }),
    "bg-surface-card",
  );
  const internal = isInternalBannerUrl(url);
  const Icon = internal ? ArrowRight : ExternalLink;
  const content = (
    <>
      {label}
      <Icon aria-hidden="true" className="h-4 w-4" />
    </>
  );

  if (preview) {
    return (
      <span className={classes} aria-hidden="true">
        {content}
      </span>
    );
  }
  if (internal) {
    const target = new URL(url, "https://internal.invalid");
    const search = Object.fromEntries(target.searchParams.entries());
    return (
      <Link
        to={target.pathname}
        search={search}
        {...(target.hash ? { hash: target.hash.slice(1) } : {})}
        className={classes}
      >
        {content}
      </Link>
    );
  }
  return (
    <a href={url} target="_blank" rel="noopener noreferrer" className={classes}>
      {content}
      <span className="sr-only">{t("banners.opensInNewTab")}</span>
    </a>
  );
}
