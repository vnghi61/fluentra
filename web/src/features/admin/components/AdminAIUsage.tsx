import React, { useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import { useQuery } from "@tanstack/react-query";
import { Activity, AlertCircle, Database, RefreshCw } from "lucide-react";
import { adminApi } from "../api/adminApi";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { EmptyState } from "@/components/ui/empty-state";
import { ErrorState } from "@/components/ui/error-state";
import { Pagination } from "@/components/ui/pagination";
import { StatusBadge } from "./StatusBadge";
import { AIUsageProviderGroup } from "./AIUsageProviderGroup";
import { quotaPercent, quotaStatus } from "../model/aiUsage";

const PAGE_SIZE = 10;

export const AdminAIUsage: React.FC = () => {
  const { t } = useTranslation();
  const [provider, setProvider] = useState("");
  const [task, setTask] = useState("");
  const [status, setStatus] = useState("");
  const [page, setPage] = useState(0);
  const { data, isLoading, isFetching, error, refetch } = useQuery({
    queryKey: ["admin", "ai-usage"],
    queryFn: () => adminApi.getAIUsage(),
  });
  const items = data?.items ?? [];
  const totalRequests = items.reduce(
    (sum, item) => sum + item.requests_today,
    0,
  );
  const totalTokens = items.reduce((sum, item) => sum + item.tokens_today, 0);
  const providers = useMemo(
    () => [...new Set(items.map((item) => item.provider))],
    [items],
  );
  const filtered = items.filter(
    (item) =>
      (!provider || item.provider === provider) &&
      (!task || item.task.toLowerCase().includes(task.toLowerCase())) &&
      (!status ||
        (status === "critical"
          ? item.is_exhausted ||
            Math.max(
              quotaPercent(item.requests_today, item.daily_request_limit) ?? 0,
              quotaPercent(item.tokens_today, item.daily_token_limit) ?? 0,
            ) > 95
          : status === "warning"
            ? Math.max(
                quotaPercent(item.requests_today, item.daily_request_limit) ??
                  0,
                quotaPercent(item.tokens_today, item.daily_token_limit) ?? 0,
              ) > 85
            : status === "active"
              ? !item.is_exhausted
              : true)),
  );
  const groups = [...new Set(filtered.map((item) => item.provider))].map(
    (name) => ({
      name,
      items: filtered.filter((item) => item.provider === name),
    }),
  );
  const visibleGroups = groups.slice(page * PAGE_SIZE, (page + 1) * PAGE_SIZE);
  const worst = items.some(
    (item) =>
      item.is_exhausted ||
      Math.max(
        quotaPercent(item.requests_today, item.daily_request_limit) ?? 0,
        quotaPercent(item.tokens_today, item.daily_token_limit) ?? 0,
      ) > 95,
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
            ) !== "normal",
        )
      ? "warning"
      : "normal";
  const clear = () => {
    setProvider("");
    setTask("");
    setStatus("");
    setPage(0);
  };

  return (
    <div className="space-y-5">
      <header className="flex flex-wrap items-center justify-between gap-3">
        <div>
          <h2 className="text-xl font-bold">{t("admin.aiUsageTitle")}</h2>
          <p className="text-sm text-muted-foreground">
            {t("admin.aiUsageSubtitle")}
          </p>
        </div>
        <Button
          variant="outline"
          className="min-h-11"
          disabled={isFetching}
          onClick={() => void refetch()}
        >
          <RefreshCw className="mr-2 h-4 w-4" />
          {t("admin.refresh")}
        </Button>
      </header>
      <div className="grid gap-3 sm:grid-cols-3">
        <Kpi
          label={t("admin.totalRequestsToday")}
          value={totalRequests.toLocaleString()}
          icon={<Activity className="h-4 w-4" />}
        />
        <Kpi
          label={t("admin.totalTokensToday")}
          value={totalTokens.toLocaleString()}
          icon={<Database className="h-4 w-4" />}
        />
        <section className="rounded-xl border bg-card p-4">
          <h3 className="text-sm font-medium text-muted-foreground">
            {t("admin.quotaStatus")}
          </h3>
          <div className="mt-3">
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
          </div>
        </section>
      </div>
      <div className="flex flex-col gap-2 sm:flex-row">
        <select
          aria-label={t("admin.provider", "Provider")}
          value={provider}
          onChange={(event) => {
            setProvider(event.target.value);
            setPage(0);
          }}
          className="h-11 rounded-md border bg-background px-3 text-base"
        >
          <option value="">{t("admin.allProviders", "All providers")}</option>
          {providers.map((value) => (
            <option key={value}>{value}</option>
          ))}
        </select>
        <Input
          aria-label={t("admin.task", "Task")}
          placeholder={t("admin.task", "Task")}
          value={task}
          onChange={(event) => {
            setTask(event.target.value);
            setPage(0);
          }}
          className="h-11 min-w-0 sm:flex-1"
        />
        <select
          aria-label={t("admin.status", "Status")}
          value={status}
          onChange={(event) => {
            setStatus(event.target.value);
            setPage(0);
          }}
          className="h-11 rounded-md border bg-background px-3 text-base"
        >
          <option value="">{t("admin.allStatuses", "All statuses")}</option>
          <option value="active">{t("admin.active")}</option>
          <option value="warning">{t("admin.nearCeiling")}</option>
          <option value="critical">{t("admin.exhausted")}</option>
        </select>
      </div>
      {error ? (
        <ErrorState
          description={
            error instanceof Error ? error.message : t("admin.noUsageRecorded")
          }
          onRetry={() => void refetch()}
        />
      ) : isLoading ? (
        <div className="py-12 text-center" aria-busy="true">
          {t("admin.loadingUsage")}
        </div>
      ) : items.length === 0 ? (
        <EmptyState
          variant="compact"
          title={t("admin.noUsageRecorded")}
          icon={AlertCircle}
        />
      ) : groups.length === 0 ? (
        <EmptyState
          variant="compact"
          title={t("states.emptyTitle", "No items found")}
          action={
            <Button variant="outline" className="min-h-11" onClick={clear}>
              {t("admin.clearFilters", "Reset filters")}
            </Button>
          }
        />
      ) : (
        <div className="space-y-3">
          {visibleGroups.map((group) => (
            <AIUsageProviderGroup
              key={group.name}
              provider={group.name}
              items={group.items}
            />
          ))}
          <Pagination
            page={page + 1}
            pageCount={Math.ceil(groups.length / PAGE_SIZE)}
            total={groups.length}
            rangeFrom={page * PAGE_SIZE + 1}
            rangeTo={Math.min((page + 1) * PAGE_SIZE, groups.length)}
            isBusy={isFetching}
            canPrevious={page > 0}
            canNext={(page + 1) * PAGE_SIZE < groups.length}
            onPrevious={() => setPage((value) => Math.max(0, value - 1))}
            onNext={() => setPage((value) => value + 1)}
            onPageSelect={(value) => setPage(value - 1)}
          />
        </div>
      )}
    </div>
  );
};

function Kpi({
  label,
  value,
  icon,
}: {
  label: string;
  value: string;
  icon: React.ReactNode;
}): React.JSX.Element {
  return (
    <section className="rounded-xl border bg-card p-4">
      <div className="flex items-center gap-2 text-sm text-muted-foreground">
        {icon}
        {label}
      </div>
      <p className="mt-2 text-2xl font-bold">{value}</p>
    </section>
  );
}
