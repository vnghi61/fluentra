import React, { useState } from "react";
import { useTranslation } from "react-i18next";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import {
  AlertCircle,
  BarChart3,
  Loader2,
  RefreshCw,
  Search,
  Sparkles,
} from "lucide-react";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Progress } from "@/components/ui/progress";
import { cn } from "@/lib/utils";
import {
  adminApi,
  type Question,
  type QuestionFilter,
} from "../api/adminApi";

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

function QuestionsTab(): React.JSX.Element {
  const { t, i18n } = useTranslation();
  const queryClient = useQueryClient();
  const [filter, setFilter] = useState<QuestionFilter>({});
  const [offset, setOffset] = useState(0);
  const [selected, setSelected] = useState<Question | null>(null);
  const [generateOpen, setGenerateOpen] = useState(false);

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

  const items = data?.items ?? [];
  const total = data?.total ?? 0;
  const totalPages = Math.max(1, Math.ceil(total / PAGE_SIZE));
  const currentPage = Math.floor(offset / PAGE_SIZE) + 1;

  const setFilterValue = (key: keyof QuestionFilter, value: string) => {
    setFilter((prev) => ({ ...prev, [key]: value || undefined }));
    setOffset(0);
  };

  return (
    <div className="space-y-4">
      <div className="flex flex-col gap-3 rounded-xl border border-border bg-card p-4">
        <div className="flex flex-wrap items-center gap-2">
          <select
            value={filter.kind ?? ""}
            onChange={(e) => setFilterValue("kind", e.target.value)}
            aria-label={t("adminQuestions.kindFilter", "Kind")}
            className="h-11 rounded-lg border border-input bg-background px-3 text-base focus:outline-none focus:ring-1 focus:ring-ring"
          >
            {KINDS.map((value) => (
              <option key={value} value={value}>
                {value === "" ? t("adminQuestions.allKinds", "All kinds") : value}
              </option>
            ))}
          </select>
          <select
            value={filter.status ?? ""}
            onChange={(e) => setFilterValue("status", e.target.value)}
            aria-label={t("adminQuestions.statusFilter", "Status")}
            className="h-11 rounded-lg border border-input bg-background px-3 text-base focus:outline-none focus:ring-1 focus:ring-ring"
          >
            {STATUSES.map((value) => (
              <option key={value} value={value}>
                {value === ""
                  ? t("adminQuestions.allStatuses", "All statuses")
                  : value}
              </option>
            ))}
          </select>
          <select
            value={filter.cefr_level ?? ""}
            onChange={(e) => setFilterValue("cefr_level", e.target.value)}
            aria-label={t("adminQuestions.levelFilter", "Level")}
            className="h-11 rounded-lg border border-input bg-background px-3 text-base focus:outline-none focus:ring-1 focus:ring-ring"
          >
            {LEVELS.map((value) => (
              <option key={value} value={value}>
                {value === "" ? t("adminQuestions.allLevels", "All levels") : value}
              </option>
            ))}
          </select>
          <label className="flex h-11 min-w-[10rem] items-center gap-2 rounded-lg border border-input bg-background px-3">
            <Search className="h-4 w-4 text-text-muted" aria-hidden="true" />
            <input
              type="text"
              value={filter.node_code ?? ""}
              onChange={(e) => setFilterValue("node_code", e.target.value)}
              placeholder={t("adminQuestions.nodePlaceholder", "Spine node code")}
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

        {generateOpen && (
          <GenerateForm
            isPending={generate.isPending}
            error={generate.isError ? generate.error : null}
            generatedCount={generate.data?.questions.length ?? null}
            onGenerate={(req) => generate.mutate(req)}
          />
        )}
      </div>

      {isError && (
        <div
          role="alert"
          className="flex items-center gap-3 rounded-lg border border-danger/20 bg-danger/10 p-4 text-danger-accent"
        >
          <AlertCircle className="h-5 w-5 shrink-0" aria-hidden="true" />
          <p className="text-sm">
            {error instanceof Error
              ? error.message
              : t("adminQuestions.errorDesc", "The question bank could not be loaded.")}
          </p>
        </div>
      )}

      {isLoading ? (
        <div className="flex justify-center py-12">
          <Loader2 className="h-6 w-6 animate-spin text-primary" aria-hidden="true" />
        </div>
      ) : items.length === 0 ? (
        <div className="rounded-xl border border-border bg-card py-12 text-center text-sm text-muted-foreground">
          {t("adminQuestions.empty", "No questions match these filters.")}
        </div>
      ) : (
        <ul className="space-y-2">
          {items.map((question) => (
            <li key={question.id}>
              <button
                type="button"
                onClick={() => setSelected(question)}
                className="flex w-full min-h-[44px] flex-col gap-2 rounded-xl border border-border-subtle bg-card p-4 text-left transition-colors hover:border-primary/40 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary sm:flex-row sm:items-center sm:justify-between"
              >
                <span className="min-w-0 space-y-1">
                  <span className="flex flex-wrap items-center gap-2">
                    <Badge variant="secondary">{question.kind}</Badge>
                    <Badge variant={statusTone(question.status)}>
                      {question.status}
                    </Badge>
                    <Badge variant="outline">{question.cefr_level}</Badge>
                    <span className="text-xs text-text-muted">
                      {question.skill}
                    </span>
                  </span>
                  <span className="block truncate font-mono text-[11px] text-text-muted">
                    {question.fingerprint.slice(0, 24)}…
                  </span>
                </span>
                <span className="text-xs text-text-muted">
                  {new Date(question.created_at).toLocaleDateString(
                    i18n.language.startsWith("vi") ? "vi-VN" : "en-US",
                    { year: "numeric", month: "short", day: "numeric" },
                  )}
                </span>
              </button>
            </li>
          ))}
        </ul>
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
              onClick={() => setOffset(Math.max(0, offset - PAGE_SIZE))}
            >
              {t("common.previous", "Previous")}
            </Button>
            <Button
              variant="outline"
              disabled={currentPage >= totalPages}
              onClick={() => setOffset(offset + PAGE_SIZE)}
            >
              {t("common.next", "Next")}
            </Button>
          </div>
        </div>
      )}

      {selected && (
        <div
          role="dialog"
          aria-modal="true"
          aria-labelledby="question-stats-title"
          className="fixed inset-0 z-50 flex items-center justify-center bg-overlay/75 p-4 backdrop-blur-sm"
        >
          <div className="w-full max-w-md space-y-4 rounded-2xl border border-border bg-surface-card p-6 shadow-2xl">
            <h2
              id="question-stats-title"
              className="flex items-center gap-2 text-lg font-bold text-text"
            >
              <BarChart3 className="h-5 w-5 text-primary-accent" aria-hidden="true" />
              {t("adminQuestions.statsTitle", "Item performance")}
            </h2>
            {stats.isLoading ? (
              <Loader2 className="mx-auto h-6 w-6 animate-spin text-primary" aria-hidden="true" />
            ) : stats.isError ? (
              <p className="text-sm text-danger-accent">
                {t("adminQuestions.statsError", "Statistics could not be loaded.")}
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
            <div className="flex justify-end">
              <Button variant="outline" onClick={() => setSelected(null)}>
                {t("common.close", "Close")}
              </Button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}

function Stat({ label, value }: { label: string; value: string }): React.JSX.Element {
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
            className="h-11 w-full rounded-lg border border-input bg-background px-3 text-base focus:outline-none focus:ring-1 focus:ring-ring"
          >
            {KINDS.filter(Boolean).map((value) => (
              <option key={value} value={value}>
                {value}
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
            className="h-11 w-full rounded-lg border border-input bg-background px-3 text-base focus:outline-none focus:ring-1 focus:ring-ring"
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
            className="h-11 w-full rounded-lg border border-input bg-background px-3 text-base focus:outline-none focus:ring-1 focus:ring-ring"
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
            placeholder="uuid"
            className="h-11 w-full rounded-lg border border-input bg-background px-3 font-mono text-base focus:outline-none focus:ring-1 focus:ring-ring"
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
          placeholder="PRESENT_PERFECT, PAST_SIMPLE"
          className="h-11 w-full rounded-lg border border-input bg-background px-3 font-mono text-base focus:outline-none focus:ring-1 focus:ring-ring"
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
      <div className="flex flex-wrap items-center gap-3 rounded-xl border border-border bg-card p-4">
        <label className="space-y-1 text-sm">
          <span className="font-medium text-text">
            {t("adminQuestions.version", "Exam version")}
          </span>
          <select
            value={effectiveVersion}
            onChange={(e) => setVersionId(e.target.value)}
            className="h-11 min-w-[16rem] rounded-lg border border-input bg-background px-3 text-base focus:outline-none focus:ring-1 focus:ring-ring"
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
        <div className="flex justify-center py-12">
          <Loader2 className="h-6 w-6 animate-spin text-primary" aria-hidden="true" />
        </div>
      ) : coverage.isError ? (
        <p className="text-sm text-danger-accent">
          {t("adminQuestions.coverageError", "Coverage could not be loaded.")}
        </p>
      ) : (
        <div className="overflow-x-auto rounded-xl border border-border bg-card">
          <table className="w-full text-left text-sm">
            <thead className="border-b border-border bg-muted/50 text-xs uppercase text-muted-foreground">
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
            <tbody className="divide-y divide-border">
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
