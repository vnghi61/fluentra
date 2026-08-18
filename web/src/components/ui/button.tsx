import * as React from "react";
import { Loader2 } from "lucide-react";
import { cn } from "@/lib/utils";

export interface ButtonProps
  extends React.ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: ("primary" | "secondary" | "outline" | "ghost" | "destructive") | undefined;
  size?: ("sm" | "md" | "lg") | undefined;
  isLoading?: boolean | undefined;
}

export const Button = React.forwardRef<HTMLButtonElement, ButtonProps>(
  (
    {
      className,
      variant = "primary",
      size = "md",
      isLoading = false,
      disabled,
      children,
      type = "button",
      ...props
    },
    ref,
  ) => {
    const baseStyles =
      "inline-flex items-center justify-center font-medium transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-indigo-500 focus-visible:ring-offset-2 disabled:pointer-events-none disabled:opacity-50 select-none cursor-pointer rounded-lg";

    const variantStyles = {
      primary:
        "bg-indigo-600 text-white hover:bg-indigo-700 active:bg-indigo-800 shadow-sm",
      secondary:
        "bg-slate-800 text-slate-100 hover:bg-slate-700 active:bg-slate-600 border border-slate-700",
      outline:
        "border border-slate-700 bg-transparent text-slate-200 hover:bg-slate-800 active:bg-slate-700",
      ghost:
        "bg-transparent text-slate-300 hover:bg-slate-800 hover:text-slate-100",
      destructive:
        "bg-rose-600 text-white hover:bg-rose-700 active:bg-rose-800 shadow-sm",
    };

    const sizeStyles = {
      sm: "min-h-[44px] px-3 text-xs gap-1.5",
      md: "min-h-[44px] px-4 text-sm gap-2",
      lg: "min-h-[48px] px-6 text-base gap-2.5",
    };

    return (
      <button
        ref={ref}
        type={type}
        disabled={disabled || isLoading}
        aria-busy={isLoading}
        className={cn(
          baseStyles,
          variantStyles[variant],
          sizeStyles[size],
          className,
        )}
        {...props}
      >
        {isLoading && <Loader2 className="h-4 w-4 animate-spin shrink-0" />}
        {children}
      </button>
    );
  },
);

Button.displayName = "Button";
