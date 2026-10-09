import React, { useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { AlertTriangle, ExternalLink, Flag } from "lucide-react";

import { adminApi, type ReportedContentVersion } from "../api/adminApi";
import { AdminContentDetailModal } from "./AdminContentDetailModal";
import { Button } from "@/components/ui/button";
import { CopyableId } from "./CopyableId";
import { StatusBadge } from "./StatusBadge";
import {
  AdminDataPanel,
  AdminFilterCard,
  AdminListSkeleton,
  AdminRefreshButton,
  AdminRowIcon,
  AdminTag,
  adminTable,
} from "./AdminSurface";
import { EmptyState } from "@/components/ui/empty-state";
import { ErrorState } from "@/components/ui/error-state";
import { Pagination } from "@/components/ui/pagination";
import { cn } from "@/lib/utils";

export const AdminReportedContentList: React.FC = () => {
  const { t, i18n } = useTranslation();
  const queryClient = useQueryClient();

  const limit = 15;
  const [offset, setOffset] = useState(0);
  const [selectedItemId, setSelectedItemId] = useState<string | null>(null);

  const { data, isLoading, isFetching, error, refetch } = useQuery({
    queryKey: ["admin", "content", "reports", limit, offset],
    queryFn: () => adminApi.listReportedContent({ limit, offset }),
  });

  const items: ReportedContentVersion[] = data?.items ?? [];
  const total = data?.total ?? 0;
  const currentPage = Math.floor(offset / limit) + 1;
  const totalPages = Math.max(1, Math.ceil(total / limit));
  const hasMore =
    (total > 0 && offset + items.length < total) ||
    (total === 0 && items.length >= limit);

  const handleItemUpdated = () => {
    void queryClient.invalidateQueries({
      queryKey: ["admin", "content", "reports"],
    });
    void refetch();
  };

  const formatDate = (isoString: string) => (
    <time
      dateTime={isoString}
      className="whitespace-nowrap text-sm tabular-nums text-text-muted"
    >
      {new Date(isoString).toLocaleString(i18n.resolvedLanguage, {
        day: "2-digit",
        month: "short",
        year: "numeric",
        hour: "2-digit",
        minute: "2-digit",
      })}
    </time>
  );
  const statusBadge = (status: string) => (
    <StatusBadge
      tone={
        status === "archived"
          ? "danger"
          : status === "published"
            ? "success"
            : "neutral"
      }
    >
      <span
        aria-hidden="true"
        className="h-1.5 w-1.5 shrink-0 rounded-full bg-current"
      />
      {t(`adminReports.itemStatus.${status}`, status)}
    </StatusBadge>
  );
  const reportsBadge = (count: number) => (
    <StatusBadge tone="warning" icon={Flag}>
      {t("adminReports.reportsCount", { count })}
    </StatusBadge>
  );
  const reviewButton = (row: ReportedContentVersion, className?: string) => (
    <Button
      type="button"
      variant="outline"
      size="sm"
      onClick={() => setSelectedItemId(row.item_id)}
      className={className}
    >
      <span>{t("adminReports.reviewBtn", "Review item")}</span>
      <ExternalLink className="h-4 w-4" aria-hidden="true" />
    </Button>
  );

  return (
    <div className="space-y-4">
      <AdminFilterCard>
        <div className="flex flex-wrap items-center justify-between gap-3">
          <p role="status" className="text-sm text-text-muted">
            {isLoading
              ? t("admin.loadingContent")
              : hasMore
                ? t("adminReports.loadedCount", {
                    count: items.length,
                    defaultValue: `${items.length} loaded`,
                  })
                : t("adminReports.resultsCount", {
                    count: items.length,
                    defaultValue:
                      items.length === 1
                        ? "1 result"
                        : `${items.length} results`,
                  })}
          </p>
          <AdminRefreshButton
            onClick={() => void refetch()}
            busy={isFetching}
          />
        </div>
      </AdminFilterCard>

      <AdminDataPanel>
        {isLoading ? (
          <AdminListSkeleton label={t("admin.loadingContent")} />
        ) : error ? (
          <ErrorState
            title={t("admin.failedToLoadContent", "Failed to load content.")}
            {...(error instanceof Error ? { description: error.message } : {})}
            onRetry={() => void refetch()}
            className="py-10"
          />
        ) : items.length === 0 ? (
          <EmptyState
            icon={AlertTriangle}
            title={t(
              "adminReports.noReports",
              "No items have been reported yet.",
            )}
            className="py-10"
          />
        ) : (
          <>
            <div className="hidden overflow-x-auto md:block">
              <table className={cn(adminTable.table, "min-w-[960px]")}>
                <caption className="sr-only">
                  {t("adminReports.title", "Reported Content Queue")}
                </caption>
                <thead className={adminTable.head}>
                  <tr>
                    <th
                      scope="col"
                      className={cn(adminTable.th, "w-14 text-right")}
                    >
                      {t("common.rowNumber")}
                    </th>
                    <th scope="col" className={adminTable.th}>
                      {t("adminReports.colItem", "Content Item")}
                    </th>
                    <th scope="col" className={cn(adminTable.th, "w-44")}>
                      {t("adminReports.colKind", "Kind")}
                    </th>
                    <th scope="col" className={cn(adminTable.th, "w-20")}>
                      {t("adminReports.colLevel", "Level")}
                    </th>
                    <th scope="col" className={cn(adminTable.th, "w-36")}>
                      {t("adminReports.colStatus", "Status")}
                    </th>
                    <th scope="col" className={cn(adminTable.th, "w-36")}>
                      {t("adminReports.colReporters", "Reporters")}
                    </th>
                    <th scope="col" className={cn(adminTable.th, "w-44")}>
                      {t("adminReports.colLastReported", "Last Reported")}
                    </th>
                    <th
                      scope="col"
                      className={cn(adminTable.th, "w-40 text-right")}
                    >
                      {t("adminReports.colActions", "Actions")}
                    </th>
                  </tr>
                </thead>
                <tbody className={adminTable.body}>
                  {items.map((row, index) => (
                    <tr
                      key={row.content_version_id}
                      className={cn(adminTable.row, "h-16")}
                    >
                      <td
                        className={cn(
                          adminTable.td,
                          "text-right tabular-nums text-text-muted",
                        )}
                      >
                        {offset + index + 1}
                      </td>
                      <td className={adminTable.td}>
                        <div className="flex min-w-0 items-center gap-3">
                          <AdminRowIcon
                            icon={AlertTriangle}
                            className="bg-warning/10 text-warning-accent"
                          />
                          <div className="min-w-0">
                            <p
                              title={row.slug}
                              className="truncate text-sm font-semibold text-text"
                            >
                              {row.slug}
                            </p>
                            <CopyableId id={row.content_version_id} />
                          </div>
                        </div>
                      </td>
                      <td className={adminTable.td}>
                        <AdminTag title={row.kind}>{row.kind}</AdminTag>
                      </td>
                      <td className={adminTable.td}>
                        <AdminTag>{row.cefr_level}</AdminTag>
                      </td>
                      <td className={adminTable.td}>
                        {statusBadge(row.item_status)}
                      </td>
                      <td className={adminTable.td}>
                        {reportsBadge(row.report_count)}
                      </td>
                      <td className={adminTable.td}>
                        {formatDate(row.last_reported_at)}
                      </td>
                      <td className={cn(adminTable.td, "text-right")}>
                        {reviewButton(row)}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
            <ul
              data-testid="reported-cards"
              className="divide-y divide-border-subtle md:hidden"
            >
              {items.map((row) => (
                <li
                  key={row.content_version_id}
                  data-testid="reported-card"
                  className="space-y-3 p-4"
                >
                  <div className="flex items-start gap-3">
                    <AdminRowIcon
                      icon={AlertTriangle}
                      className="h-10 w-10 bg-warning/10 text-warning-accent"
                    />
                    <p className="min-w-0 flex-1 break-all pt-2 text-sm font-semibold text-text">
                      {row.slug}
                    </p>
                  </div>
                  <div className="flex flex-wrap items-center gap-2">
                    <AdminTag title={row.kind}>{row.kind}</AdminTag>
                    <AdminTag>{row.cefr_level}</AdminTag>
                    {statusBadge(row.item_status)}
                    {reportsBadge(row.report_count)}
                  </div>
                  <div className="flex flex-wrap items-center justify-between gap-2">
                    <CopyableId id={row.content_version_id} />
                    {formatDate(row.last_reported_at)}
                  </div>
                  {reviewButton(row, "w-full")}
                </li>
              ))}
            </ul>
          </>
        )}
        {!error && !isLoading && items.length > 0 && (
          <Pagination
            page={currentPage}
            pageCount={totalPages}
            total={total}
            rangeFrom={offset + 1}
            rangeTo={Math.min(offset + items.length, total)}
            isBusy={isFetching}
            canPrevious={offset > 0}
            canNext={offset + limit < total}
            onPrevious={() => setOffset((prev) => Math.max(0, prev - limit))}
            onNext={() => setOffset((prev) => prev + limit)}
            onPageSelect={(next) => setOffset((next - 1) * limit)}
            className="bg-surface-card"
          />
        )}
      </AdminDataPanel>

      {/* Admin Content Detail Modal for reviewing & archiving */}
      {selectedItemId && (
        <AdminContentDetailModal
          itemId={selectedItemId}
          onClose={() => setSelectedItemId(null)}
          onItemUpdated={handleItemUpdated}
        />
      )}
    </div>
  );
};
