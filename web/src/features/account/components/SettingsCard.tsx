import * as React from "react";
import type { LucideIcon } from "lucide-react";

import { cn } from "@/lib/utils";

const TONES = {
  primary: "bg-primary/10 text-primary-accent",
  success: "bg-success/10 text-success-accent",
  warning: "bg-warning/10 text-warning-accent",
  danger: "bg-danger/10 text-danger-accent",
  muted: "bg-surface-muted text-text-muted",
} as const;

export type IconTileTone = keyof typeof TONES;

/** The rounded icon square every settings card and row leads with. */
export function IconTile({
  icon: Icon,
  tone = "primary",
  size = "md",
  className,
}: {
  icon: LucideIcon;
  tone?: IconTileTone | undefined;
  size?: "sm" | "md" | "lg" | undefined;
  className?: string | undefined;
}): React.JSX.Element {
  return (
    <span
      aria-hidden="true"
      className={cn(
        "flex shrink-0 items-center justify-center",
        size === "sm" && "h-9 w-9 rounded-lg",
        size === "md" && "h-10 w-10 rounded-xl",
        size === "lg" && "h-12 w-12 rounded-2xl sm:h-14 sm:w-14",
        TONES[tone],
        className,
      )}
    >
      <Icon
        className={cn(size === "lg" ? "h-6 w-6 sm:h-7 sm:w-7" : "h-5 w-5")}
      />
    </span>
  );
}

/** Icon tile, title, description and an optional trailing action. */
export function SettingsCardHeader({
  icon,
  tone,
  title,
  description,
  action,
  titleId,
  as: Heading = "h3",
}: {
  icon: LucideIcon;
  tone?: IconTileTone | undefined;
  title: React.ReactNode;
  description?: React.ReactNode | undefined;
  action?: React.ReactNode | undefined;
  titleId?: string | undefined;
  as?: "h2" | "h3";
}): React.JSX.Element {
  return (
    <div className="flex items-start gap-3">
      <IconTile icon={icon} tone={tone} />
      <div className="min-w-0 flex-1 space-y-0.5">
        <Heading
          id={titleId}
          className="text-base font-semibold leading-snug text-text"
        >
          {title}
        </Heading>
        {description && (
          <p className="text-sm text-text-muted">{description}</p>
        )}
      </div>
      {action && <div className="shrink-0">{action}</div>}
    </div>
  );
}
