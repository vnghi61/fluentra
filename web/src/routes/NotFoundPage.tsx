import * as React from "react";
import { Link } from "@tanstack/react-router";
import { BookOpen, Target } from "lucide-react";
import { useTranslation } from "react-i18next";

function canTilt(): boolean {
  if (
    typeof window === "undefined" ||
    typeof window.matchMedia !== "function"
  ) {
    return false;
  }
  const isFine = window.matchMedia("(pointer: fine)").matches;
  const isReduced = window.matchMedia(
    "(prefers-reduced-motion: reduce)",
  ).matches;
  return isFine && !isReduced;
}

export function NotFoundPage(): React.JSX.Element {
  const { t } = useTranslation();
  const [tilt, setTilt] = React.useState<string | undefined>(undefined);

  const handlePointerMove = (e: React.PointerEvent<HTMLDivElement>) => {
    if (e.pointerType !== "mouse" || !canTilt()) return;
    const rect = e.currentTarget.getBoundingClientRect();
    const centerX = rect.width / 2;
    const centerY = rect.height / 2;
    if (centerX <= 0 || centerY <= 0) return;

    const normX = Math.max(
      -1,
      Math.min(1, (e.clientX - rect.left - centerX) / centerX),
    );
    const normY = Math.max(
      -1,
      Math.min(1, (e.clientY - rect.top - centerY) / centerY),
    );

    const maxTilt = 8;
    const rotateX = -normY * maxTilt;
    const rotateY = normX * maxTilt;
    setTilt(
      `perspective(600px) rotateX(${rotateX.toFixed(2)}deg) rotateY(${rotateY.toFixed(2)}deg)`,
    );
  };

  const handlePointerLeave = () => {
    setTilt(undefined);
  };

  return (
    <div className="flex w-full flex-col items-center justify-center overflow-x-hidden px-4 py-12 text-center">
      {/* 3D Tiltable & Floating Geometric SVG/CSS Illustration */}
      <div
        onPointerMove={handlePointerMove}
        onPointerLeave={handlePointerLeave}
        style={tilt ? { transform: tilt } : undefined}
        className="relative flex items-center justify-center transition-transform duration-150 ease-out"
      >
        <div className="motion-safe:animate-float flex h-48 w-48 items-center justify-center sm:h-56 sm:w-56">
          <svg
            viewBox="0 0 200 200"
            fill="none"
            aria-hidden="true"
            className="h-full w-full drop-shadow-md select-none"
          >
            {/* Outer dotted orbit */}
            <circle
              cx="100"
              cy="100"
              r="84"
              stroke="currentColor"
              strokeWidth="2"
              strokeDasharray="6 6"
              className="text-border-subtle"
            />
            {/* Gradient background plate */}
            <circle
              cx="100"
              cy="100"
              r="68"
              className="fill-surface-card stroke-border-subtle"
              strokeWidth="1.5"
            />
            {/* Inner abstract geometric shapes */}
            <rect
              x="65"
              y="65"
              width="70"
              height="70"
              rx="16"
              className="fill-primary/10 stroke-primary-accent"
              strokeWidth="2"
              transform="rotate(15 100 100)"
            />
            <circle
              cx="100"
              cy="100"
              r="24"
              className="fill-primary/20 stroke-primary"
              strokeWidth="2"
            />
            {/* Accents / Stars */}
            <circle cx="50" cy="70" r="4" className="fill-warning-accent" />
            <circle cx="150" cy="130" r="5" className="fill-primary-accent" />
            <polygon
              points="100,40 104,48 112,48 106,53 108,61 100,56 92,61 94,53 88,48 96,48"
              className="fill-primary-accent"
            />
          </svg>
        </div>
      </div>

      {/* Large 404 heading area */}
      <div className="mt-4 text-7xl font-extrabold tracking-tight text-primary-accent select-none sm:text-8xl">
        404
      </div>

      {/* Friendly copy */}
      <h1 className="mt-4 text-2xl font-bold tracking-tight text-text sm:text-3xl">
        {t("notFound.title", "This page wandered off")}
      </h1>
      <p className="mt-2 max-w-md text-base text-text-muted">
        {t(
          "notFound.description",
          "The page you are looking for does not exist or has moved.",
        )}
      </p>

      {/* Primary and secondary navigation buttons */}
      <div className="mt-8 flex flex-wrap items-center justify-center gap-3">
        <Link
          to="/"
          className="inline-flex min-h-[44px] items-center justify-center rounded-lg bg-primary px-5 py-2.5 text-sm font-semibold text-primary-fg shadow-sm transition-colors hover:bg-primary-hover focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary focus-visible:ring-offset-2"
        >
          {t("notFound.home", "Back to home")}
        </Link>
        <button
          type="button"
          onClick={() => window.history.back()}
          className="inline-flex min-h-[44px] items-center justify-center rounded-lg border border-border bg-transparent px-5 py-2.5 text-sm font-medium text-text transition-colors hover:bg-surface-muted cursor-pointer focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary focus-visible:ring-offset-2"
        >
          {t("notFound.back", "Go back")}
        </button>
      </div>

      {/* Quick links */}
      <div className="mt-10 flex flex-col items-center">
        <span className="text-xs font-semibold tracking-wider text-text-muted uppercase">
          {t("notFound.quickLinks", "Quick links")}
        </span>
        <div className="mt-3 flex flex-wrap items-center justify-center gap-3">
          <Link
            to="/learn"
            className="inline-flex min-h-[44px] items-center gap-2 rounded-xl border border-border-subtle bg-surface-card px-4 py-2 text-sm font-medium text-text shadow-xs transition-colors hover:bg-surface-muted"
          >
            <BookOpen
              className="h-4 w-4 text-primary-accent"
              aria-hidden="true"
            />
            {t("nav.learn", "Learn")}
          </Link>
          <Link
            to="/practice"
            className="inline-flex min-h-[44px] items-center gap-2 rounded-xl border border-border-subtle bg-surface-card px-4 py-2 text-sm font-medium text-text shadow-xs transition-colors hover:bg-surface-muted"
          >
            <Target
              className="h-4 w-4 text-primary-accent"
              aria-hidden="true"
            />
            {t("nav.practice", "Practice")}
          </Link>
        </div>
      </div>
    </div>
  );
}
