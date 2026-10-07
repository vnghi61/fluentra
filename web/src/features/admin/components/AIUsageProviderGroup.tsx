import React from "react";
import type { components } from "@/types/api";
import { Badge } from "@/components/ui/badge";
import { quotaPercent, quotaStatus } from "../model/aiUsage";
import { QuotaBar } from "./QuotaBar";

type Usage = components["schemas"]["AdminAIUsageItem"];

export function AIUsageProviderGroup({
  provider,
  items,
}: {
  provider: string;
  items: Usage[];
}): React.JSX.Element {
  const worst = items.some(
    (item) =>
      item.is_exhausted ||
      quotaStatus(
        Math.max(
          quotaPercent(item.requests_today, item.daily_request_limit) ?? 0,
          quotaPercent(item.tokens_today, item.daily_token_limit) ?? 0,
        ),
      ) === "critical",
  )
    ? "critical"
    : items.some(
          (item) =>
            quotaStatus(
              Math.max(
                quotaPercent(item.requests_today, item.daily_request_limit) ??
                  0,
                quotaPercent(item.tokens_today, item.daily_token_limit) ?? 0,
              ),
            ) === "warning",
        )
      ? "warning"
      : "normal";
  return (
    <details className="rounded-xl border bg-card" open>
      <summary className="flex min-h-11 cursor-pointer list-none items-center justify-between gap-3 p-4">
        <span className="font-semibold">{provider}</span>
        <span className="flex items-center gap-2">
          <Badge
            variant={
              worst === "critical"
                ? "danger"
                : worst === "warning"
                  ? "warning"
                  : "success"
            }
          >
            {worst}
          </Badge>
          <span className="text-xs text-muted-foreground">
            {items.length} tasks
          </span>
        </span>
      </summary>
      <ul className="divide-y px-4">
        {items.map((item) => (
          <li
            key={item.task}
            className="grid gap-3 py-4 sm:grid-cols-[minmax(0,1fr)_minmax(8rem,1fr)_minmax(8rem,1fr)] sm:items-center"
          >
            <span className="font-mono text-sm">{item.task}</span>
            <div>
              <p className="mb-1 text-xs text-muted-foreground">
                {item.requests_today.toLocaleString()} /{" "}
                {item.daily_request_limit == null
                  ? "∞"
                  : item.daily_request_limit.toLocaleString()}{" "}
                calls
              </p>
              {item.daily_request_limit != null && (
                <QuotaBar
                  percent={
                    quotaPercent(
                      item.requests_today,
                      item.daily_request_limit,
                    ) ?? 0
                  }
                  exhausted={item.is_exhausted}
                />
              )}
            </div>
            <div>
              <p className="mb-1 text-xs text-muted-foreground">
                {item.tokens_today.toLocaleString()} /{" "}
                {item.daily_token_limit == null
                  ? "∞"
                  : item.daily_token_limit.toLocaleString()}{" "}
                tokens
              </p>
              {item.daily_token_limit != null && (
                <QuotaBar
                  percent={
                    quotaPercent(item.tokens_today, item.daily_token_limit) ?? 0
                  }
                  exhausted={item.is_exhausted}
                />
              )}
            </div>
          </li>
        ))}
      </ul>
    </details>
  );
}
