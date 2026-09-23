import React, { useState } from "react";
import { useTranslation } from "react-i18next";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import {
  AlertCircle,
  CheckCircle2,
  ClipboardCheck,
  Layers,
  Loader2,
  RefreshCw,
  X,
} from "lucide-react";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { ExerciseMultipleChoice } from "@/features/learning/components/Runner/ExerciseMultipleChoice";
import { cn } from "@/lib/utils";
import {
  adminApi,
  type AdminReviewBatch,
  type AdminReviewQueueItem,
} from "../api/adminApi";

/**
 * The machine-generated drafts awaiting a person (WO 21 Stage C).
 *
 * Nothing generated for the curriculum or the exam bank publishes itself:
 * it lands as a draft, and this is where a reviewer reads it with the answer
 * key and the blind solver's answer side by side, then approves or sends it
 * back with a note. Approving uses the same review and publish endpoints the
 * authoring screen uses — there is no second decision path.
 */

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

const PAGE_SIZE = 20;

interface Option {
  id: string;
  text: string;
}

/** The options a body carries, when it is a choice exercise. */
function readOptions(body: Record<string, unknown>): Option[] | null {
  const raw = body["options"];
  if (!Array.isArray(raw)) return null;
  const options: Option[] = [];
  for (const entry of raw) {
    if (typeof entry !== "object" || entry === null) continue;
    const option = entry as Record<string, unknown>;
    if (typeof option["id"] !== "string") continue;
    if (typeof option["text"] !== "string") continue;
    options.push({ id: option["id"], text: option["text"] });
  }
  return options.length > 0 ? options : null;
}

/** The key, in either spelling the generator and the authoring seed use. */
function correctOptionId(body: Record<string, unknown>): string | undefined {
  const byId = body["correct_option_id"];
  if (typeof byId === "string") return byId;
  const byIndex = body["correct_index"];
  if (typeof byIndex === "number") {
    const options = readOptions(body);
    return options?.[byIndex]?.id;
  }
  return undefined;
}

function blindSolveOptionId(
  answer: Record<string, unknown> | undefined,
): string | undefined {
  if (!answer) return undefined;
  const selected = answer["selected_option_id"];
  if (typeof selected === "string") return selected;
  return undefined;
}

/** A provenance field, which is whatever the model wrote, rendered safely. */
function asText(value: unknown, fallback: string): string {
  return typeof value === "string" && value.trim() !== "" ? value : fallback;
}

/**
 * The item as the reviewer reads it: the learner's own renderer where the kind
 * has one, with the key revealed, and the raw body otherwise. A reviewer
 * looking at JSON when a component exists is a reviewer working slower than
 * they have to.
 */
function ReviewItemBody({
  item,
}: {
  item: AdminReviewQueueItem;
}): React.JSX.Element {
  const body = item.body;
  const options = readOptions(body);
  const prompt = typeof body["prompt"] === "string" ? body["prompt"] : "";
  const correct = correctOptionId(body);

  if (options && prompt && correct) {
    return (
      <ExerciseMultipleChoice
        prompt={prompt}
        options={options}
        correctOptionId={correct}
        isSubmitted
        isCorrect
        onSubmit={() => undefined}
        onContinue={() => undefined}
      />
    );
  }

  return (
    <pre className="max-h-72 overflow-auto rounded-lg border border-border-subtle bg-surface-muted/50 p-3 text-xs leading-relaxed text-text">
      {JSON.stringify(body, null, 2)}
    </pre>
  );
}

/**
 * The doubts of one generation run, reviewed together (WO 22 Stage A.4).
 *
 * A verifier confirmed most of a run and doubted a few; the doubts share a
 * batch id. A reviewer opens the batch, sees each item with its key and the
 * verifier's reason, toggles the ones to leave, and approves the rest in one
 * transaction.
 */
