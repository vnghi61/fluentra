import * as React from "react";
import { AlertTriangle, type LucideIcon } from "lucide-react";
import { useTranslation } from "react-i18next";

import { cn } from "@/lib/utils";
import { Button } from "./button";

export interface ErrorStateProps {
  title?: string;
  description?: string;
  onRetry?: () => void;
  retryLabel?: string;
  icon?: LucideIcon;
  action?: React.ReactNode;
  className?: string;
}

export const ErrorState = React.forwardRef<HTMLDivElement, ErrorStateProps>(
  (
    {
      title,
      description,
      onRetry,
      retryLabel,
      icon: Icon = AlertTriangle,
      action,
      className,
    },
    ref,
  ) => {
    const { t } = useTranslation();

    const resolvedTitle =
      title ?? t("states.errorTitle", "Something went wrong");
    const resolvedDescription =
      description ??
      t(
        "states.errorDescription",
        "An unexpected error occurred. Please try again.",
      );

    return (
      <div
        ref={ref}
        role="alert"
        className={cn(
          "flex flex-col items-center justify-center p-8 text-center",
          className,
        )}
      >
        <div className="flex h-12 w-12 items-center justify-center rounded-xl bg-danger/10 text-danger-accent">
          <Icon className="h-6 w-6" aria-hidden="true" />
        </div>
        <h3 className="mt-4 text-base font-semibold text-text">
          {resolvedTitle}
        </h3>
        {resolvedDescription && (
          <p className="mt-1.5 max-w-sm text-sm text-text-muted">
            {resolvedDescription}
          </p>
        )}
        {(onRetry || action) && (
          <div className="mt-6 flex flex-wrap items-center justify-center gap-3">
            {onRetry && (
              <Button variant="outline" onClick={onRetry}>
                {retryLabel ?? t("app.tryAgain", "Try again")}
              </Button>
            )}
            {action}
          </div>
        )}
      </div>
    );
  },
);

ErrorState.displayName = "ErrorState";
