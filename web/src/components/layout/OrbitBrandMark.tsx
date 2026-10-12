import * as React from "react";

import { BrandMark } from "./BrandMark";

export interface OrbitBrandMarkProps {
  size: "sm" | "md" | "lg";
  animated?: boolean;
}

const sizeClasses: Record<OrbitBrandMarkProps["size"], string> = {
  sm: "h-20 w-20 sm:h-24 sm:w-24",
  md: "h-32 w-32 sm:h-40 sm:w-40",
  lg: "h-32 w-32 sm:h-48 sm:w-48 md:h-56 md:w-56",
};

/** The Fluentra mark held in a quiet, orbiting field of learning energy. */
export function OrbitBrandMark({
  size,
  animated = true,
}: OrbitBrandMarkProps): React.JSX.Element {
  const motionClasses = animated
    ? [
        "motion-safe:animate-orbit-slow",
        "motion-safe:animate-orbit-slower",
        "motion-safe:animate-orbit-reverse",
      ]
    : [];

  return (
    <div
      data-testid="orbit-brand-mark"
      data-animated={animated ? "true" : "false"}
      className={`relative flex ${sizeClasses[size]} items-center justify-center rounded-full bg-[radial-gradient(circle,rgba(37,99,235,0.08),transparent_68%)] dark:drop-shadow-[0_0_18px_rgba(96,165,250,0.12)]`}
    >
      <svg
        viewBox="0 0 200 200"
        fill="none"
        aria-hidden="true"
        className="absolute inset-0 h-full w-full"
      >
        <g
          className={motionClasses[0]}
          style={{ transformBox: "view-box", transformOrigin: "50% 50%" }}
        >
          <ellipse
            cx="100"
            cy="100"
            rx="88"
            ry="42"
            className="stroke-primary/25"
            strokeWidth="1.5"
          />
          <circle
            cx="188"
            cy="100"
            r="3"
            className="fill-primary/60 dark:fill-primary"
          />
        </g>
        <g
          className={motionClasses[1]}
          style={{ transformBox: "view-box", transformOrigin: "50% 50%" }}
        >
          <ellipse
            cx="100"
            cy="100"
            rx="72"
            ry="88"
            className="stroke-primary/20 dark:stroke-primary/35"
            strokeWidth="1.5"
          />
          <circle
            cx="100"
            cy="12"
            r="2.5"
            className="fill-primary-accent/70 dark:fill-primary-accent"
          />
        </g>
        <g
          className={motionClasses[2]}
          style={{ transformBox: "view-box", transformOrigin: "50% 50%" }}
        >
          <ellipse
            cx="100"
            cy="100"
            rx="52"
            ry="92"
            className="stroke-primary/15 dark:stroke-primary-accent/30"
            strokeWidth="1.5"
          />
          <circle
            cx="48"
            cy="100"
            r="2"
            className="fill-primary/50 dark:fill-primary"
          />
        </g>
      </svg>
      <BrandMark
        decorative
        className="relative z-10 h-[35%] w-[35%] text-brand"
      />
    </div>
  );
}
