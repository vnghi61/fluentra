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
}

export const EmptyState = React.forwardRef<HTMLDivElement, EmptyStateProps>(
  ({ title, description, icon: Icon = Inbox, action, className }, ref) => {
    const { t } = useTranslation();

    const resolvedTitle = title ?? t("states.emptyTitle", "No items found");

    return (
      <div
        ref={ref}
        className={cn(
          "flex flex-col items-center justify-center p-8 text-center",
          className,
        )}
      >
        <div className="flex h-12 w-12 items-center justify-center rounded-xl bg-primary/10 text-primary-accent">
          <Icon className="h-6 w-6" aria-hidden="true" />
        </div>
        <h3 className="mt-4 text-base font-semibold text-text">
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
