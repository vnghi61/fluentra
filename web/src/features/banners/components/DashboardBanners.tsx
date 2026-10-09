import * as React from "react";

import { useAuthStore } from "@/stores/authStore";
import { useActiveBanners } from "../api/useBanners";
import {
  readDismissedBanners,
  readSessionSeenBanners,
  rememberDismissedBanner,
  rememberSessionSeenBanner,
} from "../model/banner";
import { AnnouncementDialog } from "./AnnouncementDialog";
import { SlideCarousel } from "./SlideCarousel";

/**
 * What the dashboard shows from the admin banner screen, by kind:
 *
 * - **Slides** sit in a carousel at the top of the page for as long as they
 *   are live. They are pictures, and there is nothing to dismiss.
 * - **Announcements** open as a dialog when the dashboard does, one per visit,
 *   highest priority first. Closing a dismissible one hides it for good on
 *   this device; a non-dismissible one comes back on the next visit (a new
 *   browser session), never twice in one.
 *
 * Nothing is drawn until the list arrives, and nothing if it fails or is
 * empty, so the dashboard never shows an error for what is only an
 * announcement.
 */
export interface DashboardBannersProps {
  /**
   * Which surface to draw, so the page can place the carousel and the dialog
   * independently. Both by default.
   */
  part?: "all" | "slides" | "announcements" | undefined;
}

export function DashboardBanners({
  part = "all",
}: DashboardBannersProps): React.JSX.Element | null {
  const userId = useAuthStore((s) => s.user?.userId) ?? "anonymous";
  // Keyed by user so a different account on the same device starts from its
  // own dismissals rather than the previous account's.
  return <BannerSurfaces key={userId} userId={userId} part={part} />;
}

function BannerSurfaces({
  userId,
  part,
}: {
  userId: string;
  part: "all" | "slides" | "announcements";
}): React.JSX.Element | null {
  const { data } = useActiveBanners();
  const items = data?.items ?? [];

  // Read once per dashboard visit: closing one announcement must not open the
  // next straight away.
  const [hidden] = React.useState<ReadonlySet<string>>(
    () =>
      new Set([
        ...readDismissedBanners(userId),
        ...readSessionSeenBanners(userId),
      ]),
  );
  const [closedId, setClosedId] = React.useState<string | null>(null);

  const slides =
    part === "announcements"
      ? []
      : items.filter((banner) => banner.kind === "slide");
  const announcement =
    closedId === null && part !== "slides"
      ? items.find(
          (banner) => banner.kind !== "slide" && !hidden.has(banner.id),
        )
      : undefined;
  // Kept mounted through the closing animation.
  const shown =
    part === "slides"
      ? null
      : (announcement ??
        items.find((banner) => banner.id === closedId) ??
        null);

  const close = () => {
    if (!shown || closedId !== null) return;
    if (shown.dismissible) rememberDismissedBanner(userId, shown.id);
    rememberSessionSeenBanner(userId, shown.id);
    setClosedId(shown.id);
  };

  if (slides.length === 0 && !shown) return null;

  return (
    <>
      {slides.length > 0 && <SlideCarousel slides={slides} />}
      {shown && shown.kind !== "slide" && (
        <AnnouncementDialog
          announcement={shown}
          open={closedId === null}
          onClose={close}
        />
      )}
    </>
  );
}
