import * as React from "react";
import { Link } from "@tanstack/react-router";
import { useTranslation } from "react-i18next";
import { ImageIcon } from "lucide-react";

import { reachableStorageUrl } from "@/lib/storage-url";
import { cn } from "@/lib/utils";
import {
  isInternalBannerUrl,
  isSafeBannerUrl,
  type ActiveBanner,
} from "../model/banner";

export type BannerSlideData = Pick<ActiveBanner, "title" | "cta_url"> &
  Partial<Pick<ActiveBanner, "image_url" | "image_alt">>;

export interface BannerSlideProps {
  slide: BannerSlideData;
  /** The admin form's preview: drawn, never a link. */
  preview?: boolean | undefined;
  /** Eager for the first slide (it is above the fold), lazy for the rest. */
  priority?: boolean | undefined;
  className?: string | undefined;
}

/**
 * One promotional slide: the picture, cropped to the carousel's fixed frame
 * so every slide has the same height, and the whole picture as the link when
 * it has one.
 */
export function BannerSlide({
  slide,
  preview = false,
  priority = false,
  className,
}: BannerSlideProps): React.JSX.Element {
  const { t } = useTranslation();
  const [failed, setFailed] = React.useState<string | null>(null);
  const url =
    slide.image_url && slide.image_url !== failed ? slide.image_url : null;
  const alt = slide.image_alt?.trim() || slide.title;
  const link = slide.cta_url?.trim();
  const linkable = !preview && Boolean(link && isSafeBannerUrl(link));

  const picture = url ? (
    <img
      src={reachableStorageUrl(url)}
      alt={alt}
      loading={priority ? "eager" : "lazy"}
      decoding="async"
      draggable={false}
      onError={() => setFailed(url)}
      className="absolute inset-0 h-full w-full object-cover"
    />
  ) : (
    <span className="absolute inset-0 flex flex-col items-center justify-center gap-1 text-text-muted">
      <ImageIcon aria-hidden="true" className="h-6 w-6" />
      <span className="text-xs">
        {preview ? t("adminBanners.slidePreviewEmpty") : slide.title}
      </span>
    </span>
  );

  // 3:1, the shape the admin form asks for. Inside a wide carousel (a
  // container of 56rem or more, i.e. desktop) the frame is 4:1 so the slide
  // does not push the dashboard below the fold; the picture is cropped a
  // little top and bottom there, which the upload hint warns about.
  const frame = cn(
    "relative block aspect-[3/1] w-full overflow-hidden rounded-2xl border border-border-subtle bg-surface-muted @4xl:aspect-[4/1]",
    linkable &&
      "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary focus-visible:ring-offset-2",
    className,
  );

  if (linkable && link) {
    if (isInternalBannerUrl(link)) {
      const target = new URL(link, "https://internal.invalid");
      return (
        <Link
          to={target.pathname}
          search={Object.fromEntries(target.searchParams.entries())}
          {...(target.hash ? { hash: target.hash.slice(1) } : {})}
          className={frame}
          draggable={false}
        >
          {picture}
        </Link>
      );
    }
    return (
      <a
        href={link}
        target="_blank"
        rel="noopener noreferrer"
        className={frame}
        draggable={false}
      >
        {picture}
        <span className="sr-only">{t("banners.opensInNewTab")}</span>
      </a>
    );
  }
  return <div className={frame}>{picture}</div>;
}
