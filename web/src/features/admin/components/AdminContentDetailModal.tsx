import React, { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import {
  AlertCircle,
  Archive,
  ArrowRight,
  CheckCircle2,
  Clock,
  Code2,
  History,
  Loader2,
  Send,
  Sparkles,
  Tag,
  X,
  XCircle,
} from "lucide-react";
import { adminApi, type ContentVersion } from "../api/adminApi";
import { PERMISSIONS, usePermissions } from "../model/permissions";
import { Button } from "@/components/ui/button";

interface AdminContentDetailModalProps {
  // Not nullable. The list renders this modal only when an item is selected, and
  // a nullable id here meant the query key could be null while the fetch still ran.
  itemId: string;
  onClose: () => void;
  onItemUpdated: () => void;
}

const CEFR_LEVELS = ["A1", "A2", "B1", "B2", "C1", "C2"];

/**
 * Validates authored content body against grader expectations before sending to the backend.
 * Specifically checks for valid JSON and key attributes like `correct_answer` or `correct_pairs`
 * so an authored exercise never fails during a learner's submission.
 */
export function validateContentBody(
  kind: string,
  rawJson: string,
): { valid: boolean; error?: string; parsed?: unknown } {
  let parsed: unknown;
  try {
    parsed = JSON.parse(rawJson);
  } catch (err: unknown) {
    return {
      valid: false,
      error: `Invalid JSON syntax: ${err instanceof Error ? err.message : String(err)}`,
    };
  }

  if (typeof parsed !== "object" || parsed === null || Array.isArray(parsed)) {
    return { valid: false, error: "Content body must be a JSON object." };
  }

  const record = parsed as Record<string, unknown>;

  // Check grader requirements for interactive exercise kinds
  if (
    kind.startsWith("vocab_") ||
    kind.startsWith("grammar_") ||
    kind.includes("choice") ||
    kind.includes("match")
  ) {
    const hasAnswer =
      typeof record.correct_answer === "string" &&
      record.correct_answer.trim().length > 0;
    const hasOptionId =
      typeof record.correct_option_id === "string" &&
      record.correct_option_id.trim().length > 0;
    const hasPairs =
      typeof record.correct_pairs === "object" &&
      record.correct_pairs !== null &&
      !Array.isArray(record.correct_pairs) &&
      Object.keys(record.correct_pairs).length > 0;

    if (kind === "vocab_match" || kind.includes("match")) {
      if (!hasPairs) {
        return {
          valid: false,
          error:
            "Matching exercises require 'correct_pairs' (mapping word ID to definition ID) for the grader.",
        };
      }
    } else if (kind.includes("choice")) {
      if (!hasAnswer && !hasOptionId) {
        return {
          valid: false,
          error:
            "Multiple choice exercises require 'correct_answer' or 'correct_option_id' matching one of the options.",
        };
      }
    } else if ("prompt" in record || "acceptable" in record) {
      if (!hasAnswer && !hasPairs) {
        return {
          valid: false,
          error:
            "Interactive exercises must declare either 'correct_answer' or 'correct_pairs'; the grader refuses to score without them.",
        };
      }
    }
  }

  if (kind === "reading_comprehension") {
    const hasPassage =
      typeof record.passage === "string" && record.passage.trim().length > 0;
    if (!hasPassage) {
      return {
        valid: false,
        error: "Reading comprehension exercises require a non-empty 'passage'.",
      };
    }

    if (Array.isArray(record.questions)) {
      if (record.questions.length === 0) {
        return {
          valid: false,
          error:
            "Reading comprehension with 'questions' must include at least one question.",
        };
      }
      for (let i = 0; i < record.questions.length; i++) {
        const q = record.questions[i];
        if (typeof q !== "object" || q === null) {
          return {
            valid: false,
            error: `Question at index ${i} must be an object.`,
          };
        }
        const qRec = q as Record<string, unknown>;
        const qAnswer =
          (typeof qRec.answer === "string" && qRec.answer.trim().length > 0) ||
          (typeof qRec.correct_answer === "string" &&
            qRec.correct_answer.trim().length > 0) ||
          (typeof qRec.correct_option_id === "string" &&
            qRec.correct_option_id.trim().length > 0);
        if (!qAnswer) {
          return {
            valid: false,
            error: `Question at index ${i} (${typeof qRec.prompt === "string" ? qRec.prompt : "unnamed"}) must specify an answer.`,
          };
        }
      }
    } else {
      const hasAnswer =
        (typeof record.correct_answer === "string" &&
          record.correct_answer.trim().length > 0) ||
        (typeof record.correct_option_id === "string" &&
          record.correct_option_id.trim().length > 0);
      if (!hasAnswer) {
        return {
          valid: false,
          error:
            "Reading comprehension requires 'correct_answer' or a 'questions' array where every question has an answer.",
        };
      }
    }
  }

  if (kind === "writing_prompt") {
    const hasPrompt =
      typeof record.prompt === "string" && record.prompt.trim().length > 0;
    if (!hasPrompt) {
      return {
        valid: false,
        error: "Writing prompt exercises require a non-empty 'prompt'.",
      };
    }

    const minWords = record.min_words;
    if (typeof minWords !== "number" || minWords <= 0) {
      return {
        valid: false,
        error:
          "Writing prompt exercises require 'min_words' to be greater than zero.",
      };
    }
  }

  return { valid: true, parsed };
}

export const AdminContentDetailModal: React.FC<
  AdminContentDetailModalProps
> = ({ itemId, onClose, onItemUpdated }) => {
  const { t } = useTranslation();
  const { can } = usePermissions();

  const [isSubmitting, setIsSubmitting] = useState(false);
  const [actionError, setActionError] = useState<string | null>(null);
  const [actionSuccess, setActionSuccess] = useState<string | null>(null);

  // Form state
  const [bodyText, setBodyText] = useState("");
  const [cefrLevel, setCefrLevel] = useState("B1");
  const [tagsInput, setTagsInput] = useState("");
  const [validationError, setValidationError] = useState<string | null>(null);

  // Review comment state
  const [isRejecting, setIsRejecting] = useState(false);
  const [rejectComment, setRejectComment] = useState("");

  const {
    data: detail,
    isLoading,
    error: loadError,
    refetch,
  } = useQuery({
    queryKey: ["admin", "content", "detail", itemId],
    queryFn: () => adminApi.getContent(itemId),
  });

  const error =
    actionError ??
    (loadError
      ? loadError instanceof Error
        ? loadError.message
        : t("admin.failedToLoadContentDetail")
      : null);

  const latestVer: ContentVersion | null =
    detail && detail.versions.length > 0
      ? (detail.versions[detail.versions.length - 1] ?? null)
      : null;

  // Seeding the editor during render rather than from an effect. The editor is
  // writable, so its state cannot simply be derived — but re-seeding it needs to
  // happen exactly when a new version arrives, which is what the guard says. A
  // useEffect here would set state on every render pass the linter can see.
  const [seededVersionID, setSeededVersionID] = useState<string | null>(null);
  if (latestVer && latestVer.id !== seededVersionID) {
    setSeededVersionID(latestVer.id);
    setBodyText(JSON.stringify(latestVer.body ?? {}, null, 2));
    setCefrLevel(latestVer.cefr_level || "B1");
    // tags are objects with a namespace, a code and a label — not strings.
    // join()-ing them produced a field full of "[object Object]", and saving
    // that would have written it back as the tag list.
    setTagsInput((latestVer.tags ?? []).map((tag) => tag.code).join(", "));
  }

  const fetchDetail = async (_id: string) => {
    await refetch();
  };

  const handleValidateOnly = () => {
    if (!detail) return;
    const res = validateContentBody(detail.kind, bodyText);
    if (!res.valid) {
      setValidationError(res.error ?? "Validation failed.");
    } else {
      setValidationError(null);
      setActionSuccess(t("admin.jsonValid"));
      setTimeout(() => setActionSuccess(null), 3000);
    }
  };

  const handleSaveDraft = async () => {
    if (!detail) return;
    const res = validateContentBody(detail.kind, bodyText);
    if (!res.valid) {
      setValidationError(res.error ?? "Invalid body content.");
      return;
    }
    setValidationError(null);
    setIsSubmitting(true);
    setActionError(null);

    try {
      const tags = tagsInput
        .split(",")
        .map((s) => s.trim())
        .filter(Boolean);

      await adminApi.updateDraft(detail.id, res.parsed, cefrLevel, tags);
      setActionSuccess(t("admin.draftSaved"));
      await fetchDetail(detail.id);
      onItemUpdated();
    } catch (err: unknown) {
      setActionError(
        err instanceof Error ? err.message : t("admin.failedToSaveDraft"),
      );
    } finally {
      setIsSubmitting(false);
    }
  };

  const handleSubmitForReview = async () => {
    if (!detail) return;
    setIsSubmitting(true);
    setActionError(null);
    try {
      await adminApi.submitContent(detail.id);
      setActionSuccess(t("admin.submittedForReview"));
      await fetchDetail(detail.id);
      onItemUpdated();
    } catch (err: unknown) {
      setActionError(
        err instanceof Error ? err.message : t("admin.failedToSubmit"),
      );
    } finally {
      setIsSubmitting(false);
    }
  };

  const handleApprove = async () => {
    if (!detail) return;
    setIsSubmitting(true);
    setActionError(null);
    try {
      await adminApi.reviewContent(detail.id, "approved");
      setActionSuccess(t("admin.contentApproved"));
      await fetchDetail(detail.id);
      onItemUpdated();
    } catch (err: unknown) {
      setActionError(
        err instanceof Error ? err.message : t("admin.failedToApprove"),
      );
    } finally {
      setIsSubmitting(false);
    }
  };

  const handleRequestChanges = async () => {
    if (!detail) return;
    setIsSubmitting(true);
    setActionError(null);
    try {
      await adminApi.reviewContent(
        detail.id,
        "changes_requested",
        rejectComment.trim() || undefined,
      );
      setIsRejecting(false);
      setRejectComment("");
      setActionSuccess(t("admin.changesRequested"));
      await fetchDetail(detail.id);
      onItemUpdated();
    } catch (err: unknown) {
      setActionError(
        err instanceof Error ? err.message : t("admin.failedToRequestChanges"),
      );
    } finally {
      setIsSubmitting(false);
    }
  };

  const handlePublish = async () => {
    if (!detail) return;
    setIsSubmitting(true);
    setActionError(null);
    try {
      await adminApi.publishContent(detail.id);
      setActionSuccess(t("admin.contentPublished"));
      await fetchDetail(detail.id);
      onItemUpdated();
    } catch (err: unknown) {
      setActionError(
        err instanceof Error ? err.message : t("admin.failedToPublish"),
      );
    } finally {
      setIsSubmitting(false);
    }
  };

  const handleArchive = async () => {
    if (!detail) return;
    if (!window.confirm(t("admin.confirmArchive"))) {
      return;
    }
    setIsSubmitting(true);
    setActionError(null);
    try {
      await adminApi.archiveContent(detail.id);
      setActionSuccess(t("admin.contentArchived"));
      await fetchDetail(detail.id);
      onItemUpdated();
    } catch (err: unknown) {
      setActionError(
        err instanceof Error ? err.message : t("admin.failedToArchive"),
      );
    } finally {
      setIsSubmitting(false);
    }
  };

  if (!itemId) return null;

  const currentStatus = detail?.status;
  const isArchived = currentStatus === "archived";

  const getStatusBadge = (status: string | undefined) => {
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

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm p-4 overflow-y-auto">
      <div className="relative w-full max-w-4xl bg-card border border-border rounded-xl shadow-2xl overflow-hidden my-8 flex flex-col max-h-[90vh]">
        {/* Header */}
        <div className="flex items-center justify-between px-6 py-4 border-b border-border bg-muted/40">
          <div className="flex items-center gap-3">
            <div className="p-2 rounded-lg bg-primary/10 text-primary">
              <Code2 className="w-5 h-5" />
            </div>
            <div>
              <div className="flex items-center gap-2">
                <h2 className="text-lg font-semibold text-foreground">
                  {detail?.slug || t("admin.contentDetail")}
                </h2>
                {detail && (
                  <span
                    className={`text-xs px-2.5 py-0.5 rounded-full font-medium border ${getStatusBadge(
                      detail.status,
                    )}`}
                  >
                    {detail.status}
                  </span>
                )}
                {detail && (
                  <span className="text-xs px-2 py-0.5 rounded bg-muted text-muted-foreground font-mono">
                    {detail.kind}
                  </span>
                )}
              </div>
              <p className="text-xs text-muted-foreground mt-0.5 font-mono">
                ID: {detail?.id}
              </p>
            </div>
          </div>
          <Button
            variant="ghost"
            size="sm"
            onClick={onClose}
            className="text-muted-foreground hover:text-foreground"
          >
            <X className="w-5 h-5" />
          </Button>
        </div>

        {/* Content Body */}
        <div className="p-6 overflow-y-auto space-y-6 flex-1">
          {isLoading ? (
            <div className="flex flex-col items-center justify-center py-16 gap-3 text-muted-foreground">
              <Loader2 className="w-8 h-8 animate-spin text-primary" />
              <p>{t("common.loading")}</p>
            </div>
          ) : error ? (
            <div className="p-4 rounded-lg bg-destructive/10 border border-destructive/20 text-destructive flex items-center gap-3">
              <AlertCircle className="w-5 h-5 flex-shrink-0" />
              <p className="text-sm">{error}</p>
            </div>
          ) : detail ? (
            <>
              {actionSuccess && (
                <div className="p-3 rounded-lg bg-emerald-500/10 border border-emerald-500/20 text-emerald-500 flex items-center gap-2 text-sm">
                  <CheckCircle2 className="w-4 h-4 flex-shrink-0" />
                  <span>{actionSuccess}</span>
                </div>
              )}

              {/* State Machine Transition Flow Banner */}
              <div className="flex items-center gap-2 text-xs text-muted-foreground bg-muted/30 p-3 rounded-lg border border-border/50">
                <span className="font-semibold text-foreground uppercase tracking-wider text-[10px]">
                  {t("admin.workflow")}
                </span>
                <span
                  className={
                    detail.status === "draft"
                      ? "font-bold text-amber-500 underline"
                      : ""
                  }
                >
                  {t("admin.draft")}
                </span>
                <ArrowRight className="w-3 h-3 text-muted-foreground/60" />
                <span
                  className={
                    detail.status === "in_review"
                      ? "font-bold text-blue-500 underline"
                      : ""
                  }
                >
                  {t("admin.inReview")}
                </span>
                <ArrowRight className="w-3 h-3 text-muted-foreground/60" />
                <span
                  className={
                    detail.status === "approved"
                      ? "font-bold text-emerald-500 underline"
                      : ""
                  }
                >
                  {t("admin.approved")}
                </span>
                <ArrowRight className="w-3 h-3 text-muted-foreground/60" />
                <span
                  className={
                    detail.status === "published"
                      ? "font-bold text-green-500 underline"
                      : ""
                  }
                >
                  {t("admin.published")}
                </span>
                <ArrowRight className="w-3 h-3 text-muted-foreground/60" />
                <span
                  className={
                    detail.status === "archived"
                      ? "font-bold text-muted-foreground underline"
                      : ""
                  }
                >
                  {t("admin.archived")}
                </span>
              </div>

              {/* Metadata row */}
              <div className="grid grid-cols-1 sm:grid-cols-3 gap-4">
                <div>
                  <label className="text-xs font-medium text-muted-foreground block mb-1">
                    {t("admin.cefrLevel")}
                  </label>
                  <select
                    value={cefrLevel}
                    onChange={(e) => setCefrLevel(e.target.value)}
                    disabled={isArchived || !can(PERMISSIONS.contentEdit)}
                    className="w-full h-11 rounded-md border border-input bg-background px-3 py-1 text-base shadow-sm focus:outline-none focus:ring-1 focus:ring-ring"
                  >
                    {CEFR_LEVELS.map((lvl) => (
                      <option key={lvl} value={lvl}>
                        {lvl}
                      </option>
                    ))}
                  </select>
                </div>
                <div className="sm:col-span-2">
                  <label className="text-xs font-medium text-muted-foreground block mb-1">
                    {t("admin.tagsCommaSeparated")}
                  </label>
                  <div className="relative">
                    <Tag className="w-4 h-4 absolute left-3 top-2.5 text-muted-foreground" />
                    <input
                      type="text"
                      value={tagsInput}
                      onChange={(e) => setTagsInput(e.target.value)}
                      disabled={isArchived || !can(PERMISSIONS.contentEdit)}
                      placeholder={t("admin.tagsPlaceholder")}
                      className="w-full h-11 rounded-md border border-input bg-background pl-9 pr-3 py-1 text-base shadow-sm focus:outline-none focus:ring-1 focus:ring-ring"
                    />
                  </div>
                </div>
              </div>

              {/* JSON Body Editor */}
              <div className="space-y-2">
                <div className="flex items-center justify-between">
                  <label className="text-xs font-medium text-muted-foreground flex items-center gap-1.5">
                    <Code2 className="w-4 h-4" />
                    {t("admin.bodyJson")}
                  </label>
                  <Button
                    type="button"
                    variant="outline"
                    size="sm"
                    onClick={handleValidateOnly}
                    className="min-h-11 px-4 text-xs"
                  >
                    <Sparkles className="w-3.5 h-3.5 mr-1 text-amber-500" />
                    {t("admin.validateJson")}
                  </Button>
                </div>

                <div className="relative">
                  <textarea
                    value={bodyText}
                    onChange={(e) => {
                      setBodyText(e.target.value);
                      if (validationError) setValidationError(null);
                    }}
                    disabled={isArchived || !can(PERMISSIONS.contentEdit)}
                    rows={12}
                    className="w-full font-mono text-base rounded-lg border border-input bg-muted/20 p-3 leading-relaxed shadow-inner focus:outline-none focus:ring-1 focus:ring-ring"
                    spellCheck={false}
                  />
                </div>

                {validationError && (
                  <div className="p-3 rounded-lg bg-destructive/10 border border-destructive/20 text-destructive flex items-center gap-2 text-xs">
                    <AlertCircle className="w-4 h-4 flex-shrink-0" />
                    <span>{validationError}</span>
                  </div>
                )}
              </div>

              {/* Version History */}
              <div className="border border-border rounded-lg p-4 bg-muted/10 space-y-3">
                <div className="flex items-center gap-2 text-xs font-semibold text-foreground">
                  <History className="w-4 h-4 text-primary" />
                  {t("admin.versionHistory")} ({detail.versions.length})
                </div>
                <div className="space-y-2 max-h-36 overflow-y-auto">
                  {detail.versions?.map((v: ContentVersion) => (
                    <div
                      key={v.id}
                      className="flex items-center justify-between p-2 rounded bg-background border border-border/50 text-xs"
                    >
                      <div className="flex items-center gap-2">
                        <span className="font-mono font-medium">
                          v{v.version}
                        </span>
                        <span
                          className={`px-2 py-0.2 rounded-full font-medium border text-[10px] ${getStatusBadge(v.status)}`}
                        >
                          {v.status}
                        </span>
                        <span className="text-muted-foreground">
                          {v.cefr_level}
                        </span>
                      </div>
                      <div className="flex items-center gap-2 text-muted-foreground">
                        <Clock className="w-3 h-3" />
                        <span>
                          {v.published_at
                            ? new Date(v.published_at).toLocaleString()
                            : "Draft"}
                        </span>
                      </div>
                    </div>
                  ))}
                </div>
              </div>

              {/* Request Changes Comment Input if rejecting */}
              {isRejecting && (
                <div className="p-4 rounded-lg border border-amber-500/30 bg-amber-500/5 space-y-3">
                  <label className="text-xs font-medium text-amber-500 block">
                    {t("admin.editorialFeedback")}
                  </label>
                  <textarea
                    value={rejectComment}
                    onChange={(e) => setRejectComment(e.target.value)}
                    rows={3}
                    placeholder={t("admin.editorialFeedbackPlaceholder")}
                    className="w-full text-base rounded-md border border-input bg-background p-2.5 focus:outline-none focus:ring-1 focus:ring-ring"
                  />
                  <div className="flex items-center justify-end gap-2">
                    <Button
                      variant="ghost"
                      size="sm"
                      onClick={() => setIsRejecting(false)}
                      disabled={isSubmitting}
                    >
                      {t("common.cancel")}
                    </Button>
                    <Button
                      variant="destructive"
                      size="sm"
                      onClick={() => void handleRequestChanges()}
                      disabled={isSubmitting}
                    >
                      {isSubmitting ? (
                        <Loader2 className="w-3.5 h-3.5 animate-spin mr-1" />
                      ) : null}
                      {t("admin.confirmRequestChanges")}
                    </Button>
                  </div>
                </div>
              )}
            </>
          ) : null}
        </div>

        {/* Footer Actions — Strictly gated by permissions & legal state machine transitions */}
        <div className="flex items-center justify-between px-6 py-4 border-t border-border bg-muted/40">
          <Button variant="ghost" onClick={onClose} disabled={isSubmitting}>
            {t("common.close")}
          </Button>

          <div className="flex items-center gap-2">
            {/* Can edit and draft/approved/published -> Save Draft */}
            {can(PERMISSIONS.contentEdit) && !isArchived && (
              <Button
                variant="outline"
                onClick={() => void handleSaveDraft()}
                disabled={isSubmitting}
              >
                {isSubmitting ? (
                  <Loader2 className="w-4 h-4 animate-spin mr-1" />
                ) : null}
                {currentStatus === "published" ? "New Draft" : "Save Draft"}
              </Button>
            )}

            {/* Can edit and status is draft -> Submit for Review */}
            {can(PERMISSIONS.contentEdit) && currentStatus === "draft" && (
              <Button
                variant="primary"
                onClick={() => void handleSubmitForReview()}
                disabled={isSubmitting}
              >
                {isSubmitting ? (
                  <Loader2 className="w-4 h-4 animate-spin mr-1" />
                ) : (
                  <Send className="w-4 h-4 mr-1.5" />
                )}
                {t("admin.submitForReview")}
              </Button>
            )}

            {/* Can review and status is in_review -> Approve or Request Changes */}
            {can(PERMISSIONS.contentReview) &&
              currentStatus === "in_review" &&
              !isRejecting && (
                <>
                  <Button
                    variant="outline"
                    className="text-amber-500 border-amber-500/30 hover:bg-amber-500/10"
                    onClick={() => setIsRejecting(true)}
                    disabled={isSubmitting}
                  >
                    <XCircle className="w-4 h-4 mr-1.5" />
                    {t("admin.requestChanges")}
                  </Button>
                  <Button
                    variant="primary"
                    className="bg-emerald-600 hover:bg-emerald-700 text-white"
                    onClick={() => void handleApprove()}
                    disabled={isSubmitting}
                  >
                    {isSubmitting ? (
                      <Loader2 className="w-4 h-4 animate-spin mr-1" />
                    ) : (
                      <CheckCircle2 className="w-4 h-4 mr-1.5" />
                    )}
                    {t("admin.approve")}
                  </Button>
                </>
              )}

            {/* Can publish and status is approved -> Publish */}
            {can(PERMISSIONS.contentPublish) &&
              currentStatus === "approved" && (
                <Button
                  variant="primary"
                  className="bg-green-600 hover:bg-green-700 text-white"
                  onClick={() => void handlePublish()}
                  disabled={isSubmitting}
                >
                  {isSubmitting ? (
                    <Loader2 className="w-4 h-4 animate-spin mr-1" />
                  ) : (
                    <Sparkles className="w-4 h-4 mr-1.5" />
                  )}
                  {t("admin.publish")}
                </Button>
              )}

            {/* Can publish and status is published -> Archive */}
            {can(PERMISSIONS.contentPublish) &&
              currentStatus === "published" && (
                <Button
                  variant="outline"
                  className="text-destructive border-destructive/30 hover:bg-destructive/10"
                  onClick={() => void handleArchive()}
                  disabled={isSubmitting}
                >
                  {isSubmitting ? (
                    <Loader2 className="w-4 h-4 animate-spin mr-1" />
                  ) : (
                    <Archive className="w-4 h-4 mr-1.5" />
                  )}
                  {t("admin.archive")}
                </Button>
              )}
          </div>
        </div>
      </div>
    </div>
  );
};
