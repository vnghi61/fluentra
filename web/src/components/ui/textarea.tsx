import * as React from "react";
import { cn } from "@/lib/utils";

export interface TextareaProps extends React.TextareaHTMLAttributes<HTMLTextAreaElement> {
  error?: boolean | undefined;
}

/** The multi-line sibling of `Input`: same border, focus ring and 16px text. */
export const Textarea = React.forwardRef<HTMLTextAreaElement, TextareaProps>(
  ({ className, error = false, rows = 3, ...props }, ref) => (
    <textarea
      ref={ref}
      rows={rows}
      aria-invalid={error ? "true" : undefined}
      className={cn(
        // text-base (16px) so iOS Safari does not zoom the viewport on focus.
        "flex w-full rounded-lg border bg-surface px-3.5 py-2.5 text-base leading-relaxed text-text placeholder:text-text-muted transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-offset-2 disabled:cursor-not-allowed disabled:opacity-50 md:text-sm",
        error
          ? "border-danger focus-visible:ring-danger focus-visible:ring-offset-surface"
          : "border-border focus-visible:border-primary focus-visible:ring-primary focus-visible:ring-offset-surface",
        className,
      )}
      {...props}
    />
  ),
);

Textarea.displayName = "Textarea";
