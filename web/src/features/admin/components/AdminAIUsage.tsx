import React, { useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import { useQuery } from "@tanstack/react-query";
import {
  Activity,
  AlertCircle,
  Database,
  Gauge,
  Search,
  Server,
} from "lucide-react";
import { adminApi } from "../api/adminApi";
import { Input } from "@/components/ui/input";
import { EmptyState } from "@/components/ui/empty-state";
import { ErrorState } from "@/components/ui/error-state";
import { Pagination } from "@/components/ui/pagination";
import {
  AdminActiveFilters,
  AdminDataPanel,
  AdminFilterCard,
  AdminFilterChip,
  AdminListSkeleton,
  AdminRefreshButton,
  AdminStatGrid,
  adminStatTones,
  type AdminStat,
} from "./AdminSurface";
import { AIUsageProviderGroup } from "./AIUsageProviderGroup";
import { quotaPercent, quotaStatus } from "../model/aiUsage";
import { NativeSelect } from "@/components/ui/native-select";

const PAGE_SIZE = 10;

export const AdminAIUsage: React.FC = () => {
  const { t, i18n } = useTranslation();
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
  const statusLabel = (value: string) =>
    value === "critical"
      ? t("admin.exhausted")
      : value === "warning"
        ? t("admin.nearCeiling")
        : t("admin.active");
  const filteredOn = Boolean(provider || task.trim() || status);
  const stats: AdminStat[] = [
    {
      key: "requests",
      label: t("admin.totalRequestsToday"),
      value: totalRequests.toLocaleString(i18n.resolvedLanguage),
      Icon: Activity,
      tone: adminStatTones.primary,
    },
    {
      key: "tokens",
      label: t("admin.totalTokensToday"),
      value: totalTokens.toLocaleString(i18n.resolvedLanguage),
      Icon: Database,
      tone: adminStatTones.neutral,
    },
    {
      key: "providers",
      label: t("admin.provider", "Provider"),
      value: providers.length.toLocaleString(i18n.resolvedLanguage),
      Icon: Server,
      tone: adminStatTones.primary,
    },
    {
      key: "quota",
      label: t("admin.quotaStatus"),
      value:
        worst === "critical"
          ? t("admin.quotaExhausted")
          : worst === "warning"
            ? t("admin.nearCeiling")
            : t("admin.quotaHealthy"),
      Icon: Gauge,
      tone:
        worst === "critical"
          ? adminStatTones.danger
          : worst === "warning"
            ? adminStatTones.warning
            : adminStatTones.success,
    },
  ];

  return (
    <div className="space-y-4">
      <AdminFilterCard>
        <div className="grid grid-cols-[minmax(0,1fr)_minmax(0,1fr)_2.75rem] items-center gap-2 sm:flex sm:flex-wrap sm:gap-3">
          <div className="relative col-span-3 w-full sm:min-w-0 sm:flex-1">
            <Search
              aria-hidden="true"
              className="pointer-events-none absolute left-3.5 top-3.5 h-4 w-4 text-text-muted"
            />
            <Input
              type="text"
              aria-label={t("admin.task", "Task")}
              placeholder={t("admin.task", "Task")}
              value={task}
              onChange={(event) => {
                setTask(event.target.value);
                setPage(0);
              }}
              className="bg-surface-muted pl-10"
            />
          </div>
          <NativeSelect
            aria-label={t("admin.provider", "Provider")}
            value={provider}
            onChange={(event) => {
              setProvider(event.target.value);
              setPage(0);
            }}
            wrapperClassName="sm:w-auto"
          >
            <option value="">{t("admin.allProviders", "All providers")}</option>
            {providers.map((value) => (
              <option key={value}>{value}</option>
            ))}
          </NativeSelect>
          <NativeSelect
            aria-label={t("admin.status", "Status")}
            value={status}
            onChange={(event) => {
              setStatus(event.target.value);
              setPage(0);
            }}
            wrapperClassName="sm:w-auto"
          >
            <option value="">{t("admin.allStatuses", "All statuses")}</option>
            <option value="active">{t("admin.active")}</option>
            <option value="warning">{t("admin.nearCeiling")}</option>
            <option value="critical">{t("admin.exhausted")}</option>
          </NativeSelect>
          <AdminRefreshButton
            onClick={() => void refetch()}
            busy={isFetching}
          />
        </div>
        {filteredOn && (
          <AdminActiveFilters onReset={clear}>
            {task.trim() && <AdminFilterChip>{task.trim()}</AdminFilterChip>}
            {provider && <AdminFilterChip>{provider}</AdminFilterChip>}
            {status && <AdminFilterChip>{statusLabel(status)}</AdminFilterChip>}
          </AdminActiveFilters>
        )}
      </AdminFilterCard>

      <AdminStatGrid
        stats={stats}
        loading={isLoading}
        label={t("admin.quotaStatus")}
      />

      <AdminDataPanel>
        {error ? (
          <ErrorState
            description={
              error instanceof Error
                ? error.message
                : t("admin.noUsageRecorded")
            }
            onRetry={() => void refetch()}
            className="py-10"
          />
        ) : isLoading ? (
          <AdminListSkeleton label={t("admin.loadingUsage")} />
        ) : items.length === 0 ? (
          <EmptyState
            icon={AlertCircle}
            title={t("admin.noUsageRecorded")}
            className="py-10"
          />
        ) : groups.length === 0 ? (
          <EmptyState
            title={t("states.emptyTitle", "No items found")}
            description={t("admin.tryAdjustingYourSearchQueryOrFilters")}
            className="py-10"
          />
        ) : (
          <>
            <div className="divide-y divide-border-subtle">
              {visibleGroups.map((group) => (
                <AIUsageProviderGroup
                  key={group.name}
                  provider={group.name}
                  items={group.items}
                />
              ))}
            </div>
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
              className="bg-surface-card"
            />
          </>
        )}
      </AdminDataPanel>
    </div>
  );
};
