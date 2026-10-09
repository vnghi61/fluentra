import React from "react";

import { cn } from "@/lib/utils";

/** The "View details →" style link in a card header: text-sized, 44px tall. */
export const cardLinkClass =
  "inline-flex min-h-[44px] shrink-0 items-center whitespace-nowrap rounded-md px-1 text-sm font-medium text-primary-accent hover:underline focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary";

export interface CardHeadingProps {
  title: React.ReactNode;
  /** Small uppercase label above the title. */
  eyebrow?: React.ReactNode;
  description?: React.ReactNode | undefined;
  /** Usually a link built with `cardLinkClass`; a chip or select also fits. */
  action?: React.ReactNode;
  /** Heading level: the page title is the h1, cards are h2. */
  as?: "h2" | "h3";
  id?: string;
  className?: string;
}

/** Title row shared by the Dashboard cards so they line up with each other. */
export function CardHeading({
  title,
  eyebrow,
  description,
  action,
  as: Heading = "h2",
  id,
  className,
}: CardHeadingProps): React.JSX.Element {
  return (
    <div className={cn("flex items-start justify-between gap-3", className)}>
      <div className="min-w-0 space-y-1">
        {eyebrow && (
          <p className="text-xs font-semibold uppercase tracking-wider text-primary-accent">
            {eyebrow}
          </p>
        )}
        <Heading
          id={id}
          className="text-base font-bold tracking-tight text-text md:text-lg"
        >
          {title}
        </Heading>
        {description && (
          <p className="text-sm text-text-muted">{description}</p>
        )}
      </div>
      {action && <div className="-mt-2.5 shrink-0">{action}</div>}
    </div>
  );
}
