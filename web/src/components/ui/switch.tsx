import React from "react";

import { cn } from "@/lib/utils";

export interface SwitchProps extends Omit<
  React.ButtonHTMLAttributes<HTMLButtonElement>,
  "onChange"
> {
  checked: boolean;
  onCheckedChange: (checked: boolean) => void;
}

/**
 * A binary on/off control. Checkboxes are for picking several of a set (the
 * notification channels); a setting that is simply on or off is a switch.
 */
export const Switch = React.forwardRef<HTMLButtonElement, SwitchProps>(
  ({ checked, onCheckedChange, className, disabled, ...props }, ref) => (
    <button
      ref={ref}
      type="button"
      role="switch"
      aria-checked={checked}
      disabled={disabled}
      onClick={() => onCheckedChange(!checked)}
      className={cn(
        // 44 px hit area around a 24 px track.
        "group relative inline-flex h-11 w-14 shrink-0 cursor-pointer items-center justify-center rounded-full focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary focus-visible:ring-offset-2 focus-visible:ring-offset-surface disabled:cursor-not-allowed disabled:opacity-50",
        className,
      )}
      {...props}
    >
      <span
        aria-hidden="true"
        className={cn(
          "relative h-6 w-11 rounded-full transition-colors duration-150 motion-reduce:transition-none",
          checked ? "bg-primary" : "bg-border",
        )}
      >
        <span
          className={cn(
            "absolute left-0.5 top-0.5 h-5 w-5 rounded-full bg-white shadow-sm transition-transform duration-150 motion-reduce:transition-none",
            checked && "translate-x-5",
          )}
        />
      </span>
    </button>
  ),
);
Switch.displayName = "Switch";
