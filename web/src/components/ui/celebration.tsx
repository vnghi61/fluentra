import * as React from "react";
import { cn } from "@/lib/utils";

export interface CelebrationProps {
  active: boolean;
  pieces?: number;
  onDone?: () => void;
  className?: string;
}

const CONFETTI_COLORS = [
  "bg-primary",
  "bg-success",
  "bg-warning",
  "bg-brand",
] as const;

function isPrefersReducedMotion(): boolean {
  if (
    typeof window === "undefined" ||
    typeof window.matchMedia !== "function"
  ) {
    return false;
  }
  return window.matchMedia("(prefers-reduced-motion: reduce)").matches;
}

/**
 * A lightweight CSS-only celebration (confetti) component.
 *
 * NOTE: This component does not import any motion libraries to protect bundle size.
 * It renders deterministic pseudo-random confetti pieces styled with existing design
 * tokens and driven by Tailwind keyframe animations.
 */
export const Celebration: React.FC<CelebrationProps> = ({
  active,
  pieces = 24,
  onDone,
  className,
}) => {
  const reduceMotion = React.useMemo(() => isPrefersReducedMotion(), []);

  React.useEffect(() => {
    if (!active) return;

    if (reduceMotion) {
      onDone?.();
      return;
    }

    if (!onDone) return;

    const timer = setTimeout(() => {
      onDone();
    }, 1600);

    return () => {
      clearTimeout(timer);
    };
  }, [active, reduceMotion, onDone]);

  if (!active || reduceMotion) {
    return null;
  }

  return (
    <div
      aria-hidden="true"
      className={cn(
        "pointer-events-none fixed inset-0 z-50 overflow-hidden",
        className,
      )}
    >
      {Array.from({ length: pieces }, (_, i) => {
        const left = `${((i * 29 + 7) % 94) + 3}%`;
        const delay = `${(i * 23) % 200}ms`;
        const rotation = `${(i * 47) % 360}deg`;
        const color = CONFETTI_COLORS[i % CONFETTI_COLORS.length];
        const sizeClass =
          i % 2 === 0 ? "h-3 w-2 rounded-xs" : "h-2.5 w-2.5 rounded-sm";

        return (
          <div
            key={i}
            aria-hidden="true"
            className={cn("absolute -top-4 animate-confetti", color, sizeClass)}
            style={{
              left,
              animationDelay: delay,
              transform: `rotate(${rotation})`,
            }}
          />
        );
      })}
    </div>
  );
};

Celebration.displayName = "Celebration";
