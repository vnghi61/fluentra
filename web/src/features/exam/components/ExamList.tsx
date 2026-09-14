import React, { useState } from "react";
import { BookOpen, Calendar, Clock, Info, Play, Sliders } from "lucide-react";
import { useTranslation } from "react-i18next";
import { Link, useNavigate } from "@tanstack/react-router";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";
import {
  examApi,
  problemCode,
  useExams,
  useUserExamAttempts,
} from "../api/examApi";
import {
  EXAM_LEVELS,
  type ExamLevel,
  type ExamStatus,
  type ExamTemplate,
} from "../types";

export interface ExamListProps {
  userPracticeLevel?: string | undefined;
  className?: string | undefined;
}

const PRACTICE_DEFAULT_MINUTES = 60;

function toLevel(value: string | undefined): ExamLevel {
  return EXAM_LEVELS.find((level) => level === value) ?? "B1";
}

export const ExamList: React.FC<ExamListProps> = ({
  userPracticeLevel,
  className,
}) => {
  const { t, i18n } = useTranslation();
  const navigate = useNavigate();
  const vi = i18n.language.startsWith("vi");

  const [level, setLevel] = useState<ExamLevel>(() =>
    toLevel(userPracticeLevel),
  );
  const [unavailable, setUnavailable] = useState<ReadonlySet<ExamLevel>>(
    () => new Set(),
  );
  const [practiceExam, setPracticeExam] = useState<ExamTemplate | null>(null);
  const [minutes, setMinutes] = useState(PRACTICE_DEFAULT_MINUTES);
  const [isStarting, setIsStarting] = useState(false);
  const [startError, setStartError] = useState<string | null>(null);

  const exams = useExams();
  const attempts = useUserExamAttempts(10, 0);

  const sittingsToday = attempts.data?.sittings_today ?? 0;
  const dailyLimit = attempts.data?.daily_limit;
  const sittingsLeft =
    dailyLimit === undefined
      ? undefined
      : Math.max(0, dailyLimit - sittingsToday);
  const hasOpenSitting = (attempts.data?.items ?? []).some(
    (a) => a.status === "in_progress",
  );

  const levelExams = (exams.data ?? []).filter((exam) => exam.level === level);
  const levelUnavailable =
    unavailable.has(level) || (exams.isSuccess && levelExams.length === 0);

  const start = async (exam: ExamTemplate, mode: "exam" | "practice") => {
    setIsStarting(true);
    setStartError(null);
    try {
      const attempt = await examApi.startSitting(
        exam.id,
        mode === "practice"
          ? { mode, chosen_duration_minutes: minutes }
          : { mode },
      );
      setPracticeExam(null);
      void navigate({
        to: "/exams/$attemptId",
        params: { attemptId: attempt.id },
      });
    } catch (err: unknown) {
      const code = problemCode(err);
      if (code === "EXAM_POOL_EMPTY" || code === "INSUFFICIENT_ITEMS") {
        // Not a failure: the pool does not hold a sitting's worth yet.
        setUnavailable((prev) => new Set(prev).add(exam.level));
        setPracticeExam(null);
      } else if (code === "EXAM_DAILY_LIMIT_REACHED") {
        setStartError(t("exam.hub.dailyLimitReached"));
      } else if (code === "ATTEMPT_IN_PROGRESS") {
        setStartError(t("exam.hub.sittingInProgress"));
      } else {
        setStartError(t("exam.hub.startFailed"));
      }
    } finally {
      setIsStarting(false);
    }
  };

  const statusLabels: Record<ExamStatus, string> = {
    in_progress: t("exam.hub.statusInProgress"),
    completed: t("exam.hub.statusCompleted"),
    expired: t("exam.hub.statusExpired"),
  };
  const levelNames: Record<ExamLevel, string> = {
    A2: t("exam.hub.levelA2"),
    B1: t("exam.hub.levelB1"),
    B2: t("exam.hub.levelB2"),
  };
  const startDisabled = isStarting || sittingsLeft === 0 || hasOpenSitting;

  return (
    <div
      className={cn(
        "mx-auto max-w-5xl space-y-10 p-4 sm:p-6 md:p-8",
        className,
      )}
    >
      <div className="flex flex-col gap-6 border-b border-border pb-6 md:flex-row md:items-center md:justify-between">
        <div className="space-y-2">
          <h1 className="text-2xl font-extrabold tracking-tight text-text sm:text-4xl">
            {t("exam.hub.title")}
          </h1>
          <p className="max-w-xl text-xs leading-relaxed text-text-muted sm:text-sm">
            {t("exam.hub.subtitle")}
          </p>
        </div>
        {sittingsLeft !== undefined && dailyLimit !== undefined && (
          <div className="flex shrink-0 items-center gap-4 rounded-2xl border border-border bg-card p-4 sm:p-5">
            <Clock className="h-6 w-6 text-primary" aria-hidden="true" />
            <p className="text-sm font-medium text-text">
              {t("exam.hub.sittingsLeft", {
                left: sittingsLeft,
                limit: dailyLimit,
              })}
            </p>
          </div>
        )}
      </div>

      {startError && (
        <div
          role="alert"
          className="flex items-center gap-2 rounded-xl border border-danger/20 bg-danger/10 p-4 text-xs text-danger sm:text-sm"
        >
          <Info className="h-4 w-4 shrink-0" aria-hidden="true" />
          <span>{startError}</span>
        </div>
      )}

      <section className="space-y-4">
        <div className="flex flex-wrap items-center justify-between gap-2">
          <h2 className="text-base font-bold text-text sm:text-lg">
            {t("exam.hub.chooseLevel")}
          </h2>
          {userPracticeLevel && (
            <span className="text-xs text-text-muted">
              {t("exam.hub.practiceLevel", { level: userPracticeLevel })}
            </span>
          )}
        </div>
        <div
          className="grid max-w-md grid-cols-3 gap-3"
          role="group"
          aria-label={t("exam.hub.chooseLevel")}
        >
          {EXAM_LEVELS.map((lvl) => (
            <button
              key={lvl}
              type="button"
              aria-pressed={level === lvl}
              onClick={() => setLevel(lvl)}
              className={cn(
                "flex min-h-[44px] flex-col items-center justify-center rounded-xl border px-4 py-3 text-sm font-bold",
                level === lvl
                  ? "border-primary bg-primary text-primary-fg"
                  : "border-border bg-card text-text hover:bg-surface-muted",
              )}
            >
              <span>{lvl}</span>
              <span className="text-[11px] font-normal">{levelNames[lvl]}</span>
            </button>
          ))}
        </div>
      </section>

      <section className="space-y-4">
        {exams.isLoading ? (
          <p className="rounded-2xl border border-border bg-card p-12 text-center text-sm text-text-muted">
            {t("exam.hub.loading")}
          </p>
        ) : exams.isError ? (
          <p
            role="alert"
            className="rounded-2xl border border-danger/20 bg-danger/10 p-6 text-center text-sm text-danger"
          >
            {t("exam.hub.loadFailed")}
          </p>
        ) : levelUnavailable ? (
          <div className="space-y-3 rounded-2xl border border-border-subtle bg-surface-muted/50 p-10 text-center">
            <BookOpen
              className="mx-auto h-6 w-6 text-text-muted"
              aria-hidden="true"
            />
            <h2 className="text-base font-bold text-text">
              {t("exam.hub.notAvailableTitle")}
            </h2>
            <p className="mx-auto max-w-md text-xs text-text-muted">
              {t("exam.hub.notAvailableBody")}
            </p>
          </div>
        ) : (
          levelExams.map((exam) => (
            <div
              key={exam.id}
              className="flex flex-col justify-between gap-6 rounded-3xl border border-border bg-card p-6 sm:p-8 md:flex-row md:items-center"
            >
              <div className="max-w-xl space-y-3">
                <div className="flex flex-wrap items-center gap-2.5">
                  <Badge variant="outline">{exam.level}</Badge>
                  <Badge variant="outline">
                    {t("exam.hub.minutes", { count: exam.total_minutes })}
                  </Badge>
                </div>
                <h3 className="text-xl font-bold text-text sm:text-2xl">
                  {vi ? exam.title_vi : exam.title_en}
                </h3>
                <p className="text-xs leading-relaxed text-text-muted sm:text-sm">
                  {t("exam.hub.composition")}
                </p>
              </div>
              <div className="flex shrink-0 flex-col gap-3 sm:flex-row md:flex-col">
                <Button
                  type="button"
                  disabled={startDisabled}
                  onClick={() => void start(exam, "exam")}
                >
                  <Play className="h-4 w-4" aria-hidden="true" />
                  <span>
                    {t("exam.hub.startExam", { count: exam.total_minutes })}
                  </span>
                </Button>
                <Button
                  type="button"
                  variant="outline"
                  disabled={startDisabled}
                  onClick={() => {
                    setMinutes(PRACTICE_DEFAULT_MINUTES);
                    setPracticeExam(exam);
                  }}
                >
                  <Sliders className="h-4 w-4" aria-hidden="true" />
                  <span>{t("exam.hub.startPractice")}</span>
                </Button>
              </div>
            </div>
          ))
        )}
      </section>

      <section className="space-y-4 pt-4">
        <h2 className="text-base font-bold text-text sm:text-lg">
          {t("exam.hub.history")}
        </h2>
        {attempts.isLoading ? (
          <p className="rounded-xl border border-border bg-card p-6 text-center text-xs text-text-muted">
            {t("exam.hub.loading")}
          </p>
        ) : (attempts.data?.items ?? []).length === 0 ? (
          <p className="rounded-xl border border-border-subtle bg-surface-muted/30 p-8 text-center text-xs text-text-muted">
            {t("exam.hub.noHistory")}
          </p>
        ) : (
          <ul className="divide-y divide-border overflow-hidden rounded-2xl border border-border bg-card">
            {(attempts.data?.items ?? []).map((att) => (
              <li
                key={att.id}
                className="flex flex-wrap items-center justify-between gap-4 p-4 sm:p-5"
              >
                <div className="space-y-1">
                  <div className="flex flex-wrap items-center gap-2">
                    <span className="text-sm font-bold text-text">
                      {att.exam_title || t("exam.title")}
                    </span>
                    <Badge variant="outline">
                      {att.mode === "exam"
                        ? t("exam.mode.exam")
                        : t("exam.mode.practice")}
                    </Badge>
                  </div>
                  <div className="flex items-center gap-2 text-xs text-text-muted">
                    <Calendar className="h-3 w-3" aria-hidden="true" />
                    <span>
                      {new Date(att.started_at).toLocaleDateString(
                        i18n.language,
                      )}
                    </span>
                    <span aria-hidden="true">·</span>
                    <span>{statusLabels[att.status]}</span>
                  </div>
                </div>
                {att.status === "in_progress" ? (
                  <Link
                    to="/exams/$attemptId"
                    params={{ attemptId: att.id }}
                    className="inline-flex min-h-[44px] items-center rounded-lg bg-primary px-4 text-xs font-semibold text-primary-fg"
                  >
                    {t("exam.hub.resume")}
                  </Link>
                ) : (
                  <Link
                    to="/exams/$attemptId/report"
                    params={{ attemptId: att.id }}
                    className="inline-flex min-h-[44px] items-center rounded-lg border border-border px-4 text-xs font-semibold text-text"
                  >
                    {t("exam.hub.viewReport")}
                  </Link>
                )}
              </li>
            ))}
          </ul>
        )}
      </section>

      {practiceExam && (
        <div
          role="dialog"
          aria-modal="true"
          aria-labelledby="practice-title"
          className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 p-4"
        >
          <div className="w-full max-w-md space-y-6 rounded-2xl border border-border bg-card p-6 shadow-2xl">
            <div className="space-y-1.5">
              <h3 id="practice-title" className="text-lg font-bold text-text">
                {t("exam.hub.practiceTitle")}
              </h3>
              <p className="text-xs leading-relaxed text-text-muted">
                {t("exam.hub.practiceBody")}
              </p>
            </div>
            <div className="space-y-3 rounded-xl border border-border-subtle bg-surface-muted/50 p-5">
              <label
                htmlFor="practice-minutes"
                className="flex items-center justify-between text-sm font-semibold text-text"
              >
                <span>{t("exam.hub.practiceDuration")}</span>
                <span className="font-mono text-primary">
                  {t("exam.hub.minutes", { count: minutes })}
                </span>
              </label>
              <input
                id="practice-minutes"
                type="range"
                min={10}
                max={180}
                step={5}
                value={minutes}
                onChange={(e) => setMinutes(Number(e.target.value))}
                className="h-11 w-full cursor-pointer accent-primary"
              />
            </div>
            <div className="flex justify-end gap-3">
              <Button
                type="button"
                variant="outline"
                onClick={() => setPracticeExam(null)}
                disabled={isStarting}
              >
                {t("exam.hub.cancel")}
              </Button>
              <Button
                type="button"
                onClick={() => void start(practiceExam, "practice")}
                disabled={isStarting}
              >
                <Play className="h-4 w-4" aria-hidden="true" />
                <span>{t("exam.hub.practiceStart")}</span>
              </Button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
};
