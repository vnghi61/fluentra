import React, { useState } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import {
  BookOpen,
  Circle,
  ClipboardCheck,
  FileText,
  Filter,
  MoreHorizontal,
  Pause,
  Plus,
  RefreshCw,
  Search,
  Shield,
  X,
} from "lucide-react";
import {
  useAdminContent,
  useAdminContentSummary,
} from "../api/useAdminContent";
import { adminContentKeys } from "../api/contentKeys";
import { PERMISSIONS, usePermissions } from "../model/permissions";
import type { ContentItem } from "../api/adminApi";
import { AdminContentDetailModal } from "./AdminContentDetailModal";
import { CreateContentModal } from "./CreateContentModal";
import { AdminPageHeader } from "./AdminPageHeader";
import { CopyableId } from "./CopyableId";
import { StatusBadge, type StatusTone } from "./StatusBadge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Pagination } from "@/components/ui/pagination";
import { Skeleton } from "@/components/ui/skeleton";
import { EmptyState } from "@/components/ui/empty-state";
import { ErrorState } from "@/components/ui/error-state";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import {
  Sheet,
  SheetBody,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
} from "@/components/ui/sheet";
import { cn } from "@/lib/utils";
import { NativeSelect } from "@/components/ui/native-select";

const STATUS_FILTERS = [
  { value: "", labelKey: "admin.allStatusesFilter" },
  { value: "in_review", labelKey: "admin.needsReview" },
  { value: "draft", labelKey: "admin.drafts" },
  { value: "approved", labelKey: "admin.approved" },
  { value: "published", labelKey: "admin.published" },
  { value: "archived", labelKey: "admin.archived" },
] as const;
const KIND_FILTERS = [
  { value: "", labelKey: "admin.allKinds" },
  { value: "vocab_word", labelKey: "admin.kindVocabWord" },
  { value: "vocab_multiple_choice", labelKey: "admin.kindMultipleChoice" },
  { value: "vocab_match", labelKey: "admin.kindMatching" },
  { value: "vocab_cloze", labelKey: "admin.kindCloze" },
  { value: "grammar_exercise", labelKey: "admin.kindGrammarExercise" },
  { value: "reading_passage", labelKey: "admin.kindReadingPassage" },
] as const;
const statusTones: Record<ContentItem["status"], StatusTone> = {
  draft: "warning",
  in_review: "info",
  approved: "success",
  published: "success",
  archived: "neutral",
};

