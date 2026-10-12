import React, { useState } from "react";
import { useTranslation } from "react-i18next";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import {
  BarChart3,
  Filter,
  Inbox,
  ListChecks,
  MoreHorizontal,
  Search,
  Sparkles,
  Target,
  X,
} from "lucide-react";

import { SheetDialog } from "@/components/ui/sheet";
import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { EmptyState } from "@/components/ui/empty-state";
import { ErrorState } from "@/components/ui/error-state";
import {
  Dialog,
  DialogBody,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Pagination } from "@/components/ui/pagination";
import { Progress } from "@/components/ui/progress";
import { Skeleton } from "@/components/ui/skeleton";
import { cn } from "@/lib/utils";
import { adminApi, type Question, type QuestionFilter } from "../api/adminApi";
import { CopyableId } from "./CopyableId";
import { StatusBadge, type StatusTone } from "./StatusBadge";
import {
  AdminDataPanel,
  AdminFilterCard,
  AdminListSkeleton,
  AdminRefreshButton,
  AdminRowIcon,
  AdminSectionHeader,
  AdminSubTabs,
  AdminTag,
  adminStatTones,
  adminTable,
} from "./AdminSurface";
import { Input } from "@/components/ui/input";
import { NativeSelect } from "@/components/ui/native-select";

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

