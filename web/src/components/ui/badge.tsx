import * as React from "react";
import { cn } from "@/lib/utils";

export interface BadgeProps extends React.HTMLAttributes<HTMLSpanElement> {
  variant?:
    | "primary"
    | "secondary"
    | "outline"
    | "success"
    | "warning"
    | "danger"
    | undefined;
  icon?: React.ReactNode | undefined;
}

export const badgeVariants = {
  primary: "border-primary/20 bg-transparent text-primary-accent",
  secondary: "border-border-subtle bg-surface-muted text-text-muted",
  outline: "border-border-subtle bg-transparent text-text",
  success: "border-success/20 bg-transparent text-success-accent",
  warning: "border-warning/20 bg-transparent text-warning-accent",
  danger: "border-danger/20 bg-transparent text-danger-accent",
};

export const Badge = React.forwardRef<HTMLSpanElement, BadgeProps>(
  ({ className, variant = "primary", icon, children, ...props }, ref) => {
    return (
      <span
        ref={ref}
        className={cn(
          "inline-flex items-center gap-1 rounded-full border px-2.5 py-0.5 text-xs font-medium transition-colors select-none",
          badgeVariants[variant],
          className,
        )}
        {...props}
      >
        {icon && (
          <span className="shrink-0 flex items-center" aria-hidden="true">
            {icon}
          </span>
        )}
        {children}
      </span>
    );
  },
);

Badge.displayName = "Badge";
