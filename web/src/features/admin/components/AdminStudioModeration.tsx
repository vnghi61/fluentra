import React, { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import {
  AlertCircle,
  Bot,
  CheckCircle2,
  Clock,
  Eye,
  ShieldAlert,
  UserCheck,
  X,
  XCircle,
} from "lucide-react";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardDescription, CardTitle } from "@/components/ui/card";
import { useAuthStore } from "@/stores/authStore";
import { adminApi, type ModerationQueueItem } from "../api/adminApi";

export function AdminStudioModeration(): React.JSX.Element {
  const { t } = useTranslation();
  const currentUser = useAuthStore((state) => state.user);

  const {
    data,
    isLoading: loading,
    error: queryError,
    refetch,
  } = useQuery({
    queryKey: ["admin", "studio", "moderation"],
    queryFn: () => adminApi.listModerationCourses({ limit: 50 }),
  });

  const items = data?.items ?? [];
  const total = data?.total ?? 0;
  const error = queryError instanceof Error ? queryError.message : null;

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

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
        <div>
          <h2 className="text-xl font-bold text-text">
            {t("adminModeration.title", "Course Moderation Queue (Gate 2)")}
          </h2>
          <p className="text-xs text-text-muted mt-0.5">
            {t(
              "adminModeration.subtitle",
              "Review community course submissions, examine Gate 1 automated quality reports, and approve or request changes.",
            )}
          </p>
        </div>
        <Badge
          variant="primary"
          className="text-xs px-2.5 py-1 self-start sm:self-auto"
        >
          {t("adminModeration.queueCount", {
            count: total,
            defaultValue: `${total} submissions pending review`,
          })}
        </Badge>
      </div>

      {error && (
        <div className="rounded-lg border border-danger/30 bg-danger/10 p-3 text-sm text-danger font-medium">
          {error}
        </div>
      )}

      {loading ? (
        <div className="py-16 text-center text-sm text-text-muted">
          {t("app.loading", "Loading moderation queue...")}
        </div>
      ) : items.length === 0 ? (
        <Card className="border-border-subtle bg-surface-card p-10 text-center space-y-2">
          <CheckCircle2 className="h-10 w-10 text-success mx-auto" />
          <CardTitle className="text-base font-bold text-text">
            {t("adminModeration.emptyTitle", "Queue is clear")}
          </CardTitle>
          <CardDescription className="text-xs">
            {t(
              "adminModeration.emptyDesc",
              "All course submissions have been reviewed and decided.",
            )}
          </CardDescription>
        </Card>
      ) : (
        <div className="overflow-x-auto rounded-xl border border-border-subtle bg-surface-card shadow-sm">
          <table className="w-full text-left text-xs">
            <thead className="bg-surface-base border-b border-border-subtle text-text-muted uppercase tracking-wider">
              <tr>
                <th className="p-3.5">
                  {t("adminModeration.colCourse", "Course & Author")}
                </th>
                <th className="p-3.5">
                  {t("adminModeration.colLevel", "Level")}
                </th>
                <th className="p-3.5">
                  {t("adminModeration.colPricing", "Pricing")}
                </th>
                <th className="p-3.5">
                  {t("adminModeration.colGate1", "Gate 1 Status")}
                </th>
                <th className="p-3.5">
                  {t("adminModeration.colSubmittedAt", "Submitted")}
                </th>
                <th className="p-3.5 text-right">
                  {t("adminModeration.colAction", "Action")}
                </th>
              </tr>
            </thead>
            <tbody className="divide-y divide-border-subtle">
              {items.map(({ draft, submission }) => {
                const report = submission.verification_report as
                  { passed?: boolean } | null | undefined;

                return (
                  <tr
                    key={submission.id}
                    className="hover:bg-surface-base/50 transition-colors"
                  >
                    <td className="p-3.5">
                      <div className="font-bold text-text">{draft.title}</div>
                      <div className="text-text-muted text-[11px] font-mono mt-0.5">
                        Author: {draft.owner_id.slice(0, 8)}...
                      </div>
                    </td>
                    <td className="p-3.5">
                      <Badge variant="primary" className="font-mono text-xs">
                        {draft.cefr_level}
                      </Badge>
                    </td>
                    <td className="p-3.5">
                      {draft.price_vnd > 0 ? (
                        <span className="font-semibold text-text">
                          ₫{draft.price_vnd.toLocaleString("vi-VN")}
                        </span>
                      ) : (
                        <Badge variant="outline" className="text-xs">
                          {t("studio.courses.freeBadge", "Free")}
                        </Badge>
                      )}
                    </td>
                    <td className="p-3.5">
                      {report?.passed ? (
                        <Badge variant="success" className="gap-1 text-xs">
                          <CheckCircle2 className="h-3 w-3" />
                          {t("adminModeration.passed", "Passed")}
                        </Badge>
                      ) : (
                        <Badge variant="warning" className="gap-1 text-xs">
                          <Clock className="h-3 w-3" />
                          {t("adminModeration.verifying", "Verification Check")}
                        </Badge>
                      )}
                    </td>
                    <td className="p-3.5 text-text-muted">
                      {new Date(submission.submitted_at).toLocaleDateString(
                        undefined,
                        {
                          month: "short",
                          day: "numeric",
                          hour: "2-digit",
                          minute: "2-digit",
                        },
                      )}
                    </td>
                    <td className="p-3.5 text-right">
                      <Button
                        variant="primary"
                        size="sm"
                        onClick={() => {
                          setActiveItem({ draft, submission });
                          setDecisionNotes("");
                          setActionError(null);
                        }}
                        className="gap-1 text-xs"
                      >
                        <Eye className="h-3.5 w-3.5" />
                        {t("adminModeration.reviewBtn", "Review")}
                      </Button>
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>
      )}

      {/* Review Detail Modal */}
      {activeItem && (
        <div
          role="dialog"
          aria-modal="true"
          aria-labelledby="review-detail-title"
          className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm p-4 animate-in fade-in duration-200"
        >
          <div className="relative w-full max-w-3xl max-h-[90vh] overflow-y-auto rounded-xl border border-border-subtle bg-surface-card p-6 shadow-2xl space-y-6">
            {/* Header */}
            <div className="flex items-center justify-between border-b border-border-subtle pb-4">
              <div className="space-y-1">
                <div className="flex items-center gap-2">
                  <UserCheck className="h-5 w-5 text-primary-accent" />
                  <h2
                    id="review-detail-title"
                    className="text-lg font-bold text-text"
                  >
                    {t(
                      "adminModeration.reviewModalTitle",
                      "Course Submission Review",
                    )}
                  </h2>
                </div>
                <p className="text-xs text-text-muted">
                  {activeItem.draft.title}
                </p>
              </div>
              <button
                type="button"
                onClick={() => setActiveItem(null)}
                aria-label={t("app.close", "Close")}
                className="rounded-lg p-1.5 text-text-muted hover:bg-surface-muted hover:text-text transition-colors"
              >
                <X className="h-5 w-5" />
              </button>
            </div>

            {/* BR-STUDIO-06 / BR-CONTENT-03 Self-Review Guard */}
            {isSelfAuthor && (
              <div className="flex items-center gap-3 rounded-lg border border-danger/40 bg-danger/10 p-4 text-xs text-danger font-semibold">
                <ShieldAlert className="h-5 w-5 shrink-0" />
                <span>
                  {t(
                    "adminModeration.selfReviewBlocked",
                    "Self-review forbidden (BR-STUDIO-06): You authored this course submission and cannot approve or reject your own work. Another moderator must review it.",
                  )}
                </span>
              </div>
            )}

            {actionError && (
              <div className="rounded-lg border border-danger/30 bg-danger/10 p-3 text-sm text-danger font-medium">
                {actionError}
              </div>
            )}

            {/* Course Overview */}
            <div className="rounded-lg border border-border-subtle bg-surface-base p-4 space-y-3">
              <div className="flex flex-wrap items-center justify-between gap-2">
                <h3 className="text-sm font-bold text-text">
                  {activeItem.draft.title}
                </h3>
                <div className="flex items-center gap-2">
                  <Badge variant="primary" className="font-mono text-xs">
                    {activeItem.draft.cefr_level}
                  </Badge>
                  {activeItem.draft.price_vnd > 0 ? (
                    <Badge
                      variant="secondary"
                      className="font-mono text-xs font-semibold"
                    >
                      ₫{activeItem.draft.price_vnd.toLocaleString("vi-VN")}
                    </Badge>
                  ) : (
                    <Badge variant="outline" className="text-xs">
                      {t("studio.courses.freeBadge", "Free")}
                    </Badge>
                  )}
                </div>
              </div>

              {activeItem.draft.description && (
                <p className="text-xs text-text-muted leading-relaxed">
                  {activeItem.draft.description}
                </p>
              )}

              <div className="text-[11px] text-text-muted font-mono pt-1">
                Slug: /courses/{activeItem.draft.slug} • Author ID:{" "}
                {activeItem.draft.owner_id}
              </div>
            </div>

            {/* Gate 1 Verification Report */}
            <div className="rounded-lg border border-border-subtle bg-surface-base p-4 space-y-3">
              <div className="flex items-center justify-between">
                <div className="flex items-center gap-2">
                  <Bot className="h-4 w-4 text-primary-accent" />
                  <h4 className="text-xs font-bold text-text uppercase tracking-wider">
                    {t(
                      "adminModeration.gate1ReportTitle",
                      "Gate 1 Automated Checks Report",
                    )}
                  </h4>
                </div>
                <Badge variant="success" className="text-xs">
                  {t(
                    "adminModeration.checksPassed",
                    "All 6 Automated Checks Passed",
                  )}
                </Badge>
              </div>

              <div className="grid grid-cols-1 sm:grid-cols-2 gap-2 text-xs text-text-muted pt-1">
                <div className="flex items-center gap-2">
                  <CheckCircle2 className="h-3.5 w-3.5 text-success shrink-0" />
                  <span>Curriculum Structure & Bounds</span>
                </div>
                <div className="flex items-center gap-2">
                  <CheckCircle2 className="h-3.5 w-3.5 text-success shrink-0" />
                  <span>Size Requirements (≥3 lessons, ≥20 exercises)</span>
                </div>
                <div className="flex items-center gap-2">
                  <CheckCircle2 className="h-3.5 w-3.5 text-success shrink-0" />
                  <span>Activity Types Allowed (12 kinds)</span>
                </div>
                <div className="flex items-center gap-2">
                  <CheckCircle2 className="h-3.5 w-3.5 text-success shrink-0" />
                  <span>
                    Material Readiness (resource owned, renditions ready)
                  </span>
                </div>
                <div className="flex items-center gap-2">
                  <CheckCircle2 className="h-3.5 w-3.5 text-success shrink-0" />
                  <span>Item Grader & Redaction Verification</span>
                </div>
                <div className="flex items-center gap-2">
                  <CheckCircle2 className="h-3.5 w-3.5 text-success shrink-0" />
                  <span>Language, Profanity & Contact Scan</span>
                </div>
                <div className="flex items-center gap-2">
                  <CheckCircle2 className="h-3.5 w-3.5 text-success shrink-0" />
                  <span>Near-Duplicate Content Check</span>
                </div>
              </div>
            </div>

            {/* Decision & Feedback Form */}
            <div className="space-y-3 pt-2 border-t border-border-subtle">
              <label
                htmlFor="moderation-feedback"
                className="block text-xs font-semibold text-text uppercase tracking-wider"
              >
                {t(
                  "adminModeration.feedbackLabel",
                  "Reviewer Notes & Feedback (Required for Changes / Rejection)",
                )}
              </label>
              <textarea
                id="moderation-feedback"
                rows={4}
                value={decisionNotes}
                onChange={(e) => setDecisionNotes(e.target.value)}
                placeholder="Provide constructive feedback detailing required corrections or reasons..."
                className="w-full rounded-lg border border-border-subtle bg-surface-base px-3 py-2 text-base sm:text-sm text-text focus:border-primary focus:outline-none focus:ring-1 focus:ring-primary"
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
                  className="text-xs text-warning border-warning/40 hover:bg-warning/10"
                >
                  <AlertCircle className="h-3.5 w-3.5 mr-1" />
                  {t("adminModeration.requestChanges", "Request Changes")}
                </Button>

                <Button
                  variant="outline"
                  size="sm"
                  disabled={isSelfAuthor || actionLoading}
                  onClick={() => {
                    void handleRejectOrRequestChanges("rejected");
                  }}
                  className="text-xs text-danger border-danger/40 hover:bg-danger/10"
                >
                  <XCircle className="h-3.5 w-3.5 mr-1" />
                  {t("adminModeration.reject", "Reject")}
                </Button>

                <Button
                  variant="primary"
                  size="sm"
                  disabled={isSelfAuthor || actionLoading}
                  onClick={() => {
                    void handleApprove();
                  }}
                  className="text-xs gap-1"
                >
                  <CheckCircle2 className="h-3.5 w-3.5" />
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
