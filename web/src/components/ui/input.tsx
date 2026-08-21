import * as React from "react";
import { cn } from "@/lib/utils";

export interface InputProps extends React.InputHTMLAttributes<HTMLInputElement> {
  error?: boolean | undefined;
}

export const Input = React.forwardRef<HTMLInputElement, InputProps>(
  ({ className, type = "text", error = false, ...props }, ref) => {
    return (
      <input
        type={type}
        ref={ref}
        aria-invalid={error ? "true" : undefined}
        className={cn(
          // text-base (16px) is enforced so iOS Safari does not zoom the viewport on focus
          "flex h-11 w-full rounded-lg border bg-surface px-3.5 py-2 text-base text-text placeholder:text-text-muted transition-colors file:border-0 file:bg-transparent file:text-sm file:font-medium focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-offset-2 disabled:cursor-not-allowed disabled:opacity-50",
          error
            ? "border-danger focus-visible:ring-danger focus-visible:ring-offset-surface"
            : "border-border focus-visible:border-primary focus-visible:ring-primary focus-visible:ring-offset-surface",
          className,
        )}
        {...props}
      />
    );
  },
);

Input.displayName = "Input";
