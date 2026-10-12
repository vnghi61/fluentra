import React, { useState } from "react";
import {
  AlertCircle,
  Calendar,
  ChevronLeft,
  ChevronRight,
  Eye,
  Mic,
  RotateCcw,
  Sparkles,
  Volume2,
  VolumeX,
} from "lucide-react";
import { useTranslation } from "react-i18next";
import { Link } from "@tanstack/react-router";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogBody,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { EmptyState } from "@/components/ui/empty-state";
import {
  Card,
  CardDescription,
  CardFooter,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import {
  SpeakingFeedbackView,
  useSpeakingFeedback,
  useSpeakingSubmissions,
} from "@/features/speaking";

const PAGE_SIZE = 10;

export function MySpeakingPage(): React.JSX.Element {
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
  } = useSpeakingSubmissions(page, PAGE_SIZE);

  const feedbackQuery = useSpeakingFeedback(
    selectedAttemptId,
    Boolean(selectedAttemptId),
  );

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
            className="border-emerald-500/40 bg-emerald-500/10 text-success-accent"
          >
            {t("speaking.statusGraded", "Graded")}
          </Badge>
        );
      case "grading":
        return (
          <Badge
            variant="outline"
            className="border-amber-500/40 bg-amber-500/10 text-warning-accent animate-pulse"
          >
            {t("speaking.statusGrading", "Evaluating...")}
          </Badge>
        );
      case "failed":
        return (
          <Badge
            variant="outline"
            className="border-danger/40 bg-danger/10 text-danger-accent"
          >
            {t("speaking.statusFailed", "Failed")}
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

  return (
    <div className="space-y-6 max-w-4xl mx-auto py-6 px-4 animate-in fade-in duration-200">
      {/* Header */}
      <header className="space-y-1">
        <div className="flex items-center gap-2 text-xs font-semibold uppercase tracking-wider text-primary-accent">
          <Mic className="h-4 w-4" aria-hidden="true" />
          <span>{t("speaking.mySpeakingTitle", "My Speaking")}</span>
        </div>
        <h1 className="text-2xl md:text-3xl font-bold leading-tight text-text tracking-tight">
          {t("speaking.mySpeakingTitle", "My Speaking")}
        </h1>
        <p className="text-base leading-relaxed text-text-muted">
          {t(
            "speaking.mySpeakingDesc",
            "Review your recorded speaking practice, accuracy scores, transcripts, and AI coaching feedback.",
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
              <AlertCircle
                className="h-8 w-8 text-danger-accent"
                aria-hidden="true"
              />
            </div>
            <CardTitle>
              {t("speaking.errorTitle", "Unable to Load Speaking Submissions")}
            </CardTitle>
            <CardDescription>
              {error?.message ||
                t(
                  "speaking.errorLoading",
                  "We could not fetch your speaking submissions. Please check your connection.",
                )}
            </CardDescription>
          </CardHeader>
          <CardFooter className="justify-center">
            <Button
              variant="outline"
              onClick={() => void refetch()}
              className="gap-2"
            >
              <RotateCcw className="h-4 w-4" aria-hidden="true" />
              {t("common.retry", "Try again")}
            </Button>
          </CardFooter>
        </Card>
      )}

      {/* Empty state */}
      {!isLoading && !isError && items.length === 0 && (
        <EmptyState
          icon={Mic}
          title={t("speaking.noSubmissions", "No speaking submissions yet")}
          description={t(
            "speaking.noSubmissionsDesc",
            "Complete speaking activities in your lessons to evaluate your speaking pace, transcript fidelity, and receive AI coaching.",
          )}
          action={
            <Link
              to="/practice"
              className="inline-flex min-h-11 items-center justify-center gap-2 rounded-lg bg-primary px-4 text-sm font-semibold text-primary-fg shadow-sm transition-colors hover:bg-primary-hover focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary focus-visible:ring-offset-2"
            >
              {t("speaking.startPractice", "Explore practice")}
            </Link>
          }
        />
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
                  <div className="min-w-0 space-y-1.5 flex-1">
                    <div className="flex items-center gap-2 flex-wrap">
                      {getStatusBadge(sub.status)}
                      <Badge variant="outline" className="text-xs">
                        {sub.task_type === "read_aloud"
                          ? t("speaking.readAloudTitle", "Read Aloud")
                          : t("speaking.respondTitle", "Spoken Response")}
                      </Badge>
                      {/* Null on an attempt graded before the score was
                          persisted. Saying so beats printing a 0 nobody
                          awarded. */}
                      {sub.overall_band != null && (
                        <Badge
                          variant="secondary"
                          className="font-mono text-xs font-bold"
                        >
                          {t("speaking.bandLabel", {
                            band: sub.overall_band.toFixed(1),
                          })}
                        </Badge>
                      )}
                      {sub.score != null && (
                        <span className="text-sm tabular-nums text-text-muted">
                          {sub.score} / 100
                        </span>
                      )}
                      {sub.overall_band == null && sub.score == null && (
                        <span className="text-sm text-text-muted italic">
                          {t("speaking.gradeNotRecorded")}
                        </span>
                      )}
                      <span className="flex items-center gap-1.5 text-sm text-text-muted">
                        <Calendar className="h-4 w-4" aria-hidden="true" />
                        {dateStr}
                      </span>
                      {sub.has_recording ? (
                        <span
                          className="flex items-center gap-1.5 text-sm text-success-accent"
                          title={t(
                            "speaking.audioAvailable",
                            "Audio recording available",
                          )}
                        >
                          <Volume2 className="h-4 w-4" aria-hidden="true" />
                          <span>{t("speaking.hasAudio", "Audio")}</span>
                        </span>
                      ) : (
                        <span
                          className="flex items-center gap-1.5 text-sm text-text-muted"
                          title={t(
                            "speaking.audioPurged",
                            "Audio purged after 90 days",
                          )}
                        >
                          <VolumeX className="h-4 w-4" aria-hidden="true" />
                          <span>{t("speaking.purged", "Purged")}</span>
                        </span>
                      )}
                    </div>

                    {summaryPreview && (
                      <p className="text-sm text-text-muted line-clamp-2 leading-relaxed">
                        {summaryPreview}
                      </p>
                    )}
                  </div>

                  <Button
                    variant={sub.status === "graded" ? "primary" : "outline"}
                    disabled={sub.status === "grading"}
                    onClick={() => setSelectedAttemptId(sub.attempt_id)}
                    className="shrink-0 gap-2"
                  >
                    <Eye className="h-4 w-4" aria-hidden="true" />
                    <span>
                      {t("speaking.viewFeedbackBtn", "View Feedback")}
                    </span>
                  </Button>
                </div>
              </Card>
            );
          })}

          {/* Pagination bar */}
          <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4 pt-4 border-t border-border text-sm text-text-muted">
            <div>
              {t("speaking.showingRange", {
                from,
                to,
                total,
                defaultValue: `Showing ${from}–${to} of ${total}`,
              })}
            </div>
            <div className="flex flex-wrap items-center gap-2">
              <Button
                variant="outline"
                disabled={page <= 1}
                onClick={() => setPage((p) => Math.max(1, p - 1))}
                className="gap-1 min-h-[44px]"
              >
                <ChevronLeft className="h-4 w-4" aria-hidden="true" />
                <span>{t("common.previous", "Previous")}</span>
              </Button>
              <span className="text-sm tabular-nums px-2 font-medium">
                {t("speaking.pageOf", {
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
                <ChevronRight className="h-4 w-4" aria-hidden="true" />
              </Button>
            </div>
          </div>
        </div>
      )}

      {/* Modal Feedback View */}
      {selectedAttemptId && (
        <Dialog
          open
          onOpenChange={(open) => {
            if (!open) setSelectedAttemptId(null);
          }}
        >
          <DialogContent className="max-w-3xl" aria-describedby={undefined}>
            <DialogHeader className="bg-surface-muted/30">
              <DialogTitle className="flex items-center gap-2">
                <Sparkles className="h-5 w-5 text-primary" aria-hidden="true" />
                {t("speaking.feedbackTitle", "Speaking Assessment")}
              </DialogTitle>
            </DialogHeader>

            {/* Modal Body */}
            <DialogBody className="p-6">
              {feedbackQuery.isLoading ? (
                <div className="flex flex-col items-center justify-center p-12 space-y-3">
                  <div
                    className="h-8 w-8 animate-spin rounded-full border-4 border-border-subtle border-t-primary"
                    aria-hidden="true"
                  />
                  <p className="text-sm text-text-muted">
                    {t("common.loading", "Loading feedback...")}
                  </p>
                </div>
              ) : feedbackQuery.isError ? (
                <div className="text-center p-8 space-y-3">
                  <AlertCircle
                    className="h-8 w-8 text-danger-accent mx-auto"
                    aria-hidden="true"
                  />
                  <p className="text-sm text-danger-accent font-medium">
                    {feedbackQuery.error?.message ||
                      t("speaking.feedbackError", "Failed to load feedback")}
                  </p>
                  <Button
                    size="sm"
                    variant="outline"
                    onClick={() => void feedbackQuery.refetch()}
                  >
                    {t("common.retry", "Try again")}
                  </Button>
                </div>
              ) : feedbackQuery.data ? (
                <SpeakingFeedbackView feedback={feedbackQuery.data} />
              ) : null}
            </DialogBody>

            {/* Modal Footer */}
            <DialogFooter className="px-6 py-3 bg-surface-muted/30">
              <Button
                variant="outline"
                onClick={() => setSelectedAttemptId(null)}
              >
                {t("common.close", "Close")}
              </Button>
            </DialogFooter>
          </DialogContent>
        </Dialog>
      )}
    </div>
  );
}

export default MySpeakingPage;
