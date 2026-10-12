import { useState } from "react";
import { useTranslation } from "react-i18next";
import {
  ClipboardCheck,
  Filter,
  MoreHorizontal,
  RefreshCw,
  Search,
  X,
} from "lucide-react";
import type { AdminReviewQueueItem } from "../api/adminApi";
import { useAdminReviewQueue } from "../api/useAdminReviewQueue";
import { CopyableId } from "./CopyableId";
import { StatusBadge } from "./StatusBadge";
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

const PURPOSES = ["", "foundation", "bank", "resource"] as const;
const KINDS = [
  "",
  "grammar_tense_choice",
  "grammar_sentence_transform",
  "vocab_multiple_choice",
  "vocab_context_choice",
  "reading_comprehension",
  "listening_comprehension",
  "writing_prompt",
  "speaking_task",
  "foundation_topic",
  "foundation_quiz",
  "foundation_review",
] as const;

export function ReviewQueueDirectory({
  onReview,
}: {
  onReview: (item: AdminReviewQueueItem) => void;
}) {
  const { t, i18n } = useTranslation();
  const [filters, setFilters] = useState({ purpose: "", kind: "" });
  const [draftFilters, setDraftFilters] = useState(filters);
  const [filtersOpen, setFiltersOpen] = useState(false);
  const [offset, setOffset] = useState(0);
  const [limit, setLimit] = useState(15);
  const [search, setSearch] = useState("");
  const query = useAdminReviewQueue({
    purpose: filters.purpose || undefined,
    kind: filters.kind || undefined,
    limit,
    offset,
  });
  const items = query.data?.items ?? [];
  const total = query.data?.total ?? 0;
  const term = search.trim().toLocaleLowerCase();
  const visibleItems = items.filter(
    (item) =>
      !term ||
      item.slug.toLocaleLowerCase().includes(term) ||
      item.item_id.toLocaleLowerCase().includes(term),
  );
  const filtered = Boolean(filters.purpose || filters.kind || search);
  const changePage = (next: number) => {
    setOffset(next);
    setSearch("");
  };
  const applyFilters = (next: typeof filters) => {
    setFilters(next);
    changePage(0);
  };
  const reset = () => {
    applyFilters({ purpose: "", kind: "" });
    setFiltersOpen(false);
  };
  const tags = (item: AdminReviewQueueItem) => (
    <div className="flex flex-wrap gap-1.5 text-xs text-text-muted">
      <span className="rounded-md bg-primary/5 px-2 py-1">
        {item.cefr_level}
      </span>
      {(item.node_codes ?? []).map((code) => (
        <span
          key={code}
          className="max-w-full break-all rounded-md bg-primary/5 px-2 py-1"
        >
          {code}
        </span>
      ))}
    </div>
  );
  const status = (item: AdminReviewQueueItem) => (
    <StatusBadge
      tone={
        item.status === "draft" || item.status === "in_review"
          ? "warning"
          : item.status === "approved" || item.status === "published"
            ? "success"
            : "neutral"
      }
    >
      <span
        aria-hidden="true"
        className="h-1.5 w-1.5 shrink-0 rounded-full bg-current"
      />
      {item.status === "draft" || item.status === "in_review"
        ? t("admin.needsReview")
        : t(`admin.contentStatus.${item.status}`)}
    </StatusBadge>
  );
  const date = (item: AdminReviewQueueItem) => (
    <time
      dateTime={item.created_at}
      className="text-sm tabular-nums text-text-muted"
    >
      {new Date(item.created_at).toLocaleString(i18n.resolvedLanguage, {
        day: "2-digit",
        month: "short",
        year: "numeric",
        hour: "2-digit",
        minute: "2-digit",
      })}
    </time>
  );
  const actions = (item: AdminReviewQueueItem) => (
    <DropdownMenu>
      <DropdownMenuTrigger
        aria-label={t("admin.actionsFor", { name: item.slug })}
        className="flex h-11 w-11 shrink-0 items-center justify-center rounded-xl border border-border-subtle text-text-muted hover:bg-surface-muted focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary"
      >
        <MoreHorizontal className="h-4 w-4" aria-hidden="true" />
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end">
        <DropdownMenuItem onSelect={() => onReview(item)}>
          <ClipboardCheck className="h-4 w-4" aria-hidden="true" />
          {t("adminReview.openReview")}
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  );

  return (
    <div className="space-y-4">
      <div className="flex flex-wrap items-center gap-2">
        <StatusBadge tone="warning">
          <ClipboardCheck className="h-4 w-4" aria-hidden="true" />
          {t("admin.needsReview")}
          {query.data && !query.isError && (
            <span className="font-semibold tabular-nums">
              ({total.toLocaleString(i18n.resolvedLanguage)})
            </span>
          )}
        </StatusBadge>
        <p className="text-sm text-text-muted">{t("adminReview.queueScope")}</p>
      </div>
      <div className="space-y-3 rounded-2xl border border-border-subtle bg-surface-card p-4">
        <div className="grid grid-cols-[minmax(0,1fr)_2.75rem] items-center gap-2 sm:flex sm:flex-wrap sm:gap-3">
          <div className="relative col-span-2 w-full sm:min-w-0 sm:flex-1">
            <Search
              className="pointer-events-none absolute left-3.5 top-3.5 h-4 w-4 text-text-muted"
              aria-hidden="true"
            />
            <Input
              type="search"
              value={search}
              onChange={(event) => setSearch(event.target.value)}
              placeholder={t("adminReview.searchPage")}
              aria-label={t("adminReview.searchPage")}
              aria-describedby="review-search-scope"
              className="bg-surface-muted pl-10"
            />
          </div>
          <NativeSelect
            value={filters.purpose}
            onChange={(event) =>
              applyFilters({ ...filters, purpose: event.target.value })
            }
            aria-label={t("adminReview.purposeFilter")}
            wrapperClassName="hidden lg:block lg:w-auto"
          >
            {PURPOSES.map((value) => (
              <option key={value} value={value}>
                {value || t("adminReview.allPurposes")}
              </option>
            ))}
          </NativeSelect>
          <NativeSelect
            value={filters.kind}
            onChange={(event) =>
              applyFilters({ ...filters, kind: event.target.value })
            }
            aria-label={t("adminReview.kindFilter")}
            wrapperClassName="hidden lg:block lg:w-auto"
          >
            {KINDS.map((value) => (
              <option key={value} value={value}>
                {value || t("adminReview.allKinds")}
              </option>
            ))}
          </NativeSelect>
          <Button
            variant="outline"
            className="min-w-11 lg:hidden"
            onClick={() => {
              setDraftFilters(filters);
              setFiltersOpen(true);
            }}
          >
            <Filter className="h-4 w-4" aria-hidden="true" />
            {t("admin.filters")}
          </Button>
          <Button
            variant="outline"
            size="icon"
            aria-label={t("admin.refresh")}
            disabled={query.isFetching}
            onClick={() => void query.refetch()}
          >
            <RefreshCw
              className={cn(
                "h-4 w-4",
                query.isFetching && "animate-spin motion-reduce:animate-none",
              )}
              aria-hidden="true"
            />
          </Button>
        </div>
        <p id="review-search-scope" className="text-sm text-text-muted">
          {t("adminReview.searchScope")}
          {term && query.data && (
            <>
              {" "}
              {t("adminReview.matchesOnPage", { count: visibleItems.length })}
            </>
          )}
        </p>
        {filtered && (
          <div className="flex flex-wrap items-center gap-2 text-xs text-text-muted">
            <span>{t("admin.activeFilters")}</span>
            {[filters.purpose, filters.kind].filter(Boolean).map((value) => (
              <span
                key={value}
                className="max-w-full break-all rounded-lg bg-surface-muted px-2.5 py-1.5"
              >
                {value}
              </span>
            ))}
            <Button variant="ghost" size="sm" onClick={reset}>
              <X className="h-4 w-4" aria-hidden="true" />
              {t("admin.resetFilters")}
            </Button>
          </div>
        )}
      </div>
      <div className="overflow-hidden rounded-2xl border border-border-subtle bg-surface-card shadow-sm">
        {query.isLoading ? (
          <div
            role="status"
            aria-label={t("adminReview.loading")}
            aria-busy="true"
            className="divide-y divide-border-subtle"
          >
            {Array.from({ length: 5 }, (_, index) => (
              <div key={index} className="space-y-3 p-5">
                <Skeleton className="h-4 w-2/3" />
                <Skeleton className="h-3 w-1/3" />
              </div>
            ))}
          </div>
        ) : query.isError ? (
          <ErrorState
            title={t("adminReview.errorDesc")}
            onRetry={() => void query.refetch()}
            className="py-10"
          />
        ) : visibleItems.length === 0 ? (
          <EmptyState
            icon={ClipboardCheck}
            title={
              term ? t("adminReview.noPageMatches") : t("adminReview.empty")
            }
            {...(term ? { description: t("adminReview.searchScope") } : {})}
            className="py-10"
          />
        ) : (
          <>
            <div className="hidden overflow-x-auto md:block">
              <table className="w-full min-w-[920px] table-fixed text-left text-sm">
                <caption className="sr-only">{t("adminReview.title")}</caption>
                <thead className="border-b border-border-subtle bg-surface-muted/60 text-xs font-semibold text-text-muted">
                  <tr>
                    <th scope="col" className="px-5 py-3">
                      {t("adminReview.contentColumn")}
                    </th>
                    <th scope="col" className="w-52 px-4 py-3">
                      {t("admin.kind")}
                    </th>
                    <th scope="col" className="w-36 px-4 py-3">
                      {t("admin.status")}
                    </th>
                    <th scope="col" className="w-44 px-4 py-3">
                      {t("adminReview.createdColumn")}
                    </th>
                    <th scope="col" className="w-20 px-4 py-3">
                      {t("common.actions")}
                    </th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-border-subtle">
                  {visibleItems.map((item) => (
                    <tr
                      key={item.id}
                      className="transition-colors hover:bg-surface-muted/40"
                    >
                      <td className="space-y-2 px-5 py-3">
                        <div className="flex min-w-0 items-center gap-2">
                          <button
                            type="button"
                            onClick={() => onReview(item)}
                            className="flex min-h-11 min-w-11 flex-1 flex-col items-start justify-center gap-1 rounded-md text-left font-semibold text-text hover:text-primary-accent focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary"
                          >
                            <span
                              title={item.slug}
                              className="max-w-full truncate"
                            >
                              {item.slug}
                            </span>
                            <span
                              aria-hidden="true"
                              className="font-mono text-xs font-normal text-text-muted"
                            >
                              {item.item_id.slice(0, 8)}…
                            </span>
                          </button>
                          <CopyableId id={item.item_id} compact />
                        </div>
                        {tags(item)}
                      </td>
                      <td className="px-4 py-3">
                        <span className="max-w-full break-all rounded-full bg-primary/5 px-2.5 py-1 text-xs text-text-muted">
                          {item.kind}
                        </span>
                      </td>
                      <td className="px-4 py-3">{status(item)}</td>
                      <td className="px-4 py-3">{date(item)}</td>
                      <td className="px-4 py-3">{actions(item)}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
            <ul
              data-testid="review-cards"
              className="divide-y divide-border-subtle md:hidden"
            >
              {visibleItems.map((item) => (
                <li key={item.id} className="space-y-3 p-4">
                  <div className="flex items-start gap-3">
                    <button
                      type="button"
                      onClick={() => onReview(item)}
                      className="min-h-11 min-w-11 flex-1 break-all rounded-md text-left text-sm font-semibold text-text hover:text-primary-accent focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary"
                    >
                      {item.slug}
                    </button>
                    {actions(item)}
                  </div>
                  <div className="flex flex-wrap gap-2">
                    <span className="max-w-full break-all rounded-full bg-primary/5 px-2.5 py-1 text-xs text-text-muted">
                      {item.kind}
                    </span>
                    {status(item)}
                  </div>
                  {tags(item)}
                  <div className="flex flex-wrap items-center justify-between gap-2">
                    <CopyableId id={item.item_id} />
                    {date(item)}
                  </div>
                </li>
              ))}
            </ul>
          </>
        )}
        {!query.isError && (
          <Pagination
            page={Math.floor(offset / limit) + 1}
            pageCount={Math.max(1, Math.ceil(total / limit))}
            total={total}
            rangeFrom={items.length ? offset + 1 : 0}
            rangeTo={Math.min(offset + items.length, total)}
            isBusy={query.isFetching}
            canPrevious={offset > 0}
            canNext={offset + limit < total}
            onPrevious={() => changePage(Math.max(0, offset - limit))}
            onNext={() => changePage(offset + limit)}
            onPageSelect={(page) => changePage((page - 1) * limit)}
            pageSize={limit}
            pageSizeOptions={[10, 15, 25, 50]}
            onPageSizeChange={(next) => {
              setLimit(next);
              changePage(0);
            }}
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
              {t("adminReview.filterHint")}
            </SheetDescription>
          </SheetHeader>
          <SheetBody className="space-y-5">
            <label className="block space-y-2 text-sm font-medium text-text">
              <span>{t("adminReview.purposeFilter")}</span>
              <NativeSelect
                value={draftFilters.purpose}
                onChange={(event) =>
                  setDraftFilters((previous) => ({
                    ...previous,
                    purpose: event.target.value,
                  }))
                }
              >
                {PURPOSES.map((value) => (
                  <option key={value} value={value}>
                    {value || t("adminReview.allPurposes")}
                  </option>
                ))}
              </NativeSelect>
            </label>
            <label className="block space-y-2 text-sm font-medium text-text">
              <span>{t("adminReview.kindFilter")}</span>
              <NativeSelect
                value={draftFilters.kind}
                onChange={(event) =>
                  setDraftFilters((previous) => ({
                    ...previous,
                    kind: event.target.value,
                  }))
                }
              >
                {KINDS.map((value) => (
                  <option key={value} value={value}>
                    {value || t("adminReview.allKinds")}
                  </option>
                ))}
              </NativeSelect>
            </label>
            <div className="flex flex-wrap justify-between gap-3">
              <Button variant="outline" onClick={reset}>
                {t("admin.resetFilters")}
              </Button>
              <Button
                onClick={() => {
                  applyFilters(draftFilters);
                  setFiltersOpen(false);
                }}
              >
                {t("common.apply")}
              </Button>
            </div>
          </SheetBody>
        </SheetContent>
      </Sheet>
    </div>
  );
}
