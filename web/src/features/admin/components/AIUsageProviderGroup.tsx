import React from "react";
import { useTranslation } from "react-i18next";
import { ChevronRight, Server } from "lucide-react";
import type { components } from "@/types/api";
import { StatusBadge } from "./StatusBadge";
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
  const { t, i18n } = useTranslation();
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
    <details className="group" open>
      <summary className="flex min-h-14 cursor-pointer list-none items-center justify-between gap-3 px-4 py-3 transition-colors hover:bg-surface-muted/40 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-primary">
        <span className="flex min-w-0 items-center gap-3">
          <ChevronRight
            aria-hidden="true"
            className="h-4 w-4 shrink-0 text-text-muted transition-transform group-open:rotate-90"
          />
          <span className="flex h-9 w-9 shrink-0 items-center justify-center rounded-full bg-primary/5 text-primary-accent">
            <Server aria-hidden="true" className="h-4 w-4" />
          </span>
          <span className="truncate text-sm font-semibold text-text">
            {provider}
          </span>
        </span>
        <span className="flex shrink-0 items-center gap-2">
          <StatusBadge
            tone={
              worst === "critical"
                ? "danger"
                : worst === "warning"
                  ? "warning"
                  : "success"
            }
          >
            {worst === "critical"
              ? t("admin.quotaExhausted")
              : worst === "warning"
                ? t("admin.nearCeiling")
                : t("admin.quotaHealthy")}
          </StatusBadge>
          <span className="hidden text-xs text-text-muted sm:inline">
            {t("admin.aiTaskCount", { count: items.length })}
          </span>
        </span>
      </summary>
      <ul className="divide-y divide-border-subtle border-t border-border-subtle bg-surface-muted/20">
        {items.map((item) => (
          <li
            key={item.task}
            className="grid gap-3 px-4 py-3 sm:grid-cols-[minmax(0,1fr)_minmax(8rem,1fr)_minmax(8rem,1fr)] sm:items-center sm:pl-[4.25rem]"
          >
            <span className="break-all font-mono text-sm text-text">
              {item.task}
            </span>
            <div>
              <p className="mb-1 text-sm tabular-nums text-text-muted">
                {item.requests_today.toLocaleString(i18n.resolvedLanguage)} /{" "}
                {item.daily_request_limit == null
                  ? "∞"
                  : item.daily_request_limit.toLocaleString(
                      i18n.resolvedLanguage,
                    )}{" "}
                {t("admin.aiCalls")}
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
              <p className="mb-1 text-sm tabular-nums text-text-muted">
                {item.tokens_today.toLocaleString(i18n.resolvedLanguage)} /{" "}
                {item.daily_token_limit == null
                  ? "∞"
                  : item.daily_token_limit.toLocaleString(
                      i18n.resolvedLanguage,
                    )}{" "}
                {t("admin.aiTokens")}
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
