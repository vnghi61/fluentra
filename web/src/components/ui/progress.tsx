import * as React from "react";
import { cn } from "@/lib/utils";

export interface ProgressBaseProps extends React.HTMLAttributes<HTMLDivElement> {
  value?: number | undefined;
  min?: number | undefined;
  max?: number | undefined;
  getValueLabel?: ((value: number, max: number) => string) | undefined;
  variant?: ("primary" | "success" | "warning" | "danger") | undefined;
}

export type ProgressProps = ProgressBaseProps &
  ({ "aria-label": string } | { "aria-labelledby": string });

const indicatorVariants = {
  primary: "bg-primary",
  success: "bg-success",
  warning: "bg-warning",
  danger: "bg-danger",
};

export const Progress = React.forwardRef<HTMLDivElement, ProgressProps>(
  (
    {
      className,
      value = 0,
      min = 0,
      max = 100,
      getValueLabel,
      variant = "primary",
      ...props
    },
    ref,
  ) => {
    const safeMin = Math.min(min, max);
    const safeMax = Math.max(min, max);
    const range = safeMax - safeMin || 1;
    const clampedValue = Math.min(Math.max(value, safeMin), safeMax);
    const percentage = Math.round(((clampedValue - safeMin) / range) * 100);

    const valueLabel = getValueLabel
      ? getValueLabel(clampedValue, safeMax)
      : `${percentage}%`;

    return (
      <div
        ref={ref}
        role="progressbar"
        aria-valuemin={safeMin}
        aria-valuemax={safeMax}
        aria-valuenow={clampedValue}
        aria-valuetext={props["aria-valuetext"] ?? valueLabel}
        className={cn(
          "relative h-2.5 w-full overflow-hidden rounded-full bg-surface-muted border border-border-subtle",
          className,
        )}
        {...props}
      >
        <div
          className={cn(
            "h-full w-full flex-1 transition-all duration-300 ease-in-out motion-reduce:transition-none",
            indicatorVariants[variant],
          )}
          style={{ transform: `translateX(-${100 - percentage}%)` }}
        />
      </div>
    );
  },
);

Progress.displayName = "Progress";
