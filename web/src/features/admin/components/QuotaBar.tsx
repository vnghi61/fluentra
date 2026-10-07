import React from "react";
import { quotaStatus } from "../model/aiUsage";

export function QuotaBar({
  percent,
  exhausted = false,
}: {
  percent: number;
  exhausted?: boolean;
}): React.JSX.Element {
  const status = quotaStatus(percent, exhausted);
  const color =
    status === "critical"
      ? "bg-danger"
      : status === "warning"
        ? "bg-warning"
        : status === "highlight"
          ? "bg-primary"
          : "bg-primary/50";
  return (
    <div
      className="flex min-w-20 items-center gap-2"
      aria-label={`${percent}% quota used`}
    >
      <div className="h-2 min-w-10 flex-1 overflow-hidden rounded-full bg-muted">
        <div
          className={`h-full ${color}`}
          style={{ width: `${Math.min(100, percent)}%` }}
        />
      </div>
      <span className="text-xs tabular-nums">{percent}%</span>
    </div>
  );
}
