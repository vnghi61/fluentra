import * as React from "react";

import { cn } from "@/lib/utils";

export interface ProgressRingProps {
  /** 0-100. Clamped; a non-finite value draws an empty ring. */
  value: number;
  /** Outer size in CSS px. */
  size?: number;
  strokeWidth?: number;
  /**
   * The accessible name of the progressbar. Omit it only together with
   * `decorative`, when the figure is already spelled out next to the ring.
   */
  label?: string;
  /** Hide the ring from assistive technology (the numbers are in text nearby). */
  decorative?: boolean;
  /** Content centred inside the ring: a percentage, a count. */
  children?: React.ReactNode;
  className?: string;
  /** Colour of the filled arc, as a text-* class (the stroke is currentColor). */
  indicatorClassName?: string;
  /** Colour of the track, as a text-* class. */
  trackClassName?: string;
}

/**
 * A circular progress figure drawn with two SVG circles. The arc grows with a
 * CSS transition that `motion-reduce` switches off; there is no JS animation.
 */
export function ProgressRing({
  value,
  size = 48,
  strokeWidth = 5,
  label,
  decorative = false,
  children,
  className,
  indicatorClassName = "text-primary",
  trackClassName = "text-surface-muted",
}: ProgressRingProps): React.JSX.Element {
  const clamped = Math.min(
    100,
    Math.max(0, Number.isFinite(value) ? value : 0),
  );
  const radius = (size - strokeWidth) / 2;
  const circumference = 2 * Math.PI * radius;
  const offset = circumference * (1 - clamped / 100);

  const a11y = decorative
    ? { "aria-hidden": true as const }
    : {
        role: "progressbar",
        "aria-valuenow": Math.round(clamped),
        "aria-valuemin": 0,
        "aria-valuemax": 100,
        "aria-label": label,
      };

  return (
    <div
      {...a11y}
      className={cn(
        "relative inline-flex shrink-0 items-center justify-center",
        className,
      )}
      style={{ width: size, height: size }}
    >
      <svg
        width={size}
        height={size}
        viewBox={`0 0 ${size} ${size}`}
        className="-rotate-90"
        aria-hidden="true"
        focusable="false"
      >
        <circle
          cx={size / 2}
          cy={size / 2}
          r={radius}
          fill="none"
          stroke="currentColor"
          strokeWidth={strokeWidth}
          className={trackClassName}
        />
        <circle
          cx={size / 2}
          cy={size / 2}
          r={radius}
          fill="none"
          stroke="currentColor"
          strokeWidth={strokeWidth}
          strokeLinecap="round"
          strokeDasharray={circumference}
          strokeDashoffset={offset}
          className={cn(
            "transition-[stroke-dashoffset] duration-500 ease-out motion-reduce:transition-none",
            clamped === 0 && "opacity-0",
            indicatorClassName,
          )}
        />
      </svg>
      {children !== undefined && (
        <span className="absolute inset-0 flex items-center justify-center text-center leading-none">
          {children}
        </span>
      )}
    </div>
  );
}