function statusTone(status: string): StatusTone {
  if (status === "published") return "success";
  if (status === "draft" || status === "in_review") return "warning";
  return "neutral";
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

function QuestionsTab({
  generateOpen,
}: {
  generateOpen: boolean;
}): React.JSX.Element {
  const { t, i18n } = useTranslation();
  const queryClient = useQueryClient();
  const [filter, setFilter] = useState<QuestionFilter>({});
  const [offset, setOffset] = useState(0);
  const [pageSize, setPageSize] = useState(PAGE_SIZE);
  const [selected, setSelected] = useState<Question | null>(null);
  const [filterSheetOpen, setFilterSheetOpen] = useState(false);

  const { data, isLoading, isFetching, isError, refetch } = useQuery({
    queryKey: ["admin", "questions", filter, offset, pageSize],
    queryFn: () =>
      adminApi.listQuestions({ ...filter, limit: pageSize, offset }),
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
  const totalPages = Math.max(1, Math.ceil(total / pageSize));
  const currentPage = Math.floor(offset / pageSize) + 1;
  const hasMore =
    (total > 0 && offset + items.length < total) ||
    (total === 0 && items.length >= pageSize);

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

  const chip = (key: keyof QuestionFilter, label: string, value: string) => (
    <span className="inline-flex max-w-full items-center gap-1 rounded-lg bg-surface-muted py-0.5 pl-2.5 pr-0.5 text-text">
      <span className="break-all">
        {label}: {value}
      </span>
      <button
        type="button"
        onClick={() => removeFilter(key)}
        aria-label={t("adminQuestions.removeFilter", {
          filter: label,
          defaultValue: "Remove {{filter}} filter",
        })}
        className="inline-flex min-h-[44px] min-w-[44px] items-center justify-center rounded-lg text-text-muted hover:bg-surface-card hover:text-text focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary"
      >
        <X className="h-4 w-4" aria-hidden="true" />
      </button>
    </span>
  );
  const kindSelect = (className?: string) => (
    <NativeSelect
      value={filter.kind ?? ""}
      onChange={(e) => setFilterValue("kind", e.target.value)}
      aria-label={t("adminQuestions.kindFilter", "Kind")}
      wrapperClassName={className}
    >
      {KINDS.map((value) => (
        <option key={value} value={value}>
          {value === ""
            ? t("adminQuestions.allKinds", "All kinds")
            : humanizeKind(value, t)}
        </option>
      ))}
    </NativeSelect>
  );
  const statusSelect = (className?: string) => (
    <NativeSelect
      value={filter.status ?? ""}
      onChange={(e) => setFilterValue("status", e.target.value)}
      aria-label={t("adminQuestions.statusFilter", "Status")}
      wrapperClassName={className}
    >
      {STATUSES.map((value) => (
        <option key={value} value={value}>
          {value === ""
            ? t("adminQuestions.allStatuses", "All statuses")
            : humanizeStatus(value, t)}
        </option>
      ))}
    </NativeSelect>
  );
  const levelSelect = (className?: string) => (
    <NativeSelect
      value={filter.cefr_level ?? ""}
      onChange={(e) => setFilterValue("cefr_level", e.target.value)}
      aria-label={t("adminQuestions.levelFilter", "Level")}
      wrapperClassName={className}
    >
      {LEVELS.map((value) => (
        <option key={value} value={value}>
          {value === "" ? t("adminQuestions.allLevels", "All levels") : value}
        </option>
      ))}
    </NativeSelect>
  );
  const statusBadge = (status: string) => (
    <StatusBadge tone={statusTone(status)}>
      <span
        aria-hidden="true"
        className="h-1.5 w-1.5 shrink-0 rounded-full bg-current"
      />
      {humanizeStatus(status, t)}
    </StatusBadge>
  );
  const createdAt = (iso: string) => (
    <time dateTime={iso} className="text-sm tabular-nums text-text-muted">
      {formatDate(iso)}
    </time>
  );

  return (
    <div className="space-y-4">
      <AdminFilterCard>
        <div className="flex flex-wrap items-center gap-2 sm:gap-3">
          <div className="relative min-w-0 flex-1 basis-full sm:basis-auto">
            <Search
              aria-hidden="true"
              className="pointer-events-none absolute left-3.5 top-3.5 h-4 w-4 text-text-muted"
            />
            <Input
              type="text"
              value={filter.node_code ?? ""}
              onChange={(e) => setFilterValue("node_code", e.target.value)}
              placeholder={t(
                "adminQuestions.nodePlaceholder",
                "Spine node code",
              )}
              aria-label={t("adminQuestions.search", "Search questions")}
              className="bg-surface-muted pl-10"
            />
          </div>
          {kindSelect("hidden md:block md:w-auto")}
          {statusSelect("hidden md:block md:w-auto")}
          {levelSelect("hidden md:block md:w-auto")}
          <Button
            type="button"
            variant="outline"
            onClick={() => setFilterSheetOpen(true)}
            className="relative min-h-11 flex-1 gap-2 md:hidden"
            aria-label={t("adminQuestions.filtersBtn", "Filters")}
          >
            <Filter className="h-4 w-4" aria-hidden="true" />
            <span>{t("adminQuestions.filtersBtn", "Filters")}</span>
            {activeFilterCount > 0 && (
              <span className="rounded-full bg-primary/10 px-1.5 py-0.5 text-xs tabular-nums text-primary-accent">
                {activeFilterCount}
              </span>
            )}
          </Button>
          <AdminRefreshButton
            onClick={() => void refetch()}
            busy={isFetching}
          />
        </div>

        <div className="flex flex-wrap items-center justify-between gap-2">
          {hasActiveFilters ? (
            <div
              className="flex flex-wrap items-center gap-2 text-xs"
              data-testid="applied-filter-chips"
            >
              <span className="text-text-muted">
                {t("admin.activeFilters")}
              </span>
              {filter.kind &&
                chip(
                  "kind",
                  t("adminQuestions.kindFilter", "Kind"),
                  humanizeKind(filter.kind, t),
                )}
              {filter.status &&
                chip(
                  "status",
                  t("adminQuestions.statusFilter", "Status"),
                  humanizeStatus(filter.status, t),
                )}
              {filter.cefr_level &&
                chip(
                  "cefr_level",
                  t("adminQuestions.levelFilter", "Level"),
                  filter.cefr_level,
                )}
              {filter.node_code &&
                chip(
                  "node_code",
                  t("adminQuestions.nodeLabel", "Node"),
                  filter.node_code,
                )}
              <Button variant="ghost" size="sm" onClick={clearAllFilters}>
                <X aria-hidden="true" className="h-4 w-4" />
                {t("adminQuestions.clearFilters", "Clear filters")}
              </Button>
            </div>
          ) : (
            <div />
          )}

          {!isLoading && (
            <p role="status" className="text-sm text-text-muted">
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
        <SheetDialog
          open={filterSheetOpen}
          onOpenChange={setFilterSheetOpen}
          title={t("adminQuestions.filtersTitle", "Filter Questions")}
        >
          <div data-testid="filter-sheet" className="space-y-5 p-4">
            <label className="block space-y-2 text-sm font-medium text-text">
              <span>{t("adminQuestions.kindFilter", "Kind")}</span>
              {kindSelect("w-full")}
            </label>
            <label className="block space-y-2 text-sm font-medium text-text">
              <span>{t("adminQuestions.statusFilter", "Status")}</span>
              {statusSelect("w-full")}
            </label>
            <label className="block space-y-2 text-sm font-medium text-text">
              <span>{t("adminQuestions.levelFilter", "Level")}</span>
              {levelSelect("w-full")}
            </label>
            <div className="flex flex-wrap justify-between gap-3 pt-2">
              {hasActiveFilters && (
                <Button variant="outline" onClick={clearAllFilters}>
                  {t("adminQuestions.clearFilters", "Clear filters")}
                </Button>
              )}
              <Button
                onClick={() => setFilterSheetOpen(false)}
                className="ml-auto"
              >
                {t("common.close", "Close")}
              </Button>
            </div>
          </div>
        </SheetDialog>

        {generateOpen && (
          <GenerateForm
            isPending={generate.isPending}
            error={generate.isError ? generate.error : null}
            generatedCount={generate.data?.questions.length ?? null}
            onGenerate={(req) => generate.mutate(req)}
          />
        )}
      </AdminFilterCard>

      <AdminDataPanel>
        {isError ? (
          <ErrorState
            className="py-10"
            description={t(
              "adminQuestions.errorDesc",
              "The question bank could not be loaded.",
            )}
            onRetry={() => void refetch()}
          />
        ) : isLoading ? (
          <AdminListSkeleton
            label={t("adminQuestions.loading", "Loading questions…")}
            data-testid="question-skeleton"
          />
        ) : items.length === 0 ? (
          <EmptyState
            className="py-10"
            icon={Inbox}
            title={t(
              "adminQuestions.empty",
              "No questions match these filters.",
            )}
            action={
              hasActiveFilters ? (
                <Button variant="outline" onClick={clearAllFilters}>
                  {t("adminQuestions.clearFilters", "Clear filters")}
                </Button>
              ) : undefined
            }
          />
        ) : (
          <>
            <div
              data-testid="question-table"
              className="hidden overflow-x-auto md:block"
            >
              {/* min-w keeps the kind column readable and every column
                  reachable beside the expanded sidebar; it scrolls instead. */}
              <table className={cn(adminTable.table, "min-w-[860px]")}>
                <caption className="sr-only">
                  {t("page.questionBank", "Question Bank")}
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
                      {t("adminQuestions.kind", "Kind")}
                    </th>
                    <th scope="col" className={cn(adminTable.th, "w-36")}>
                      {t("adminQuestions.statusFilter", "Status")}
                    </th>
                    <th scope="col" className={cn(adminTable.th, "w-20")}>
                      {t("adminQuestions.levelFilter", "Level")}
                    </th>
                    <th scope="col" className={cn(adminTable.th, "w-32")}>
                      {t("adminQuestions.skill", "Skill")}
                    </th>
                    <th scope="col" className={cn(adminTable.th, "w-36")}>
                      {t("adminQuestions.created", "Created")}
                    </th>
                    <th
                      scope="col"
                      className={cn(adminTable.th, "w-20 text-right")}
                    >
                      {t("common.actions")}
                    </th>
                  </tr>
                </thead>
                <tbody className={adminTable.body}>
                  {items.map((question, index) => (
                    <tr
                      key={question.id}
                      className={cn(adminTable.row, "h-14")}
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
                          <AdminRowIcon icon={ListChecks} />
                          <div className="min-w-0">
                            <p className="truncate text-sm font-semibold text-text">
                              {humanizeKind(question.kind, t)}
                            </p>
                            <CopyableId id={question.id} />
                          </div>
                        </div>
                      </td>
                      <td className={adminTable.td}>
                        {statusBadge(question.status)}
                      </td>
                      <td className={adminTable.td}>
                        <AdminTag>{question.cefr_level}</AdminTag>
                      </td>
                      <td
                        className={cn(
                          adminTable.td,
                          "truncate text-sm text-text-muted",
                        )}
                      >
                        {question.skill}
                      </td>
                      <td className={adminTable.td}>
                        {createdAt(question.created_at)}
                      </td>
                      <td className={adminTable.td}>
                        <DropdownMenu>
                          <DropdownMenuTrigger
                            aria-label={t("adminQuestions.actionsFor", {
                              kind: humanizeKind(question.kind, t),
                              defaultValue: "Actions for {{kind}}",
                            })}
                            className="ml-auto flex h-11 w-11 shrink-0 items-center justify-center rounded-xl border border-border-subtle text-text-muted transition-colors hover:bg-surface-muted hover:text-text focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary"
                          >
                            <MoreHorizontal
                              className="h-4 w-4"
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

            <ul
              className="divide-y divide-border-subtle md:hidden"
              data-testid="question-cards"
            >
              {items.map((question) => (
                <li
                  key={question.id}
                  data-testid="question-card"
                  className="space-y-3 p-4"
                >
                  <div className="flex items-start gap-3">
                    <AdminRowIcon icon={ListChecks} className="h-10 w-10" />
                    <div className="min-w-0 flex-1">
                      <p className="break-words text-sm font-semibold leading-snug text-text">
                        {humanizeKind(question.kind, t)}
                      </p>
                      <CopyableId id={question.id} />
                    </div>
                  </div>
                  <div className="flex flex-wrap items-center gap-2">
                    {statusBadge(question.status)}
                    <AdminTag>{question.cefr_level}</AdminTag>
                    <span className="text-sm text-text-muted">
                      {question.skill}
                    </span>
                    <span className="ml-auto">
                      {createdAt(question.created_at)}
                    </span>
                  </div>
                  <Button
                    type="button"
                    variant="outline"
                    size="sm"
                    onClick={() => setSelected(question)}
                    className="w-full"
                  >
                    <BarChart3 className="h-4 w-4" aria-hidden="true" />
                    {t("adminQuestions.viewStats", "Performance")}
                  </Button>
                </li>
              ))}
            </ul>
          </>
        )}

        {!isError && !isLoading && (
          <Pagination
            page={currentPage}
            pageCount={totalPages}
            total={total}
            rangeFrom={items.length ? offset + 1 : 0}
            rangeTo={Math.min(offset + items.length, total)}
            isBusy={isFetching}
            canPrevious={offset > 0}
            canNext={offset + pageSize < total}
            onPrevious={() => setOffset(Math.max(0, offset - pageSize))}
            onNext={() => setOffset(offset + pageSize)}
            onPageSelect={(page) => setOffset((page - 1) * pageSize)}
            pageSize={pageSize}
            pageSizeOptions={[10, 20, 50]}
            onPageSizeChange={(next) => {
              setPageSize(next);
              setOffset(0);
            }}
            className="bg-surface-card"
          />
        )}
      </AdminDataPanel>

      <Dialog
        open={selected !== null}
        onOpenChange={(open) => {
          if (!open) setSelected(null);
        }}
      >
        <DialogContent className="max-w-md">
          <DialogHeader>
            <DialogTitle className="flex items-center gap-2">
              <BarChart3
                className="h-5 w-5 shrink-0 text-primary-accent"
                aria-hidden="true"
              />
              {t("adminQuestions.statsTitle", "Item performance")}
            </DialogTitle>
          </DialogHeader>
          <DialogBody>
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
            ) : stats.data && !stats.data.last_computed_at ? (
              <p className="rounded-lg border border-border-subtle bg-surface-muted/50 p-3 text-sm text-text-muted">
                {t(
                  "adminQuestions.statsNotComputed",
                  "No statistics yet. They appear once learners have answered this question and the statistics have been computed.",
                )}
              </p>
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
          </DialogBody>
          <DialogFooter>
            <Button
              variant="outline"
              onClick={() => setSelected(null)}
              className="min-h-[44px]"
            >
              {t("common.close", "Close")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
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
          <NativeSelect value={kind} onChange={(e) => setKind(e.target.value)}>
            {KINDS.filter(Boolean).map((value) => (
              <option key={value} value={value}>
                {humanizeKind(value, t)}
              </option>
            ))}
          </NativeSelect>
        </label>
        <label className="space-y-1 text-sm">
          <span className="font-medium text-text">
            {t("adminQuestions.level", "Level")}
          </span>
          <NativeSelect
            value={level}
            onChange={(e) => setLevel(e.target.value)}
          >
            {LEVELS.filter(Boolean).map((value) => (
              <option key={value} value={value}>
                {value}
              </option>
            ))}
          </NativeSelect>
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
            className="h-11 w-full rounded-lg border border-border bg-surface px-3 text-base placeholder:text-text-muted focus:outline-none focus:ring-1 focus:ring-ring md:text-sm"
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
            className="h-11 w-full rounded-lg border border-border bg-surface px-3 font-mono text-base placeholder:text-text-muted focus:outline-none focus:ring-1 focus:ring-ring md:text-sm"
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
          className="h-11 w-full rounded-lg border border-border bg-surface px-3 font-mono text-base placeholder:text-text-muted focus:outline-none focus:ring-1 focus:ring-ring md:text-sm"
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

  const parts = coverage.data?.parts ?? [];
  const fill = (testsPossible: number, needed: number) =>
    Math.min(100, Math.round((testsPossible / Math.max(1, needed)) * 100));
  const bottleneckBadge = (
    <StatusBadge tone="warning">
      {t("adminQuestions.bottleneck", "Bottleneck")}
    </StatusBadge>
  );

  return (
    <div className="space-y-4">
      <AdminFilterCard>
        <div className="flex flex-wrap items-end gap-3">
          <label className="block w-full space-y-2 text-sm font-medium text-text sm:w-auto">
            <span>{t("adminQuestions.version", "Exam version")}</span>
            <NativeSelect
              value={effectiveVersion}
              onChange={(e) => setVersionId(e.target.value)}
              wrapperClassName="sm:min-w-[16rem]"
            >
              {(versions.data?.items ?? []).map((version) => (
                <option key={version.id} value={version.id}>
                  {version.code} — {version.title}
                </option>
              ))}
            </NativeSelect>
          </label>
          {coverage.data && (
            <div className="flex items-center gap-3 rounded-xl border border-border-subtle px-4 py-2">
              <span
                className={cn(
                  "flex h-10 w-10 shrink-0 items-center justify-center rounded-full",
                  adminStatTones.primary,
                )}
              >
                <Target aria-hidden="true" className="h-5 w-5" />
              </span>
              <div>
                <p className="text-2xl font-bold tabular-nums text-text">
                  {coverage.data.distinct_tests_possible}
                </p>
                <p className="text-sm text-text-muted">
                  {t("adminQuestions.distinctTests", "Distinct tests possible")}
                </p>
              </div>
            </div>
          )}
        </div>
      </AdminFilterCard>

      <AdminDataPanel>
        {coverage.isLoading ? (
          <AdminListSkeleton
            label={t("adminQuestions.loadingCoverage", "Loading coverage…")}
            rows={4}
          />
        ) : coverage.isError ? (
          <ErrorState
            className="py-8"
            description={t(
              "adminQuestions.coverageError",
              "Coverage could not be loaded.",
            )}
            onRetry={() => void coverage.refetch()}
          />
        ) : (
          <>
            {/* Mobile: one card per part */}
            <ul
              data-testid="coverage-cards"
              className="divide-y divide-border-subtle md:hidden"
            >
              {parts.map((part) => {
                const isBottleneck =
                  coverage.data?.bottleneck_part_id === part.part_id;
                return (
                  <li
                    key={part.part_id}
                    className={cn(
                      "space-y-3 p-4",
                      isBottleneck && "bg-warning/5",
                    )}
                  >
                    <div className="flex items-start justify-between gap-2">
                      <div className="min-w-0">
                        <p className="font-semibold text-text">
                          {part.section} {part.part_number}
                        </p>
                        <p className="break-all font-mono text-sm text-text-muted">
                          {part.kind}
                        </p>
                      </div>
                      {isBottleneck && bottleneckBadge}
                    </div>
                    <dl className="grid grid-cols-2 gap-2 text-sm">
                      <div>
                        <dt className="text-sm text-text-muted">
                          {t("adminQuestions.needed", "Needed")}
                        </dt>
                        <dd className="tabular-nums text-text">
                          {part.groups_needed_per_test}
                        </dd>
                      </div>
                      <div>
                        <dt className="text-sm text-text-muted">
                          {t("adminQuestions.available", "Available")}
                        </dt>
                        <dd className="tabular-nums text-text">
                          {part.published_groups_available}
                        </dd>
                      </div>
                    </dl>
                    <div className="flex items-center gap-2">
                      <Progress
                        value={fill(
                          part.tests_possible,
                          part.groups_needed_per_test,
                        )}
                        max={100}
                        variant={isBottleneck ? "warning" : "primary"}
                        aria-label={t("adminQuestions.testsPossible", "Tests")}
                      />
                      <span className="text-sm tabular-nums text-text">
                        {part.tests_possible}
                      </span>
                    </div>
                  </li>
                );
              })}
            </ul>

            <div className="hidden overflow-x-auto md:block">
              <table className={cn(adminTable.table, "min-w-[760px]")}>
                <caption className="sr-only">
                  {t("adminQuestions.tabCoverage", "Coverage")}
                </caption>
                <thead className={adminTable.head}>
                  <tr>
                    <th scope="col" className={adminTable.th}>
                      {t("adminQuestions.part", "Part")}
                    </th>
                    <th scope="col" className={cn(adminTable.th, "w-56")}>
                      {t("adminQuestions.kind", "Kind")}
                    </th>
                    <th scope="col" className={cn(adminTable.th, "w-28")}>
                      {t("adminQuestions.needed", "Needed")}
                    </th>
                    <th scope="col" className={cn(adminTable.th, "w-28")}>
                      {t("adminQuestions.available", "Available")}
                    </th>
                    <th scope="col" className={cn(adminTable.th, "w-48")}>
                      {t("adminQuestions.testsPossible", "Tests")}
                    </th>
                  </tr>
                </thead>
                <tbody className={adminTable.body}>
                  {parts.map((part) => {
                    const isBottleneck =
                      coverage.data?.bottleneck_part_id === part.part_id;
                    return (
                      <tr
                        key={part.part_id}
                        className={cn(
                          adminTable.row,
                          "h-14",
                          isBottleneck && "bg-warning/5",
                        )}
                      >
                        <td className={adminTable.td}>
                          <div className="flex flex-wrap items-center gap-2">
                            <span className="font-semibold text-text">
                              {part.section} {part.part_number}
                            </span>
                            {isBottleneck && bottleneckBadge}
                          </div>
                        </td>
                        <td className={adminTable.td}>
                          <AdminTag title={part.kind}>{part.kind}</AdminTag>
                        </td>
                        <td className={cn(adminTable.td, "tabular-nums")}>
                          {part.groups_needed_per_test}
                        </td>
                        <td className={cn(adminTable.td, "tabular-nums")}>
                          {part.published_groups_available}
                        </td>
                        <td className={adminTable.td}>
                          <div className="flex items-center gap-2">
                            <Progress
                              value={fill(
                                part.tests_possible,
                                part.groups_needed_per_test,
                              )}
                              max={100}
                              variant={isBottleneck ? "warning" : "primary"}
                              aria-label={t(
                                "adminQuestions.testsPossible",
                                "Tests",
                              )}
                            />
                            <span className="text-sm tabular-nums text-text">
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
      </AdminDataPanel>
    </div>
  );
}

export function AdminQuestionBank(): React.JSX.Element {
  const { t } = useTranslation();
  const [tab, setTab] = useState<"questions" | "coverage">("questions");
  const [generateOpen, setGenerateOpen] = useState(false);

  return (
    <div className="space-y-4">
      <AdminSectionHeader
        title={t("page.questionBank", "Question Bank")}
        description={t("adminQuestions.description")}
        actions={
          tab === "questions" ? (
            <Button
              type="button"
              variant={generateOpen ? "secondary" : "primary"}
              onClick={() => setGenerateOpen((open) => !open)}
              aria-expanded={generateOpen}
              className="whitespace-nowrap"
            >
              <Sparkles className="h-4 w-4" aria-hidden="true" />
              {t("adminQuestions.generateBtn", "Generate drafts")}
            </Button>
          ) : undefined
        }
      />
      <AdminSubTabs
        label={t("adminQuestions.tabs", "Question bank views")}
        active={tab}
        onChange={setTab}
        tabs={[
          {
            key: "questions",
            label: t("adminQuestions.tabQuestions", "Questions"),
            Icon: ListChecks,
          },
          {
            key: "coverage",
            label: t("adminQuestions.tabCoverage", "Coverage"),
            Icon: Target,
          },
        ]}
      />
      {tab === "questions" ? (
        <QuestionsTab generateOpen={generateOpen} />
      ) : (
        <CoverageTab />
      )}
    </div>
  );
}
