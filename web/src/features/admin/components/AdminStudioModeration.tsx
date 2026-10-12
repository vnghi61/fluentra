import React, { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import {
  AlertCircle,
  BookOpen,
  Bot,
  CheckCircle2,
  Clock,
  DollarSign,
  Eye,
  FileCheck,
  ShieldAlert,
  UserCheck,
  X,
  XCircle,
} from "lucide-react";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { EmptyState } from "@/components/ui/empty-state";
import { ErrorState } from "@/components/ui/error-state";
import { cn } from "@/lib/utils";
import { useAuthStore } from "@/stores/authStore";
import { adminApi, type ModerationQueueItem } from "../api/adminApi";
import { formatVnd } from "@/lib/currency";
import { CopyableId } from "./CopyableId";
import { StatusBadge } from "./StatusBadge";
import {
  AdminDataPanel,
  AdminListSkeleton,
  AdminRefreshButton,
  AdminRowIcon,
  AdminStatGrid,
  AdminTag,
  adminStatTones,
  adminTable,
  type AdminStat,
} from "./AdminSurface";
import { Textarea } from "@/components/ui/textarea";

export function AdminStudioModeration(): React.JSX.Element {
  const { t, i18n } = useTranslation();
  const currentUser = useAuthStore((state) => state.user);

  const {
    data,
    isLoading: loading,
    isFetching,
    error: queryError,
    refetch,
  } = useQuery({
    queryKey: ["admin", "studio", "moderation"],
    queryFn: () => adminApi.listModerationCourses({ limit: 50 }),
  });

  const items = data?.items ?? [];
  const total = data?.total ?? 0;
  const error = queryError
    ? queryError instanceof Error
      ? queryError.message
      : t("adminModeration.errLoadFailed")
    : null;

  const [activeItem, setActiveItem] = useState<ModerationQueueItem | null>(
    null,
  );
  const [decisionNotes, setDecisionNotes] = useState("");
  const [actionLoading, setActionLoading] = useState(false);
  const [actionError, setActionError] = useState<string | null>(null);

  const isSelfAuthor =
    Boolean(currentUser?.userId) &&
    activeItem?.submission.submitted_by === currentUser?.userId;

  const handleApprove = async () => {
    if (!activeItem) return;
    setActionLoading(true);
    setActionError(null);
    try {
      await adminApi.approveCourseSubmission(activeItem.submission.id);
      setActiveItem(null);
      void refetch();
    } catch (err) {
      setActionError(
        err instanceof Error
          ? err.message
          : t("adminModeration.errApproveFailed", "Failed to approve course"),
      );
    } finally {
      setActionLoading(false);
    }
  };

  const handleRejectOrRequestChanges = async (
    status: "rejected" | "changes_requested",
  ) => {
    if (!activeItem) return;
    setActionError(null);

    if (!decisionNotes.trim()) {
      setActionError(
        t(
          "adminModeration.errNotesRequired",
          "Feedback notes are required when requesting changes or rejecting a course.",
        ),
      );
      return;
    }

    setActionLoading(true);
    try {
      await adminApi.rejectCourseSubmission(activeItem.submission.id, {
        status,
        feedback: decisionNotes.trim(),
      });
      setActiveItem(null);
      setDecisionNotes("");
      void refetch();
    } catch (err) {
      setActionError(
        err instanceof Error
          ? err.message
          : t(
              "adminModeration.errDecisionFailed",
              "Failed to record review decision",
            ),
      );
    } finally {
      setActionLoading(false);
    }
  };

  const openReview = (item: ModerationQueueItem) => {
    setActiveItem(item);
    setDecisionNotes("");
    setActionError(null);
  };
  const submittedAt = (date: string) => (
    <time
      dateTime={date}
      className="whitespace-nowrap text-sm tabular-nums text-text-muted"
    >
      {new Date(date).toLocaleString(i18n.resolvedLanguage, {
        day: "2-digit",
        month: "short",
        year: "numeric",
        hour: "2-digit",
        minute: "2-digit",
      })}
    </time>
  );
  const gateBadge = (passed: boolean | undefined) =>
    passed ? (
      <StatusBadge tone="success" icon={CheckCircle2}>
        {t("adminModeration.passed", "Passed")}
      </StatusBadge>
    ) : (
      <StatusBadge tone="warning" icon={Clock}>
        {t("adminModeration.verifying", "Verification Check")}
      </StatusBadge>
    );
  const price = (amount: number) =>
    amount > 0 ? (
      <span className="font-semibold tabular-nums text-text">
        {formatVnd(amount)}
      </span>
    ) : (
      <AdminTag>{t("studio.courses.freeBadge", "Free")}</AdminTag>
    );
  const reviewButton = (item: ModerationQueueItem, className?: string) => (
    <Button
      type="button"
      size="sm"
      onClick={() => openReview(item)}
      className={className}
    >
      <Eye className="h-4 w-4" aria-hidden="true" />
      {t("adminModeration.reviewBtn", "Review")}
    </Button>
  );
  const passedCount = items.filter(
    ({ submission }) => submission.verification_report?.passed,
  ).length;
  const stats: AdminStat[] = [
    {
      key: "total",
      label: t("adminModeration.statTotal"),
      value: total.toLocaleString(i18n.resolvedLanguage),
      Icon: FileCheck,
      tone: adminStatTones.primary,
    },
    {
      key: "passed",
      label: t("adminModeration.passed", "Passed"),
      value: passedCount.toLocaleString(i18n.resolvedLanguage),
      Icon: CheckCircle2,
      tone: adminStatTones.success,
    },
    {
      key: "checking",
      label: t("adminModeration.verifying", "Verification Check"),
      value: (items.length - passedCount).toLocaleString(i18n.resolvedLanguage),
      Icon: Clock,
      tone: adminStatTones.warning,
    },
    {
      key: "paid",
      label: t("adminModeration.statPaid"),
      value: items
        .filter(({ draft }) => draft.price_vnd > 0)
        .length.toLocaleString(i18n.resolvedLanguage),
      Icon: DollarSign,
      tone: adminStatTones.neutral,
    },
  ];

  return (
    <div className="space-y-4">
      <div className="space-y-2">
        <AdminStatGrid
          stats={stats}
          loading={loading}
          label={t("adminModeration.summary")}
        />
        {!loading && total > items.length && (
          <p className="text-sm text-text-muted">
            {t("adminModeration.statScope", { count: items.length })}
          </p>
        )}
      </div>

      <AdminDataPanel>
        <div className="flex items-center justify-between gap-3 border-b border-border-subtle px-4 py-3">
          <p className="text-sm text-text-muted">
            {t("adminModeration.queueCount", {
              count: total,
              defaultValue: `${total} submissions pending review`,
            })}
          </p>
          <AdminRefreshButton
            onClick={() => void refetch()}
            busy={isFetching}
          />
        </div>
        {loading ? (
          <AdminListSkeleton
            label={t("app.loading", "Loading moderation queue...")}
          />
        ) : error ? (
          <ErrorState
            description={error}
            onRetry={() => void refetch()}
            className="py-10"
          />
        ) : items.length === 0 ? (
          <EmptyState
            icon={CheckCircle2}
            title={t("adminModeration.emptyTitle", "Queue is clear")}
            description={t(
              "adminModeration.emptyDesc",
              "All course submissions have been reviewed and decided.",
            )}
            className="py-10"
          />
        ) : (
          <>
            <div className="hidden overflow-x-auto md:block">
              <table className={cn(adminTable.table, "min-w-[900px]")}>
                <caption className="sr-only">
                  {t("adminModeration.title", "Course Moderation Queue")}
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
                      {t("adminModeration.colCourse", "Course & Author")}
                    </th>
                    <th scope="col" className={cn(adminTable.th, "w-20")}>
                      {t("adminModeration.colLevel", "Level")}
                    </th>
                    <th scope="col" className={cn(adminTable.th, "w-36")}>
                      {t("adminModeration.colPricing", "Pricing")}
                    </th>
                    <th scope="col" className={cn(adminTable.th, "w-48")}>
                      {t("adminModeration.colGate1", "Gate 1 Status")}
                    </th>
                    <th scope="col" className={cn(adminTable.th, "w-44")}>
                      {t("adminModeration.colSubmittedAt", "Submitted")}
                    </th>
                    <th
                      scope="col"
                      className={cn(adminTable.th, "w-32 text-right")}
                    >
                      {t("adminModeration.colAction", "Action")}
                    </th>
                  </tr>
                </thead>
                <tbody className={adminTable.body}>
                  {items.map((item, index) => (
                    <tr
                      key={item.submission.id}
                      className={cn(adminTable.row, "h-16")}
                    >
                      <td
                        className={cn(
                          adminTable.td,
                          "text-right tabular-nums text-text-muted",
                        )}
                      >
                        {index + 1}
                      </td>
                      <td className={adminTable.td}>
                        <div className="flex min-w-0 items-center gap-3">
                          <AdminRowIcon icon={BookOpen} />
                          <div className="min-w-0">
                            <p
                              title={item.draft.title}
                              className="truncate text-sm font-semibold text-text"
                            >
                              {item.draft.title}
                            </p>
                            <CopyableId id={item.draft.owner_id} />
                          </div>
                        </div>
                      </td>
                      <td className={adminTable.td}>
                        <AdminTag>{item.draft.cefr_level}</AdminTag>
                      </td>
                      <td className={adminTable.td}>
                        {price(item.draft.price_vnd)}
                      </td>
                      <td className={adminTable.td}>
                        {gateBadge(item.submission.verification_report?.passed)}
                      </td>
                      <td className={adminTable.td}>
                        {submittedAt(item.submission.submitted_at)}
                      </td>
                      <td className={cn(adminTable.td, "text-right")}>
                        {reviewButton(item)}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
            <ul className="divide-y divide-border-subtle md:hidden">
              {items.map((item) => {
                const report = item.submission.verification_report;
                return (
                  <li key={item.submission.id} className="space-y-3 p-4">
                    <div className="flex items-start gap-3">
                      <AdminRowIcon icon={BookOpen} className="h-10 w-10" />
                      <div className="min-w-0 flex-1">
                        <p className="break-words text-sm font-semibold text-text">
                          {item.draft.title}
                        </p>
                        <CopyableId id={item.draft.owner_id} />
                      </div>
                    </div>
                    <div className="flex flex-wrap items-center gap-2">
                      <AdminTag>{item.draft.cefr_level}</AdminTag>
                      {price(item.draft.price_vnd)}
                      {gateBadge(report?.passed)}
                    </div>
                    <div className="flex flex-wrap items-center justify-between gap-2 text-sm text-text-muted">
                      <span>
                        {report?.items_checked ?? 0}{" "}
                        {t("adminModeration.checksPassed", "checks")} ·{" "}
                        {(report?.failures ?? []).length}{" "}
                        {t("adminModeration.flags", "flags")}
                      </span>
                      {submittedAt(item.submission.submitted_at)}
                    </div>
                    {reviewButton(item, "w-full")}
                  </li>
                );
              })}
            </ul>
          </>
        )}
      </AdminDataPanel>

      {/* Review Detail Modal */}
      {activeItem && (
        <div
          role="dialog"
          aria-modal="true"
          aria-labelledby="review-detail-title"
          className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm p-4 animate-in fade-in duration-200"
        >
          <div className="relative max-h-[90dvh] w-full max-w-3xl space-y-6 overflow-y-auto rounded-xl border border-border-subtle bg-surface-card p-4 shadow-2xl sm:p-6">
            {/* Header */}
            <div className="flex items-center justify-between gap-2 border-b border-border-subtle pb-4">
              <div className="min-w-0 space-y-1">
                <div className="flex min-w-0 items-center gap-2">
                  <UserCheck
                    aria-hidden="true"
                    className="h-5 w-5 shrink-0 text-primary-accent"
                  />
                  <h2
                    id="review-detail-title"
                    className="min-w-0 break-words text-lg font-semibold leading-tight text-text"
                  >
                    {t(
                      "adminModeration.reviewModalTitle",
                      "Course Submission Review",
                    )}
                  </h2>
                </div>
                <p className="break-words text-sm text-text-muted">
                  {activeItem.draft.title}
                </p>
              </div>
              <button
                type="button"
                onClick={() => setActiveItem(null)}
                aria-label={t("app.close", "Close")}
                className="flex min-h-11 min-w-11 shrink-0 items-center justify-center rounded-lg text-text-muted transition-colors hover:bg-surface-muted hover:text-text"
              >
                <X aria-hidden="true" className="h-5 w-5" />
              </button>
            </div>

            {/* BR-STUDIO-06 / BR-CONTENT-03 Self-Review Guard */}
            {isSelfAuthor && (
              <div className="flex items-start gap-3 rounded-lg border border-danger/40 bg-danger/10 p-4 text-sm font-semibold leading-relaxed text-danger-accent">
                <ShieldAlert aria-hidden="true" className="h-5 w-5 shrink-0" />
                <span>
                  {t(
                    "adminModeration.selfReviewBlocked",
                    "Self-review forbidden (BR-STUDIO-06): You authored this course submission and cannot approve or reject your own work. Another moderator must review it.",
                  )}
                </span>
              </div>
            )}

            {actionError && (
              <div className="rounded-lg border border-danger/30 bg-danger/10 p-3 text-sm font-medium text-danger-accent">
                {actionError}
              </div>
            )}

            {/* Course Overview */}
            <div className="rounded-lg border border-border-subtle bg-surface-muted p-4 space-y-3">
              <div className="flex flex-wrap items-center justify-between gap-2">
                <h3 className="min-w-0 break-words text-base font-semibold leading-snug text-text">
                  {activeItem.draft.title}
                </h3>
                <div className="flex flex-wrap items-center gap-2">
                  <Badge variant="primary" className="font-mono text-xs">
                    {activeItem.draft.cefr_level}
                  </Badge>
                  {activeItem.draft.price_vnd > 0 ? (
                    <Badge
                      variant="secondary"
                      className="font-mono text-xs font-semibold"
                    >
                      {formatVnd(activeItem.draft.price_vnd)}
                    </Badge>
                  ) : (
                    <Badge variant="outline" className="text-xs">
                      {t("studio.courses.freeBadge", "Free")}
                    </Badge>
                  )}
                </div>
              </div>

              {activeItem.draft.description && (
                <p className="break-words text-base leading-relaxed text-text-muted md:text-sm">
                  {activeItem.draft.description}
                </p>
              )}

              <div className="break-all pt-1 font-mono text-xs text-text-muted">
                Slug: /courses/{activeItem.draft.slug} • Author ID:{" "}
                {activeItem.draft.owner_id}
              </div>
            </div>

            {/* Gate 1 Verification Report */}
            <div className="rounded-lg border border-border-subtle bg-surface-muted p-4 space-y-3">
              <div className="flex flex-wrap items-center justify-between gap-2">
                <div className="flex min-w-0 items-center gap-2">
                  <Bot
                    aria-hidden="true"
                    className="h-4 w-4 shrink-0 text-primary-accent"
                  />
                  <h3 className="text-xs font-semibold uppercase tracking-wider text-text">
                    {t(
                      "adminModeration.gate1ReportTitle",
                      "Gate 1 Automated Checks Report",
                    )}
                  </h3>
                </div>
                <Badge
                  variant={
                    activeItem.submission.verification_report?.passed
                      ? "success"
                      : "warning"
                  }
                  className="text-xs"
                >
                  {activeItem.submission.verification_report?.items_checked ??
                    0}{" "}
                  {t("adminModeration.checksPassed", "checks")}
                </Badge>
              </div>

              {activeItem.submission.verification_report?.failures?.length ? (
                <ul className="space-y-2 text-sm text-danger-accent">
                  {activeItem.submission.verification_report.failures.map(
                    (failure, index) => (
                      <li key={`${failure.check}-${index}`}>
                        <strong>{failure.check}:</strong> {failure.message}
                      </li>
                    ),
                  )}
                </ul>
              ) : (
                <p className="text-sm text-text-muted">
                  {t(
                    "adminModeration.noGate1Flags",
                    "No Gate 1 failures reported.",
                  )}
                </p>
              )}
              <div className="hidden">
                <div className="flex items-center gap-2">
                  <CheckCircle2
                    aria-hidden="true"
                    className="h-4 w-4 shrink-0 text-success"
                  />
                  <span>
                    {t(
                      "adminModeration.checkCurriculumStructure",
                      "Curriculum Structure & Bounds",
                    )}
                  </span>
                </div>
                <div className="flex items-center gap-2">
                  <CheckCircle2
                    aria-hidden="true"
                    className="h-4 w-4 shrink-0 text-success"
                  />
                  <span>
                    {t(
                      "adminModeration.checkSizeRequirements",
                      "Size Requirements (≥3 lessons, ≥20 exercises)",
                    )}
                  </span>
                </div>
                <div className="flex items-center gap-2">
                  <CheckCircle2
                    aria-hidden="true"
                    className="h-4 w-4 shrink-0 text-success"
                  />
                  <span>
                    {t(
                      "adminModeration.checkActivityTypes",
                      "Activity Types Allowed (12 kinds)",
                    )}
                  </span>
                </div>
                <div className="flex items-center gap-2">
                  <CheckCircle2
                    aria-hidden="true"
                    className="h-4 w-4 shrink-0 text-success"
                  />
                  <span>
                    {t(
                      "adminModeration.checkMaterialReadiness",
                      "Material Readiness (resource owned, renditions ready)",
                    )}
                  </span>
                </div>
                <div className="flex items-center gap-2">
                  <CheckCircle2
                    aria-hidden="true"
                    className="h-4 w-4 shrink-0 text-success"
                  />
                  <span>
                    {t(
                      "adminModeration.checkItemGrader",
                      "Item Grader & Redaction Verification",
                    )}
                  </span>
                </div>
                <div className="flex items-center gap-2">
                  <CheckCircle2
                    aria-hidden="true"
                    className="h-4 w-4 shrink-0 text-success"
                  />
                  <span>
                    {t(
                      "adminModeration.checkLanguageScan",
                      "Language, Profanity & Contact Scan",
                    )}
                  </span>
                </div>
                <div className="flex items-center gap-2">
                  <CheckCircle2
                    aria-hidden="true"
                    className="h-4 w-4 shrink-0 text-success"
                  />
                  <span>
                    {t(
                      "adminModeration.checkDuplicateContent",
                      "Near-Duplicate Content Check",
                    )}
                  </span>
                </div>
              </div>
            </div>

            {/* Decision & Feedback Form */}
            <div className="space-y-3 pt-2 border-t border-border-subtle">
              <label
                htmlFor="moderation-feedback"
                className="block text-sm font-medium text-text"
              >
                {t(
                  "adminModeration.feedbackLabel",
                  "Reviewer Notes & Feedback (Required for Changes / Rejection)",
                )}
              </label>
              <Textarea
                id="moderation-feedback"
                rows={4}
                value={decisionNotes}
                onChange={(e) => setDecisionNotes(e.target.value)}
                placeholder={t(
                  "adminModeration.feedbackPlaceholder",
                  "Provide constructive feedback detailing required corrections or reasons...",
                )}
              />
            </div>

            {/* Actions */}
            <div className="flex flex-wrap items-center justify-between gap-3 pt-3 border-t border-border-subtle">
              <Button
                variant="outline"
                size="sm"
                onClick={() => setActiveItem(null)}
                disabled={actionLoading}
              >
                {t("app.cancel", "Cancel")}
              </Button>

              <div className="flex flex-wrap items-center gap-2">
                <Button
                  variant="outline"
                  size="sm"
                  disabled={isSelfAuthor || actionLoading}
                  onClick={() => {
                    void handleRejectOrRequestChanges("changes_requested");
                  }}
                  className="border-warning/40 text-warning-accent hover:bg-warning/10"
                >
                  <AlertCircle aria-hidden="true" className="h-4 w-4" />
                  {t("adminModeration.requestChanges", "Request Changes")}
                </Button>

                <Button
                  variant="outline"
                  size="sm"
                  disabled={isSelfAuthor || actionLoading}
                  onClick={() => {
                    void handleRejectOrRequestChanges("rejected");
                  }}
                  className="border-danger/40 text-danger-accent hover:bg-danger/10"
                >
                  <XCircle aria-hidden="true" className="h-4 w-4" />
                  {t("adminModeration.reject", "Reject")}
                </Button>

                <Button
                  variant="primary"
                  size="sm"
                  disabled={isSelfAuthor || actionLoading}
                  onClick={() => {
                    void handleApprove();
                  }}
                >
                  <CheckCircle2 aria-hidden="true" className="h-4 w-4" />
                  {actionLoading
                    ? t("adminModeration.publishing", "Publishing...")
                    : t(
                        "adminModeration.approveAndPublish",
                        "Approve & Publish",
                      )}
                </Button>
              </div>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
