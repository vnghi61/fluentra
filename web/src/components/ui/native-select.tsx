import * as React from "react";
import { ChevronDown } from "lucide-react";
import { cn } from "@/lib/utils";

export interface NativeSelectProps extends React.SelectHTMLAttributes<HTMLSelectElement> {
  error?: boolean | undefined;
  /** Classes for the wrapper, which carries the width. */
  wrapperClassName?: string | undefined;
}

/**
 * A styled native `<select>`. Native on purpose: phones open their own picker,
 * keyboard and screen-reader support come free, and it costs no bundle bytes.
 * Reach for `Combobox` only when the list needs search.
 */
export const NativeSelect = React.forwardRef<
  HTMLSelectElement,
  NativeSelectProps
>(({ className, wrapperClassName, error = false, children, ...props }, ref) => (
  <div className={cn("relative w-full", wrapperClassName)}>
    <select
      ref={ref}
      aria-invalid={error ? "true" : undefined}
      className={cn(
        "h-11 w-full cursor-pointer appearance-none rounded-lg border bg-surface pl-3.5 pr-10 text-base text-text transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-offset-2 disabled:cursor-not-allowed disabled:opacity-50 md:text-sm",
        error
          ? "border-danger focus-visible:ring-danger focus-visible:ring-offset-surface"
          : "border-border focus-visible:border-primary focus-visible:ring-primary focus-visible:ring-offset-surface",
        className,
      )}
      {...props}
    >
      {children}
    </select>
    <ChevronDown
      className="pointer-events-none absolute right-3 top-1/2 h-4 w-4 -translate-y-1/2 text-text-muted"
      aria-hidden="true"
    />
  </div>
));

NativeSelect.displayName = "NativeSelect";
