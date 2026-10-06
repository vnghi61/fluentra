import React, { useState } from "react";
import { useTranslation } from "react-i18next";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import {
  BarChart3,
  Filter,
  Inbox,
  MoreHorizontal,
  RefreshCw,
  Search,
  Sparkles,
  X,
} from "lucide-react";

import { Badge } from "@/components/ui/badge";
import { BottomSheet } from "@/components/ui/bottom-sheet";
import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { EmptyState } from "@/components/ui/empty-state";
import { ErrorState } from "@/components/ui/error-state";
import { Modal, ModalBody, ModalFooter } from "@/components/ui/modal";
import { Progress } from "@/components/ui/progress";
import { Skeleton } from "@/components/ui/skeleton";
import { cn } from "@/lib/utils";
import { adminApi, type Question, type QuestionFilter } from "../api/adminApi";
import { CopyableId } from "./CopyableId";

const panelClass =
  "overflow-hidden rounded-2xl border border-border-subtle bg-surface-card";

/**
 * The exam question bank and its coverage report (WO 21 Stage D).
 *
 * The bank is generated, reviewed and then drawn: this screen shows what is
 * in it, how a single item has performed, and — per exam version — how many
 * distinct tests the published items can compose. Generated items land in the
 * review queue as drafts; the coverage number only counts what has been
 * published, because that is what composition can actually draw.
 */

const KINDS = [
  "",
  "grammar_tense_choice",
  "grammar_sentence_transform",
  "vocab_multiple_choice",
  "vocab_context_choice",
  "reading_comprehension",
  "listening_comprehension",
  "mcq_gap",
  "text_completion",
  "photo_description",
  "question_response",
  "writing_prompt",
  "speaking_task",
] as const;

const STATUSES = ["", "draft", "in_review", "published", "retired"] as const;
const LEVELS = ["", "A1", "A2", "B1", "B2", "C1", "C2"] as const;

const PAGE_SIZE = 20;

function statusTone(status: string): "success" | "warning" | "secondary" {
  if (status === "published") return "success";
  if (status === "draft" || status === "in_review") return "warning";
  return "secondary";
}

type TranslateFn = ReturnType<typeof useTranslation>["t"];

function humanizeKind(kind: string, t: TranslateFn): string {
  if (!kind) return "";
  const key = `adminQuestions.kinds.${kind}`;
  const fallback = kind
    .replace(/_/g, " ")
    .replace(/^\w/, (c) => c.toUpperCase());
  return t(key, fallback);
}

function humanizeStatus(status: string, t: TranslateFn): string {
  if (!status) return "";
  const key = `adminQuestions.statuses.${status}`;
  const fallback = status
    .replace(/_/g, " ")
    .replace(/^\w/, (c) => c.toUpperCase());
  return t(key, fallback);
}

