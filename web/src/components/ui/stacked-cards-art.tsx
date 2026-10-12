import * as React from "react";
import type { LucideIcon } from "lucide-react";

import { cn } from "@/lib/utils";

export interface StackedCardsArtProps {
  icon: LucideIcon;
  /** sm 56px, md 88px, lg 112px. */
  size?: "sm" | "md" | "lg";
  className?: string;
}

const boxSize = {
  sm: "h-14 w-14",
  md: "h-[88px] w-[88px]",
  lg: "h-28 w-28",
} as const;

const iconSize = {
  sm: "h-6 w-6",
  md: "h-9 w-9",
  lg: "h-11 w-11",
} as const;

/**
 * The decorative stack of cards beside a hero's call to action: three tilted
 * plates in primary tints with the subject's icon on the front one. Pure CSS
 * and a lucide icon, no image asset; hidden from assistive technology. The
 * tilt is a static transform, not motion.
 */
export function StackedCardsArt({
  icon: Icon,
  size = "md",
  className,
}: StackedCardsArtProps): React.JSX.Element {
  return (
    <div
      aria-hidden="true"
      className={cn(
        "pointer-events-none relative shrink-0 select-none",
        boxSize[size],
        className,
      )}
    >
      <div className="absolute -inset-3 rounded-full bg-primary/10 blur-2xl" />
      <div className="absolute inset-0 translate-x-2 -translate-y-1 rotate-[10deg] rounded-2xl border border-primary/20 bg-primary/15" />
      <div className="absolute inset-0 translate-x-1 rotate-[4deg] rounded-2xl border border-primary/25 bg-surface-muted" />
      <div className="absolute inset-0 -rotate-[4deg] rounded-2xl border border-primary/30 bg-gradient-to-br from-surface-card to-primary/10 shadow-[var(--shadow-card)]">
        <div className="flex h-full w-full items-center justify-center">
          <Icon className={cn("text-primary-accent", iconSize[size])} />
        </div>
        <div className="absolute inset-x-[22%] bottom-[16%] h-1 rounded-full bg-primary/25" />
      </div>
    </div>
  );
}