function BatchReview(): React.JSX.Element {
  const { t, i18n } = useTranslation();
  const queryClient = useQueryClient();
  const [offset, setOffset] = useState(0);
  const [openBatch, setOpenBatch] = useState<AdminReviewBatch | null>(null);
  const [reject, setReject] = useState<Set<string>>(new Set());
  const [note, setNote] = useState("");
  const [isApproving, setIsApproving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const { data, isLoading } = useQuery({
    queryKey: ["admin", "review-batches", offset],
    queryFn: () => adminApi.listReviewBatches({ limit: PAGE_SIZE, offset }),
  });

  const { data: itemsData, isLoading: itemsLoading } = useQuery({
    queryKey: ["admin", "review-queue", "batch", openBatch?.batch],
    enabled: openBatch !== null,
    queryFn: () =>
      adminApi.listReviewQueue({
        batch: openBatch?.batch,
        limit: 100,
        offset: 0,
      }),
  });

  const batches = data?.items ?? [];
  const total = data?.total ?? 0;
  const totalPages = Math.max(1, Math.ceil(total / PAGE_SIZE));
  const currentPage = Math.floor(offset / PAGE_SIZE) + 1;
  const items = itemsData?.items ?? [];

  const toggleReject = (id: string) => {
    setReject((prev) => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  };

  const approve = async () => {
    if (!openBatch) return;
    setIsApproving(true);
    setError(null);
    try {
      await adminApi.approveReviewBatch(
        openBatch.batch,
        [...reject],
        note.trim() || undefined,
      );
      setOpenBatch(null);
      setReject(new Set());
      setNote("");
      await queryClient.invalidateQueries({
        queryKey: ["admin", "review-batches"],
      });
      await queryClient.invalidateQueries({
        queryKey: ["admin", "review-queue"],
      });
    } catch (err) {
      setError(
        err instanceof Error
          ? err.message
          : t("adminReview.batchFailed", "The batch could not be approved."),
      );
    } finally {
      setIsApproving(false);
    }
  };

  if (isLoading) {
    return (
      <div className="flex justify-center py-12">
        <Loader2
          className="h-6 w-6 animate-spin text-primary"
          aria-hidden="true"
        />
      </div>
    );
  }

  return (
    <div className="space-y-4">
      {batches.length === 0 ? (
        <div className="rounded-xl border border-border bg-card py-12 text-center text-muted-foreground">
          <Layers
            className="mx-auto mb-2 h-8 w-8 opacity-40"
            aria-hidden="true"
          />
          <p className="text-sm">
            {t("adminReview.noBatches", "No generation run is waiting.")}
          </p>
        </div>
      ) : (
        <ul className="space-y-2">
          {batches.map((batch) => (
            <li key={batch.batch}>
              <button
                type="button"
                onClick={() => {
                  setOpenBatch(batch);
                  setReject(new Set());
                  setNote("");
                  setError(null);
                }}
                className="flex w-full min-h-[44px] flex-col gap-2 rounded-xl border border-border-subtle bg-card p-4 text-left transition-colors hover:border-primary/40 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary sm:flex-row sm:items-center sm:justify-between"
              >
                <span className="min-w-0 space-y-1">
                  <span className="block truncate text-sm font-medium text-text">
                    {batch.batch}
                  </span>
                  <span className="flex flex-wrap items-center gap-2 text-xs text-text-muted">
                    <Badge variant="secondary">
                      {t("adminReview.batchItems", {
                        count: batch.item_count,
                        defaultValue: `${batch.item_count} items`,
                      })}
                    </Badge>
                    {(batch.kinds ?? []).map((kind) => (
                      <Badge key={kind} variant="outline">
                        {kind}
                      </Badge>
                    ))}
                  </span>
                </span>
                <span className="text-xs text-text-muted">
                  {new Date(batch.created_at).toLocaleDateString(
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
            {t("adminReview.pageOf", {
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

      {openBatch && (
        <div
          role="dialog"
          aria-modal="true"
          aria-labelledby="review-batch-title"
          className="fixed inset-0 z-50 flex items-start justify-center overflow-y-auto bg-overlay/75 p-4 backdrop-blur-sm"
        >
          <div className="my-8 w-full max-w-2xl space-y-4 rounded-2xl border border-border bg-surface-card p-4 shadow-2xl sm:p-6">
            <div className="flex items-start justify-between gap-3">
              <div className="min-w-0 space-y-1">
                <h2
                  id="review-batch-title"
                  className="break-words text-lg font-bold text-text"
                >
                  {openBatch.batch}
                </h2>
                <p className="text-xs text-text-muted">
                  {t(
                    "adminReview.batchHint",
                    "Reject the items a person must read; the rest publish together.",
                  )}
                </p>
              </div>
              <Button
                variant="ghost"
                onClick={() => setOpenBatch(null)}
                aria-label={t("common.close", "Close")}
                className="h-11 w-11 shrink-0 rounded-full p-0"
              >
                <X className="h-4 w-4" aria-hidden="true" />
              </Button>
            </div>

            {itemsLoading ? (
              <div className="flex justify-center py-8">
                <Loader2
                  className="h-5 w-5 animate-spin text-primary"
                  aria-hidden="true"
                />
              </div>
            ) : (
              <ul className="max-h-80 space-y-2 overflow-y-auto">
                {items.map((item) => (
                  <li
                    key={item.id}
                    className="flex items-start justify-between gap-3 rounded-lg border border-border-subtle p-3"
                  >
                    <div className="min-w-0 space-y-1">
                      <span className="block truncate text-sm font-medium text-text">
                        {item.slug}
                      </span>
                      <span className="flex flex-wrap items-center gap-2 text-xs text-text-muted">
                        <Badge variant="secondary">{item.kind}</Badge>
                        <Badge variant="outline">{item.cefr_level}</Badge>
                      </span>
                      {typeof item.provenance?.["verification"] === "object" &&
                        item.provenance["verification"] !== null && (
                          <span className="block text-xs text-danger-accent">
                            {asText(
                              (
                                item.provenance["verification"] as Record<
                                  string,
                                  unknown
                                >
                              )["reason"],
                              t("adminReview.doubted", "The verifier doubted it."),
                            )}
                          </span>
                        )}
                    </div>
                    <Button
                      type="button"
                      variant={reject.has(item.id) ? "destructive" : "outline"}
                      size="sm"
                      aria-pressed={reject.has(item.id)}
                      onClick={() => toggleReject(item.id)}
                      className="h-11 shrink-0"
                    >
                      {t("adminReview.reject", "Reject")}
                    </Button>
                  </li>
                ))}
              </ul>
            )}

            <label className="block space-y-1">
              <span className="text-sm font-medium text-text">
                {t("adminReview.noteLabel", "Note")}
              </span>
              <textarea
                value={note}
                onChange={(e) => setNote(e.target.value)}
                rows={2}
                className="w-full rounded-lg border border-input bg-background p-3 text-base focus:outline-none focus:ring-1 focus:ring-ring"
              />
            </label>

            {error && (
              <p
                role="alert"
                className="flex items-start gap-2 text-sm text-danger-accent"
              >
                <AlertCircle
                  className="mt-0.5 h-4 w-4 shrink-0"
                  aria-hidden="true"
                />
                <span>{error}</span>
              </p>
            )}

            <div className="flex flex-col-reverse gap-2 sm:flex-row sm:justify-end">
              <Button
                variant="outline"
                onClick={() => setOpenBatch(null)}
                disabled={isApproving}
              >
                {t("common.cancel", "Cancel")}
              </Button>
              <Button
                disabled={isApproving}
                isLoading={isApproving}
                onClick={() => void approve()}
                className="gap-2"
              >
                <CheckCircle2 className="h-4 w-4" aria-hidden="true" />
                {t("adminReview.approveBatch", "Approve the batch")}
              </Button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}

export function AdminReviewQueue(): React.JSX.Element {
  const { t, i18n } = useTranslation();
  const queryClient = useQueryClient();
  const [purpose, setPurpose] = useState<string>("");
  const [kind, setKind] = useState<string>("");
  const [offset, setOffset] = useState(0);
  const [selected, setSelected] = useState<AdminReviewQueueItem | null>(null);
  const [note, setNote] = useState("");
  const [isDeciding, setIsDeciding] = useState(false);
  const [decisionError, setDecisionError] = useState<string | null>(null);
  const [view, setView] = useState<"items" | "batches">("items");

  const { data, isLoading, isFetching, isError, error, refetch } = useQuery({
    queryKey: ["admin", "review-queue", purpose, kind, offset],
    queryFn: () =>
      adminApi.listReviewQueue({
        purpose: purpose || undefined,
        kind: kind || undefined,
        limit: PAGE_SIZE,
        offset,
      }),
  });

  const items = data?.items ?? [];
  const total = data?.total ?? 0;
  const totalPages = Math.max(1, Math.ceil(total / PAGE_SIZE));
  const currentPage = Math.floor(offset / PAGE_SIZE) + 1;

  const decide = async (decision: "approved" | "changes_requested") => {
    if (!selected) return;
    setIsDeciding(true);
    setDecisionError(null);
    try {
      await adminApi.reviewContent(
        selected.item_id,
        decision,
        note.trim() || undefined,
      );
      // Approval is not publication: the version moves to approved, and the
      // publish call is what puts it in front of a learner. One action for the
      // reviewer, two steps underneath.
      if (decision === "approved") {
        await adminApi.publishContent(selected.item_id);
      }
      setSelected(null);
      setNote("");
      await queryClient.invalidateQueries({
        queryKey: ["admin", "review-queue"],
      });
      await refetch();
    } catch (err) {
      setDecisionError(
        err instanceof Error
          ? err.message
          : t("adminReview.decisionFailed", "The decision could not be saved."),
      );
    } finally {
      setIsDeciding(false);
    }
  };

  const viewToggle = (
    <div
      className="flex gap-2"
      role="tablist"
      aria-label={t("adminReview.view", "View")}
    >
      <Button
        variant={view === "items" ? "primary" : "outline"}
        size="sm"
        role="tab"
        aria-selected={view === "items"}
        onClick={() => setView("items")}
      >
        {t("adminReview.itemsView", "Items")}
      </Button>
      <Button
        variant={view === "batches" ? "primary" : "outline"}
        size="sm"
        role="tab"
        aria-selected={view === "batches"}
        onClick={() => setView("batches")}
      >
        {t("adminReview.batchesView", "Batches")}
      </Button>
    </div>
  );

  if (view === "batches") {
    return (
      <div className="space-y-4">
        {viewToggle}
        <BatchReview />
      </div>
    );
  }

  return (
    <div className="space-y-4">
      {viewToggle}
      <div className="flex flex-col gap-3 rounded-xl border border-border bg-card p-4 sm:flex-row sm:items-center sm:justify-between">
        <div className="flex flex-wrap items-center gap-2">
          <select
            value={purpose}
            onChange={(e) => {
              setPurpose(e.target.value);
              setOffset(0);
            }}
            aria-label={t("adminReview.purposeFilter", "Purpose")}
            className="h-11 rounded-lg border border-input bg-background px-3 text-base focus:outline-none focus:ring-1 focus:ring-ring"
          >
            {PURPOSES.map((value) => (
              <option key={value} value={value}>
                {value === ""
                  ? t("adminReview.allPurposes", "All purposes")
                  : value}
              </option>
            ))}
          </select>
          <select
            value={kind}
            onChange={(e) => {
              setKind(e.target.value);
              setOffset(0);
            }}
            aria-label={t("adminReview.kindFilter", "Kind")}
            className="h-11 rounded-lg border border-input bg-background px-3 text-base focus:outline-none focus:ring-1 focus:ring-ring"
          >
            {KINDS.map((value) => (
              <option key={value} value={value}>
                {value === "" ? t("adminReview.allKinds", "All kinds") : value}
              </option>
            ))}
          </select>
        </div>
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

      {isError && (
        <div
          role="alert"
          className="flex items-center gap-3 rounded-lg border border-danger/20 bg-danger/10 p-4 text-danger-accent"
        >
          <AlertCircle className="h-5 w-5 shrink-0" aria-hidden="true" />
          <p className="text-sm">
            {error instanceof Error
              ? error.message
              : t(
                  "adminReview.errorDesc",
                  "The review queue could not be loaded.",
                )}
          </p>
        </div>
      )}

      {isLoading ? (
        <div className="flex justify-center py-12">
          <Loader2
            className="h-6 w-6 animate-spin text-primary"
            aria-hidden="true"
          />
        </div>
      ) : items.length === 0 ? (
        <div className="rounded-xl border border-border bg-card py-12 text-center text-muted-foreground">
          <ClipboardCheck
            className="mx-auto mb-2 h-8 w-8 opacity-40"
            aria-hidden="true"
          />
          <p className="text-sm">
            {t("adminReview.empty", "Nothing is waiting for review.")}
          </p>
        </div>
      ) : (
        <ul className="space-y-2">
          {items.map((item) => (
            <li key={item.id}>
              <button
                type="button"
                onClick={() => {
                  setSelected(item);
                  setNote("");
                  setDecisionError(null);
                }}
                className="flex w-full min-h-[44px] flex-col gap-2 rounded-xl border border-border-subtle bg-card p-4 text-left transition-colors hover:border-primary/40 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary sm:flex-row sm:items-center sm:justify-between"
              >
                <span className="min-w-0 space-y-1">
                  <span className="block truncate text-sm font-medium text-text">
                    {item.slug}
                  </span>
                  <span className="flex flex-wrap items-center gap-2 text-xs text-text-muted">
                    <Badge variant="secondary">{item.kind}</Badge>
                    <Badge variant="outline">{item.cefr_level}</Badge>
                    {(item.node_codes ?? []).map((code) => (
                      <Badge key={code} variant="outline">
                        {code}
                      </Badge>
                    ))}
                  </span>
                </span>
                <span className="text-xs text-text-muted">
                  {new Date(item.created_at).toLocaleDateString(
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
            {t("adminReview.pageOf", {
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
          aria-labelledby="review-queue-title"
          className="fixed inset-0 z-50 flex items-start justify-center overflow-y-auto bg-overlay/75 p-4 backdrop-blur-sm"
        >
          <div className="my-8 w-full max-w-2xl space-y-4 rounded-2xl border border-border bg-surface-card p-4 shadow-2xl sm:p-6">
            <div className="flex items-start justify-between gap-3">
              <div className="min-w-0 space-y-1">
                <h2
                  id="review-queue-title"
                  className="break-words text-lg font-bold text-text"
                >
                  {selected.slug}
                </h2>
                <div className="flex flex-wrap items-center gap-2 text-xs text-text-muted">
                  <Badge variant="secondary">{selected.kind}</Badge>
                  <Badge variant="outline">{selected.cefr_level}</Badge>
                  {(selected.node_codes ?? []).map((code) => (
                    <Badge key={code} variant="outline">
                      {code}
                    </Badge>
                  ))}
                </div>
              </div>
              <Button
                variant="ghost"
                onClick={() => setSelected(null)}
                aria-label={t("common.close", "Close")}
                className="h-11 w-11 shrink-0 rounded-full p-0"
              >
                <X className="h-4 w-4" aria-hidden="true" />
              </Button>
            </div>

            <div className="rounded-xl border border-border-subtle bg-surface-muted/30 p-2">
              <ReviewItemBody item={selected} />
            </div>

            <dl className="grid grid-cols-1 gap-2 text-sm sm:grid-cols-2">
              <div className="rounded-lg border border-border-subtle p-3">
                <dt className="text-xs font-semibold uppercase tracking-wider text-text-muted">
                  {t("adminReview.blindSolve", "Blind solve")}
                </dt>
                <dd className="mt-1 font-mono text-xs text-text">
                  {blindSolveOptionId(selected.blind_solve_answer) ??
                    JSON.stringify(selected.blind_solve_answer ?? {})}
                </dd>
              </div>
              <div className="rounded-lg border border-border-subtle p-3">
                <dt className="text-xs font-semibold uppercase tracking-wider text-text-muted">
                  {t("adminReview.provenance", "Provenance")}
                </dt>
                <dd className="mt-1 font-mono text-xs text-text">
                  {asText(
                    selected.provenance?.["model"],
                    t("adminReview.unknownModel", "unknown model"),
                  )}
                  {" · "}
                  {asText(selected.provenance?.["prompt_version"], "—")}
                </dd>
              </div>
            </dl>

            {selected.cefr_reasoning && (
              <div className="rounded-lg border border-border-subtle p-3 text-sm text-text-muted">
                <p className="text-xs font-semibold uppercase tracking-wider text-text-muted">
                  {t("adminReview.cefrReasoning", "Level check")}
                </p>
                <p className="mt-1">{selected.cefr_reasoning}</p>
              </div>
            )}

            <label className="block space-y-1">
              <span className="text-sm font-medium text-text">
                {t(
                  "adminReview.noteLabel",
                  "Note (required to request changes)",
                )}
              </span>
              <textarea
                value={note}
                onChange={(e) => setNote(e.target.value)}
                rows={3}
                className="w-full rounded-lg border border-input bg-background p-3 text-base focus:outline-none focus:ring-1 focus:ring-ring"
              />
            </label>

            {decisionError && (
              <p
                role="alert"
                className="flex items-start gap-2 text-sm text-danger-accent"
              >
                <AlertCircle
                  className="mt-0.5 h-4 w-4 shrink-0"
                  aria-hidden="true"
                />
                <span>{decisionError}</span>
              </p>
            )}

            <div className="flex flex-col-reverse gap-2 sm:flex-row sm:justify-end">
              <Button
                variant="outline"
                disabled={isDeciding || note.trim() === ""}
                onClick={() => void decide("changes_requested")}
              >
                {t("adminReview.requestChanges", "Request changes")}
              </Button>
              <Button
                disabled={isDeciding}
                isLoading={isDeciding}
                onClick={() => void decide("approved")}
                className="gap-2"
              >
                <CheckCircle2 className="h-4 w-4" aria-hidden="true" />
                {t("adminReview.approveAndPublish", "Approve and publish")}
              </Button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
