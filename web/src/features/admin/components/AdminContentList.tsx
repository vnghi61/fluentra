import React, { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import {
  AlertCircle,
  BookOpen,
  Loader2,
  RefreshCw,
  Search,
} from "lucide-react";
import { adminApi, type ContentItem } from "../api/adminApi";
import { AdminContentDetailModal } from "./AdminContentDetailModal";
import { Button } from "@/components/ui/button";

// The filter values are the server's, so they stay literals; only the labels are
// translated. Both lists live inside the component because a module-level
// constant cannot call t(), which is what froze every one of these in English.
const STATUS_FILTER_VALUES = [
  { value: "", labelKey: "admin.allStatusesFilter" },
  { value: "in_review", labelKey: "admin.needsReview" },
  { value: "draft", labelKey: "admin.drafts" },
  { value: "approved", labelKey: "admin.approved" },
  { value: "published", labelKey: "admin.published" },
  { value: "archived", labelKey: "admin.archived" },
] as const;

const KIND_FILTER_VALUES = [
  { value: "", labelKey: "admin.allKinds" },
  { value: "vocab_word", labelKey: "admin.kindVocabWord" },
  { value: "vocab_multiple_choice", labelKey: "admin.kindMultipleChoice" },
  { value: "vocab_match", labelKey: "admin.kindMatching" },
  { value: "vocab_cloze", labelKey: "admin.kindCloze" },
  { value: "grammar_exercise", labelKey: "admin.kindGrammarExercise" },
  { value: "reading_passage", labelKey: "admin.kindReadingPassage" },
] as const;

export const AdminContentList: React.FC = () => {
  const { t } = useTranslation();

  const limit = 15;
  const [offset, setOffset] = useState(0);

  // Filters
  const [statusFilter, setStatusFilter] = useState("");
  const [kindFilter, setKindFilter] = useState("");
  const [searchQuery, setSearchQuery] = useState("");
  // What the list is actually filtered by. Separate from searchQuery so typing
  // does not refetch on every keystroke; the form submit promotes one to the
  // other.
  const [appliedSearch, setAppliedSearch] = useState("");

  // Modal
  const [selectedItemId, setSelectedItemId] = useState<string | null>(null);

  // useQuery rather than a fetch inside useEffect: the filters are the cache key,
  // so changing one refetches without a second copy of the loading, error and
  // pagination state to keep in step with it.
  const { data, isLoading, isFetching, error, refetch } = useQuery({
    queryKey: [
      "admin",
      "content",
      statusFilter,
      kindFilter,
      appliedSearch,
      limit,
      offset,
    ],
    queryFn: () =>
      adminApi.listContent({
        status: statusFilter || undefined,
        kind: kindFilter || undefined,
        q: appliedSearch || undefined,
        limit,
        offset,
      }),
  });

  const items: ContentItem[] = data?.items ?? [];
  const total = data?.total ?? 0;
  const errorMessage =
    error === null
      ? null
      : error instanceof Error
        ? error.message
        : t("admin.failedToLoadContent");

  const handleSearchSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    setOffset(0);
    setAppliedSearch(searchQuery.trim());
  };

  const getStatusBadge = (status: string) => {
    switch (status) {
      case "draft":
        return "bg-amber-500/10 text-amber-500 border-amber-500/20";
      case "in_review":
        return "bg-blue-500/10 text-blue-500 border-blue-500/20";
      case "approved":
        return "bg-emerald-500/10 text-emerald-500 border-emerald-500/20";
      case "published":
        return "bg-green-500/10 text-green-500 border-green-500/20";
      case "archived":
        return "bg-muted text-muted-foreground border-border";
      default:
        return "bg-muted text-muted-foreground";
    }
  };

  const totalPages = Math.ceil(total / limit) || 1;
  const currentPage = Math.floor(offset / limit) + 1;

  return (
    <div className="space-y-4">
      {/* Search and Filters Bar */}
      <div className="flex flex-col sm:flex-row items-stretch sm:items-center justify-between gap-3 p-4 bg-card border border-border rounded-xl">
        <form onSubmit={handleSearchSubmit} className="relative flex-1">
          <Search className="w-4 h-4 absolute left-3 top-3 text-muted-foreground" />
          <input
            type="text"
            value={searchQuery}
            onChange={(e) => setSearchQuery(e.target.value)}
            placeholder={t("admin.searchBySlugPrefix")}
            className="w-full h-11 pl-9 pr-4 rounded-lg border border-input bg-background text-base focus:outline-none focus:ring-1 focus:ring-ring"
          />
        </form>

        <div className="flex items-center gap-2">
          {/* Status Filter */}
          <div className="relative">
            <select
              value={statusFilter}
              onChange={(e) => {
                setStatusFilter(e.target.value);
                setOffset(0);
              }}
              className="h-11 px-3 pr-8 rounded-lg border border-input bg-background text-base focus:outline-none focus:ring-1 focus:ring-ring"
            >
              {STATUS_FILTER_VALUES.map((f) => (
                <option key={f.value} value={f.value}>
                  {t(f.labelKey)}
                </option>
              ))}
            </select>
          </div>

          {/* Kind Filter */}
          <div className="relative">
            <select
              value={kindFilter}
              onChange={(e) => {
                setKindFilter(e.target.value);
                setOffset(0);
              }}
              className="h-11 px-3 pr-8 rounded-lg border border-input bg-background text-base focus:outline-none focus:ring-1 focus:ring-ring"
            >
              {KIND_FILTER_VALUES.map((f) => (
                <option key={f.value} value={f.value}>
                  {t(f.labelKey)}
                </option>
              ))}
            </select>
          </div>

          <Button
            variant="outline"
            size="sm"
            onClick={() => void refetch()}
            disabled={isFetching}
            title={t("admin.refresh")}
            className="h-11 w-11 shrink-0 p-0 flex items-center justify-center"
          >
            <RefreshCw
              className={`w-4 h-4 ${isFetching ? "animate-spin" : ""}`}
            />
          </Button>
        </div>
      </div>

      {/* Error State */}
      {errorMessage !== null && (
        <div className="p-4 rounded-lg bg-destructive/10 border border-destructive/20 text-destructive flex items-center gap-3">
          <AlertCircle className="w-5 h-5 flex-shrink-0" />
          <p className="text-sm">{errorMessage}</p>
        </div>
      )}

      {/* Content Table */}
      <div className="bg-card border border-border rounded-xl overflow-hidden shadow-sm">
        <div className="overflow-x-auto">
          <table className="w-full text-left text-sm">
            <thead className="bg-muted/50 border-b border-border text-xs uppercase text-muted-foreground">
              <tr>
                <th className="px-4 py-3 font-medium w-12 text-right">
                  {t("common.rowNumber")}
                </th>
                <th className="px-4 py-3 font-medium">{t("admin.slugOrId")}</th>
                <th className="px-4 py-3 font-medium">{t("admin.kind")}</th>
                <th className="px-4 py-3 font-medium">{t("admin.status")}</th>
                <th className="px-4 py-3 font-medium">{t("admin.updated")}</th>
                <th className="px-4 py-3 font-medium text-right">
                  {t("common.actions")}
                </th>
              </tr>
            </thead>
            <tbody className="divide-y divide-border">
              {isLoading && items.length === 0 ? (
                <tr>
                  <td
                    colSpan={6}
                    className="py-12 text-center text-muted-foreground"
                  >
                    <Loader2 className="w-6 h-6 animate-spin mx-auto mb-2 text-primary" />
                    {t("admin.loadingContent")}
                  </td>
                </tr>
              ) : items.length === 0 ? (
                <tr>
                  <td
                    colSpan={6}
                    className="py-12 text-center text-muted-foreground"
                  >
                    <BookOpen className="w-8 h-8 mx-auto mb-2 opacity-40" />
                    {t("admin.noContentFound")}
                  </td>
                </tr>
              ) : (
                items.map((item, index) => (
                  <tr
                    key={item.id}
                    className="hover:bg-muted/30 transition-colors cursor-pointer"
                    onClick={() => setSelectedItemId(item.id)}
                  >
                    <td className="px-4 py-3 text-right tabular-nums text-muted-foreground">
                      {offset + index + 1}
                    </td>
                    <td className="px-4 py-3">
                      <div className="font-semibold text-foreground">
                        {item.slug}
                      </div>
                      <div className="font-mono text-[11px] text-muted-foreground">
                        {item.id}
                      </div>
                    </td>
                    <td className="px-4 py-3">
                      <span className="font-mono text-xs px-2 py-0.5 rounded bg-muted text-muted-foreground">
                        {item.kind}
                      </span>
                    </td>
                    <td className="px-4 py-3">
                      <span
                        className={`text-xs px-2.5 py-0.5 rounded-full font-medium border ${getStatusBadge(
                          item.status,
                        )}`}
                      >
                        {item.status}
                      </span>
                    </td>
                    <td className="px-4 py-3 text-xs text-muted-foreground">
                      {new Date(item.updated_at).toLocaleString()}
                    </td>
                    <td className="px-4 py-3 text-right">
                      <Button
                        size="sm"
                        variant="ghost"
                        onClick={(e) => {
                          e.stopPropagation();
                          setSelectedItemId(item.id);
                        }}
                      >
                        {t("admin.editAndReview")}
                      </Button>
                    </td>
                  </tr>
                ))
              )}
            </tbody>
          </table>
        </div>

        {/* Pagination Bar */}
        <div className="flex items-center justify-between px-4 py-3 border-t border-border bg-muted/20 text-xs text-muted-foreground">
          <div>
            {t("admin.showingRange", {
              from: items.length > 0 ? offset + 1 : 0,
              to: Math.min(offset + items.length, total),
              total,
            })}
          </div>
          <div className="flex items-center gap-2">
            <Button
              variant="outline"
              size="sm"
              disabled={offset === 0 || isLoading}
              onClick={() => setOffset((prev) => Math.max(0, prev - limit))}
              className="min-h-11 px-4 text-xs"
            >
              {t("common.previous")}
            </Button>
            <span>
              {t("admin.pageOf", { current: currentPage, total: totalPages })}
            </span>
            <Button
              variant="outline"
              size="sm"
              disabled={offset + limit >= total || isLoading}
              onClick={() => setOffset((prev) => prev + limit)}
              className="min-h-11 px-4 text-xs"
            >
              {t("common.next")}
            </Button>
          </div>
        </div>
      </div>

      {/* Content Detail & State Machine Transition Modal */}
      {selectedItemId && (
        <AdminContentDetailModal
          itemId={selectedItemId}
          onClose={() => setSelectedItemId(null)}
          onItemUpdated={() => void refetch()}
        />
      )}
    </div>
  );
};
