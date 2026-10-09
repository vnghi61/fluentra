import * as React from "react";
import { Link } from "@tanstack/react-router";
import { ChevronLeft } from "lucide-react";

import { cn } from "@/lib/utils";

export interface PageHeaderProps {
  title: React.ReactNode;
  description?: React.ReactNode;
  /** Small chips above the title: a section badge, a status. */
  eyebrow?: React.ReactNode;
  /** Buttons on the right on desktop, below the title on mobile. */
  actions?: React.ReactNode;
  /** A back link above everything else ("‹ Studio"). */
  back?: { to: string; label: string } | undefined;
  className?: string;
}

/**
 * The one page header: optional back link, eyebrow chips, an H1 sized
 * 24/30 px (mobile/md+, on the type scale), a muted description and the page actions.
 * Every top-level page uses it so titles line up across the app.
 */
export function PageHeader({
  title,
  description,
  eyebrow,
  actions,
  back,
  className,
}: PageHeaderProps): React.JSX.Element {
  return (
    <header className={cn("space-y-3", className)}>
      {back && (
        <Link
          to={back.to}
          className="-ml-1.5 inline-flex min-h-11 items-center gap-1 rounded-lg px-1.5 text-sm font-medium text-text-muted transition-colors hover:text-text focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary"
        >
          <ChevronLeft className="h-4 w-4" aria-hidden="true" />
          {back.label}
        </Link>
      )}
      <div className="flex flex-col gap-4 lg:flex-row lg:items-end lg:justify-between">
        <div className="min-w-0 space-y-1.5">
          {eyebrow && (
            <div className="flex flex-wrap items-center gap-2">{eyebrow}</div>
          )}
          <h1 className="break-words text-2xl font-bold leading-tight tracking-tight text-text md:text-3xl">
            {title}
          </h1>
          {description && (
            <p className="max-w-prose text-base leading-relaxed text-text-muted">
              {description}
            </p>
          )}
        </div>
        {actions && (
          <div className="flex flex-wrap items-center gap-2 sm:gap-3 lg:shrink-0">
            {actions}
          </div>
        )}
      </div>
    </header>
  );
}
