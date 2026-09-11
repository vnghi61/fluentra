import React, { useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import {
  AlertCircle,
  AlertTriangle,
  ChevronLeft,
  ChevronRight,
  ExternalLink,
  Loader2,
  RefreshCw,
} from "lucide-react";

import { adminApi, type ReportedContentVersion } from "../api/adminApi";
import { AdminContentDetailModal } from "./AdminContentDetailModal";
import { Button } from "@/components/ui/button";

export const AdminReportedContentList: React.FC = () => {
  const { t } = useTranslation();
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

  const handleItemUpdated = () => {
    void queryClient.invalidateQueries({
      queryKey: ["admin", "content", "reports"],
    });
    void refetch();
  };

  const formatDate = (isoString: string) => {
    try {
      const d = new Date(isoString);
      return d.toLocaleDateString(undefined, {
        year: "numeric",
        month: "short",
        day: "numeric",
        hour: "2-digit",
        minute: "2-digit",
      });
    } catch {
      return isoString;
    }
  };

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
        <div>
          <h2 className="text-xl font-bold text-text flex items-center gap-2">
            <AlertTriangle className="h-5 w-5 text-warning-accent" />
            {t("adminReports.title", "Reported Content Queue")}
          </h2>
          <p className="text-sm text-text-muted mt-0.5">
            {t(
              "adminReports.desc",
              "Exercises and content versions reported by learners, sorted by distinct reporters. Review and archive problematic items.",
            )}
          </p>
        </div>
        <Button
          variant="outline"
          size="sm"
          onClick={() => void refetch()}
          disabled={isFetching}
          className="gap-2 shrink-0 self-start sm:self-auto"
        >
          <RefreshCw
            className={`h-4 w-4 ${isFetching ? "animate-spin" : ""}`}
          />
          {t("admin.refresh", "Refresh")}
        </Button>
      </div>

      {/* Error state */}
      {error && (
        <div
          role="alert"
          className="p-4 rounded-xl border border-danger/30 bg-danger/10 text-danger-accent text-sm flex items-center gap-3"
        >
          <AlertCircle className="h-5 w-5 shrink-0" />
          <span>
            {error instanceof Error
              ? error.message
              : t("admin.failedToLoadContent", "Failed to load content.")}
          </span>
        </div>
      )}

      {/* Loading state */}
      {isLoading ? (
        <div className="flex items-center justify-center min-h-[240px]">
          <Loader2 className="h-8 w-8 animate-spin text-primary-accent" />
        </div>
      ) : items.length === 0 ? (
        /* Empty state */
        <div className="text-center py-16 px-4 border border-dashed border-border rounded-2xl bg-surface-card/40">
          <AlertCircle className="h-10 w-10 mx-auto text-text-muted mb-3" />
          <p className="font-semibold text-text">
            {t("adminReports.noReports", "No items have been reported yet.")}
          </p>
        </div>
      ) : (
        /* Table of reported items */
        <div className="border border-border rounded-2xl bg-surface-card overflow-hidden shadow-sm">
          <div className="overflow-x-auto">
            <table className="w-full text-left text-sm">
              <thead className="border-b border-border bg-surface-muted/60 text-xs uppercase font-semibold text-text-muted tracking-wider">
                <tr>
                  <th scope="col" className="px-5 py-3.5">
                    {t("adminReports.colItem", "Content Item")}
                  </th>
                  <th scope="col" className="px-5 py-3.5">
                    {t("adminReports.colKind", "Kind")}
                  </th>
                  <th scope="col" className="px-5 py-3.5">
                    {t("adminReports.colLevel", "Level")}
                  </th>
                  <th scope="col" className="px-5 py-3.5">
                    {t("adminReports.colStatus", "Status")}
                  </th>
                  <th scope="col" className="px-5 py-3.5 text-center">
                    {t("adminReports.colReporters", "Reporters")}
                  </th>
                  <th scope="col" className="px-5 py-3.5">
                    {t("adminReports.colLastReported", "Last Reported")}
                  </th>
                  <th scope="col" className="px-5 py-3.5 text-right">
                    {t("adminReports.colActions", "Actions")}
                  </th>
                </tr>
              </thead>
              <tbody className="divide-y divide-border">
                {items.map((row) => (
                  <tr
                    key={row.content_version_id}
                    className="hover:bg-surface-muted/30 transition-colors"
                  >
                    <td className="px-5 py-3.5 font-medium text-text">
                      <div className="font-mono text-xs text-primary-accent">
                        {row.slug}
                      </div>
                      <div className="text-[11px] text-text-muted font-mono truncate max-w-[200px]">
                        {row.content_version_id}
                      </div>
                    </td>
                    <td className="px-5 py-3.5">
                      <span className="inline-flex items-center px-2 py-0.5 rounded-full text-xs font-medium bg-surface-muted border border-border text-text">
                        {row.kind}
                      </span>
                    </td>
                    <td className="px-5 py-3.5">
                      <span className="inline-flex items-center px-2 py-0.5 rounded-md text-xs font-bold bg-primary/10 text-primary-accent border border-primary/20">
                        {row.cefr_level}
                      </span>
                    </td>
                    <td className="px-5 py-3.5">
                      <span
                        className={`inline-flex items-center px-2 py-0.5 rounded-full text-xs font-medium border ${
                          row.item_status === "archived"
                            ? "bg-danger/10 border-danger/30 text-danger-accent"
                            : row.item_status === "published"
                              ? "bg-success/10 border-success/30 text-success-accent"
                              : "bg-surface-muted border-border text-text-muted"
                        }`}
                      >
                        {row.item_status}
                      </span>
                    </td>
                    <td className="px-5 py-3.5 text-center">
                      <span className="inline-flex items-center px-2.5 py-1 rounded-full text-xs font-bold bg-warning/15 text-warning-accent border border-warning/30">
                        {t("adminReports.reportsCount", {
                          count: row.report_count,
                        })}
                      </span>
                    </td>
                    <td className="px-5 py-3.5 text-xs text-text-muted whitespace-nowrap">
                      {formatDate(row.last_reported_at)}
                    </td>
                    <td className="px-5 py-3.5 text-right">
                      <Button
                        variant="outline"
                        onClick={() => setSelectedItemId(row.item_id)}
                        className="gap-1.5 font-medium min-h-[44px]"
                      >
                        <span>
                          {t("adminReports.reviewBtn", "Review item")}
                        </span>
                        <ExternalLink className="h-3.5 w-3.5" />
                      </Button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>

          {/* Pagination */}
          {totalPages > 1 && (
            <div className="flex items-center justify-between px-5 py-3.5 border-t border-border bg-surface-muted/30">
              <span className="text-xs text-text-muted">
                {t("admin.pageOf", "Page {{current}} of {{total}}", {
                  current: currentPage,
                  total: totalPages,
                })}
              </span>
              <div className="flex items-center gap-2">
                <Button
                  variant="outline"
                  disabled={offset === 0 || isFetching}
                  onClick={() => setOffset((prev) => Math.max(0, prev - limit))}
                  className="h-11 w-11 min-h-[44px] min-w-[44px] p-0"
                  aria-label={t("common.previous", "Previous")}
                >
                  <ChevronLeft className="h-4 w-4" />
                </Button>
                <Button
                  variant="outline"
                  disabled={offset + limit >= total || isFetching}
                  onClick={() => setOffset((prev) => prev + limit)}
                  className="h-11 w-11 min-h-[44px] min-w-[44px] p-0"
                  aria-label={t("common.next", "Next")}
                >
                  <ChevronRight className="h-4 w-4" />
                </Button>
              </div>
            </div>
          )}
        </div>
      )}

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
