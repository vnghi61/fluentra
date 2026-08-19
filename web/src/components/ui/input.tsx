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
          "flex h-11 w-full rounded-lg border bg-slate-900/60 px-3.5 py-2 text-base text-slate-100 placeholder:text-slate-500 transition-colors file:border-0 file:bg-transparent file:text-sm file:font-medium focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-offset-2 disabled:cursor-not-allowed disabled:opacity-50",
          error
            ? "border-rose-500 focus-visible:ring-rose-500 focus-visible:ring-offset-slate-950"
            : "border-slate-700 focus-visible:border-indigo-500 focus-visible:ring-indigo-500 focus-visible:ring-offset-slate-950",
          className,
        )}
        {...props}
      />
    );
  },
);

Input.displayName = "Input";
