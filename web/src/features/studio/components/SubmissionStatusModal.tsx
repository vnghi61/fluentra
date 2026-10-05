import React from "react";
import { useTranslation } from "react-i18next";
import {
  AlertCircle,
  Bot,
  CheckCircle2,
  Clock,
  FileCheck,
  UserCheck,
  X,
  XCircle,
} from "lucide-react";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { useDraftSubmissions } from "../hooks/useStudio";

interface SubmissionStatusModalProps {
  draftId: string;
  draftTitle: string;
  isOpen: boolean;
  onClose: () => void;
}

export function SubmissionStatusModal({
  draftId,
  draftTitle,
  isOpen,
  onClose,
}: SubmissionStatusModalProps): React.JSX.Element | null {
  const { t } = useTranslation();
  const {
    data: submissions,
    isLoading,
    isError,
    refetch,
  } = useDraftSubmissions(isOpen ? draftId : undefined);

  if (!isOpen) return null;

  const latestSubmission = submissions?.[0];

  const getStatusBadge = (status?: string) => {
    switch (status) {
      case "approved":
        return (
          <Badge variant="success" className="gap-1.5 py-1 px-2.5">
            <CheckCircle2 className="h-3.5 w-3.5" />
            {t("studio.status.approved", "Approved & Published")}
          </Badge>
        );
      case "in_review":
        return (
          <Badge variant="primary" className="gap-1.5 py-1 px-2.5">
            <Clock className="h-3.5 w-3.5" />
            {t("studio.status.inReview", "In Human Review (Gate 2)")}
          </Badge>
        );
      case "verifying":
        return (
          <Badge
            variant="warning"
            className="gap-1.5 py-1 px-2.5 animate-pulse"
          >
            <Bot className="h-3.5 w-3.5" />
            {t("studio.status.verifying", "Running Gate 1 Checks...")}
          </Badge>
        );
      case "changes_requested":
        return (
          <Badge variant="warning" className="gap-1.5 py-1 px-2.5">
            <AlertCircle className="h-3.5 w-3.5" />
            {t("studio.status.changesRequested", "Changes Requested")}
          </Badge>
        );
      case "rejected":
        return (
          <Badge variant="danger" className="gap-1.5 py-1 px-2.5">
            <XCircle className="h-3.5 w-3.5" />
            {t("studio.status.rejected", "Rejected")}
          </Badge>
        );
      default:
        return (
          <Badge variant="secondary" className="gap-1.5 py-1 px-2.5">
            {status ?? t("studio.status.submitted", "Submitted")}
          </Badge>
        );
    }
  };

  const gate1Report = latestSubmission?.verification_report;

  return (
    <div
      role="dialog"
      aria-modal="true"
      aria-labelledby="submission-modal-title"
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm p-4 animate-in fade-in duration-200"
    >
      <div className="relative w-full max-w-2xl max-h-[90vh] overflow-y-auto rounded-xl border border-border-subtle bg-surface-card p-6 shadow-2xl space-y-6">
        {/* Header */}
        <div className="flex items-center justify-between border-b border-border-subtle pb-4">
          <div className="space-y-1">
            <div className="flex items-center gap-2">
              <FileCheck className="h-5 w-5 text-primary-accent" />
              <h2
                id="submission-modal-title"
                className="text-lg font-bold text-text"
              >
                {t("studio.submission.title", "Course Submission Status")}
              </h2>
            </div>
            <p className="text-xs text-text-muted">{draftTitle}</p>
          </div>
          <button
            type="button"
            onClick={onClose}
            aria-label={t("app.close", "Close")}
            className="min-h-[44px] min-w-[44px] flex items-center justify-center rounded-lg p-1.5 text-text-muted hover:bg-surface-muted hover:text-text transition-colors"
          >
            <X className="h-5 w-5" />
          </button>
        </div>

        {isLoading ? (
          <div className="py-12 text-center text-sm text-text-muted">
            {t("app.loading", "Loading submission status...")}
          </div>
        ) : isError ? (
          <div role="alert" className="space-y-3 text-sm text-danger-accent">
            <p>
              {t(
                "studio.submission.loadError",
                "Could not load submission status.",
              )}
            </p>
            <Button variant="outline" onClick={() => void refetch()}>
              {t("common.retry", "Retry")}
            </Button>
          </div>
        ) : !latestSubmission ? (
          <div className="py-8 text-center text-sm text-text-muted">
            {t(
              "studio.submission.noSubmissions",
              "This draft has not been submitted for review yet.",
            )}
          </div>
        ) : (
          <div className="space-y-5">
            {/* Status overview */}
            <div className="flex flex-wrap items-center justify-between gap-3 rounded-lg border border-border-subtle bg-surface-base p-4">
              <div>
                <span className="block text-xs text-text-muted mb-1">
                  {t("studio.submission.currentStatus", "Current Status")}
                </span>
                <div>{getStatusBadge(latestSubmission.status)}</div>
              </div>
              <div className="text-right text-xs text-text-muted">
                <div>
                  {t("studio.submission.submittedAt", "Submitted")}:{" "}
                  <span className="font-medium text-text">
                    {new Date(latestSubmission.submitted_at).toLocaleDateString(
                      undefined,
                      {
                        year: "numeric",
                        month: "short",
                        day: "numeric",
                        hour: "2-digit",
                        minute: "2-digit",
                      },
                    )}
                  </span>
                </div>
                {latestSubmission.reviewed_at && (
                  <div className="mt-0.5">
                    {t("studio.submission.reviewedAt", "Reviewed")}:{" "}
                    <span className="font-medium text-text">
                      {new Date(
                        latestSubmission.reviewed_at,
                      ).toLocaleDateString(undefined, {
                        year: "numeric",
                        month: "short",
                        day: "numeric",
                        hour: "2-digit",
                        minute: "2-digit",
                      })}
                    </span>
                  </div>
                )}
              </div>
            </div>

            {/* Gate 1 Automated Verification */}
            <div className="rounded-lg border border-border-subtle bg-surface-base p-4 space-y-3">
              <div className="flex items-center justify-between">
                <div className="flex items-center gap-2">
                  <Bot className="h-4 w-4 text-primary-accent" />
                  <h3 className="text-sm font-bold text-text">
                    {t(
                      "studio.submission.gate1Title",
                      "Gate 1: Automated Verification",
                    )}
                  </h3>
                </div>
                {gate1Report ? (
                  <Badge
                    variant={gate1Report.passed ? "success" : "danger"}
                    className="text-xs"
                  >
                    {gate1Report.passed
                      ? t("studio.submission.passed", "Passed")
                      : t("studio.submission.issuesFound", "Issues Found")}
                  </Badge>
                ) : (
                  <Badge variant="secondary" className="text-xs">
                    {latestSubmission.status === "verifying"
                      ? t("studio.submission.inProgress", "In Progress")
                      : t("studio.submission.pending", "Pending")}
                  </Badge>
                )}
              </div>

              {gate1Report && (
                <p className="text-xs text-text-muted">
                  {t("studio.submission.itemsChecked", {
                    count: gate1Report.items_checked,
                    defaultValue: `${gate1Report.items_checked} activities checked`,
                  })}
                </p>
              )}

              {gate1Report?.failures && gate1Report.failures.length > 0 && (
                <ul className="space-y-2 pt-2 border-t border-border-subtle">
                  {gate1Report.failures.map((check, idx) => (
                    <li
                      key={idx}
                      className="flex items-start gap-2.5 text-xs rounded-md bg-surface-card p-2 border border-border-subtle"
                    >
                      <XCircle className="h-4 w-4 shrink-0 text-danger mt-0.5" />
                      <div className="flex-1 space-y-0.5">
                        <div className="font-semibold text-text">
                          {check.check}
                        </div>
                        <div className="text-text-muted">{check.message}</div>
                      </div>
                    </li>
                  ))}
                </ul>
              )}
            </div>

            {/* Gate 2 Human Moderation & Reviewer Feedback */}
            <div className="rounded-lg border border-border-subtle bg-surface-base p-4 space-y-3">
              <div className="flex items-center gap-2">
                <UserCheck className="h-4 w-4 text-primary-accent" />
                <h3 className="text-sm font-bold text-text">
                  {t(
                    "studio.submission.gate2Title",
                    "Gate 2: Moderator Review",
                  )}
                </h3>
              </div>

              {latestSubmission.feedback ? (
                <div className="rounded-lg border border-warning/30 bg-warning/10 p-3 text-xs space-y-1">
                  <span className="font-bold text-text uppercase tracking-wider text-[11px] block">
                    {t(
                      "studio.submission.reviewerFeedback",
                      "Reviewer Feedback & Notes",
                    )}
                  </span>
                  <p className="text-text whitespace-pre-wrap leading-relaxed">
                    {latestSubmission.feedback}
                  </p>
                </div>
              ) : latestSubmission.status === "in_review" ? (
                <p className="text-xs text-text-muted">
                  {t(
                    "studio.submission.inReviewNotice",
                    "A moderator is reviewing your curriculum outline, activity quality, and safety checks.",
                  )}
                </p>
              ) : latestSubmission.status === "approved" ? (
                <p className="text-xs text-success font-medium">
                  {t(
                    "studio.submission.approvedNotice",
                    "Your course has been approved and published to the public catalogue!",
                  )}
                </p>
              ) : (
                <p className="text-xs text-text-muted">
                  {t(
                    "studio.submission.gate2Pending",
                    "Human moderation will begin once Gate 1 automated checks have passed.",
                  )}
                </p>
              )}
            </div>
          </div>
        )}

        <div className="flex justify-end pt-3 border-t border-border-subtle">
          <Button variant="outline" onClick={onClose}>
            {t("app.close", "Close")}
          </Button>
        </div>
      </div>
    </div>
  );
}
