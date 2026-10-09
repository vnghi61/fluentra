import * as React from "react";
import { Link } from "@tanstack/react-router";
import { useTranslation } from "react-i18next";
import {
  AlertTriangle,
  CheckCircle2,
  ExternalLink,
  Info,
  Megaphone,
  X,
  type LucideIcon,
} from "lucide-react";

import { buttonVariants } from "@/components/ui/button";
import {
  Dialog,
  DialogClose,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogTitle,
} from "@/components/ui/dialog";
import { reachableStorageUrl } from "@/lib/storage-url";
import { cn } from "@/lib/utils";
import {
  isInternalBannerUrl,
  isSafeBannerUrl,
  type ActiveBanner,
  type BannerTone,
} from "../model/banner";

const TONE: Record<BannerTone, { icon: string; Icon: LucideIcon }> = {
  info: { icon: "bg-primary/10 text-primary-accent", Icon: Info },
  success: { icon: "bg-success/10 text-success-accent", Icon: CheckCircle2 },
  warning: { icon: "bg-warning/10 text-warning-accent", Icon: AlertTriangle },
  promo: { icon: "bg-mission/10 text-mission", Icon: Megaphone },
};

export interface AnnouncementDialogProps {
  announcement: ActiveBanner;
  open: boolean;
  /** Called once, whichever way the dialog is closed (×, Esc, overlay, CTA). */
  onClose: () => void;
}

/**
 * An announcement, shown once as a dialog when the dashboard opens. Every
 * string is rendered as text; the API serves plain text and nothing here
 * interprets it.
 */
export function AnnouncementDialog({
  announcement,
  open,
  onClose,
}: AnnouncementDialogProps): React.JSX.Element {
  const { t } = useTranslation();
  const tone = TONE[announcement.tone] ?? TONE.info;
  const [imageFailed, setImageFailed] = React.useState(false);
  const imageUrl =
    announcement.image_url && !imageFailed ? announcement.image_url : null;
  const label = announcement.cta_label?.trim();
  const url = announcement.cta_url?.trim();
  const hasCta = Boolean(label && url && isSafeBannerUrl(url));

  return (
    <Dialog open={open} onOpenChange={(next) => !next && onClose()}>
      <DialogContent
        showClose={false}
        className="max-w-md overflow-hidden"
        {...(announcement.body ? {} : { "aria-describedby": undefined })}
      >
        {imageUrl && (
          <div
            className="relative w-full shrink-0 bg-surface-muted"
            style={{
              aspectRatio:
                announcement.image_width && announcement.image_height
                  ? `${announcement.image_width} / ${announcement.image_height}`
                  : "16 / 9",
              maxHeight: "40dvh",
            }}
          >
            <img
              src={reachableStorageUrl(imageUrl)}
              alt={announcement.image_alt ?? ""}
              decoding="async"
              onError={() => setImageFailed(true)}
              className="absolute inset-0 h-full w-full object-cover"
            />
          </div>
        )}

        <div className="min-h-0 flex-1 space-y-2 overflow-y-auto p-5 pr-14">
          {!imageUrl && (
            <span
              aria-hidden="true"
              className={cn(
                "mb-1 flex h-10 w-10 items-center justify-center rounded-xl",
                tone.icon,
              )}
            >
              <tone.Icon className="h-5 w-5" />
            </span>
          )}
          <DialogTitle className="break-words text-lg font-semibold text-text">
            {announcement.title}
          </DialogTitle>
          {announcement.body && (
            <DialogDescription className="whitespace-pre-line break-words text-sm text-text-muted">
              {announcement.body}
            </DialogDescription>
          )}
        </div>

        <DialogFooter className="flex-col-reverse sm:flex-row">
          <DialogClose
            className={cn(
              buttonVariants({ variant: hasCta ? "outline" : "primary" }),
              "min-h-[44px]",
            )}
          >
            {t("banners.close")}
          </DialogClose>
          {hasCta && label && url && (
            <AnnouncementCta label={label} url={url} onFollow={onClose} />
          )}
        </DialogFooter>

        <DialogClose
          aria-label={t("banners.closeAnnouncement", {
            title: announcement.title,
          })}
          className={cn(
            "absolute right-3 top-3 flex h-11 w-11 items-center justify-center rounded-lg focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary",
            // Over a picture it needs its own backdrop to stay legible.
            imageUrl
              ? "bg-black/45 text-white backdrop-blur-sm hover:bg-black/60"
              : "text-text-muted hover:bg-surface-muted hover:text-text",
          )}
        >
          <X aria-hidden="true" className="h-5 w-5" />
        </DialogClose>
      </DialogContent>
    </Dialog>
  );
}

function AnnouncementCta({
  label,
  url,
  onFollow,
}: {
  label: string;
  url: string;
  onFollow: () => void;
}): React.JSX.Element {
  const { t } = useTranslation();
  const classes = cn(buttonVariants({ variant: "primary" }), "min-h-[44px]");
  if (isInternalBannerUrl(url)) {
    const target = new URL(url, "https://internal.invalid");
    return (
      <Link
        to={target.pathname}
        search={Object.fromEntries(target.searchParams.entries())}
        {...(target.hash ? { hash: target.hash.slice(1) } : {})}
        onClick={onFollow}
        className={classes}
      >
        {label}
      </Link>
    );
  }
  return (
    <a
      href={url}
      target="_blank"
      rel="noopener noreferrer"
      onClick={onFollow}
      className={classes}
    >
      {label}
      <ExternalLink aria-hidden="true" className="h-4 w-4" />
      <span className="sr-only">{t("banners.opensInNewTab")}</span>
    </a>
  );
}
