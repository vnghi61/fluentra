import React from "react";
import { useTranslation } from "react-i18next";
import { useQuery } from "@tanstack/react-query";
import {
  Activity,
  AlertCircle,
  AlertTriangle,
  CheckCircle2,
  Cpu,
  Database,
  Loader2,
  RefreshCw,
} from "lucide-react";
import { adminApi } from "../api/adminApi";
import { Button } from "@/components/ui/button";

export const AdminAIUsage: React.FC = () => {
  const { t } = useTranslation();

  const {
    data,
    isLoading,
    isFetching,
    error,
    refetch,
  } = useQuery({
    queryKey: ["admin", "ai-usage"],
    queryFn: () => adminApi.getAIUsage(),
  });

  const items = data?.items ?? [];
  const totalRequests = items.reduce((acc, it) => acc + it.requests_today, 0);
  const totalTokens = items.reduce((acc, it) => acc + it.tokens_today, 0);
  const anyExhausted = items.some((it) => it.is_exhausted);
  const errorMessage = error instanceof Error ? error.message : null;

  return (
    <div className="space-y-6">
      <div className="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
        <div>
          <h2 className="text-xl font-bold tracking-tight text-foreground sm:text-2xl">
            {t("admin.aiUsageTitle", "AI Provider Usage & Budgets")}
          </h2>
          <p className="text-sm text-muted-foreground">
            {t(
              "admin.aiUsageSubtitle",
              "Real-time daily consumption and quota limits across models and tasks.",
            )}
          </p>
        </div>
        <Button
          variant="outline"
          size="sm"
          onClick={() => void refetch()}
          disabled={isFetching}
          className="self-start sm:self-auto"
        >
          <RefreshCw
            className={`mr-2 h-4 w-4 ${isFetching ? "animate-spin" : ""}`}
          />
          {t("admin.refresh", "Refresh")}
        </Button>
      </div>

      {errorMessage && (
        <div className="flex items-center gap-3 rounded-lg border border-destructive/20 bg-destructive/10 p-4 text-sm text-destructive">
          <AlertCircle className="h-5 w-5 shrink-0" />
          <span>{errorMessage}</span>
        </div>
      )}

      {/* KPI Overview */}
      <div className="grid grid-cols-1 gap-4 sm:grid-cols-3">
        <div className="rounded-xl border bg-card p-4 shadow-sm">
          <div className="flex items-center gap-3 text-muted-foreground">
            <Activity className="h-4 w-4 text-primary" />
            <span className="text-sm font-medium">
              {t("admin.totalRequestsToday", "Requests Today")}
            </span>
          </div>
          <p className="mt-2 text-2xl font-bold text-foreground">
            {totalRequests.toLocaleString()}
          </p>
        </div>

        <div className="rounded-xl border bg-card p-4 shadow-sm">
          <div className="flex items-center gap-3 text-muted-foreground">
            <Database className="h-4 w-4 text-primary" />
            <span className="text-sm font-medium">
              {t("admin.totalTokensToday", "Tokens Today")}
            </span>
          </div>
          <p className="mt-2 text-2xl font-bold text-foreground">
            {totalTokens.toLocaleString()}
          </p>
        </div>

        <div className="rounded-xl border bg-card p-4 shadow-sm">
          <div className="flex items-center gap-3 text-muted-foreground">
            <Cpu className="h-4 w-4 text-primary" />
            <span className="text-sm font-medium">
              {t("admin.quotaStatus", "System Status")}
            </span>
          </div>
          <div className="mt-2 flex items-center gap-2">
            {anyExhausted ? (
              <span className="inline-flex items-center gap-1.5 rounded-md bg-destructive/10 px-2.5 py-1 text-xs font-semibold text-destructive">
                <AlertTriangle className="h-3.5 w-3.5" />
                {t("admin.quotaExhausted", "Quota Limit Reached")}
              </span>
            ) : (
              <span className="inline-flex items-center gap-1.5 rounded-md bg-emerald-500/10 px-2.5 py-1 text-xs font-semibold text-emerald-600 dark:text-emerald-400">
                <CheckCircle2 className="h-3.5 w-3.5" />
                {t("admin.quotaHealthy", "Within Ceilings")}
              </span>
            )}
          </div>
        </div>
      </div>

      {/* Usage Table */}
      <div className="overflow-hidden rounded-xl border bg-card shadow-sm">
        {isLoading ? (
          <div className="flex flex-col items-center justify-center p-12 text-center text-muted-foreground">
            <Loader2 className="h-8 w-8 animate-spin text-primary" />
            <p className="mt-3 text-sm">
              {t("admin.loadingUsage", "Loading usage data...")}
            </p>
          </div>
        ) : items.length === 0 ? (
          <div className="p-12 text-center text-muted-foreground">
            <p className="text-sm">
              {t(
                "admin.noUsageRecorded",
                "No AI consumption recorded for today.",
              )}
            </p>
          </div>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full text-left text-sm">
              <thead className="border-b bg-muted/40 text-xs font-semibold uppercase tracking-wider text-muted-foreground">
                <tr>
                  <th className="px-4 py-3">
                    {t("admin.provider", "Provider")}
                  </th>
                  <th className="px-4 py-3">{t("admin.task", "Task")}</th>
                  <th className="px-4 py-3">
                    {t("admin.requestsVsLimit", "Requests / Limit")}
                  </th>
                  <th className="px-4 py-3">
                    {t("admin.tokensVsLimit", "Tokens / Limit")}
                  </th>
                  <th className="px-4 py-3">{t("admin.status", "Status")}</th>
                </tr>
              </thead>
              <tbody className="divide-y">
                {items.map((it) => {
                  const reqPercent =
                    it.daily_request_limit && it.daily_request_limit > 0
                      ? Math.min(
                          100,
                          Math.round(
                            (it.requests_today / it.daily_request_limit) * 100,
                          ),
                        )
                      : null;
                  const tokenPercent =
                    it.daily_token_limit && it.daily_token_limit > 0
                      ? Math.min(
                          100,
                          Math.round(
                            (it.tokens_today / it.daily_token_limit) * 100,
                          ),
                        )
                      : null;

                  return (
                    <tr
                      key={`${it.provider}-${it.task}`}
                      className="hover:bg-muted/30 transition-colors"
                    >
                      <td className="px-4 py-3.5 font-medium text-foreground">
                        {it.provider}
                      </td>
                      <td className="px-4 py-3.5 text-muted-foreground">
                        <span className="rounded bg-muted px-2 py-0.5 font-mono text-xs">
                          {it.task}
                        </span>
                      </td>
                      <td className="px-4 py-3.5">
                        <div className="flex items-center gap-2">
                          <span className="font-semibold text-foreground">
                            {it.requests_today.toLocaleString()}
                          </span>
                          <span className="text-xs text-muted-foreground">
                            /{" "}
                            {it.daily_request_limit != null
                              ? it.daily_request_limit.toLocaleString()
                              : "∞"}
                          </span>
                          {reqPercent !== null && (
                            <span className="text-xs text-muted-foreground">
                              ({reqPercent}%)
                            </span>
                          )}
                        </div>
                      </td>
                      <td className="px-4 py-3.5">
                        <div className="flex items-center gap-2">
                          <span className="font-semibold text-foreground">
                            {it.tokens_today.toLocaleString()}
                          </span>
                          <span className="text-xs text-muted-foreground">
                            /{" "}
                            {it.daily_token_limit != null
                              ? it.daily_token_limit.toLocaleString()
                              : "∞"}
                          </span>
                          {tokenPercent !== null && (
                            <span className="text-xs text-muted-foreground">
                              ({tokenPercent}%)
                            </span>
                          )}
                        </div>
                      </td>
                      <td className="px-4 py-3.5">
                        {it.is_exhausted ? (
                          <span className="inline-flex items-center gap-1 rounded bg-destructive/10 px-2 py-0.5 text-xs font-semibold text-destructive">
                            <AlertCircle className="h-3 w-3" />
                            {t("admin.exhausted", "Exhausted")}
                          </span>
                        ) : (reqPercent !== null && reqPercent >= 80) ||
                          (tokenPercent !== null && tokenPercent >= 80) ? (
                          <span className="inline-flex items-center gap-1 rounded bg-amber-500/10 px-2 py-0.5 text-xs font-semibold text-amber-600 dark:text-amber-400">
                            <AlertTriangle className="h-3 w-3" />
                            {t("admin.nearCeiling", "Near Ceiling")}
                          </span>
                        ) : (
                          <span className="inline-flex items-center gap-1 rounded bg-emerald-500/10 px-2 py-0.5 text-xs font-semibold text-emerald-600 dark:text-emerald-400">
                            <CheckCircle2 className="h-3 w-3" />
                            {t("admin.active", "Active")}
                          </span>
                        )}
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
        )}
      </div>
    </div>
  );
};
