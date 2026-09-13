import React, { useMemo, useState } from "react";
import {
  BookOpen,
  Calendar,
  Clock,
  GraduationCap,
  Headphones,
  Info,
  Mic,
  PenTool,
  Play,
  Sliders,
} from "lucide-react";
import { useTranslation } from "react-i18next";
import { Link, useNavigate } from "@tanstack/react-router";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";
import { examApi, useExams, useUserExamAttempts } from "../api/examApi";
import type { ExamLevel, ExamTemplate } from "../types";

export interface ExamListProps {
  userPracticeLevel?: string | undefined;
  className?: string | undefined;
}

export const ExamList: React.FC<ExamListProps> = ({
  userPracticeLevel,
  className,
}) => {
  const { t } = useTranslation();
  const navigate = useNavigate();

  const [selectedLevel, setSelectedLevel] = useState<ExamLevel>(() => {
    if (userPracticeLevel === "A2" || userPracticeLevel === "B1" || userPracticeLevel === "B2") {
      return userPracticeLevel;
    }
    return "B1";
  });

  // Practice duration modal state
  const [practiceExam, setPracticeExam] = useState<ExamTemplate | null>(null);
  const [chosenDurationMinutes, setChosenDurationMinutes] = useState<number>(60);
  const [isStarting, setIsStarting] = useState<boolean>(false);
  const [startError, setStartError] = useState<string | null>(null);

  const { data: exams = [], isLoading: isLoadingExams } = useExams();
  const { data: attemptsData, isLoading: isLoadingAttempts } = useUserExamAttempts(1, 10);

  // Daily limit estimate (count today's attempts)
  const todaySittingsCount = useMemo(() => {
    if (!attemptsData?.attempts) return 0;
    const now = new Date();
    const todayStr = `${now.getFullYear()}-${String(now.getMonth() + 1).padStart(2, "0")}-${String(now.getDate()).padStart(2, "0")}`;
    return attemptsData.attempts.filter((att) => {
      const attDate = new Date(att.started_at);
      const attDateStr = `${attDate.getFullYear()}-${String(attDate.getMonth() + 1).padStart(2, "0")}-${String(attDate.getDate()).padStart(2, "0")}`;
      return attDateStr === todayStr;
    }).length;
  }, [attemptsData]);

  const sittingsLeft = Math.max(0, 5 - todaySittingsCount);

  // Filter exams by selected CEFR level
  const filteredExams = useMemo(() => {
    return exams.filter((ex) => ex.level === selectedLevel);
  }, [exams, selectedLevel]);

  const handleStartExamMode = async (exam: ExamTemplate) => {
    setIsStarting(true);
    setStartError(null);
    try {
      const attempt = await examApi.startSitting(exam.id, {
        mode: "exam",
      });
      void navigate({
        to: "/exams/$attemptId",
        params: { attemptId: attempt.id },
      });
    } catch (err: any) {
      const isDailyLimit =
        err?.problem?.code === "EXAM_DAILY_LIMIT_REACHED" ||
        err?.status === 429 ||
        err?.message?.includes("EXAM_DAILY_LIMIT_REACHED");
      if (isDailyLimit) {
        setStartError(
          t(
            "exam.hub.dailyLimitReached",
            "Daily limit reached (5 sittings per day). Please return tomorrow for more attempts.",
          ),
        );
      } else {
        setStartError(err?.message || t("exam.hub.startFailed", "Unable to start exam sitting."));
      }
      setIsStarting(false);
    }
  };

  const handleStartPracticeMode = async () => {
    if (!practiceExam) return;
    setIsStarting(true);
    setStartError(null);
    try {
      const attempt = await examApi.startSitting(practiceExam.id, {
        mode: "practice",
        chosen_duration_minutes: chosenDurationMinutes,
      });
      setPracticeExam(null);
      void navigate({
        to: "/exams/$attemptId",
        params: { attemptId: attempt.id },
      });
    } catch (err: any) {
      setStartError(err?.message || t("exam.hub.startFailed", "Unable to start practice sitting."));
      setIsStarting(false);
    }
  };

  const levels: ExamLevel[] = ["A2", "B1", "B2"];

  return (
    <div className={cn("max-w-5xl mx-auto space-y-10 p-4 sm:p-6 md:p-8", className)}>
      {/* Hero Hub Header */}
      <div className="flex flex-col md:flex-row md:items-center md:justify-between gap-6 pb-6 border-b border-border">
        <div className="space-y-2">
          <div className="flex items-center gap-2">
            <Badge variant="outline" className="text-xs uppercase font-semibold text-primary px-3 py-1">
              <GraduationCap className="h-3.5 w-3.5 mr-1.5" />
              {t("exam.hub.badge", "4-Skill Mock Exams")}
            </Badge>
          </div>
          <h1 className="text-2xl sm:text-4xl font-extrabold text-text tracking-tight">
            {t("exam.hub.title", "Standardized Exam Simulation")}
          </h1>
          <p className="text-xs sm:text-sm text-text-muted max-w-xl leading-relaxed">
            {t(
              "exam.hub.subtitle",
              "Comprehensive 4-skill testing: Listening clips, Reading passages, Essay writing, and Spoken audio responses uniquely generated from our pool.",
            )}
          </p>
        </div>

        {/* Daily Limit Tracker */}
        <div className="rounded-2xl border border-border bg-card p-4 sm:p-5 shadow-xs flex items-center gap-4 shrink-0">
          <div className="flex h-12 w-12 items-center justify-center rounded-xl bg-primary/10 text-primary">
            <Clock className="h-6 w-6" />
          </div>
          <div>
            <div className="text-xs text-text-muted font-medium">
              {t("exam.hub.dailySittings", "Daily Sittings Remaining")}
            </div>
            <div className="text-xl sm:text-2xl font-bold font-mono text-text">
              {sittingsLeft} / 5
            </div>
          </div>
        </div>
      </div>

      {/* Start Error Alert */}
      {startError && (
        <div className="flex items-center gap-2 text-xs sm:text-sm text-danger bg-danger/10 p-4 rounded-xl border border-danger/20">
          <Info className="h-4 w-4 shrink-0" />
          <span>{startError}</span>
        </div>
      )}

      {/* Level Selection Tabs */}
      <div className="space-y-4">
        <div className="flex items-center justify-between">
          <h2 className="text-base sm:text-lg font-bold text-text">
            {t("exam.hub.chooseLevel", "Select CEFR Proficiency Level")}
          </h2>
          {userPracticeLevel && (
            <span className="text-xs text-text-muted">
              {t("exam.hub.recommendedLevel", { level: userPracticeLevel, defaultValue: `Recommended: ${userPracticeLevel}` })}
            </span>
          )}
        </div>

        <div className="grid grid-cols-3 gap-3 max-w-md">
          {levels.map((lvl) => {
            const isSelected = selectedLevel === lvl;
            return (
              <button
                key={lvl}
                type="button"
                onClick={() => setSelectedLevel(lvl)}
                className={cn(
                  "flex flex-col items-center justify-center py-3 px-4 rounded-xl border text-sm font-bold transition-all min-h-[44px]",
                  isSelected
                    ? "border-primary bg-primary text-white shadow-sm scale-102"
                    : "border-border bg-card text-text hover:bg-surface-muted",
                )}
              >
                <span>{lvl}</span>
                <span className="text-[10px] font-normal opacity-80">
                  {lvl === "A2" ? "Elementary" : lvl === "B1" ? "Intermediate" : "Upper-Int"}
                </span>
              </button>
            );
          })}
        </div>
      </div>

      {/* Available Exam Cards */}
      <div className="space-y-4">
        <h2 className="text-base sm:text-lg font-bold text-text">
          {t("exam.hub.availableExams", "Available Mock Exams")}
        </h2>

        {isLoadingExams ? (
          <div className="rounded-2xl border border-border bg-card p-12 text-center text-text-muted text-sm">
            {t("app.loading", "Loading exams...")}
          </div>
        ) : filteredExams.length === 0 ? (
          /* Empty Pool State handling (Requirement §7 & §8) */
          <div className="rounded-2xl border border-border-subtle bg-surface-muted/50 p-10 text-center space-y-3">
            <div className="flex h-12 w-12 mx-auto items-center justify-center rounded-2xl bg-surface-muted text-text-muted">
              <BookOpen className="h-6 w-6" />
            </div>
            <h3 className="font-bold text-text text-base">
              {t("exam.hub.notAvailableYetTitle", "Exams not available yet at this level")}
            </h3>
            <p className="text-xs text-text-muted max-w-md mx-auto">
              {t(
                "exam.hub.notAvailableYetDesc",
                "Our hourly pool generator is actively assembling verified listening, reading, writing, and speaking items for this level. Please check back soon or try another level.",
              )}
            </p>
          </div>
        ) : (
          <div className="grid grid-cols-1 gap-6">
            {filteredExams.map((exam) => (
              <div
                key={exam.id}
                className="rounded-3xl border border-border bg-card p-6 sm:p-8 shadow-xs flex flex-col md:flex-row md:items-center justify-between gap-6"
              >
                <div className="space-y-4 max-w-xl">
                  <div className="flex flex-wrap items-center gap-2.5">
                    <Badge variant="outline" className="font-bold text-xs">
                      CEFR {exam.level}
                    </Badge>
                    <Badge variant="outline" className="text-xs text-text-muted">
                      {exam.total_minutes} {t("exam.hub.minutes", "mins")}
                    </Badge>
                    <span className="text-xs text-text-muted">• 4 skills</span>
                  </div>

                  <div>
                    <h3 className="text-xl sm:text-2xl font-bold text-text">
                      {exam.title_en || exam.slug}
                    </h3>
                    <p className="text-xs sm:text-sm text-text-muted mt-1 leading-relaxed">
                      {exam.description_en || t("exam.hub.defaultDesc", "Complete 4-skill mock exam including listening clips, reading comprehension passages, essay writing, and voice recording speaking tasks.")}
                    </p>
                  </div>

                  {/* Skills Pills */}
                  <div className="flex flex-wrap gap-2 pt-1 text-xs text-text-muted">
                    <span className="flex items-center gap-1 bg-surface-muted px-2.5 py-1 rounded-md">
                      <Headphones className="h-3.5 w-3.5 text-primary" /> Listening (3 clips)
                    </span>
                    <span className="flex items-center gap-1 bg-surface-muted px-2.5 py-1 rounded-md">
                      <BookOpen className="h-3.5 w-3.5 text-primary" /> Reading (2 passages)
                    </span>
                    <span className="flex items-center gap-1 bg-surface-muted px-2.5 py-1 rounded-md">
                      <PenTool className="h-3.5 w-3.5 text-primary" /> Writing (1 essay + 3 rewrites)
                    </span>
                    <span className="flex items-center gap-1 bg-surface-muted px-2.5 py-1 rounded-md">
                      <Mic className="h-3.5 w-3.5 text-primary" /> Speaking (4 tasks)
                    </span>
                  </div>
                </div>

                {/* Actions: Exam vs Practice Mode */}
                <div className="flex flex-col sm:flex-row md:flex-col gap-3 shrink-0">
                  <Button
                    type="button"
                    disabled={isStarting || sittingsLeft <= 0}
                    onClick={() => handleStartExamMode(exam)}
                    className="bg-primary text-white hover:bg-primary/90 font-semibold min-h-[44px] px-6 gap-2 shadow-xs"
                  >
                    <Play className="h-4 w-4 fill-current" />
                    <span>{t("exam.hub.startExamMode", "Exam Mode (75 min)")}</span>
                  </Button>

                  <Button
                    type="button"
                    variant="outline"
                    disabled={isStarting || sittingsLeft <= 0}
                    onClick={() => {
                      setPracticeExam(exam);
                      setChosenDurationMinutes(60);
                    }}
                    className="font-semibold min-h-[44px] px-6 gap-2 border-border hover:bg-surface-muted"
                  >
                    <Sliders className="h-4 w-4" />
                    <span>{t("exam.hub.practiceMode", "Practice Mode (Custom)")}</span>
                  </Button>
                </div>
              </div>
            ))}
          </div>
        )}
      </div>

      {/* Past Attempts History */}
      <div className="space-y-4 pt-4">
        <h2 className="text-base sm:text-lg font-bold text-text">
          {t("exam.hub.pastAttempts", "Your Exam History")}
        </h2>

        {isLoadingAttempts ? (
          <div className="rounded-xl border border-border bg-card p-6 text-center text-text-muted text-xs">
            {t("app.loading", "Loading past attempts...")}
          </div>
        ) : !attemptsData?.attempts || attemptsData.attempts.length === 0 ? (
          <div className="rounded-xl border border-border-subtle bg-surface-muted/30 p-8 text-center text-xs text-text-muted">
            {t("exam.hub.noAttemptsYet", "You have not completed any mock exams yet. Start your first session above!")}
          </div>
        ) : (
          <div className="rounded-2xl border border-border bg-card overflow-hidden shadow-xs divide-y divide-border">
            {attemptsData.attempts.map((att) => (
              <div
                key={att.id}
                className="p-4 sm:p-5 flex flex-wrap items-center justify-between gap-4 hover:bg-surface-muted/40 transition-colors"
              >
                <div className="space-y-1">
                  <div className="flex items-center gap-2">
                    <span className="font-bold text-sm text-text">
                      {att.exam_title || t("exam.title", "4-Skill Mock Exam")}
                    </span>
                    <Badge variant="outline" className="text-[10px] uppercase">
                      {att.mode}
                    </Badge>
                  </div>
                  <div className="flex items-center gap-2 text-xs text-text-muted">
                    <Calendar className="h-3 w-3" />
                    <span>{new Date(att.started_at).toLocaleDateString()}</span>
                    <span>•</span>
                    <span className="capitalize">{att.status}</span>
                  </div>
                </div>

                <div className="flex items-center gap-3">
                  {att.status === "in_progress" ? (
                    <Link to="/exams/$attemptId" params={{ attemptId: att.id }}>
                      <Button
                        type="button"
                        size="sm"
                        className="bg-primary text-white text-xs min-h-[38px] px-4 font-semibold"
                      >
                        {t("exam.hub.resume", "Resume Sitting")}
                      </Button>
                    </Link>
                  ) : (
                    <Link to="/exams/$attemptId/report" params={{ attemptId: att.id }}>
                      <Button
                        type="button"
                        variant="outline"
                        size="sm"
                        className="text-xs min-h-[38px] px-4 font-semibold"
                      >
                        {t("exam.hub.viewReport", "View Report")}
                      </Button>
                    </Link>
                  )}
                </div>
              </div>
            ))}
          </div>
        )}
      </div>

      {/* Practice Duration Picker Modal (10–180 minutes) */}
      {practiceExam && (
        <div
          role="dialog"
          aria-modal="true"
          className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm p-4 animate-in fade-in duration-150"
        >
          <div className="max-w-md w-full rounded-2xl bg-card border border-border p-6 shadow-2xl space-y-6">
            <div className="space-y-1.5">
              <h3 className="text-lg font-bold text-text">
                {t("exam.hub.practiceDurationTitle", "Customize Practice Duration")}
              </h3>
              <p className="text-xs text-text-muted leading-relaxed">
                {t(
                  "exam.hub.practiceDurationDesc",
                  "In Practice Mode, you can choose any time limit from 10 to 180 minutes and freely navigate between sections.",
                )}
              </p>
            </div>

            {/* Duration Slider / Picker */}
            <div className="space-y-4 bg-surface-muted/50 p-5 rounded-xl border border-border-subtle">
              <div className="flex justify-between items-center">
                <span className="text-xs font-semibold text-text-muted uppercase tracking-wider">
                  {t("exam.hub.chosenDuration", "Time Limit:")}
                </span>
                <span className="font-mono text-2xl font-bold text-primary">
                  {chosenDurationMinutes} {t("exam.hub.mins", "mins")}
                </span>
              </div>

              <input
                type="range"
                min={10}
                max={180}
                step={5}
                value={chosenDurationMinutes}
                onChange={(e) => setChosenDurationMinutes(Number(e.target.value))}
                className="w-full accent-primary h-2 bg-border-subtle rounded-lg cursor-pointer"
              />

              <div className="flex justify-between text-[11px] text-text-muted font-mono">
                <span>10 min</span>
                <span>60 min</span>
                <span>120 min</span>
                <span>180 min</span>
              </div>
            </div>

            <div className="flex justify-end gap-3 pt-2">
              <Button
                type="button"
                variant="outline"
                onClick={() => setPracticeExam(null)}
                disabled={isStarting}
                className="min-h-[44px]"
              >
                {t("common.cancel", "Cancel")}
              </Button>
              <Button
                type="button"
                onClick={handleStartPracticeMode}
                disabled={isStarting}
                className="bg-primary text-white hover:bg-primary/90 font-semibold min-h-[44px] px-6 gap-2"
              >
                <Play className="h-4 w-4 fill-current" />
                <span>{isStarting ? t("app.starting", "Starting...") : t("exam.hub.startPractice", "Start Practice")}</span>
              </Button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
};
