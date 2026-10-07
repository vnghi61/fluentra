import * as React from "react";
import { Inbox, type LucideIcon } from "lucide-react";
import { useTranslation } from "react-i18next";

import { cn } from "@/lib/utils";

export interface EmptyStateProps {
  title?: string;
  description?: string;
  icon?: LucideIcon;
  action?: React.ReactNode;
  className?: string;
  variant?: "default" | "compact" | "card";
}

export const EmptyState = React.forwardRef<HTMLDivElement, EmptyStateProps>(
  ({ title, description, icon: Icon = Inbox, action, className, variant = "default" }, ref) => {
    const { t } = useTranslation();

    const resolvedTitle = title ?? t("states.emptyTitle", "No items found");

    return (
      <div
        ref={ref}
        className={cn(
          "flex flex-col items-center justify-center text-center",
          variant === "default" ? "px-6 py-12" : "px-4 py-10",
          variant === "card" && "rounded-xl border border-border-subtle bg-surface-card",
          className,
        )}
      >
        <div className={cn("flex items-center justify-center rounded-xl bg-primary/10 text-primary-accent", variant === "default" ? "h-12 w-12" : "h-10 w-10")}>
          <Icon className={cn("", variant === "default" ? "h-6 w-6" : "h-5 w-5")} aria-hidden="true" />
        </div>
        <h3 className={cn("font-semibold text-text", variant === "default" ? "mt-4 text-base" : "mt-3 text-base")}>
          {resolvedTitle}
        </h3>
        {description && (
          <p className="mt-1.5 max-w-sm text-sm text-text-muted">
            {description}
          </p>
        )}
        {action && (
          <div className="mt-6 flex flex-wrap items-center justify-center gap-3">
            {action}
          </div>
        )}
      </div>
    );
  },
);

EmptyState.displayName = "EmptyState";