export const AdminContentList: React.FC = () => {
  const { t, i18n } = useTranslation();
  const { can } = usePermissions();
  const queryClient = useQueryClient();
  const [limit, setLimit] = useState(10);
  const [offset, setOffset] = useState(0);
  const [statusFilter, setStatusFilter] = useState("");
  const [kindFilter, setKindFilter] = useState("");
  const [searchQuery, setSearchQuery] = useState("");
  const [appliedSearch, setAppliedSearch] = useState("");
  const [filtersOpen, setFiltersOpen] = useState(false);
  const [draftFilters, setDraftFilters] = useState({ status: "", kind: "" });
  const [createOpen, setCreateOpen] = useState(false);
  const [selectedItemId, setSelectedItemId] = useState<string | null>(null);
  const { data, isLoading, isFetching, error, refetch } = useAdminContent({
    status: statusFilter || undefined,
    kind: kindFilter || undefined,
    q: appliedSearch || undefined,
    limit,
    offset,
  });
  const summary = useAdminContentSummary({
    kind: kindFilter || undefined,
    q: appliedSearch || undefined,
  });
  const items = data?.items ?? [];
  const total = data?.total ?? 0;
  const currentPage = Math.floor(offset / limit) + 1;
  const totalPages = Math.max(1, Math.ceil(total / limit));
  const filtered = Boolean(statusFilter || kindFilter || appliedSearch);
  const refresh = () => {
    void refetch();
    void summary.refetch();
  };
  const invalidateContent = () => {
    void queryClient.invalidateQueries({ queryKey: adminContentKeys.lists });
    void queryClient.invalidateQueries({
      queryKey: adminContentKeys.summaries,
    });
  };
  const resetFilters = () => {
    setOffset(0);
    setStatusFilter("");
    setKindFilter("");
    setSearchQuery("");
    setAppliedSearch("");
    setFiltersOpen(false);
  };
  const handleSearchSubmit = (event: React.FormEvent) => {
    event.preventDefault();
    setOffset(0);
    const term = searchQuery.trim();
    setAppliedSearch(term);
    if (offset === 0 && term === appliedSearch) void refetch();
  };
  const kindLabel = (kind: string) => {
    const option = KIND_FILTERS.find((entry) => entry.value === kind);
    return option ? t(option.labelKey) : kind;
  };
  const statusBadge = (status: ContentItem["status"]) => (
    <StatusBadge tone={statusTones[status]}>
      <span
        aria-hidden="true"
        className="h-1.5 w-1.5 shrink-0 rounded-full bg-current"
      />
      {t(`admin.contentStatus.${status}`, status)}
    </StatusBadge>
  );
  const kindBadge = (kind: string) => (
    <span
      title={kind}
      className="inline-block max-w-full break-all rounded-full bg-primary/5 px-2.5 py-1 text-xs font-medium text-text-muted"
    >
      {kindLabel(kind)}
    </span>
  );
  const updatedTime = (date: string) => (
    <time dateTime={date} className="text-sm tabular-nums text-text-muted">
      {new Date(date).toLocaleString(i18n.resolvedLanguage, {
        day: "2-digit",
        month: "short",
        year: "numeric",
        hour: "2-digit",
        minute: "2-digit",
      })}
    </time>
  );
  const actionMenu = (item: ContentItem) => (
    <DropdownMenu>
      <DropdownMenuTrigger
        aria-label={t("admin.actionsFor", { name: item.slug })}
        className="flex h-11 w-11 shrink-0 items-center justify-center rounded-xl border border-border-subtle text-text-muted transition-colors hover:bg-surface-muted hover:text-text focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary"
      >
        <MoreHorizontal aria-hidden="true" className="h-4 w-4" />
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end">
        <DropdownMenuItem onSelect={() => setSelectedItemId(item.id)}>
          <FileText aria-hidden="true" className="h-4 w-4" />
          {t("admin.editAndReview")}
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  );
  const stats = [
    {
      key: "total",
      label: t("admin.totalContent"),
      count: summary.data?.total,
      Icon: FileText,
      tone: "bg-primary/10 text-primary-accent",
    },
    {
      key: "published",
      label: t("admin.published"),
      count: summary.data?.published,
      Icon: Circle,
      tone: "bg-success/10 text-success-accent",
    },
    {
      key: "review",
      label: t("admin.needsReview"),
      count: summary.data?.review,
      Icon: ClipboardCheck,
      tone: "bg-warning/10 text-warning-accent",
    },
    {
      key: "other",
      label: t("admin.otherContent"),
      count: summary.data?.other,
      Icon: Pause,
      tone: "bg-surface-muted text-text-muted",
    },
  ];

  return (
    <div className="space-y-4">
      <header className="space-y-2">
        <p className="inline-flex items-center gap-1 text-xs font-semibold uppercase tracking-wider text-primary-accent">
          <Shield aria-hidden="true" className="h-4 w-4" />
          {t("page.administration")}
        </p>
        <AdminPageHeader
          title={t("admin.contentLibrary")}
          description={t("admin.contentLibraryDescription")}
          actions={
            can(PERMISSIONS.contentCreate) ? (
              <Button
                type="button"
                onClick={() => setCreateOpen(true)}
                className="whitespace-nowrap"
              >
                <Plus aria-hidden="true" className="h-4 w-4" />
                {t("admin.addContent")}
              </Button>
            ) : undefined
          }
        />
      </header>

      <div className="space-y-3 rounded-2xl border border-border-subtle bg-surface-card p-4">
        <form
          onSubmit={handleSearchSubmit}
          className="grid grid-cols-[minmax(0,1fr)_minmax(0,1fr)_2.75rem] items-center gap-2 sm:flex sm:flex-wrap sm:gap-3"
        >
          <div className="relative col-span-3 w-full sm:min-w-0 sm:flex-1">
            <Search
              aria-hidden="true"
              className="pointer-events-none absolute left-3.5 top-3.5 h-4 w-4 text-text-muted"
            />
            <Input
              type="search"
              value={searchQuery}
              onChange={(event) => setSearchQuery(event.target.value)}
              aria-label={t("admin.searchBySlugPrefix")}
              placeholder={t("admin.searchBySlugPrefix")}
              className="bg-surface-muted pl-10"
            />
          </div>
          <NativeSelect
            value={statusFilter}
            aria-label={t("admin.filterByStatus")}
            onChange={(event) => {
              setStatusFilter(event.target.value);
              setOffset(0);
            }}
            wrapperClassName="hidden lg:block lg:w-auto"
          >
            {STATUS_FILTERS.map((filter) => (
              <option key={filter.value} value={filter.value}>
                {t(filter.labelKey)}
              </option>
            ))}
          </NativeSelect>
          <NativeSelect
            value={kindFilter}
            aria-label={t("admin.filterByKind")}
            onChange={(event) => {
              setKindFilter(event.target.value);
              setOffset(0);
            }}
            wrapperClassName="hidden lg:block lg:w-auto"
          >
            {KIND_FILTERS.map((filter) => (
              <option key={filter.value} value={filter.value}>
                {t(filter.labelKey)}
              </option>
            ))}
          </NativeSelect>
          <Button
            variant="outline"
            type="button"
            className="relative min-w-11 gap-1.5 whitespace-nowrap px-2 sm:flex-none sm:gap-2 sm:px-3 lg:hidden"
            onClick={() => {
              setDraftFilters({ status: statusFilter, kind: kindFilter });
              setFiltersOpen(true);
            }}
          >
            <Filter aria-hidden="true" className="h-4 w-4" />
            {t("admin.filters")}
            {(kindFilter || statusFilter) && (
              <span
                aria-hidden="true"
                className="absolute right-1.5 top-1.5 h-1.5 w-1.5 rounded-full bg-primary"
              />
            )}
          </Button>
          <Button
            type="submit"
            className="min-w-11 gap-1.5 whitespace-nowrap px-2 sm:flex-none sm:gap-2 sm:px-3"
          >
            <Search aria-hidden="true" className="h-4 w-4" />
            {t("admin.search")}
          </Button>
          <Button
            type="button"
            variant="outline"
            size="icon"
            onClick={refresh}
            disabled={isFetching || summary.isFetching}
            aria-label={t("admin.refresh")}
          >
            <RefreshCw
              aria-hidden="true"
              className={cn(
                "h-4 w-4",
                (isFetching || summary.isFetching) &&
                  "animate-spin motion-reduce:animate-none",
              )}
            />
          </Button>
        </form>
        {filtered && (
          <div className="flex flex-wrap items-center gap-2 text-xs">
            <span className="text-text-muted">{t("admin.activeFilters")}</span>
            {appliedSearch && (
              <span className="max-w-full break-all rounded-lg bg-surface-muted px-2.5 py-1.5 text-text">
                {appliedSearch}
              </span>
            )}
            {statusFilter && (
              <span className="rounded-lg bg-surface-muted px-2.5 py-1.5 text-text">
                {t(
                  STATUS_FILTERS.find((filter) => filter.value === statusFilter)
                    ?.labelKey ?? "admin.allStatusesFilter",
                )}
              </span>
            )}
            {kindFilter && kindBadge(kindFilter)}
            <Button
              type="button"
              variant="ghost"
              size="sm"
              onClick={resetFilters}
            >
              <X aria-hidden="true" className="h-4 w-4" />
              {t("admin.resetFilters")}
            </Button>
          </div>
        )}
      </div>

      <section
        aria-label={t("admin.contentSummary")}
        aria-describedby="content-summary-scope"
        className="space-y-2"
      >
        <div className="grid grid-cols-2 gap-3 lg:grid-cols-4">
          {stats.map(({ key, label, count, Icon, tone }) => (
            <div
              key={key}
              className="flex min-w-0 flex-col gap-3 rounded-2xl border border-border-subtle bg-surface-card p-4 sm:flex-row sm:items-center sm:gap-4 sm:p-5"
            >
              <span
                className={cn(
                  "flex h-10 w-10 shrink-0 items-center justify-center rounded-full",
                  tone,
                )}
              >
                <Icon aria-hidden="true" className="h-5 w-5" />
              </span>
              <div className="min-w-0 flex-1">
                {summary.isLoading ? (
                  <Skeleton className="mb-1 h-8 w-16" />
                ) : (
                  <p
                    data-testid={`content-stat-${key}`}
                    className="truncate text-2xl font-bold tabular-nums text-text"
                  >
                    {summary.isError || count == null
                      ? "—"
                      : count.toLocaleString(i18n.resolvedLanguage)}
                  </p>
                )}
                <p className="mt-0.5 text-sm text-text-muted">{label}</p>
              </div>
              {key !== "total" &&
                !summary.isError &&
                count != null &&
                summary.data &&
                summary.data.total > 0 &&
                count <= summary.data.total && (
                  <span
                    className={cn(
                      "hidden shrink-0 rounded-lg px-2 py-1 text-xs font-medium tabular-nums 2xl:inline-block",
                      tone,
                    )}
                  >
                    {(count / summary.data.total).toLocaleString(
                      i18n.resolvedLanguage,
                      { style: "percent", maximumFractionDigits: 1 },
                    )}
                  </span>
                )}
            </div>
          ))}
        </div>
        <p
          id="content-summary-scope"
          className={cn("text-sm text-text-muted", !filtered && "sr-only")}
        >
          {t("admin.contentSummaryScope")}
        </p>
        {summary.isError && (
          <div
            role="alert"
            className="flex flex-wrap items-center justify-between gap-2 rounded-xl border border-warning/20 bg-warning/5 px-4 py-2 text-sm text-text-muted"
          >
            <span>{t("admin.failedToLoadContentSummary")}</span>
            <Button
              type="button"
              size="sm"
              variant="outline"
              onClick={() => void summary.refetch()}
            >
              {t("admin.retryContentSummary")}
            </Button>
          </div>
        )}
      </section>

      <div className="overflow-hidden rounded-2xl border border-border-subtle bg-surface-card shadow-sm">
        {isLoading ? (
          <div
            role="status"
            aria-label={t("admin.loadingContent")}
            aria-busy="true"
            className="divide-y divide-border-subtle"
          >
            {Array.from({ length: 5 }, (_, index) => (
              <div key={index} className="flex h-20 items-center gap-3 px-4">
                <Skeleton className="h-9 w-9 shrink-0 rounded-full" />
                <div className="flex-1 space-y-2">
                  <Skeleton className="h-3.5 w-1/2" />
                  <Skeleton className="h-3 w-1/3" />
                </div>
                <Skeleton className="hidden h-6 w-24 rounded-full md:block" />
              </div>
            ))}
          </div>
        ) : error ? (
          <ErrorState
            title={t("admin.failedToLoadContent")}
            onRetry={() => void refetch()}
            className="py-10"
          />
        ) : items.length === 0 ? (
          <EmptyState
            icon={BookOpen}
            title={t("admin.noContentFound")}
            description={t("admin.tryAdjustingYourSearchQueryOrFilters")}
            className="py-10"
          />
        ) : (
          <>
            <div className="hidden overflow-x-auto md:block">
              <table className="w-full min-w-[920px] table-fixed border-collapse text-left text-sm">
                <caption className="sr-only">
                  {t("admin.contentLibrary")}
                </caption>
                <thead className="border-b border-border-subtle bg-surface-muted/60 text-xs font-semibold text-text-muted">
                  <tr>
                    <th scope="col" className="w-14 px-4 py-3 text-right">
                      {t("common.rowNumber")}
                    </th>
                    <th scope="col" className="px-4 py-3">
                      {t("admin.slugOrId")}
                    </th>
                    <th scope="col" className="w-48 px-4 py-3">
                      {t("admin.kind")}
                    </th>
                    <th scope="col" className="w-40 px-4 py-3">
                      {t("admin.status")}
                    </th>
                    <th scope="col" className="w-44 px-4 py-3">
                      {t("admin.updated")}
                    </th>
                    <th scope="col" className="w-20 px-4 py-3 text-right">
                      {t("common.actions")}
                    </th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-border-subtle">
                  {items.map((item, index) => (
                    <tr
                      key={item.id}
                      className="h-16 transition-colors hover:bg-surface-muted/40"
                    >
                      <td className="px-4 py-2 text-right tabular-nums text-text-muted">
                        {offset + index + 1}
                      </td>
                      <td className="px-4 py-2">
                        <div className="flex min-w-0 items-center gap-3">
                          <span className="flex h-9 w-9 shrink-0 items-center justify-center rounded-full bg-primary/5 text-primary-accent">
                            <FileText aria-hidden="true" className="h-4 w-4" />
                          </span>
                          <div className="flex min-w-0 flex-1 items-center justify-between gap-2">
                            <button
                              type="button"
                              title={item.slug}
                              onClick={() => setSelectedItemId(item.id)}
                              className="flex min-h-11 min-w-11 flex-1 flex-col items-start justify-center gap-1 rounded-md text-left text-sm font-semibold text-text hover:text-primary-accent focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary"
                            >
                              <span className="max-w-full truncate">
                                {item.slug}
                              </span>
                              <span
                                className="font-mono text-xs font-normal text-text-muted"
                                aria-hidden="true"
                              >
                                {item.id.slice(0, 8)}…
                              </span>
                            </button>
                            <CopyableId id={item.id} compact />
                          </div>
                        </div>
                      </td>
                      <td className="px-4 py-2">{kindBadge(item.kind)}</td>
                      <td className="px-4 py-2">{statusBadge(item.status)}</td>
                      <td className="px-4 py-2">
                        {updatedTime(item.updated_at)}
                      </td>
                      <td className="px-4 py-2">{actionMenu(item)}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
            <ul
              data-testid="content-cards"
              className="divide-y divide-border-subtle md:hidden"
            >
              {items.map((item) => (
                <li key={item.id} className="space-y-3 p-4">
                  <div className="flex items-start gap-3">
                    <span className="flex h-10 w-10 shrink-0 items-center justify-center rounded-full bg-primary/5 text-primary-accent">
                      <FileText aria-hidden="true" className="h-5 w-5" />
                    </span>
                    <button
                      type="button"
                      onClick={() => setSelectedItemId(item.id)}
                      className="min-h-11 min-w-11 flex-1 break-all rounded-md text-left text-sm font-semibold text-text hover:text-primary-accent focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary"
                    >
                      {item.slug}
                    </button>
                    {actionMenu(item)}
                  </div>
                  <div className="flex flex-wrap items-center gap-2">
                    {kindBadge(item.kind)}
                    {statusBadge(item.status)}
                  </div>
                  <div className="flex flex-wrap items-center justify-between gap-2">
                    <CopyableId id={item.id} />
                    {updatedTime(item.updated_at)}
                  </div>
                </li>
              ))}
            </ul>
          </>
        )}
        {!error && (
          <Pagination
            page={currentPage}
            pageCount={totalPages}
            total={total}
            rangeFrom={items.length > 0 ? offset + 1 : 0}
            rangeTo={Math.min(offset + items.length, total)}
            isBusy={isFetching}
            canPrevious={offset > 0}
            canNext={offset + limit < total}
            onPrevious={() =>
              setOffset((previous) => Math.max(0, previous - limit))
            }
            onNext={() => setOffset((previous) => previous + limit)}
            onPageSelect={(next) => setOffset((next - 1) * limit)}
            pageSize={limit}
            pageSizeOptions={[10, 15, 25, 50]}
            onPageSizeChange={(next) => {
              setLimit(next);
              setOffset(0);
            }}
            className="bg-surface-card"
          />
        )}
      </div>

      <Sheet open={filtersOpen} onOpenChange={setFiltersOpen}>
        <SheetContent>
          <SheetHeader>
            <SheetTitle className="text-lg font-semibold leading-tight text-text">
              {t("admin.filters")}
            </SheetTitle>
            <SheetDescription className="mt-1 text-sm text-text-muted">
              {t("admin.contentFiltersHint")}
            </SheetDescription>
          </SheetHeader>
          <SheetBody className="space-y-5">
            <label className="block space-y-2 text-sm font-medium text-text">
              <span>{t("admin.filterByStatus")}</span>
              <NativeSelect
                value={draftFilters.status}
                onChange={(event) =>
                  setDraftFilters((previous) => ({
                    ...previous,
                    status: event.target.value,
                  }))
                }
              >
                {STATUS_FILTERS.map((filter) => (
                  <option key={filter.value} value={filter.value}>
                    {t(filter.labelKey)}
                  </option>
                ))}
              </NativeSelect>
            </label>
            <label className="block space-y-2 text-sm font-medium text-text">
              <span>{t("admin.filterByKind")}</span>
              <NativeSelect
                value={draftFilters.kind}
                onChange={(event) =>
                  setDraftFilters((previous) => ({
                    ...previous,
                    kind: event.target.value,
                  }))
                }
              >
                {KIND_FILTERS.map((filter) => (
                  <option key={filter.value} value={filter.value}>
                    {t(filter.labelKey)}
                  </option>
                ))}
              </NativeSelect>
            </label>
            <div className="flex flex-wrap justify-between gap-3">
              <Button variant="outline" onClick={resetFilters}>
                {t("admin.resetFilters")}
              </Button>
              <Button
                onClick={() => {
                  setOffset(0);
                  setStatusFilter(draftFilters.status);
                  setKindFilter(draftFilters.kind);
                  setFiltersOpen(false);
                }}
              >
                {t("common.apply")}
              </Button>
            </div>
          </SheetBody>
        </SheetContent>
      </Sheet>
      {createOpen && can(PERMISSIONS.contentCreate) && (
        <CreateContentModal
          open
          onOpenChange={setCreateOpen}
          kindOptions={KIND_FILTERS.filter((option) => option.value).map(
            (option) => ({ value: option.value, label: t(option.labelKey) }),
          )}
          onCreated={(item) => {
            invalidateContent();
            setSelectedItemId(item.id);
          }}
        />
      )}
      {selectedItemId && (
        <AdminContentDetailModal
          itemId={selectedItemId}
          onClose={() => setSelectedItemId(null)}
          onItemUpdated={invalidateContent}
        />
      )}
    </div>
  );
};
