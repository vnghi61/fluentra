import React, { useState } from "react";
import { AlertTriangle, CheckCircle2, Flag, X } from "lucide-react";
import { useTranslation } from "react-i18next";

import { Button } from "@/components/ui/button";
import {
  Card,
  CardDescription,
  CardFooter,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import {
  learningApi,
  type ItemReportReason,
} from "@/features/learning/api/learningApi";

export interface ReportDialogProps {
  isOpen: boolean;
  contentVersionId: string | null;
  initialNote?: string | undefined;
  onClose: () => void;
}

const REPORT_REASONS: ItemReportReason[] = [
  "wrong_answer",
  "unclear",
  "typo",
  "my_answer_was_right",
  "other",
];

export const ReportDialog: React.FC<ReportDialogProps> = ({
  isOpen,
  contentVersionId,
  initialNote = "",
  onClose,
}) => {
  const { t } = useTranslation();
  const [reason, setReason] = useState<ItemReportReason>("wrong_answer");
  const [note, setNote] = useState(initialNote);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [isSubmitted, setIsSubmitted] = useState(false);
  const [error, setError] = useState<string | null>(null);

  if (!isOpen) return null;

  const handleClose = () => {
    setIsSubmitted(false);
    setError(null);
    setNote(initialNote);
    onClose();
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!contentVersionId) return;

    setIsSubmitting(true);
    setError(null);

    try {
      await learningApi.reportContentVersion(contentVersionId, {
        reason,
        note: note.trim() ? note.trim() : null,
      });
      setIsSubmitted(true);
    } catch {
      setError(
        t(
          "report.errorFailed",
          "Failed to submit report. Please try again later.",
        ),
      );
    } finally {
      setIsSubmitting(false);
    }
  };

  const charsRemaining = 500 - note.length;

  return (
    <div
      role="dialog"
      aria-modal="true"
      aria-labelledby="report-dialog-title"
      className="fixed inset-0 z-50 flex items-center justify-center bg-overlay/75 p-4 backdrop-blur-sm animate-in fade-in"
    >
      <Card className="max-w-lg w-full border-border bg-surface-card shadow-2xl overflow-hidden relative">
        <button
          type="button"
          onClick={handleClose}
          className="absolute top-4 right-4 text-text-muted hover:text-text p-1 rounded-lg hover:bg-surface-muted transition-colors"
          aria-label={t("report.cancel", "Cancel")}
        >
          <X className="h-5 w-5" />
        </button>

        {isSubmitted ? (
          <div className="p-8 text-center space-y-4">
            <div className="mx-auto w-12 h-12 rounded-full bg-success/15 flex items-center justify-center text-success-accent">
              <CheckCircle2 className="h-6 w-6" />
            </div>
            <div className="space-y-1">
              <CardTitle className="text-xl font-bold">
                {t("report.successTitle", "Report submitted")}
              </CardTitle>
              <CardDescription className="text-sm">
                {t(
                  "report.successDesc",
                  "Thank you for your feedback. Our team will review this item.",
                )}
              </CardDescription>
            </div>
            <div className="pt-2">
              <Button
                onClick={handleClose}
                className="min-w-[120px] font-semibold"
              >
                {t("report.close", "Close")}
              </Button>
            </div>
          </div>
        ) : (
          <form
            onSubmit={(e) => {
              void handleSubmit(e);
            }}
          >
            <CardHeader className="space-y-1.5 pb-3">
              <div className="flex items-center gap-2 text-warning-accent">
                <Flag className="h-5 w-5 shrink-0" aria-hidden="true" />
                <CardTitle
                  id="report-dialog-title"
                  className="text-lg font-bold"
                >
                  {t("report.title", "Report an issue")}
                </CardTitle>
              </div>
              <CardDescription className="text-sm text-text-muted">
                {t(
                  "report.desc",
                  "Help us improve by telling us what is wrong with this exercise or sentence.",
                )}
              </CardDescription>
            </CardHeader>

            <div className="px-6 py-2 space-y-4 text-left">
              {error && (
                <div
                  role="alert"
                  className="p-3 rounded-xl border border-danger/30 bg-danger/10 text-danger-accent text-sm flex items-center gap-2"
                >
                  <AlertTriangle className="h-4 w-4 shrink-0" />
                  <span>{error}</span>
                </div>
              )}

              <fieldset className="space-y-2">
                <legend className="text-xs font-semibold uppercase tracking-wider text-text-muted">
                  {t("report.selectReason", "Reason for report")}
                </legend>
                <div className="space-y-2 pt-1">
                  {REPORT_REASONS.map((r) => (
                    <label
                      key={r}
                      className={`flex items-center gap-3 p-2.5 rounded-xl border min-h-[44px] transition-colors cursor-pointer text-sm font-medium ${
                        reason === r
                          ? "border-primary bg-primary/5 text-primary-accent"
                          : "border-border/60 hover:bg-surface-muted text-text"
                      }`}
                    >
                      <input
                        type="radio"
                        name="report_reason"
                        value={r}
                        checked={reason === r}
                        onChange={() => setReason(r)}
                        className="sr-only"
                      />
                      <span
                        className={`h-4 w-4 rounded-full border flex items-center justify-center shrink-0 ${
                          reason === r
                            ? "border-primary bg-primary"
                            : "border-border"
                        }`}
                        aria-hidden="true"
                      >
                        {reason === r && (
                          <span className="h-1.5 w-1.5 rounded-full bg-white" />
                        )}
                      </span>
                      <span>{t(`report.reason.${r}`)}</span>
                    </label>
                  ))}
                </div>
              </fieldset>

              <div className="space-y-1.5 pt-1">
                <div className="flex justify-between items-center">
                  <label
                    htmlFor="report-note"
                    className="text-xs font-semibold uppercase tracking-wider text-text-muted"
                  >
                    {t("report.noteLabel", "Additional details (optional)")}
                  </label>
                  <span className="text-[11px] text-text-muted">
                    {t("report.charsRemaining", { count: charsRemaining })}
                  </span>
                </div>
                <textarea
                  id="report-note"
                  value={note}
                  maxLength={500}
                  onChange={(e) => setNote(e.target.value)}
                  placeholder={t(
                    "report.notePlaceholder",
                    "Briefly explain the issue (max 500 characters)...",
                  )}
                  className="w-full min-h-[80px] rounded-xl border border-border bg-surface-muted/40 p-3 text-base text-text placeholder:text-text-muted/60 focus:border-primary focus:outline-none focus:ring-1 focus:ring-primary resize-none"
                />
              </div>
            </div>

            <CardFooter className="flex justify-end gap-2.5 pt-4 pb-6 px-6">
              <Button
                type="button"
                variant="outline"
                onClick={handleClose}
                disabled={isSubmitting}
                className="min-w-[90px]"
              >
                {t("report.cancel", "Cancel")}
              </Button>
              <Button
                type="submit"
                disabled={isSubmitting || !contentVersionId}
                isLoading={isSubmitting}
                className="min-w-[130px] font-bold"
              >
                {t("report.submit", "Submit report")}
              </Button>
            </CardFooter>
          </form>
        )}
      </Card>
    </div>
  );
};
