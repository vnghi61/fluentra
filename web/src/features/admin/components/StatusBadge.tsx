import * as React from "react";
import { cn } from "@/lib/utils";

const toneClasses = {
  success: "bg-success/10 text-success-accent",
  warning: "bg-warning/10 text-warning-accent",
  danger: "bg-danger/10 text-danger-accent",
  neutral: "bg-surface-muted text-text-muted",
  info: "bg-primary/10 text-primary-accent",
} as const;
export type StatusTone = keyof typeof toneClasses;

export function StatusBadge({
  tone,
  children,
  icon: Icon,
}: {
  tone: StatusTone;
  children: React.ReactNode;
  icon?: React.ElementType;
}): React.JSX.Element {
  return (
    <span
      className={cn(
        "inline-flex min-h-7 items-center gap-1.5 rounded-full px-2.5 py-1 text-xs font-medium",
        toneClasses[tone],
      )}
    >
      {Icon && <Icon className="h-4 w-4 shrink-0" aria-hidden="true" />}
      {children}
    </span>
  );
}