function QuestionsTab(): React.JSX.Element {
  const { t, i18n } = useTranslation();
  const queryClient = useQueryClient();
  const [filter, setFilter] = useState<QuestionFilter>({});
  const [offset, setOffset] = useState(0);
  const [selected, setSelected] = useState<Question | null>(null);
  const [generateOpen, setGenerateOpen] = useState(false);
  const [filterSheetOpen, setFilterSheetOpen] = useState(false);

  const { data, isLoading, isFetching, isError, error, refetch } = useQuery({
    queryKey: ["admin", "questions", filter, offset],
    queryFn: () =>
      adminApi.listQuestions({ ...filter, limit: PAGE_SIZE, offset }),
  });

  const stats = useQuery({
    queryKey: ["admin", "question-stats", selected?.id],
    queryFn: () => adminApi.getQuestionStats(selected!.id),
    enabled: selected !== null,
  });

  const generate = useMutation({
    mutationFn: (req: Parameters<typeof adminApi.generateQuestions>[0]) =>
      adminApi.generateQuestions(req),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ["admin", "questions"] });
      void queryClient.invalidateQueries({
        queryKey: ["admin", "review-queue"],
      });
    },
  });

  const formatDate = (iso: string) =>
    new Date(iso).toLocaleDateString(
      i18n.language.startsWith("vi") ? "vi-VN" : "en-US",
      { year: "numeric", month: "short", day: "numeric" },
    );

  const items = data?.items ?? [];
  const total = data?.total ?? 0;
  const totalPages = Math.max(1, Math.ceil(total / PAGE_SIZE));
  const currentPage = Math.floor(offset / PAGE_SIZE) + 1;
  const hasMore =
    (total > 0 && offset + items.length < total) ||
    (total === 0 && items.length >= PAGE_SIZE);

  const setFilterValue = (key: keyof QuestionFilter, value: string) => {
    setFilter((prev) => ({ ...prev, [key]: value || undefined }));
    setOffset(0);
  };

  const removeFilter = (key: keyof QuestionFilter) => {
    setFilter((prev) => {
      const next = { ...prev };
      delete next[key];
      return next;
    });
    setOffset(0);
  };

  const clearAllFilters = () => {
    setFilter({});
    setOffset(0);
  };

  const hasActiveFilters = Boolean(
    filter.kind || filter.status || filter.cefr_level || filter.node_code,
  );

  const activeFilterCount = [
    filter.kind,
    filter.status,
    filter.cefr_level,
    filter.node_code,
  ].filter(Boolean).length;

  return (
    <div className="space-y-4">
      <div className="flex flex-col gap-3 rounded-2xl border border-border-subtle bg-surface-card p-4">
        {/* Desktop Filter Row (md+) */}
        <div className="hidden md:flex flex-wrap items-center gap-2">
          <select
            value={filter.kind ?? ""}
            onChange={(e) => setFilterValue("kind", e.target.value)}
            aria-label={t("adminQuestions.kindFilter", "Kind")}
            className="h-11 rounded-lg border border-border bg-surface px-3 text-base focus:outline-none focus:ring-1 focus:ring-ring"
          >
            {KINDS.map((value) => (
              <option key={value} value={value}>
                {value === ""
                  ? t("adminQuestions.allKinds", "All kinds")
                  : humanizeKind(value, t)}
              </option>
            ))}
          </select>
          <select
            value={filter.status ?? ""}
            onChange={(e) => setFilterValue("status", e.target.value)}
            aria-label={t("adminQuestions.statusFilter", "Status")}
            className="h-11 rounded-lg border border-border bg-surface px-3 text-base focus:outline-none focus:ring-1 focus:ring-ring"
          >
            {STATUSES.map((value) => (
              <option key={value} value={value}>
                {value === ""
                  ? t("adminQuestions.allStatuses", "All statuses")
                  : humanizeStatus(value, t)}
              </option>
            ))}
          </select>
          <select
            value={filter.cefr_level ?? ""}
            onChange={(e) => setFilterValue("cefr_level", e.target.value)}
            aria-label={t("adminQuestions.levelFilter", "Level")}
            className="h-11 rounded-lg border border-border bg-surface px-3 text-base focus:outline-none focus:ring-1 focus:ring-ring"
          >
            {LEVELS.map((value) => (
              <option key={value} value={value}>
                {value === ""
                  ? t("adminQuestions.allLevels", "All levels")
                  : value}
              </option>
            ))}
          </select>
          <label className="flex h-11 min-w-[10rem] items-center gap-2 rounded-lg border border-border bg-surface px-3">
            <Search className="h-4 w-4 text-text-muted" aria-hidden="true" />
            <input
              type="text"
              value={filter.node_code ?? ""}
              onChange={(e) => setFilterValue("node_code", e.target.value)}
              placeholder={t(
                "adminQuestions.nodePlaceholder",
                "Spine node code",
              )}
              className="w-full bg-transparent text-base focus:outline-none"
            />
          </label>
          <Button
            variant="outline"
            size="sm"
            onClick={() => void refetch()}
            disabled={isFetching}
            className="h-11 w-11 shrink-0 p-0"
            title={t("admin.refresh", "Refresh")}
          >
            <RefreshCw
              className={cn("h-4 w-4", isFetching && "animate-spin")}
              aria-hidden="true"
            />
          </Button>
          <Button
            variant="secondary"
            onClick={() => setGenerateOpen((open) => !open)}
            className="h-11 gap-2"
          >
            <Sparkles className="h-4 w-4" aria-hidden="true" />
            {t("adminQuestions.generateBtn", "Generate drafts")}
          </Button>
        </div>

        {/* Mobile Filter Controls (below md) */}
        <div className="flex md:hidden items-center justify-between gap-2">
          <div className="flex items-center gap-2">
            <Button
              variant="outline"
              onClick={() => setFilterSheetOpen(true)}
              className="h-11 min-h-[44px] gap-2"
              aria-label={t("adminQuestions.filtersBtn", "Filters")}
            >
              <Filter className="h-4 w-4" aria-hidden="true" />
              <span>{t("adminQuestions.filtersBtn", "Filters")}</span>
              {activeFilterCount > 0 && (
                <Badge
                  variant="secondary"
                  className="ml-1 px-1.5 py-0.5 text-xs"
                >
                  {activeFilterCount}
                </Badge>
              )}
            </Button>
            <Button
              variant="outline"
              size="sm"
              onClick={() => void refetch()}
              disabled={isFetching}
              className="h-11 w-11 shrink-0 p-0"
              title={t("admin.refresh", "Refresh")}
            >
              <RefreshCw
                className={cn("h-4 w-4", isFetching && "animate-spin")}
                aria-hidden="true"
              />
            </Button>
          </div>
          <Button
            variant="secondary"
            onClick={() => setGenerateOpen((open) => !open)}
            className="h-11 min-h-[44px] gap-2"
          >
            <Sparkles className="h-4 w-4" aria-hidden="true" />
            <span>{t("adminQuestions.generateBtn", "Generate drafts")}</span>
          </Button>
        </div>

        {/* Applied Filter Chips & Status Bar */}
        <div className="flex flex-wrap items-center justify-between gap-2">
          {hasActiveFilters ? (
            <div
              className="flex flex-wrap items-center gap-1.5"
              data-testid="applied-filter-chips"
            >
              {filter.kind && (
                <Badge
                  variant="secondary"
                  className="flex items-center gap-1 py-0.5 pl-2 pr-1 text-xs"
                >
                  <span>
                    {t("adminQuestions.kindFilter", "Kind")}:{" "}
                    {humanizeKind(filter.kind, t)}
                  </span>
                  <button
                    type="button"
                    onClick={() => removeFilter("kind")}
                    aria-label={t("adminQuestions.removeFilter", {
                      filter: t("adminQuestions.kindFilter", "Kind"),
                      defaultValue: "Remove {{filter}} filter",
                    })}
                    className="inline-flex min-h-[44px] min-w-[44px] items-center justify-center rounded-full hover:bg-surface-muted focus:outline-none"
                  >
                    <X className="h-3 w-3" aria-hidden="true" />
                  </button>
                </Badge>
              )}
              {filter.status && (
                <Badge
                  variant="secondary"
                  className="flex items-center gap-1 py-0.5 pl-2 pr-1 text-xs"
                >
                  <span>
                    {t("adminQuestions.statusFilter", "Status")}:{" "}
                    {humanizeStatus(filter.status, t)}
                  </span>
                  <button
                    type="button"
                    onClick={() => removeFilter("status")}
                    aria-label={t("adminQuestions.removeFilter", {
                      filter: t("adminQuestions.statusFilter", "Status"),
                      defaultValue: "Remove {{filter}} filter",
                    })}
                    className="inline-flex min-h-[44px] min-w-[44px] items-center justify-center rounded-full hover:bg-surface-muted focus:outline-none"
                  >
                    <X className="h-3 w-3" aria-hidden="true" />
                  </button>
                </Badge>
              )}
              {filter.cefr_level && (
                <Badge
                  variant="secondary"
                  className="flex items-center gap-1 py-0.5 pl-2 pr-1 text-xs"
                >
                  <span>
                    {t("adminQuestions.levelFilter", "Level")}:{" "}
                    {filter.cefr_level}
                  </span>
                  <button
                    type="button"
                    onClick={() => removeFilter("cefr_level")}
                    aria-label={t("adminQuestions.removeFilter", {
                      filter: t("adminQuestions.levelFilter", "Level"),
                      defaultValue: "Remove {{filter}} filter",
                    })}
                    className="inline-flex min-h-[44px] min-w-[44px] items-center justify-center rounded-full hover:bg-surface-muted focus:outline-none"
                  >
                    <X className="h-3 w-3" aria-hidden="true" />
                  </button>
                </Badge>
              )}
              {filter.node_code && (
                <Badge
                  variant="secondary"
                  className="flex items-center gap-1 py-0.5 pl-2 pr-1 text-xs"
                >
                  <span>
                    {t("adminQuestions.nodeLabel", "Node")}: {filter.node_code}
                  </span>
                  <button
                    type="button"
                    onClick={() => removeFilter("node_code")}
                    aria-label={t("adminQuestions.removeFilter", {
                      filter: t("adminQuestions.nodeLabel", "Node"),
                      defaultValue: "Remove {{filter}} filter",
                    })}
                    className="inline-flex min-h-[44px] min-w-[44px] items-center justify-center rounded-full hover:bg-surface-muted focus:outline-none"
                  >
                    <X className="h-3 w-3" aria-hidden="true" />
                  </button>
                </Badge>
              )}
              <Button
                variant="ghost"
                size="sm"
                onClick={clearAllFilters}
                className="px-2 text-xs text-text-muted hover:text-text"
              >
                {t("adminQuestions.clearFilters", "Clear filters")}
              </Button>
            </div>
          ) : (
            <div />
          )}

          {!isLoading && (
            <p role="status" className="text-xs text-text-muted">
              {hasMore
                ? t("adminQuestions.loadedCount", {
                    count: items.length,
                    defaultValue: `${items.length} loaded`,
                  })
                : t("adminQuestions.resultsCount", {
                    count: items.length,
                    defaultValue:
                      items.length === 1
                        ? "1 result"
                        : `${items.length} results`,
                  })}
            </p>
          )}
        </div>

        {/* Mobile BottomSheet for filters */}
        <BottomSheet
          open={filterSheetOpen}
          onOpenChange={setFilterSheetOpen}
          title={t("adminQuestions.filtersTitle", "Filter Questions")}
        >
          <div data-testid="filter-sheet" className="space-y-4 p-4">
            <div className="space-y-2">
              <label className="text-sm font-medium text-text">
                {t("adminQuestions.kindFilter", "Kind")}
              </label>
              <select
                value={filter.kind ?? ""}
                onChange={(e) => setFilterValue("kind", e.target.value)}
                aria-label={t("adminQuestions.kindFilter", "Kind")}
                className="h-11 w-full rounded-lg border border-border bg-surface px-3 text-base focus:outline-none focus:ring-1 focus:ring-ring"
              >
                {KINDS.map((value) => (
                  <option key={value} value={value}>
                    {value === ""
                      ? t("adminQuestions.allKinds", "All kinds")
                      : humanizeKind(value, t)}
                  </option>
                ))}
              </select>
            </div>

            <div className="space-y-2">
              <label className="text-sm font-medium text-text">
                {t("adminQuestions.statusFilter", "Status")}
              </label>
              <select
                value={filter.status ?? ""}
                onChange={(e) => setFilterValue("status", e.target.value)}
                aria-label={t("adminQuestions.statusFilter", "Status")}
                className="h-11 w-full rounded-lg border border-border bg-surface px-3 text-base focus:outline-none focus:ring-1 focus:ring-ring"
              >
                {STATUSES.map((value) => (
                  <option key={value} value={value}>
                    {value === ""
                      ? t("adminQuestions.allStatuses", "All statuses")
                      : humanizeStatus(value, t)}
                  </option>
                ))}
              </select>
            </div>

            <div className="space-y-2">
              <label className="text-sm font-medium text-text">
                {t("adminQuestions.levelFilter", "Level")}
              </label>
              <select
                value={filter.cefr_level ?? ""}
                onChange={(e) => setFilterValue("cefr_level", e.target.value)}
                aria-label={t("adminQuestions.levelFilter", "Level")}
                className="h-11 w-full rounded-lg border border-border bg-surface px-3 text-base focus:outline-none focus:ring-1 focus:ring-ring"
              >
                {LEVELS.map((value) => (
                  <option key={value} value={value}>
                    {value === ""
                      ? t("adminQuestions.allLevels", "All levels")
                      : value}
                  </option>
                ))}
              </select>
            </div>

            <div className="space-y-2">
              <label className="text-sm font-medium text-text">
                {t("adminQuestions.nodeLabel", "Spine node code")}
              </label>
              <div className="flex h-11 items-center gap-2 rounded-lg border border-border bg-surface px-3">
                <Search
                  className="h-4 w-4 text-text-muted"
                  aria-hidden="true"
                />
                <input
                  type="text"
                  value={filter.node_code ?? ""}
                  onChange={(e) => setFilterValue("node_code", e.target.value)}
                  placeholder={t(
                    "adminQuestions.nodePlaceholder",
                    "Spine node code",
                  )}
                  aria-label={t("adminQuestions.nodeLabel", "Spine node code")}
                  className="w-full bg-transparent text-base focus:outline-none"
                />
              </div>
            </div>

            <div className="flex gap-2 pt-2">
              {hasActiveFilters && (
                <Button
                  variant="outline"
                  onClick={clearAllFilters}
                  className="min-h-[44px] flex-1"
                >
                  {t("adminQuestions.clearFilters", "Clear filters")}
                </Button>
              )}
              <Button
                onClick={() => setFilterSheetOpen(false)}
                className="min-h-[44px] flex-1"
              >
                {t("common.close", "Close")}
              </Button>
            </div>
          </div>
        </BottomSheet>

        {generateOpen && (
          <GenerateForm
            isPending={generate.isPending}
            error={generate.isError ? generate.error : null}
            generatedCount={generate.data?.questions.length ?? null}
            onGenerate={(req) => generate.mutate(req)}
          />
        )}
      </div>

      {isError ? (
        <div className={panelClass}>
          <ErrorState
            className="py-10"
            description={
              error instanceof Error
                ? error.message
                : t(
                    "adminQuestions.errorDesc",
                    "The question bank could not be loaded.",
                  )
            }
            onRetry={() => void refetch()}
          />
        </div>
      ) : isLoading ? (
        <div
          aria-busy="true"
          data-testid="question-skeleton"
          className={cn(panelClass, "divide-y divide-border-subtle")}
        >
          {Array.from({ length: 5 }, (_, i) => (
            <div key={i} className="flex h-14 items-center gap-3 px-4">
              <div className="flex-1 space-y-1.5">
                <Skeleton className="h-3.5 w-1/3" />
                <Skeleton className="h-3 w-1/4" />
              </div>
              <Skeleton className="h-5 w-16 rounded-full" />
            </div>
          ))}
        </div>
      ) : items.length === 0 ? (
        <div className={panelClass}>
          <EmptyState
            className="py-10"
            icon={Inbox}
            title={t(
              "adminQuestions.empty",
              "No questions match these filters.",
            )}
            action={
              hasActiveFilters ? (
                <Button
                  variant="outline"
                  onClick={clearAllFilters}
                  className="min-h-[44px]"
                >
                  {t("adminQuestions.clearFilters", "Clear filters")}
                </Button>
              ) : undefined
            }
          />
        </div>
      ) : (
        <>
          {/* Mobile Card List (below md) */}
          <div className="space-y-3 md:hidden" data-testid="question-cards">
            {items.map((question) => (
              <div
                key={question.id}
                data-testid="question-card"
                className="space-y-3 rounded-2xl border border-border-subtle bg-surface-card p-4"
              >
                <div className="flex items-start justify-between gap-2">
                  <div className="min-w-0 space-y-1">
                    <p className="break-words font-semibold leading-snug text-text">
                      {humanizeKind(question.kind, t)}
                    </p>
                    <CopyableId id={question.id} />
                  </div>
                  <Badge variant={statusTone(question.status)}>
                    {humanizeStatus(question.status, t)}
                  </Badge>
                </div>

                <div className="flex flex-wrap items-center gap-2 text-xs text-text-muted">
                  <Badge variant="outline">{question.cefr_level}</Badge>
                  <span>{question.skill}</span>
                  <span className="ml-auto">
                    {formatDate(question.created_at)}
                  </span>
                </div>

                <div className="flex justify-end border-t border-border-subtle/50 pt-2">
                  <Button
                    type="button"
                    variant="outline"
                    size="sm"
                    onClick={() => setSelected(question)}
                    className="min-h-[44px] w-full gap-1.5 sm:w-auto"
                  >
                    <BarChart3 className="h-4 w-4" aria-hidden="true" />
                    {t("adminQuestions.viewStats", "Performance")}
                  </Button>
                </div>
              </div>
            ))}
          </div>

          {/* Desktop Table/List (md+) */}
          <div
            data-testid="question-table"
            className={cn(panelClass, "hidden overflow-x-auto md:block")}
          >
            {/* min-w keeps the kind column readable and every column
                reachable beside the expanded sidebar; it scrolls instead. */}
            <table className="w-full min-w-[760px] table-fixed border-collapse text-left">
              <thead>
                <tr className="border-b border-border-subtle bg-surface-muted text-xs font-semibold uppercase tracking-wider text-text-muted">
                  <th className="px-4 py-3">
                    {t("adminQuestions.kind", "Kind")}
                  </th>
                  <th className="w-32 px-3 py-3">
                    {t("adminQuestions.statusFilter", "Status")}
                  </th>
                  <th className="w-24 px-3 py-3">
                    {t("adminQuestions.levelFilter", "Level")}
                  </th>
                  <th className="w-32 px-3 py-3">
                    {t("adminQuestions.skill", "Skill")}
                  </th>
                  <th className="w-36 px-3 py-3">
                    {t("adminQuestions.created", "Created")}
                  </th>
                  <th className="w-16 px-3 py-3">
                    <span className="sr-only">
                      {t("admin.actions", "Actions")}
                    </span>
                  </th>
                </tr>
              </thead>
              <tbody className="divide-y divide-border-subtle">
                {items.map((question) => (
                  <tr
                    key={question.id}
                    className="h-14 transition-colors hover:bg-surface-muted/40"
                  >
                    <td className="px-4">
                      <p className="truncate text-sm font-medium text-text">
                        {humanizeKind(question.kind, t)}
                      </p>
                      <CopyableId id={question.id} />
                    </td>
                    <td className="px-3">
                      <Badge variant={statusTone(question.status)}>
                        {humanizeStatus(question.status, t)}
                      </Badge>
                    </td>
                    <td className="px-3">
                      <Badge variant="outline">{question.cefr_level}</Badge>
                    </td>
                    <td className="truncate px-3 text-sm text-text-muted">
                      {question.skill}
                    </td>
                    <td className="px-3 text-xs text-text-muted">
                      {formatDate(question.created_at)}
                    </td>
                    <td className="px-3 text-right">
                      <DropdownMenu>
                        <DropdownMenuTrigger
                          aria-label={t("adminQuestions.actionsFor", {
                            kind: humanizeKind(question.kind, t),
                            defaultValue: "Actions for {{kind}}",
                          })}
                          className="flex h-11 w-11 min-h-[44px] min-w-[44px] cursor-pointer items-center justify-center rounded-lg text-text-muted transition-colors hover:bg-surface-muted hover:text-text focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary"
                        >
                          <MoreHorizontal
                            className="h-5 w-5"
                            aria-hidden="true"
                          />
                        </DropdownMenuTrigger>
                        <DropdownMenuContent align="end">
                          <DropdownMenuItem
                            onSelect={() => setSelected(question)}
                          >
                            <BarChart3
                              className="h-4 w-4 shrink-0"
                              aria-hidden="true"
                            />
                            {t("adminQuestions.viewStats", "Performance")}
                          </DropdownMenuItem>
                        </DropdownMenuContent>
                      </DropdownMenu>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </>
      )}

      {total > PAGE_SIZE && (
        <div className="flex items-center justify-between text-sm text-text-muted">
          <span>
            {t("adminQuestions.pageOf", {
              current: currentPage,
              total: totalPages,
              defaultValue: `Page ${currentPage} of ${totalPages}`,
            })}
          </span>
          <div className="flex gap-2">
            <Button
              variant="outline"
              disabled={offset === 0}
              className="min-h-[44px]"
              onClick={() => setOffset(Math.max(0, offset - PAGE_SIZE))}
            >
              {t("common.previous", "Previous")}
            </Button>
            <Button
              variant="outline"
              disabled={currentPage >= totalPages}
              className="min-h-[44px]"
              onClick={() => setOffset(offset + PAGE_SIZE)}
            >
              {t("common.next", "Next")}
            </Button>
          </div>
        </div>
      )}

      <Modal
        open={selected !== null}
        onClose={() => setSelected(null)}
        size="md"
        title={t("adminQuestions.statsTitle", "Item performance")}
        icon={
          <BarChart3
            className="h-5 w-5 text-primary-accent"
            aria-hidden="true"
          />
        }
      >
        <ModalBody>
          {stats.isLoading ? (
            <div aria-busy="true" className="grid grid-cols-2 gap-3">
              {Array.from({ length: 4 }, (_, i) => (
                <Skeleton key={i} className="h-16 rounded-lg" />
              ))}
            </div>
          ) : stats.isError ? (
            <div className="space-y-3">
              <p role="alert" className="text-sm text-danger-accent">
                {t(
                  "adminQuestions.statsError",
                  "Statistics could not be loaded.",
                )}
              </p>
              <Button
                variant="outline"
                onClick={() => void stats.refetch()}
                className="min-h-[44px]"
              >
                {t("adminQuestions.retry", "Retry")}
              </Button>
            </div>
          ) : (
            <dl className="grid grid-cols-2 gap-3 text-sm">
              <Stat
                label={t("adminQuestions.attempts", "Attempts")}
                value={String(stats.data?.attempts ?? 0)}
              />
              <Stat
                label={t("adminQuestions.pValue", "P-value")}
                value={stats.data?.p_value?.toFixed(2) ?? "—"}
              />
              <Stat
                label={t("adminQuestions.discrimination", "Discrimination")}
                value={stats.data?.discrimination?.toFixed(2) ?? "—"}
              />
              <Stat
                label={t("adminQuestions.avgTime", "Avg time")}
                value={
                  stats.data?.avg_time_ms
                    ? `${(stats.data.avg_time_ms / 1000).toFixed(1)} s`
                    : "—"
                }
              />
            </dl>
          )}
        </ModalBody>
        <ModalFooter>
          <Button
            variant="outline"
            onClick={() => setSelected(null)}
            className="min-h-[44px]"
          >
            {t("common.close", "Close")}
          </Button>
        </ModalFooter>
      </Modal>
    </div>
  );
}

function Stat({
  label,
  value,
}: {
  label: string;
  value: string;
}): React.JSX.Element {
  return (
    <div className="rounded-lg border border-border-subtle p-3">
      <dt className="text-xs font-semibold uppercase tracking-wider text-text-muted">
        {label}
      </dt>
      <dd className="mt-1 font-mono text-base text-text">{value}</dd>
    </div>
  );
}

function GenerateForm({
  isPending,
  error,
  generatedCount,
  onGenerate,
}: {
  isPending: boolean;
  error: unknown;
  generatedCount: number | null;
  onGenerate: (req: {
    kind: string;
    cefr_level: string;
    node_codes: string[];
    count: number;
    exam_part_id?: string | null;
  }) => void;
}): React.JSX.Element {
  const { t } = useTranslation();
  const [kind, setKind] = useState("grammar_tense_choice");
  const [level, setLevel] = useState("B1");
  const [nodes, setNodes] = useState("");
  const [count, setCount] = useState(5);
  const [examPartId, setExamPartId] = useState("");
  const [validation, setValidation] = useState<string | null>(null);

  const submit = () => {
    const nodeCodes = nodes
      .split(",")
      .map((code) => code.trim())
      .filter(Boolean);
    if (nodeCodes.length === 0) {
      setValidation(
        t(
          "adminQuestions.nodesRequired",
          "At least one spine node code is required.",
        ),
      );
      return;
    }
    setValidation(null);
    onGenerate({
      kind,
      cefr_level: level,
      node_codes: nodeCodes,
      count,
      ...(examPartId.trim() !== "" && { exam_part_id: examPartId.trim() }),
    });
  };

  return (
    <div className="space-y-3 rounded-xl border border-border-subtle bg-surface-muted/30 p-4">
      <div className="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-4">
        <label className="space-y-1 text-sm">
          <span className="font-medium text-text">
            {t("adminQuestions.kind", "Kind")}
          </span>
          <select
            value={kind}
            onChange={(e) => setKind(e.target.value)}
            className="h-11 w-full rounded-lg border border-border bg-surface px-3 text-base focus:outline-none focus:ring-1 focus:ring-ring"
          >
            {KINDS.filter(Boolean).map((value) => (
              <option key={value} value={value}>
                {humanizeKind(value, t)}
              </option>
            ))}
          </select>
        </label>
        <label className="space-y-1 text-sm">
          <span className="font-medium text-text">
            {t("adminQuestions.level", "Level")}
          </span>
          <select
            value={level}
            onChange={(e) => setLevel(e.target.value)}
            className="h-11 w-full rounded-lg border border-border bg-surface px-3 text-base focus:outline-none focus:ring-1 focus:ring-ring"
          >
            {LEVELS.filter(Boolean).map((value) => (
              <option key={value} value={value}>
                {value}
              </option>
            ))}
          </select>
        </label>
        <label className="space-y-1 text-sm">
          <span className="font-medium text-text">
            {t("adminQuestions.count", "Count")}
          </span>
          <input
            type="number"
            min={1}
            max={100}
            value={count}
            onChange={(e) => setCount(Number(e.target.value))}
            className="h-11 w-full rounded-lg border border-border bg-surface px-3 text-base focus:outline-none focus:ring-1 focus:ring-ring"
          />
        </label>
        <label className="space-y-1 text-sm">
          <span className="font-medium text-text">
            {t("adminQuestions.examPart", "Exam part (optional)")}
          </span>
          <input
            type="text"
            value={examPartId}
            onChange={(e) => setExamPartId(e.target.value)}
            placeholder={t("adminQuestions.examPartPlaceholder", "uuid")}
            className="h-11 w-full rounded-lg border border-border bg-surface px-3 font-mono text-base focus:outline-none focus:ring-1 focus:ring-ring"
          />
        </label>
      </div>
      <label className="block space-y-1 text-sm">
        <span className="font-medium text-text">
          {t("adminQuestions.nodes", "Spine node codes (comma separated)")}
        </span>
        <input
          type="text"
          value={nodes}
          onChange={(e) => setNodes(e.target.value)}
          placeholder={t(
            "adminQuestions.nodesPlaceholder",
            "PRESENT_PERFECT, PAST_SIMPLE",
          )}
          className="h-11 w-full rounded-lg border border-border bg-surface px-3 font-mono text-base focus:outline-none focus:ring-1 focus:ring-ring"
        />
      </label>
      {validation && <p className="text-sm text-danger-accent">{validation}</p>}
      {error !== null && (
        <p role="alert" className="text-sm text-danger-accent">
          {error instanceof Error
            ? error.message
            : t("adminQuestions.generateError", "Generation failed.")}
        </p>
      )}
      {generatedCount !== null && (
        <p role="status" className="text-sm text-success-accent">
          {t("adminQuestions.generateOk", {
            count: generatedCount,
            defaultValue: `${generatedCount} drafts generated. They are waiting in the review queue.`,
          })}
        </p>
      )}
      <div className="flex justify-end">
        <Button
          onClick={submit}
          isLoading={isPending}
          disabled={isPending}
          className="gap-2"
        >
          <Sparkles className="h-4 w-4" aria-hidden="true" />
          {t("adminQuestions.generateSubmit", "Generate")}
        </Button>
      </div>
    </div>
  );
}

function CoverageTab(): React.JSX.Element {
  const { t } = useTranslation();
  const [versionId, setVersionId] = useState("");

  const versions = useQuery({
    queryKey: ["admin", "exam-versions"],
    queryFn: () => adminApi.listExamVersions(),
  });

  const effectiveVersion = versionId || versions.data?.items[0]?.id || "";
  const coverage = useQuery({
    queryKey: ["admin", "coverage", effectiveVersion],
    queryFn: () => adminApi.getExamVersionCoverage(effectiveVersion),
    enabled: effectiveVersion !== "",
  });

  return (
    <div className="space-y-4">
      <div className="flex flex-wrap items-end gap-3 rounded-2xl border border-border-subtle bg-surface-card p-4">
        <label className="block w-full space-y-1 text-sm sm:w-auto">
          <span className="font-medium text-text">
            {t("adminQuestions.version", "Exam version")}
          </span>
          <select
            value={effectiveVersion}
            onChange={(e) => setVersionId(e.target.value)}
            className="block h-11 min-h-[44px] w-full min-w-[44px] rounded-lg border border-border bg-surface px-3 text-base focus:outline-none focus:ring-1 focus:ring-ring sm:min-w-[16rem]"
          >
            {(versions.data?.items ?? []).map((version) => (
              <option key={version.id} value={version.id}>
                {version.code} — {version.title}
              </option>
            ))}
          </select>
        </label>
        {coverage.data && (
          <div className="rounded-lg border border-border-subtle p-3">
            <p className="text-xs font-semibold uppercase tracking-wider text-text-muted">
              {t("adminQuestions.distinctTests", "Distinct tests possible")}
            </p>
            <p className="text-2xl font-bold text-text">
              {coverage.data.distinct_tests_possible}
            </p>
          </div>
        )}
      </div>

      {coverage.isLoading ? (
        <div aria-busy="true" className={cn(panelClass, "space-y-3 p-4")}>
          {Array.from({ length: 4 }, (_, i) => (
            <Skeleton key={i} className="h-10 w-full" />
          ))}
        </div>
      ) : coverage.isError ? (
        <div className={panelClass}>
          <ErrorState
            className="py-8"
            description={t(
              "adminQuestions.coverageError",
              "Coverage could not be loaded.",
            )}
            onRetry={() => void coverage.refetch()}
          />
        </div>
      ) : (
        <>
          {/* Mobile: one card per part */}
          <ul data-testid="coverage-cards" className="space-y-3 md:hidden">
            {(coverage.data?.parts ?? []).map((part) => {
              const isBottleneck =
                coverage.data?.bottleneck_part_id === part.part_id;
              return (
                <li
                  key={part.part_id}
                  className={cn(
                    "space-y-2 rounded-2xl border border-border-subtle bg-surface-card p-4",
                    isBottleneck && "border-warning/40 bg-warning/5",
                  )}
                >
                  <div className="flex items-start justify-between gap-2">
                    <p className="font-semibold text-text">
                      {part.section} {part.part_number}
                    </p>
                    {isBottleneck && (
                      <Badge variant="warning">
                        {t("adminQuestions.bottleneck", "Bottleneck")}
                      </Badge>
                    )}
                  </div>
                  <p className="font-mono text-xs text-text-muted">
                    {part.kind}
                  </p>
                  <dl className="grid grid-cols-2 gap-2 text-sm">
                    <div>
                      <dt className="text-xs text-text-muted">
                        {t("adminQuestions.needed", "Needed")}
                      </dt>
                      <dd className="text-text">
                        {part.groups_needed_per_test}
                      </dd>
                    </div>
                    <div>
                      <dt className="text-xs text-text-muted">
                        {t("adminQuestions.available", "Available")}
                      </dt>
                      <dd className="text-text">
                        {part.published_groups_available}
                      </dd>
                    </div>
                  </dl>
                  <div className="flex items-center gap-2">
                    <Progress
                      value={Math.min(
                        100,
                        Math.round(
                          (part.tests_possible /
                            Math.max(1, part.groups_needed_per_test)) *
                            100,
                        ),
                      )}
                      max={100}
                      variant={isBottleneck ? "warning" : "primary"}
                      aria-label={t("adminQuestions.testsPossible", "Tests")}
                    />
                    <span className="font-mono text-xs text-text">
                      {part.tests_possible}
                    </span>
                  </div>
                </li>
              );
            })}
          </ul>

          <div className="hidden overflow-x-auto rounded-2xl border border-border-subtle bg-surface-card md:block">
            <table className="w-full text-left text-sm">
              <thead className="border-b border-border-subtle bg-surface-muted text-xs uppercase text-text-muted">
                <tr>
                  <th className="px-4 py-3 font-medium">
                    {t("adminQuestions.part", "Part")}
                  </th>
                  <th className="px-4 py-3 font-medium">
                    {t("adminQuestions.kind", "Kind")}
                  </th>
                  <th className="px-4 py-3 font-medium">
                    {t("adminQuestions.needed", "Needed")}
                  </th>
                  <th className="px-4 py-3 font-medium">
                    {t("adminQuestions.available", "Available")}
                  </th>
                  <th className="px-4 py-3 font-medium">
                    {t("adminQuestions.testsPossible", "Tests")}
                  </th>
                </tr>
              </thead>
              <tbody className="divide-y divide-border-subtle">
                {(coverage.data?.parts ?? []).map((part) => {
                  const isBottleneck =
                    coverage.data?.bottleneck_part_id === part.part_id;
                  return (
                    <tr
                      key={part.part_id}
                      className={cn(isBottleneck && "bg-warning/5")}
                    >
                      <td className="px-4 py-3">
                        <span className="font-medium text-text">
                          {part.section} {part.part_number}
                        </span>
                        {isBottleneck && (
                          <Badge variant="warning" className="ml-2">
                            {t("adminQuestions.bottleneck", "Bottleneck")}
                          </Badge>
                        )}
                      </td>
                      <td className="px-4 py-3 font-mono text-xs text-text-muted">
                        {part.kind}
                      </td>
                      <td className="px-4 py-3 text-text">
                        {part.groups_needed_per_test}
                      </td>
                      <td className="px-4 py-3 text-text">
                        {part.published_groups_available}
                      </td>
                      <td className="w-40 px-4 py-3">
                        <div className="flex items-center gap-2">
                          <Progress
                            value={Math.min(
                              100,
                              Math.round(
                                (part.tests_possible /
                                  Math.max(1, part.groups_needed_per_test)) *
                                  100,
                              ),
                            )}
                            max={100}
                            variant={isBottleneck ? "warning" : "primary"}
                            aria-label={t(
                              "adminQuestions.testsPossible",
                              "Tests",
                            )}
                          />
                          <span className="font-mono text-xs text-text">
                            {part.tests_possible}
                          </span>
                        </div>
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
        </>
      )}
    </div>
  );
}

export function AdminQuestionBank(): React.JSX.Element {
  const { t } = useTranslation();
  const [tab, setTab] = useState<"questions" | "coverage">("questions");

  return (
    <div className="space-y-4">
      <div className="flex gap-2 border-b border-border-subtle pb-px">
        {(
          [
            ["questions", t("adminQuestions.tabQuestions", "Questions")],
            ["coverage", t("adminQuestions.tabCoverage", "Coverage")],
          ] as const
        ).map(([key, label]) => (
          <button
            key={key}
            type="button"
            onClick={() => setTab(key)}
            className={cn(
              "min-h-[44px] whitespace-nowrap border-b-2 px-4 py-2 text-sm font-medium transition-colors",
              tab === key
                ? "border-primary text-primary-accent"
                : "border-transparent text-text-muted hover:text-text",
            )}
          >
            {label}
          </button>
        ))}
      </div>
      {tab === "questions" ? <QuestionsTab /> : <CoverageTab />}
    </div>
  );
}
