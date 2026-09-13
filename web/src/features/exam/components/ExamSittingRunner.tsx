import React, { useCallback, useEffect, useMemo, useRef, useState } from "react";
import {
  AlertCircle,
  ArrowRight,
  BookOpen,
  CheckCircle2,
  Clock,
  Headphones,
  Mic,
  PenTool,
  Save,
  Send,
} from "lucide-react";
import { useTranslation } from "react-i18next";
import { useNavigate } from "@tanstack/react-router";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";
import { examApi } from "../api/examApi";
import type {
  ExamAttempt,
  IntegrityEvent,
  SittingActivity,
} from "../types";
import { ListeningPlayer } from "./ListeningPlayer";
import { SpeakingRecorder } from "./SpeakingRecorder";

export interface ExamSittingRunnerProps {
  attempt: ExamAttempt;
  onSubmitted?: () => void;
}

export const ExamSittingRunner: React.FC<ExamSittingRunnerProps> = ({
  attempt,
  onSubmitted,
}) => {
  const { t } = useTranslation();
  const navigate = useNavigate();

  // Active section index (1-based: 1=Listening, 2=Reading, 3=Writing, 4=Speaking)
  const [currentSectionNum, setCurrentSectionNum] = useState<number>(
    attempt.current_section || 1,
  );
  const [draftAnswers, setDraftAnswers] = useState<Record<string, any>>(
    attempt.draft_answers || {},
  );
  const [remainingSeconds, setRemainingSeconds] = useState<number>(
    attempt.remaining_seconds,
  );
  const [isSubmitting, setIsSubmitting] = useState<boolean>(false);
  const [isAutoSaving, setIsAutoSaving] = useState<boolean>(false);
  const [isExpiring, setIsExpiring] = useState<boolean>(false);
  const [showSubmitModal, setShowSubmitModal] = useState<boolean>(false);
  const [saveError, setSaveError] = useState<string | null>(null);

  // Integrity events buffer
  const integrityEventsRef = useRef<IntegrityEvent[]>([]);
  const draftAnswersRef = useRef<Record<string, any>>(draftAnswers);
  draftAnswersRef.current = draftAnswers;

  const isExamMode = attempt.mode === "exam";
  const sections = attempt.section_activities || [];

  // -------------------------------------------------------------------------
  // Server-Synced Countdown Timer
  // -------------------------------------------------------------------------
  useEffect(() => {
    if (attempt.status !== "in_progress") return;

    const timer = setInterval(() => {
      setRemainingSeconds((prev) => {
        if (prev <= 1) {
          clearInterval(timer);
          void handleTimeExpired();
          return 0;
        }
        return prev - 1;
      });
    }, 1000);

    return () => clearInterval(timer);
  }, [attempt.status, attempt.id]);

  // Handle auto-submit when server time expires
  const handleTimeExpired = async () => {
    setIsExpiring(true);
    try {
      // Flush any pending drafts first
      await examApi.saveAnswers(attempt.id, {
        section_number: currentSectionNum,
        answers: draftAnswersRef.current,
        integrity_events: integrityEventsRef.current,
      });
    } catch {
      // Ignore save error on timeout
    }

    try {
      await examApi.submitExam(attempt.id);
    } catch {
      // Even if call fails, server marked expired on read
    }

    if (onSubmitted) {
      onSubmitted();
    } else {
      void navigate({
        to: "/exams/$attemptId/report",
        params: { attemptId: attempt.id },
      });
    }
  };

  // -------------------------------------------------------------------------
  // Integrity Event Tracking (Tab switch, Blur, Paste)
  // -------------------------------------------------------------------------
  useEffect(() => {
    if (attempt.status !== "in_progress") return;

    const recordEvent = (kind: IntegrityEvent["kind"]) => {
      integrityEventsRef.current.push({
        kind,
        occurred_at: new Date().toISOString(),
      });
    };

    const handleVisibilityChange = () => {
      if (document.visibilityState === "hidden") {
        recordEvent("tab_hidden");
      } else {
        // Re-sync timer with server on return
        void syncTimeWithServer();
      }
    };

    const handleBlur = () => {
      recordEvent("window_blurred");
    };

    const handlePaste = () => {
      recordEvent("paste");
    };

    document.addEventListener("visibilitychange", handleVisibilityChange);
    window.addEventListener("blur", handleBlur);
    document.addEventListener("paste", handlePaste);

    return () => {
      document.removeEventListener("visibilitychange", handleVisibilityChange);
      window.removeEventListener("blur", handleBlur);
      document.removeEventListener("paste", handlePaste);
    };
  }, [attempt.id, attempt.status]);

  const syncTimeWithServer = async () => {
    try {
      const refreshed = await examApi.getAttempt(attempt.id);
      if (refreshed.status === "expired" || refreshed.status === "completed") {
        void handleTimeExpired();
      } else {
        setRemainingSeconds(refreshed.remaining_seconds);
      }
    } catch {
      // Ignore background sync error
    }
  };

  // -------------------------------------------------------------------------
  // Autosave logic (Debounced 15s)
  // -------------------------------------------------------------------------
  const saveCurrentDrafts = useCallback(async () => {
    if (attempt.status !== "in_progress") return;

    setIsAutoSaving(true);
    setSaveError(null);
    try {
      const eventsToFlush = [...integrityEventsRef.current];
      integrityEventsRef.current = [];

      const res = await examApi.saveAnswers(attempt.id, {
        section_number: currentSectionNum,
        answers: draftAnswersRef.current,
        integrity_events: eventsToFlush,
      });
      setRemainingSeconds(res.remaining_seconds);
    } catch (err: any) {
      setSaveError(err?.message || "Autosave failed");
    } finally {
      setIsAutoSaving(false);
    }
  }, [attempt.id, attempt.status, currentSectionNum]);

  useEffect(() => {
    if (attempt.status !== "in_progress") return;

    const interval = setInterval(() => {
      void saveCurrentDrafts();
    }, 15_000);

    return () => clearInterval(interval);
  }, [saveCurrentDrafts, attempt.status]);

  // Save on tab close / beforeunload
  useEffect(() => {
    const handleBeforeUnload = () => {
      void saveCurrentDrafts();
    };
    window.addEventListener("beforeunload", handleBeforeUnload);
    return () => window.removeEventListener("beforeunload", handleBeforeUnload);
  }, [saveCurrentDrafts]);

  // -------------------------------------------------------------------------
  // Answer Update Handlers
  // -------------------------------------------------------------------------
  const handleUpdateActivityAnswer = (activityId: string, answer: any) => {
    setDraftAnswers((prev) => ({
      ...prev,
      [activityId]: answer,
    }));
  };

  // -------------------------------------------------------------------------
  // Section Navigation
  // -------------------------------------------------------------------------
  const currentSection = useMemo(() => {
    return (
      sections.find((s) => s.section_position === currentSectionNum) ||
      sections[0]
    );
  }, [sections, currentSectionNum]);

  const handleNextSection = async () => {
    await saveCurrentDrafts();

    if (isExamMode) {
      // Exam mode completes current section permanently
      try {
        const res = await examApi.completeSection(
          attempt.id,
          currentSectionNum,
        );
        setCurrentSectionNum(res.current_section);
        setRemainingSeconds(res.remaining_seconds);
      } catch (err: any) {
        setSaveError(err?.message || "Failed to complete section");
      }
    } else {
      // Practice mode advances freely
      if (currentSectionNum < 4) {
        setCurrentSectionNum((prev) => prev + 1);
      }
    }
  };

  const handleSwitchSectionPractice = (targetNum: number) => {
    if (isExamMode) return; // Disallowed in exam mode
    void saveCurrentDrafts();
    setCurrentSectionNum(targetNum);
  };

  // -------------------------------------------------------------------------
  // Final Submission
  // -------------------------------------------------------------------------
  const handleSubmitExam = async () => {
    setIsSubmitting(true);
    setSaveError(null);
    try {
      await saveCurrentDrafts();
      await examApi.submitExam(attempt.id);
      if (onSubmitted) {
        onSubmitted();
      } else {
        void navigate({
          to: "/exams/$attemptId/report",
          params: { attemptId: attempt.id },
        });
      }
    } catch (err: any) {
      setSaveError(err?.message || "Submission failed");
      setIsSubmitting(false);
    }
  };

  // Count answered items
  const totalActivitiesCount = useMemo(() => {
    return sections.reduce((acc, sec) => acc + (sec.activities?.length || 0), 0);
  }, [sections]);

  const answeredActivitiesCount = useMemo(() => {
    let count = 0;
    for (const sec of sections) {
      for (const act of sec.activities || []) {
        const ans = draftAnswers[act.id];
        if (ans) {
          if (typeof ans === "string" && ans.trim()) count++;
          else if (ans.answers && Object.keys(ans.answers).length > 0) count++;
          else if (ans.submission && ans.submission.trim()) count++;
          else if (ans.answer && ans.answer.trim()) count++;
          else if (ans.recording_key) count++;
        }
      }
    }
    return count;
  }, [sections, draftAnswers]);

  // Format remaining timer
  const formatTimer = (totalSecs: number) => {
    const m = Math.floor(Math.max(0, totalSecs) / 60);
    const s = Math.floor(Math.max(0, totalSecs) % 60);
    return `${m.toString().padStart(2, "0")}:${s.toString().padStart(2, "0")}`;
  };

  const isLowTime = remainingSeconds < 120; // under 2 mins

  const sectionMeta = [
    { num: 1, label: t("exam.sections.listening", "Listening"), Icon: Headphones },
    { num: 2, label: t("exam.sections.reading", "Reading"), Icon: BookOpen },
    { num: 3, label: t("exam.sections.writing", "Writing"), Icon: PenTool },
    { num: 4, label: t("exam.sections.speaking", "Speaking"), Icon: Mic },
  ];

  return (
    <div className="min-h-screen bg-background text-text flex flex-col">
      {/* Expiry Overlay Banner */}
      {isExpiring && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/80 backdrop-blur-md p-4 text-center">
          <div className="max-w-md space-y-3 text-white">
            <Clock className="h-12 w-12 mx-auto animate-spin text-danger" />
            <h2 className="text-2xl font-bold">
              {t("exam.runner.timeIsUp", "Time's up!")}
            </h2>
            <p className="text-sm text-gray-300">
              {t(
                "exam.runner.submittingNow",
                "Your exam is being automatically submitted. Please wait...",
              )}
            </p>
          </div>
        </div>
      )}

      {/* Top Runner Header */}
      <header className="sticky top-0 z-40 bg-card border-b border-border px-4 sm:px-8 py-3 flex flex-wrap items-center justify-between gap-3 shadow-xs">
        {/* Left: Title & Mode */}
        <div className="flex items-center gap-3">
          <div>
            <h1 className="text-base sm:text-lg font-bold text-text truncate max-w-[200px] sm:max-w-md">
              {attempt.exam_title || t("exam.title", "4-Skill Mock Exam")}
            </h1>
            <div className="flex items-center gap-2 text-xs text-text-muted">
              <Badge variant="outline" className="text-[10px] uppercase font-semibold">
                {attempt.mode === "exam" ? t("exam.mode.exam", "Exam Mode") : t("exam.mode.practice", "Practice Mode")}
              </Badge>
              <span>•</span>
              <span>
                {t("exam.runner.answeredStatus", {
                  answered: answeredActivitiesCount,
                  total: totalActivitiesCount,
                  defaultValue: `${answeredActivitiesCount}/${totalActivitiesCount} answered`,
                })}
              </span>
            </div>
          </div>
        </div>

        {/* Center: Section Step Tabs */}
        <nav className="flex items-center gap-1 sm:gap-2">
          {sectionMeta.map(({ num, label, Icon }) => {
            const isCurrent = currentSectionNum === num;
            const isPassed = isExamMode && num < currentSectionNum;
            const isLocked = isExamMode && num > currentSectionNum;

            return (
              <button
                key={num}
                type="button"
                disabled={isLocked || isPassed}
                onClick={() => handleSwitchSectionPractice(num)}
                className={cn(
                  "flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs font-medium transition-all min-h-[44px]",
                  isCurrent
                    ? "bg-primary text-white shadow-xs"
                    : isPassed
                      ? "bg-surface-muted text-text-muted opacity-60 cursor-not-allowed line-through"
                      : isLocked
                        ? "text-text-muted opacity-40 cursor-not-allowed"
                        : "text-text-muted hover:bg-surface-muted hover:text-text",
                )}
              >
                <Icon className="h-4 w-4 shrink-0" />
                <span className="hidden md:inline">{label}</span>
                <span className="md:hidden">{num}</span>
              </button>
            );
          })}
        </nav>

        {/* Right: Server Countdown Timer & Submit Button */}
        <div className="flex items-center gap-3">
          <div
            className={cn(
              "flex items-center gap-2 px-3 py-1.5 rounded-lg font-mono text-sm sm:text-base font-bold transition-colors",
              isLowTime
                ? "bg-danger/10 text-danger border border-danger/30 animate-pulse"
                : "bg-surface-muted text-text border border-border-subtle",
            )}
            title={t("exam.runner.remainingTime", "Time Remaining")}
          >
            <Clock className="h-4 w-4 shrink-0" />
            <span>{formatTimer(remainingSeconds)}</span>
          </div>

          <Button
            type="button"
            variant="outline"
            onClick={() => setShowSubmitModal(true)}
            className="min-h-[44px] text-xs sm:text-sm font-semibold border-primary text-primary hover:bg-primary/10"
          >
            <Send className="h-4 w-4 mr-1.5" />
            <span>{t("exam.runner.finishExam", "Submit Exam")}</span>
          </Button>
        </div>
      </header>

      {/* Save Error Alert */}
      {saveError && (
        <div className="bg-danger/10 text-danger px-4 py-2 text-xs flex items-center justify-between border-b border-danger/20">
          <div className="flex items-center gap-2">
            <AlertCircle className="h-4 w-4 shrink-0" />
            <span>{saveError}</span>
          </div>
          <Button
            type="button"
            size="sm"
            variant="ghost"
            onClick={() => void saveCurrentDrafts()}
            className="text-xs text-danger underline min-h-[32px]"
          >
            {t("common.retry", "Retry save")}
          </Button>
        </div>
      )}

      {/* Main Section Content Area */}
      <main className="flex-1 max-w-5xl w-full mx-auto p-4 sm:p-6 md:p-8 space-y-6">
        {/* Section Title Banner */}
        <div className="flex flex-wrap items-center justify-between gap-3 border-b border-border pb-4">
          <div>
            <div className="text-xs uppercase font-bold tracking-wider text-primary">
              {t("exam.runner.sectionLabel", { num: currentSectionNum, defaultValue: `Section ${currentSectionNum} of 4` })}
            </div>
            <h2 className="text-xl sm:text-2xl font-bold text-text capitalize">
              {currentSection?.skill}
            </h2>
          </div>

          <div className="flex items-center gap-2 text-xs text-text-muted">
            {isAutoSaving ? (
              <span className="flex items-center gap-1.5 text-primary">
                <Save className="h-3.5 w-3.5 animate-spin" />
                {t("exam.runner.saving", "Saving answers...")}
              </span>
            ) : (
              <span className="flex items-center gap-1.5 text-success">
                <CheckCircle2 className="h-3.5 w-3.5" />
                {t("exam.runner.saved", "All answers saved")}
              </span>
            )}
          </div>
        </div>

        {/* Section Activities List */}
        <div className="space-y-8">
          {currentSection?.activities?.map((act, index) => (
            <SittingActivityCard
              key={act.id}
              activity={act}
              index={index + 1}
              attemptId={attempt.id}
              mode={attempt.mode}
              draftAnswer={draftAnswers[act.id]}
              onUpdateAnswer={(ans) => handleUpdateActivityAnswer(act.id, ans)}
            />
          ))}
        </div>

        {/* Section Navigation Footer */}
        <div className="pt-8 border-t border-border flex flex-wrap items-center justify-between gap-4">
          <div className="text-xs text-text-muted">
            {isExamMode ? (
              <span>
                {t(
                  "exam.runner.forwardOnlyNotice",
                  "Note: Advancing will permanently lock this section in Exam Mode.",
                )}
              </span>
            ) : (
              <span>{t("exam.runner.practiceNavNotice", "You can freely revisit sections in Practice Mode.")}</span>
            )}
          </div>

          <div className="flex items-center gap-3">
            {currentSectionNum < 4 ? (
              <Button
                type="button"
                onClick={handleNextSection}
                className="bg-primary text-white hover:bg-primary/90 font-semibold min-h-[44px] px-6 gap-2"
              >
                <span>{t("exam.runner.nextSection", "Next Section")}</span>
                <ArrowRight className="h-4 w-4" />
              </Button>
            ) : (
              <Button
                type="button"
                onClick={() => setShowSubmitModal(true)}
                className="bg-success text-white hover:bg-success/90 font-semibold min-h-[44px] px-6 gap-2"
              >
                <span>{t("exam.runner.finishAndSubmit", "Complete & Submit")}</span>
                <CheckCircle2 className="h-4 w-4" />
              </Button>
            )}
          </div>
        </div>
      </main>

      {/* Submit Confirmation Dialog */}
      {showSubmitModal && (
        <div
          role="dialog"
          aria-modal="true"
          className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm p-4 animate-in fade-in duration-150"
        >
          <div className="max-w-md w-full rounded-2xl bg-card border border-border p-6 shadow-2xl space-y-4">
            <h3 className="text-lg font-bold text-text">
              {t("exam.runner.submitConfirmTitle", "Ready to submit your exam?")}
            </h3>

            <div className="bg-surface-muted/60 p-4 rounded-xl space-y-2 text-sm">
              <div className="flex justify-between">
                <span className="text-text-muted">{t("exam.runner.answeredQuestions", "Answered Questions:")}</span>
                <span className="font-semibold text-text">
                  {answeredActivitiesCount} / {totalActivitiesCount}
                </span>
              </div>
              {answeredActivitiesCount < totalActivitiesCount && (
                <p className="text-xs text-warning font-medium">
                  {t(
                    "exam.runner.unansweredWarning",
                    "You have unanswered items. Unanswered questions score 0 points.",
                  )}
                </p>
              )}
            </div>

            <p className="text-xs text-text-muted leading-relaxed">
              {t(
                "exam.runner.submitNotice",
                "Once submitted, your answers will be graded and your score report will be generated. You cannot change your responses after submitting.",
              )}
            </p>

            <div className="flex justify-end gap-3 pt-2">
              <Button
                type="button"
                variant="outline"
                onClick={() => setShowSubmitModal(false)}
                disabled={isSubmitting}
                className="min-h-[44px]"
              >
                {t("common.cancel", "Cancel")}
              </Button>
              <Button
                type="button"
                onClick={handleSubmitExam}
                disabled={isSubmitting}
                className="bg-primary text-white hover:bg-primary/90 font-semibold min-h-[44px] px-5"
              >
                {isSubmitting
                  ? t("exam.runner.submitting", "Submitting...")
                  : t("exam.runner.confirmSubmit", "Confirm & Submit")}
              </Button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
};

// ---------------------------------------------------------------------------
// Activity Renderer Card
// ---------------------------------------------------------------------------
interface SittingActivityCardProps {
  activity: SittingActivity;
  index: number;
  attemptId: string;
  mode: "exam" | "practice";
  draftAnswer: any;
  onUpdateAnswer: (answer: any) => void;
}

const SittingActivityCard: React.FC<SittingActivityCardProps> = ({
  activity,
  index,
  attemptId,
  mode,
  draftAnswer,
  onUpdateAnswer,
}) => {
  const { t } = useTranslation();
  const config = activity.config || {};

  // -------------------------------------------------------------------------
  // 1. Listening Comprehension
  // -------------------------------------------------------------------------
  if (activity.kind === "listening_comprehension") {
    const questions = config.questions || [];
    const currentAnswers = draftAnswer?.answers || {};

    const handleOptionSelect = (qId: string, optId: string) => {
      onUpdateAnswer({
        answers: {
          ...currentAnswers,
          [qId]: optId,
        },
      });
    };

    return (
      <div className="rounded-2xl border border-border bg-card p-5 sm:p-7 shadow-xs space-y-6">
        <div className="flex items-center gap-2">
          <Badge variant="outline" className="text-xs px-2.5 py-0.5 font-bold">
            #{index}
          </Badge>
          <h3 className="font-bold text-base text-text">
            {config.title || t("exam.listening.audioClip", "Audio Clip")}
          </h3>
        </div>

        {/* Audio Player */}
        <ListeningPlayer
          versionId={activity.content_version_id}
          contextId={attemptId}
          title={config.title}
        />

        {/* Multi-Questions List */}
        <div className="space-y-6 pt-2">
          {questions.map((q: any, qIdx: number) => (
            <div key={q.id || qIdx} className="space-y-3 bg-surface-muted/30 p-4 rounded-xl border border-border-subtle">
              <p className="text-sm font-semibold text-text">
                {qIdx + 1}. {q.prompt}
              </p>

              <div className="grid grid-cols-1 sm:grid-cols-2 gap-2.5">
                {(q.options || []).map((opt: any) => {
                  const isSelected = currentAnswers[q.id] === opt.id;
                  return (
                    <button
                      key={opt.id}
                      type="button"
                      onClick={() => handleOptionSelect(q.id, opt.id)}
                      className={cn(
                        "flex items-center gap-3 p-3 rounded-lg border text-left text-sm transition-all min-h-[44px]",
                        isSelected
                          ? "border-primary bg-primary/10 text-primary font-medium shadow-xs"
                          : "border-border bg-card text-text hover:bg-surface-muted",
                      )}
                    >
                      <span
                        className={cn(
                          "flex h-6 w-6 shrink-0 items-center justify-center rounded-full text-xs font-bold border",
                          isSelected
                            ? "bg-primary text-white border-primary"
                            : "border-border-subtle bg-surface-muted text-text-muted",
                        )}
                      >
                        {opt.id}
                      </span>
                      <span className="min-w-0 flex-1">{opt.text}</span>
                    </button>
                  );
                })}
              </div>
            </div>
          ))}
        </div>
      </div>
    );
  }

  // -------------------------------------------------------------------------
  // 2. Reading Comprehension
  // -------------------------------------------------------------------------
  if (activity.kind === "reading_comprehension") {
    const questions = config.questions || [];
    const currentAnswers = draftAnswer?.answers || {};

    const handleOptionSelect = (qId: string, optId: string) => {
      onUpdateAnswer({
        answers: {
          ...currentAnswers,
          [qId]: optId,
        },
      });
    };

    return (
      <div className="rounded-2xl border border-border bg-card p-5 sm:p-7 shadow-xs space-y-6">
        <div className="flex items-center gap-2">
          <Badge variant="outline" className="text-xs px-2.5 py-0.5 font-bold">
            #{index}
          </Badge>
          <h3 className="font-bold text-base text-text">
            {config.title || t("exam.reading.passageTitle", "Reading Passage")}
          </h3>
        </div>

        {/* Passage Box */}
        <div className="rounded-xl bg-surface-muted/60 p-5 border border-border-subtle max-h-96 overflow-y-auto leading-relaxed text-sm sm:text-base text-text select-none">
          <p className="whitespace-pre-line">{config.passage}</p>
        </div>

        {/* Questions List */}
        <div className="space-y-6 pt-2">
          {questions.map((q: any, qIdx: number) => (
            <div key={q.id || qIdx} className="space-y-3 bg-surface-muted/30 p-4 rounded-xl border border-border-subtle">
              <p className="text-sm font-semibold text-text">
                {qIdx + 1}. {q.prompt}
              </p>

              <div className="grid grid-cols-1 sm:grid-cols-2 gap-2.5">
                {(q.options || []).map((opt: any) => {
                  const isSelected = currentAnswers[q.id] === opt.id;
                  return (
                    <button
                      key={opt.id}
                      type="button"
                      onClick={() => handleOptionSelect(q.id, opt.id)}
                      className={cn(
                        "flex items-center gap-3 p-3 rounded-lg border text-left text-sm transition-all min-h-[44px]",
                        isSelected
                          ? "border-primary bg-primary/10 text-primary font-medium shadow-xs"
                          : "border-border bg-card text-text hover:bg-surface-muted",
                      )}
                    >
                      <span
                        className={cn(
                          "flex h-6 w-6 shrink-0 items-center justify-center rounded-full text-xs font-bold border",
                          isSelected
                            ? "bg-primary text-white border-primary"
                            : "border-border-subtle bg-surface-muted text-text-muted",
                        )}
                      >
                        {opt.id}
                      </span>
                      <span className="min-w-0 flex-1">{opt.text}</span>
                    </button>
                  );
                })}
              </div>
            </div>
          ))}
        </div>
      </div>
    );
  }

  // -------------------------------------------------------------------------
  // 3. Writing: Essay Prompt
  // -------------------------------------------------------------------------
  if (activity.kind === "writing_prompt") {
    const submissionText = draftAnswer?.submission || "";
    const minWords = config.min_words || 150;
    const currentWordCount = submissionText.trim()
      ? submissionText.trim().split(/\s+/).length
      : 0;

    return (
      <div className="rounded-2xl border border-border bg-card p-5 sm:p-7 shadow-xs space-y-4">
        <div className="flex flex-wrap items-center justify-between gap-3">
          <div className="flex items-center gap-2">
            <Badge variant="outline" className="text-xs px-2.5 py-0.5 font-bold">
              #{index}
            </Badge>
            <h3 className="font-bold text-base text-text">
              {config.topic || t("exam.writing.essayPrompt", "Essay Writing Task")}
            </h3>
          </div>

          <Badge
            variant={currentWordCount >= minWords ? "success" : "outline"}
            className="text-xs font-mono px-3 py-1"
          >
            {currentWordCount} / {minWords} {t("exam.writing.words", "words")}
          </Badge>
        </div>

        <div className="rounded-xl bg-surface-muted p-4 border border-border-subtle">
          <p className="text-sm sm:text-base font-medium text-text leading-relaxed">
            {config.prompt}
          </p>
        </div>

        <div className="space-y-1.5">
          <textarea
            rows={10}
            value={submissionText}
            onChange={(e) => onUpdateAnswer({ submission: e.target.value })}
            placeholder={t(
              "exam.writing.placeholder",
              "Write your essay here. Support your argument with clear reasons and relevant examples...",
            )}
            className="w-full rounded-xl border border-border bg-card p-4 text-sm sm:text-base leading-relaxed text-text placeholder:text-text-muted focus:outline-hidden focus:ring-2 focus:ring-primary transition-all resize-y min-h-[220px]"
          />
        </div>
      </div>
    );
  }

  // -------------------------------------------------------------------------
  // 4. Writing: Sentence Transform / Rewrite
  // -------------------------------------------------------------------------
  if (activity.kind === "grammar_sentence_transform") {
    const currentAnswer = draftAnswer?.answer || "";

    return (
      <div className="rounded-2xl border border-border bg-card p-5 sm:p-6 shadow-xs space-y-4">
        <div className="flex items-center gap-2">
          <Badge variant="outline" className="text-xs px-2.5 py-0.5 font-bold">
            #{index}
          </Badge>
          <h3 className="font-bold text-base text-text">
            {t("exam.writing.sentenceTransform", "Sentence Transformation")}
          </h3>
        </div>

        <div className="rounded-xl bg-surface-muted p-4 border border-border-subtle">
          <p className="text-sm sm:text-base font-medium text-text leading-relaxed">
            {config.prompt}
          </p>
        </div>

        <div className="space-y-1.5">
          <label className="text-xs font-medium text-text-muted">
            {t("exam.writing.yourAnswer", "Your Rewritten Sentence:")}
          </label>
          <input
            type="text"
            value={currentAnswer}
            onChange={(e) => onUpdateAnswer({ answer: e.target.value })}
            placeholder={t("exam.writing.transformPlaceholder", "Type rewritten sentence here...")}
            className="w-full rounded-xl border border-border bg-card px-4 py-3 text-sm text-text placeholder:text-text-muted focus:outline-hidden focus:ring-2 focus:ring-primary min-h-[44px]"
          />
        </div>
      </div>
    );
  }

  // -------------------------------------------------------------------------
  // 5. Speaking Task (Read Aloud or Respond)
  // -------------------------------------------------------------------------
  if (activity.kind === "speaking_task") {
    const taskType = config.task_type === "read_aloud" ? "read_aloud" : "respond";
    const recordingKey = draftAnswer?.recording_key;

    return (
      <div className="space-y-2">
        <div className="flex items-center gap-2 mb-2">
          <Badge variant="outline" className="text-xs px-2.5 py-0.5 font-bold">
            #{index}
          </Badge>
          <h3 className="font-bold text-base text-text">
            {taskType === "read_aloud"
              ? t("exam.speaking.taskReadAloud", "Speaking: Read Aloud")
              : t("exam.speaking.taskRespond", "Speaking: Spoken Response")}
          </h3>
        </div>

        <SpeakingRecorder
          taskType={taskType}
          promptText={config.prompt}
          referenceText={config.reference_text}
          speakingTimeSeconds={config.speaking_time_seconds || 45}
          mode={mode}
          currentRecordingKey={recordingKey}
          onRecordingComplete={(key) => onUpdateAnswer({ recording_key: key })}
        />
      </div>
    );
  }

  return null;
};
