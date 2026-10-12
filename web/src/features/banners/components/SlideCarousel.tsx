import * as React from "react";
import { useTranslation } from "react-i18next";
import { ChevronLeft, ChevronRight, Pause, Play } from "lucide-react";

import { useMediaQuery } from "@/hooks/useMediaQuery";
import { cn } from "@/lib/utils";
import type { ActiveBanner } from "../model/banner";
import { BannerSlide } from "./BannerSlide";

/** How long each slide shows before the carousel moves on by itself. */
export const SLIDE_AUTOPLAY_MS = 6000;
/** A horizontal swipe shorter than this is a tap or a wobble, not a swipe. */
const SWIPE_THRESHOLD_PX = 40;

const REDUCED_MOTION = "(prefers-reduced-motion: reduce)";

/**
 * The dashboard's promotional slides, image only, always shown while live.
 *
 * With two or more it is a carousel (WAI-ARIA APG pattern): it moves on every
 * six seconds, and can be moved with the previous/next buttons, the dots, or a
 * swipe. Rotation pauses while the pointer is over it, while focus is inside
 * it and while the tab is hidden, stops for good once the learner presses
 * Pause, and never starts under prefers-reduced-motion. Every slide has the
 * same fixed frame, so moving between them never changes the page's height.
 */
export function SlideCarousel({
  slides,
}: {
  slides: ActiveBanner[];
}): React.JSX.Element {
  const { t } = useTranslation();
  const reducedMotion = useMediaQuery(REDUCED_MOTION, false);
  const total = slides.length;
  const many = total > 1;

  const [index, setIndex] = React.useState(0);
  // The list can shrink when a slide ends; clamp rather than reset.
  const position = Math.min(index, total - 1);

  const [userPaused, setUserPaused] = React.useState(false);
  const [hovering, setHovering] = React.useState(false);
  const [focusWithin, setFocusWithin] = React.useState(false);
  const [pageHidden, setPageHidden] = React.useState(
    () => typeof document !== "undefined" && document.hidden,
  );
  const [dragPx, setDragPx] = React.useState<number | null>(null);

  const go = React.useCallback(
    (next: number) => setIndex(((next % total) + total) % total),
    [total],
  );

  React.useEffect(() => {
    const onVisibility = () => setPageHidden(document.hidden);
    document.addEventListener("visibilitychange", onVisibility);
    return () => document.removeEventListener("visibilitychange", onVisibility);
  }, []);

  const rotating =
    many &&
    !reducedMotion &&
    !userPaused &&
    !hovering &&
    !focusWithin &&
    !pageHidden &&
    dragPx === null;

  React.useEffect(() => {
    if (!rotating) return;
    // Restarts on every move, so a slide always gets its full six seconds.
    const timer = window.setTimeout(() => go(position + 1), SLIDE_AUTOPLAY_MS);
    return () => window.clearTimeout(timer);
  }, [rotating, position, go]);

  // ── Swipe ───────────────────────────────────────────────────────────────
  // Touch and pen only: a mouse drag would fight text selection, and the
  // desktop has the buttons. `touch-action: pan-y` leaves vertical scrolling
  // to the browser and hands horizontal movement to us.
  const gesture = React.useRef<{
    id: number;
    x: number;
    y: number;
    horizontal: boolean | null;
  } | null>(null);
  const swiped = React.useRef(false);
  const viewportRef = React.useRef<HTMLDivElement>(null);

  const onPointerDown = (event: React.PointerEvent) => {
    if (!many || event.pointerType === "mouse") return;
    gesture.current = {
      id: event.pointerId,
      x: event.clientX,
      y: event.clientY,
      horizontal: null,
    };
    swiped.current = false;
  };

  const onPointerMove = (event: React.PointerEvent) => {
    const g = gesture.current;
    if (!g || g.id !== event.pointerId) return;
    const dx = event.clientX - g.x;
    const dy = event.clientY - g.y;
    if (g.horizontal === null) {
      if (Math.abs(dx) < 8 && Math.abs(dy) < 8) return;
      g.horizontal = Math.abs(dx) > Math.abs(dy);
      if (!g.horizontal) {
        gesture.current = null; // a scroll: let the page have it
        return;
      }
      viewportRef.current?.setPointerCapture?.(event.pointerId);
    }
    setDragPx(dx);
  };

  const endGesture = (event: React.PointerEvent) => {
    const g = gesture.current;
    gesture.current = null;
    if (g?.id === event.pointerId && g.horizontal) {
      const dx = event.clientX - g.x;
      if (Math.abs(dx) >= SWIPE_THRESHOLD_PX) {
        swiped.current = true;
        go(position + (dx < 0 ? 1 : -1));
      }
    }
    setDragPx(null);
  };

  const onPointerCancel = () => {
    gesture.current = null;
    setDragPx(null);
  };

  const slideNodes = slides.map((slide, i) => {
    const current = i === position;
    return (
      <div
        key={slide.id}
        role={many ? "group" : undefined}
        aria-roledescription={many ? t("banners.slide") : undefined}
        aria-label={
          many ? t("banners.slideLabel", { index: i + 1, total }) : undefined
        }
        aria-hidden={current ? undefined : true}
        // Off-screen slides stay out of the tab order and the pointer's way.
        inert={!current}
        className="w-full min-w-0 shrink-0"
      >
        <BannerSlide slide={slide} priority={i === 0} />
      </div>
    );
  });

  if (!many) {
    return (
      <section aria-label={t("banners.slidesLabel")} className="@container">
        {slideNodes}
      </section>
    );
  }

  // Controls sit on the picture (as on most promo carousels), so they carry
  // their own dark, blurred backdrop to stay legible over any image. Each
  // keeps a 44px hit area even where the visible mark is smaller.
  const overlayButton =
    "pointer-events-auto flex h-11 w-11 items-center justify-center rounded-full bg-black/35 text-white shadow-sm backdrop-blur-sm transition-colors hover:bg-black/55 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-white motion-reduce:transition-none";

  return (
    <section
      aria-label={t("banners.slidesLabel")}
      aria-roledescription={t("banners.carousel")}
      onMouseEnter={() => setHovering(true)}
      onMouseLeave={() => setHovering(false)}
      onFocus={() => setFocusWithin(true)}
      onBlur={(event) => {
        if (!event.currentTarget.contains(event.relatedTarget)) {
          setFocusWithin(false);
        }
      }}
      className="@container relative"
    >
      <div
        ref={viewportRef}
        // Announce a change only when the learner caused it; a carousel that
        // rotates on its own must not talk over the page.
        aria-live={rotating ? "off" : "polite"}
        className="touch-pan-y select-none overflow-hidden rounded-2xl"
        onPointerDown={onPointerDown}
        onPointerMove={onPointerMove}
        onPointerUp={endGesture}
        onPointerCancel={onPointerCancel}
        onClickCapture={(event) => {
          // The click that ends a swipe is not a tap on the slide's link.
          if (swiped.current) {
            swiped.current = false;
            event.preventDefault();
            event.stopPropagation();
          }
        }}
      >
        <div
          className={cn(
            "flex",
            dragPx === null &&
              "transition-transform duration-500 ease-[var(--ease-nav)] motion-reduce:transition-none",
          )}
          style={{
            transform:
              dragPx === null
                ? `translateX(${-position * 100}%)`
                : `translateX(calc(${-position * 100}% + ${dragPx}px))`,
          }}
        >
          {slideNodes}
        </div>
      </div>

      <div className="pointer-events-none absolute inset-y-0 left-2 flex items-center @2xl:left-4">
        <button
          type="button"
          onClick={() => go(position - 1)}
          aria-label={t("banners.previous")}
          className={overlayButton}
        >
          <ChevronLeft aria-hidden="true" className="h-5 w-5" />
        </button>
      </div>
      <div className="pointer-events-none absolute inset-y-0 right-2 flex items-center @2xl:right-4">
        <button
          type="button"
          onClick={() => go(position + 1)}
          aria-label={t("banners.next")}
          className={overlayButton}
        >
          <ChevronRight aria-hidden="true" className="h-5 w-5" />
        </button>
      </div>

      <div className="pointer-events-none absolute inset-x-0 bottom-0 flex items-end justify-center">
        <div
          role="group"
          aria-label={t("banners.pickerLabel")}
          className="pointer-events-auto flex items-center"
        >
          {slides.map((slide, i) => (
            <button
              key={slide.id}
              type="button"
              onClick={() => go(i)}
              aria-label={t("banners.showBanner", { index: i + 1, total })}
              aria-current={i === position ? "true" : undefined}
              className="group flex h-11 w-11 items-center justify-center rounded-full focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-white"
            >
              <span
                aria-hidden="true"
                className={cn(
                  "relative block h-2 overflow-hidden rounded-full shadow-[0_0_0_1px_rgb(0_0_0/0.2)] transition-[width,background-color] duration-200 motion-reduce:transition-none",
                  i === position
                    ? "w-6 bg-white/45"
                    : "w-2 bg-white/70 group-hover:bg-white",
                )}
              >
                {/* The current dot fills over the six seconds, so the learner
                  can see when it will move; a full dot when it will not. */}
                {i === position && (
                  <span
                    key={`${position}-${rotating}`}
                    className={cn(
                      "absolute inset-y-0 left-0 rounded-full bg-white",
                      rotating ? "banner-dot-progress" : "w-full",
                    )}
                    style={
                      rotating
                        ? { animationDuration: `${SLIDE_AUTOPLAY_MS}ms` }
                        : undefined
                    }
                  />
                )}
              </span>
            </button>
          ))}
        </div>
      </div>

      {!reducedMotion && (
        <div className="pointer-events-none absolute bottom-0 right-1 flex @2xl:right-3">
          <button
            type="button"
            onClick={() => setUserPaused((value) => !value)}
            aria-label={userPaused ? t("banners.play") : t("banners.pause")}
            className="group pointer-events-auto flex h-11 w-11 items-center justify-center rounded-full focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-white"
          >
            <span
              aria-hidden="true"
              className="flex h-7 w-7 items-center justify-center rounded-full bg-black/35 text-white backdrop-blur-sm transition-colors group-hover:bg-black/55 motion-reduce:transition-none"
            >
              {userPaused ? (
                <Play className="h-3.5 w-3.5" />
              ) : (
                <Pause className="h-3.5 w-3.5" />
              )}
            </span>
          </button>
        </div>
      )}
    </section>
  );
}
