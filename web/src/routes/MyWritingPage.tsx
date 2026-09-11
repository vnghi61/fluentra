import React, { useState } from "react";
import {
  AlertCircle,
  Calendar,
  ChevronLeft,
  ChevronRight,
  Eye,
  FileText,
  PenTool,
  RotateCcw,
  Sparkles,
  X,
} from "lucide-react";
import { useTranslation } from "react-i18next";
import { useQuery } from "@tanstack/react-query";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  Card,
  CardDescription,
  CardFooter,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { learningApi } from "@/features/learning";
import {
  useWritingFeedback,
  useWritingSubmissions,
  WritingFeedbackView,
} from "@/features/writing";

const PAGE_SIZE = 10;

export function MyWritingPage(): React.JSX.Element {
  const { t, i18n } = useTranslation();
  const isVi = i18n.language.startsWith("vi");
  const [page, setPage] = useState(1);
  const [selectedAttemptId, setSelectedAttemptId] = useState<string | null>(
    null,
  );

  const {
    data: submissionsData,
    isLoading,
    isError,
    error,
    refetch,
  } = useWritingSubmissions(page, PAGE_SIZE);

  // When modal is open, query both feedback and attempt detail to retrieve submitted essay text
  const feedbackQuery = useWritingFeedback(
    selectedAttemptId,
    !!selectedAttemptId,
  );

  const attemptQuery = useQuery({
    queryKey: ["attempt", selectedAttemptId],
    queryFn: () => learningApi.getAttempt(selectedAttemptId!),
    enabled: !!selectedAttemptId,
  });

  const items = submissionsData?.items ?? [];
  const total = submissionsData?.total ?? 0;
  const totalPages = Math.max(1, Math.ceil(total / PAGE_SIZE));
  const from = total === 0 ? 0 : (page - 1) * PAGE_SIZE + 1;
  const to = Math.min(page * PAGE_SIZE, total);

  const getStatusBadge = (status: string) => {
    switch (status) {
      case "graded":
        return (
          <Badge
            variant="outline"
            className="border-emerald-500/40 bg-emerald-500/10 text-emerald-500"
          >
            {t("writing.statusGraded", "Graded")}
          </Badge>
        );
      case "grading":
        return (
          <Badge
            variant="outline"
            className="border-amber-500/40 bg-amber-500/10 text-amber-500 animate-pulse"
          >
            {t("writing.statusGrading", "Marking...")}
          </Badge>
        );
      case "failed":
        return (
          <Badge
            variant="outline"
            className="border-danger/40 bg-danger/10 text-danger-accent"
          >
            {t("writing.statusFailed", "Failed")}
          </Badge>
        );
      default:
        return (
          <Badge variant="outline" className="border-border text-text-muted">
            {status}
          </Badge>
        );
    }
  };

  const essayText =
    typeof attemptQuery.data?.response?.text_answer === "string"
      ? attemptQuery.data.response.text_answer
      : undefined;

  return (
    <div className="space-y-6 max-w-4xl mx-auto py-6 px-4 animate-in fade-in duration-200">
      {/* Header */}
      <header className="space-y-1">
        <div className="flex items-center gap-2 text-xs font-semibold uppercase tracking-wider text-primary">
          <PenTool className="h-4 w-4" aria-hidden="true" />
          <span>{t("writing.myWritingTitle", "My Writing")}</span>
        </div>
        <h1 className="text-2xl md:text-3xl font-extrabold text-text tracking-tight">
          {t("writing.myWritingTitle", "My Writing")}
        </h1>
        <p className="text-sm text-text-muted">
          {t(
            "writing.myWritingDesc",
            "Review your submitted essays, band scores, and AI examiner feedback.",
          )}
        </p>
      </header>

      {/* Loading state */}
      {isLoading && (
        <div className="space-y-4">
          {[1, 2, 3].map((i) => (
            <Card key={i} className="p-4 border-border">
              <div className="flex items-center justify-between gap-4">
                <div className="space-y-2 flex-1">
                  <Skeleton className="h-5 w-48" />
                  <Skeleton className="h-4 w-72" />
                </div>
                <Skeleton className="h-9 w-28" />
              </div>
            </Card>
          ))}
        </div>
      )}

      {/* Error state */}
      {isError && (
        <Card className="border-danger/30 text-center p-6">
          <CardHeader>
            <div className="flex justify-center mb-2">
              <AlertCircle className="h-8 w-8 text-danger-accent" />
            </div>
            <CardTitle>
              {t("writing.errorTitle", "Unable to Load Submissions")}
            </CardTitle>
            <CardDescription>
              {error?.message ||
                t(
                  "writing.errorLoading",
                  "We could not fetch your writing submissions. Please check your connection.",
                )}
            </CardDescription>
          </CardHeader>
          <CardFooter className="justify-center">
            <Button
              variant="outline"
              onClick={() => void refetch()}
              className="gap-2"
            >
              <RotateCcw className="h-4 w-4" />
              {t("writing.retry", "Try again")}
            </Button>
          </CardFooter>
        </Card>
      )}

      {/* Empty state */}
      {!isLoading && !isError && items.length === 0 && (
        <Card className="text-center p-8 border-border bg-surface-card">
          <CardHeader className="space-y-2">
            <div className="flex justify-center">
              <div className="h-12 w-12 rounded-full bg-primary/10 flex items-center justify-center text-primary">
                <FileText className="h-6 w-6" />
              </div>
            </div>
            <CardTitle className="text-lg font-semibold text-text">
              {t("writing.noSubmissions", "No writing submissions yet")}
            </CardTitle>
            <CardDescription className="max-w-md mx-auto text-sm">
              {t(
                "writing.noSubmissionsDesc",
                "Complete writing exercises in your lessons to receive IELTS band scores and personalized AI feedback.",
              )}
            </CardDescription>
          </CardHeader>
        </Card>
      )}

      {/* Submissions list */}
      {!isLoading && !isError && items.length > 0 && (
        <div className="space-y-3">
          {items.map((sub) => {
            const dateStr = new Date(sub.created_at).toLocaleDateString(
              isVi ? "vi-VN" : "en-US",
              {
                year: "numeric",
                month: "short",
                day: "numeric",
                hour: "2-digit",
                minute: "2-digit",
              },
            );
            const summaryPreview = isVi
              ? sub.feedback_vi || sub.feedback_en
              : sub.feedback_en;

            return (
              <Card
                key={sub.attempt_id}
                className="border-border bg-surface-card hover:border-border/80 transition-all p-4 shadow-sm"
              >
                <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
                  <div className="space-y-1.5 flex-1">
                    <div className="flex items-center gap-2 flex-wrap">
                      {getStatusBadge(sub.status)}
                      {sub.overall_band > 0 && (
                        <Badge
                          variant="secondary"
                          className="font-mono text-xs font-bold"
                        >
                          Band {sub.overall_band.toFixed(1)}
                        </Badge>
                      )}
                      {sub.score > 0 && (
                        <span className="text-xs text-text-muted">
                          {sub.score} / 100
                        </span>
                      )}
                      <span className="flex items-center gap-1 text-xs text-text-muted">
                        <Calendar className="h-3.5 w-3.5" />
                        {dateStr}
                      </span>
                    </div>

                    {summaryPreview && (
                      <p className="text-xs text-text-muted line-clamp-2 leading-relaxed">
                        {summaryPreview}
                      </p>
                    )}
                  </div>

                  <Button
                    variant={sub.status === "graded" ? "primary" : "outline"}
                    disabled={sub.status === "grading"}
                    onClick={() => setSelectedAttemptId(sub.attempt_id)}
                    className="shrink-0 gap-1.5 min-h-[44px]"
                  >
                    <Eye className="h-3.5 w-3.5" />
                    <span>{t("writing.viewFeedbackBtn", "View Feedback")}</span>
                  </Button>
                </div>
              </Card>
            );
          })}

          {/* Pagination bar */}
          <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4 pt-4 border-t border-border text-sm text-text-muted">
            <div>
              {t("writing.showingRange", {
                from,
                to,
                total,
                defaultValue: `Showing ${from}–${to} of ${total}`,
              })}
            </div>
            <div className="flex items-center gap-2">
              <Button
                variant="outline"
                disabled={page <= 1}
                onClick={() => setPage((p) => Math.max(1, p - 1))}
                className="gap-1 min-h-[44px]"
              >
                <ChevronLeft className="h-4 w-4" />
                <span>{t("common.previous", "Previous")}</span>
              </Button>
              <span className="text-xs px-2 font-medium">
                {t("writing.pageOf", {
                  current: page,
                  total: totalPages,
                  defaultValue: `Page ${page} of ${totalPages}`,
                })}
              </span>
              <Button
                variant="outline"
                disabled={page >= totalPages}
                onClick={() => setPage((p) => Math.min(totalPages, p + 1))}
                className="gap-1 min-h-[44px]"
              >
                <span>{t("common.next", "Next")}</span>
                <ChevronRight className="h-4 w-4" />
              </Button>
            </div>
          </div>
        </div>
      )}

      {/* Modal Feedback View */}
      {selectedAttemptId && (
        <div
          role="dialog"
          aria-modal="true"
          aria-labelledby="feedback-dialog-title"
          className="fixed inset-0 z-50 flex items-center justify-center bg-overlay/75 p-4 backdrop-blur-sm animate-in fade-in overflow-y-auto"
        >
          <div className="bg-surface-card border border-border rounded-2xl shadow-2xl max-w-3xl w-full max-h-[90vh] flex flex-col my-auto overflow-hidden">
            {/* Modal Header */}
            <div className="flex items-center justify-between px-6 py-4 border-b border-border bg-surface-muted/30">
              <div className="flex items-center gap-2">
                <Sparkles className="h-5 w-5 text-primary" />
                <h2
                  id="feedback-dialog-title"
                  className="text-lg font-bold text-text"
                >
                  {t("writing.feedbackTitle", "Writing Assessment")}
                </h2>
              </div>
              <Button
                variant="ghost"
                onClick={() => setSelectedAttemptId(null)}
                className="h-11 w-11 min-h-[44px] min-w-[44px] p-0 rounded-full text-text-muted hover:text-text"
                aria-label={t("common.close", "Close")}
              >
                <X className="h-4 w-4" />
              </Button>
            </div>

            {/* Modal Body */}
            <div className="overflow-y-auto p-6 flex-1">
              {feedbackQuery.isLoading || attemptQuery.isLoading ? (
                <div className="flex flex-col items-center justify-center p-12 space-y-3">
                  <div className="h-8 w-8 animate-spin rounded-full border-4 border-border-subtle border-t-primary" />
                  <p className="text-sm text-text-muted">
                    {t("common.loading", "Loading feedback...")}
                  </p>
                </div>
              ) : feedbackQuery.isError ? (
                <div className="text-center p-8 space-y-3">
                  <AlertCircle className="h-8 w-8 text-danger-accent mx-auto" />
                  <p className="text-sm text-danger-accent font-medium">
                    {feedbackQuery.error?.message ||
                      t("writing.feedbackError", "Failed to load feedback")}
                  </p>
                  <Button
                    size="sm"
                    variant="outline"
                    onClick={() => void feedbackQuery.refetch()}
                  >
                    {t("writing.retry", "Try again")}
                  </Button>
                </div>
              ) : feedbackQuery.data ? (
                <WritingFeedbackView
                  feedback={feedbackQuery.data}
                  essayText={essayText}
                />
              ) : null}
            </div>

            {/* Modal Footer */}
            <div className="px-6 py-3 border-t border-border bg-surface-muted/30 flex justify-end">
              <Button
                variant="outline"
                onClick={() => setSelectedAttemptId(null)}
              >
                {t("common.close", "Close")}
              </Button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}

export default MyWritingPage;
