import * as React from "react";
import type { LucideIcon } from "lucide-react";

import { cn } from "@/lib/utils";

export interface StatTileProps {
  /** An icon drawn in a tinted tile. Ignored when `visual` is given. */
  icon?: LucideIcon;
  /** Tint of the icon tile, e.g. "bg-primary/10 text-primary-accent". */
  iconClassName?: string;
  /** Anything else in the icon's place: a progress ring, an avatar. */
  visual?: React.ReactNode;
  label: React.ReactNode;
  value: React.ReactNode;
  /** A secondary line under the figure. Only for real data. */
  hint?: React.ReactNode;
  /** Label above the value (KPI tiles) or below it (compact header tiles). */
  labelFirst?: boolean;
  /** "md" for KPI grids, "sm" for compact tiles beside a page title. */
  size?: "sm" | "md";
  valueTestId?: string;
  className?: string;
}

/**
 * One figure with its label: the KPI tile on Progress and the compact streak and
 * goal tiles beside the Dashboard greeting. Presentation only — the caller
 * decides which figures are real enough to show.
 */
export function StatTile({
  icon: Icon,
  iconClassName = "bg-primary/10 text-primary-accent",
  visual,
  label,
  value,
  hint,
  labelFirst = false,
  size = "md",
  valueTestId,
  className,
}: StatTileProps): React.JSX.Element {
  const small = size === "sm";
  const labelNode = (
    <span
      className={cn(
        "block text-text-muted",
        small ? "text-xs" : "text-xs font-medium sm:text-sm",
      )}
    >
      {label}
    </span>
  );
  const valueNode = (
    <span
      data-testid={valueTestId}
      className={cn(
        "block font-bold tracking-tight text-text tabular-nums",
        small ? "text-base leading-tight" : "mt-0.5 text-2xl leading-tight",
      )}
    >
      {value}
    </span>
  );

  return (
    <div
      className={cn(
        "flex min-w-0 items-center rounded-[var(--radius-card)] border border-border-subtle bg-surface-card shadow-[var(--shadow-card)]",
        small ? "gap-3 px-3.5 py-2.5" : "gap-3.5 p-4 md:p-5",
        className,
      )}
    >
      {visual ??
        (Icon ? (
          <span
            className={cn(
              "flex shrink-0 items-center justify-center rounded-xl",
              small ? "h-10 w-10" : "h-11 w-11 md:h-12 md:w-12",
              iconClassName,
            )}
            aria-hidden="true"
          >
            <Icon className={small ? "h-5 w-5" : "h-5 w-5 md:h-6 md:w-6"} />
          </span>
        ) : null)}
      <span className="min-w-0 flex-1">
        {labelFirst ? (
          <>
            {labelNode}
            {valueNode}
          </>
        ) : (
          <>
            {valueNode}
            {labelNode}
          </>
        )}
        {hint && (
          <span className="mt-0.5 block text-xs text-text-muted">{hint}</span>
        )}
      </span>
    </div>
  );
}
