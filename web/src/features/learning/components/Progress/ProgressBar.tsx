import * as React from "react";
import { cn } from "@/lib/utils";

export interface ProgressBarProps {
  value: number;
  label: string;
  barClassName?: string;
  className?: string;
}

function isPrefersReducedMotion(): boolean {
  if (
    typeof window === "undefined" ||
    typeof window.matchMedia !== "function"
  ) {
    return false;
  }
  return window.matchMedia("(prefers-reduced-motion: reduce)").matches;
}

export const ProgressBar: React.FC<ProgressBarProps> = ({
  value,
  label,
  barClassName = "bg-primary",
  className,
}) => {
  const clampedValue = Math.min(
    Math.max(Number.isFinite(value) ? value : 0, 0),
    100,
  );

  const reduceMotion = isPrefersReducedMotion();
  const [mounted, setMounted] = React.useState(reduceMotion);

  React.useEffect(() => {
    if (reduceMotion) return;
    const id = window.requestAnimationFrame(() => {
      setMounted(true);
    });
    return () => window.cancelAnimationFrame(id);
  }, [reduceMotion]);

  const width = mounted || reduceMotion ? `${clampedValue}%` : "0%";

  return (
    <div
      role="progressbar"
      aria-valuenow={clampedValue}
      aria-valuemin={0}
      aria-valuemax={100}
      aria-label={label}
      className={cn(
        "h-1.5 w-full overflow-hidden rounded-full bg-surface-muted",
        className,
      )}
    >
      <div
        className={cn(
          "h-full rounded-full transition-[width] duration-500 ease-out motion-reduce:transition-none",
          barClassName,
        )}
        style={{ width }}
      />
    </div>
  );
};
