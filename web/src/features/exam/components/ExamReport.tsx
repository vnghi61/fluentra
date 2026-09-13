import React, { useState } from "react";
import {
  AlertTriangle,
  Award,
  BookOpen,
  CheckCircle2,
  ChevronDown,
  ChevronUp,
  Flag,
  Headphones,
  Info,
  Loader2,
  Mic,
  PenTool,
  RotateCcw,
  ShieldAlert,
  Trash2,
  XCircle,
} from "lucide-react";
import { useTranslation } from "react-i18next";
import { Link } from "@tanstack/react-router";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";
import { examApi } from "../api/examApi";
import type { ScoreReport } from "../types";

export interface ExamReportProps {
  report: ScoreReport;
  onRefresh?: () => void;
  className?: string | undefined;
}

export const ExamReport: React.FC<ExamReportProps> = ({
  report,
  onRefresh,
  className,
}) => {
  const { t } = useTranslation();
  const [expandedSection, setExpandedSection] = useState<string | null>(null);
  const [isDeletingRecording, setIsDeletingRecording] = useState<boolean>(false);
  const [recordingDeleted, setRecordingDeleted] = useState<boolean>(false);
  const [reportedItems, setReportedItems] = useState<Record<string, boolean>>({});

  const handleDeleteRecording = async () => {
    if (!confirm(t("exam.report.confirmDeleteRecording", "Permanently delete your speaking audio recording? Scores and transcript will be retained."))) {
      return;
    }
    setIsDeletingRecording(true);
    try {
      await examApi.deleteSpeakingRecording(report.attempt_id);
      setRecordingDeleted(true);
    } catch {
      alert(t("exam.report.deleteRecordingFailed", "Failed to delete audio recording."));
    } finally {
      setIsDeletingRecording(false);
    }
  };

  const handleReportItem = (itemId: string) => {
    setReportedItems((prev) => ({ ...prev, [itemId]: true }));
    alert(t("exam.report.itemReportedSuccess", "Question reported to curriculum team for review. Thank you!"));
  };

  const getSectionIcon = (skill: string) => {
    switch (skill) {
      case "listening":
        return Headphones;
      case "reading":
        return BookOpen;
      case "writing":
        return PenTool;
      case "speaking":
        return Mic;
      default:
        return Award;
    }
  };

  const isPending = report.status === "pending";
  const isPartial = report.status === "partial";

  return (
    <div className={cn("max-w-4xl mx-auto space-y-8 p-4 sm:p-6 md:p-8", className)}>
      {/* Pending Banner */}
      {isPending && (
        <div className="flex items-center justify-between rounded-xl bg-primary/10 border border-primary/20 p-4 text-primary">
          <div className="flex items-center gap-3">
            <Loader2 className="h-5 w-5 animate-spin shrink-0" />
            <div>
              <h4 className="font-semibold text-sm">
                {t("exam.report.gradingInProgressTitle", "Scoring in Progress")}
              </h4>
              <p className="text-xs text-primary/80">
                {t(
                  "exam.report.gradingInProgressDesc",
                  "AI assessment for Writing and Speaking is finalizing. This screen refreshes automatically.",
                )}
              </p>
            </div>
          </div>
          {onRefresh && (
            <Button
              type="button"
              variant="outline"
              size="sm"
              onClick={onRefresh}
              className="text-xs min-h-[36px] bg-card border-primary/30"
            >
              <RotateCcw className="h-3.5 w-3.5 mr-1.5" />
              {t("common.refresh", "Refresh")}
            </Button>
          )}
        </div>
      )}

      {/* Partial Banner */}
      {isPartial && (
        <div className="flex items-center gap-3 rounded-xl bg-warning/10 border border-warning/30 p-4 text-warning">
          <AlertTriangle className="h-5 w-5 shrink-0" />
          <div className="text-xs">
            <h4 className="font-semibold text-sm">
              {t("exam.report.partialReportTitle", "Partial Score Report")}
            </h4>
            <p>
              {t(
                "exam.report.partialReportDesc",
                "Some asynchronous grading tasks could not complete and are marked 'Not Scored'. Only completed sections factor into your total score.",
              )}
            </p>
          </div>
        </div>
      )}

      {/* Hero Overview Card */}
      <div className="rounded-3xl border border-border bg-card p-6 sm:p-8 shadow-sm text-center space-y-6">
        <div className="space-y-2">
          <Badge variant="outline" className="text-xs font-semibold uppercase tracking-wider px-3 py-1">
            {t("exam.report.officialScoreSummary", "Mock Exam Score Summary")}
          </Badge>
          <h1 className="text-2xl sm:text-3xl font-extrabold text-text">
            {t("exam.report.congratulations", "Exam Completed")}
          </h1>
          <p className="text-xs sm:text-sm text-text-muted max-w-lg mx-auto">
            {t(
              "exam.report.summarySubtitle",
              "Comprehensive assessment across Listening, Reading, Writing, and Speaking skills.",
            )}
          </p>
        </div>

        {/* Score & CEFR Display */}
        <div className="flex flex-wrap items-center justify-center gap-6 sm:gap-12 py-4">
          {/* Total Scaled Score */}
          <div className="flex flex-col items-center">
            <div className="text-5xl sm:text-6xl font-extrabold font-mono text-primary tracking-tight">
              {Math.round(report.overall_score || 0)}
              <span className="text-lg sm:text-xl font-normal text-text-muted">/100</span>
            </div>
            <span className="text-xs font-semibold uppercase tracking-wider text-text-muted mt-1">
              {t("exam.report.overallScore", "Overall Score")}
            </span>
          </div>

          <div className="h-16 w-px bg-border hidden sm:block" />

          {/* CEFR Band */}
          <div className="flex flex-col items-center">
            <div className="text-5xl sm:text-6xl font-extrabold font-mono text-accent tracking-tight">
              {report.overall_band || "B1"}
            </div>
            <span className="text-xs font-semibold uppercase tracking-wider text-text-muted mt-1">
              {t("exam.report.estimatedCEFR", "Estimated CEFR Band")}
            </span>
          </div>
        </div>

        {/* Mandatory Official Disclaimer (Requirement §8) */}
        <div className="rounded-2xl bg-surface-muted/70 border border-border-subtle p-4 sm:p-5 text-left space-y-2 text-xs text-text-muted leading-relaxed">
          <div className="flex items-center gap-2 text-text font-bold text-sm">
            <Info className="h-4 w-4 text-primary shrink-0" />
            <span>{t("exam.report.disclaimerTitle", "Important Score Disclaimers")}</span>
          </div>
          <ul className="list-disc list-inside space-y-1 pl-1">
            <li className="font-semibold text-text">
              {t("exam.report.notOfficialTOEIC", "Not an official TOEIC score.")}
            </li>
            <li>
              {t(
                "exam.report.pronunciationNotice",
                "Pronunciation is not assessed: speaking tasks are evaluated on fluency, vocabulary, and grammar through automatic transcription.",
              )}
            </li>
            <li>
              {t(
                "exam.report.poolNotice",
                "Each exam sitting is uniquely drawn from our verified question pool. Tests taken at different times or by different learners contain different items.",
              )}
            </li>
          </ul>
        </div>
      </div>

      {/* Per-Section Score Breakdown Cards */}
      <div className="space-y-4">
        <h2 className="text-lg font-bold text-text">
          {t("exam.report.sectionBreakdown", "Section Performance Breakdown")}
        </h2>

        <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
          {(report.per_section || []).map((sec) => {
            const Icon = getSectionIcon(sec.skill);
            const isScored = sec.status !== "not_scored";
            const percent = sec.max_score > 0 ? (sec.score / sec.max_score) * 100 : 0;

            return (
              <div
                key={sec.skill}
                className="rounded-2xl border border-border bg-card p-5 shadow-xs space-y-4 flex flex-col justify-between"
              >
                <div className="space-y-3">
                  <div className="flex items-center justify-between">
                    <div className="flex items-center gap-2.5">
                      <div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-xl bg-primary/10 text-primary">
                        <Icon className="h-5 w-5" />
                      </div>
                      <div>
                        <h3 className="font-bold text-base text-text capitalize">
                          {sec.skill}
                        </h3>
                        <span className="text-xs text-text-muted font-mono">
                          {sec.band ? `CEFR ${sec.band}` : t("exam.report.unbanded", "Unbanded")}
                        </span>
                      </div>
                    </div>

                    <div className="text-right">
                      {isScored ? (
                        <div className="text-2xl font-bold font-mono text-text">
                          {sec.score}
                          <span className="text-xs text-text-muted font-normal">/{sec.max_score}</span>
                        </div>
                      ) : (
                        <Badge variant="outline" className="text-xs text-text-muted">
                          {t("exam.report.notScored", "Not Scored")}
                        </Badge>
                      )}
                    </div>
                  </div>

                  {/* Progress Bar */}
                  {isScored && (
                    <div className="space-y-1">
                      <div className="h-2 w-full bg-border-subtle rounded-full overflow-hidden">
                        <div
                          className="h-full bg-primary rounded-full transition-all duration-300"
                          style={{ width: `${percent}%` }}
                        />
                      </div>
                      <div className="text-[11px] text-text-muted text-right font-mono">
                        {Math.round(percent)}%
                      </div>
                    </div>
                  )}
                </div>

                {/* Section Specific Feedback Badges */}
                {sec.skill === "speaking" && report.feedback?.speaking && (
                  <div className="pt-3 border-t border-border-subtle space-y-2 text-xs">
                    {report.feedback.speaking.read_aloud_accuracy !== undefined && (
                      <div className="flex justify-between">
                        <span className="text-text-muted">{t("exam.report.readAloudAccuracy", "Read Aloud Accuracy:")}</span>
                        <span className="font-semibold text-text">
                          {report.feedback.speaking.read_aloud_accuracy}%
                        </span>
                      </div>
                    )}
                    {report.feedback.speaking.words_per_minute && (
                      <div className="flex justify-between">
                        <span className="text-text-muted">{t("exam.report.speechRate", "Speech Rate:")}</span>
                        <span className="font-semibold text-text">
                          {report.feedback.speaking.words_per_minute} WPM
                        </span>
                      </div>
                    )}
                    {!recordingDeleted ? (
                      <Button
                        type="button"
                        variant="ghost"
                        size="sm"
                        disabled={isDeletingRecording}
                        onClick={handleDeleteRecording}
                        className="text-xs text-danger hover:bg-danger/10 w-full justify-start px-0 min-h-[36px]"
                      >
                        <Trash2 className="h-3.5 w-3.5 mr-1.5" />
                        <span>{t("exam.report.deleteVoiceRecording", "Delete Voice Recording (GDPR)")}</span>
                      </Button>
                    ) : (
                      <span className="text-xs text-text-muted italic">
                        {t("exam.report.recordingDeleted", "Voice recording deleted.")}
                      </span>
                    )}
                  </div>
                )}

                {sec.skill === "writing" && report.feedback?.writing && (
                  <div className="pt-3 border-t border-border-subtle text-xs space-y-1">
                    <p className="text-text-muted line-clamp-2">
                      {report.feedback.writing.feedback_en || report.feedback.writing.feedback}
                    </p>
                  </div>
                )}
              </div>
            );
          })}
        </div>
      </div>

      {/* Item-by-Item Review Section */}
      <div className="rounded-2xl border border-border bg-card p-6 shadow-xs space-y-4">
        <div className="flex items-center justify-between">
          <h2 className="text-lg font-bold text-text">
            {t("exam.report.itemReviewTitle", "Item-by-Item Review")}
          </h2>
          <span className="text-xs text-text-muted">
            {t("exam.report.reviewDetailed", "Review questions and model answers")}
          </span>
        </div>

        <div className="space-y-4">
          {(report.per_section || []).map((sec) => {
            const items = Array.isArray(sec.item_results) ? sec.item_results : [];
            if (items.length === 0) return null;

            const isExpanded = expandedSection === sec.skill;

            return (
              <div
                key={sec.skill}
                className="rounded-xl border border-border-subtle bg-surface-muted/30 overflow-hidden"
              >
                <button
                  type="button"
                  onClick={() => setExpandedSection(isExpanded ? null : sec.skill)}
                  className="w-full flex items-center justify-between p-4 text-left font-semibold text-sm hover:bg-surface-muted/60 transition-colors min-h-[44px]"
                >
                  <span className="capitalize">{sec.skill} ({items.length} questions)</span>
                  {isExpanded ? <ChevronUp className="h-4 w-4" /> : <ChevronDown className="h-4 w-4" />}
                </button>

                {isExpanded && (
                  <div className="p-4 pt-0 space-y-3 divide-y divide-border-subtle">
                    {items.map((item: any, idx: number) => {
                      const isReported = reportedItems[item.id || idx];
                      return (
                        <div key={item.id || idx} className="pt-3 first:pt-0 space-y-2 text-xs">
                          <div className="flex items-center justify-between">
                            <div className="flex items-center gap-2">
                              {item.correct ? (
                                <CheckCircle2 className="h-4 w-4 text-success shrink-0" />
                              ) : (
                                <XCircle className="h-4 w-4 text-danger shrink-0" />
                              )}
                              <span className="font-semibold text-text">
                                {t("exam.report.questionNum", { num: idx + 1, defaultValue: `Question ${idx + 1}` })}
                              </span>
                            </div>

                            <Button
                              type="button"
                              variant="ghost"
                              size="sm"
                              disabled={isReported}
                              onClick={() => handleReportItem(item.id || String(idx))}
                              className="text-[11px] text-text-muted hover:text-danger min-h-[32px] gap-1"
                            >
                              <Flag className="h-3 w-3" />
                              <span>{isReported ? t("exam.report.reported", "Reported") : t("exam.report.reportItem", "Report error")}</span>
                            </Button>
                          </div>

                          {item.prompt && (
                            <p className="text-text font-medium">{item.prompt}</p>
                          )}
                          {item.explanation && (
                            <p className="text-text-muted italic bg-surface-muted p-2 rounded-md">
                              {typeof item.explanation === "object"
                                ? item.explanation.explanation_en || item.explanation.explanation_vi
                                : String(item.explanation)}
                            </p>
                          )}
                        </div>
                      );
                    })}
                  </div>
                )}
              </div>
            );
          })}
        </div>
      </div>

      {/* Integrity Signals Summary Card */}
      {report.integrity_signals && report.integrity_signals.length > 0 && (
        <div className="rounded-2xl border border-border bg-card p-6 shadow-xs space-y-3">
          <div className="flex items-center gap-2 text-sm font-bold text-text">
            <ShieldAlert className="h-4 w-4 text-warning" />
            <span>{t("exam.report.integritySummaryTitle", "Test Session Integrity Record")}</span>
          </div>

          <p className="text-xs text-text-muted">
            {t(
              "exam.report.integritySummaryDesc",
              "Integrity telemetry records test environment events for self-study accountability.",
            )}
          </p>

          <div className="flex flex-wrap gap-3 pt-1">
            {report.integrity_signals.map((sig, idx) => (
              <Badge key={idx} variant="outline" className="text-xs px-3 py-1 font-mono">
                {sig.kind}: {sig.count || 1}
              </Badge>
            ))}
          </div>
        </div>
      )}

      {/* Action Footer */}
      <div className="flex justify-center pt-4">
        <Link to="/exams">
          <Button
            type="button"
            className="bg-primary text-white hover:bg-primary/90 min-h-[44px] px-6 font-semibold"
          >
            {t("exam.report.returnToExams", "Back to Exam Hub")}
          </Button>
        </Link>
      </div>
    </div>
  );
};
