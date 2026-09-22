import React, { useState } from "react";
import {
  BookOpen,
  Calendar,
  Check,
  ChevronDown,
  Clock,
  Headphones,
  Info,
  Layers,
  ListChecks,
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
import {
  examApi,
  problemCode,
  useExams,
  useUserExamAttempts,
} from "../api/examApi";
import { EXAM_PARTS, partsOfSection, type ExamPart } from "../parts";
import {
  EXAM_LEVELS,
  type ExamLevel,
  type ExamSkill,
  type ExamStatus,
  type ExamTemplate,
} from "../types";

export interface ExamListProps {
  userPracticeLevel?: string | undefined;
  className?: string | undefined;
}

const PRACTICE_DEFAULT_MINUTES = 60;

const SECTION_ICONS: Record<ExamSkill, typeof Headphones> = {
  listening: Headphones,
  reading: BookOpen,
  writing: PenTool,
  speaking: Mic,
};

interface SectionSummary {
  position: number;
  skill: ExamSkill;
  items: number;
}

function toLevel(value: string | undefined): ExamLevel {
  return EXAM_LEVELS.find((level) => level === value) ?? "B1";
}

/** The exam's sections, from the template when it carries them. */
function sectionsOf(exam: ExamTemplate): SectionSummary[] {
  if (exam.sections && exam.sections.length > 0) {
    return [...exam.sections]
      .sort((a, b) => a.position - b.position)
      .map((section) => ({
        position: section.position,
        skill: section.skill,
        items: section.item_count,
      }));
  }
  const bySection = new Map<number, SectionSummary>();
  for (const part of EXAM_PARTS) {
    const entry = bySection.get(part.section) ?? {
      position: part.section,
      skill: part.skill,
      items: 0,
    };
    entry.items += part.items;
    bySection.set(part.section, entry);
  }
  return [...bySection.values()];
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
  const [unlimited, setUnlimited] = useState(false);
  const [chosenSections, setChosenSections] = useState<number[]>([]);
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

  const openPractice = (exam: ExamTemplate) => {
    setMinutes(PRACTICE_DEFAULT_MINUTES);
    setUnlimited(false);
    setChosenSections(sectionsOf(exam).map((section) => section.position));
    setPracticeExam(exam);
  };

  const start = async (exam: ExamTemplate, mode: "exam" | "practice") => {
    setIsStarting(true);
    setStartError(null);
    const allSections = sectionsOf(exam).length;
    try {
      const attempt = await examApi.startSitting(
        exam.id,
        mode === "practice"
          ? {
              mode,
              ...(unlimited
                ? { unlimited: true }
                : { chosen_duration_minutes: minutes }),
              // Every section is the default; send the choice only when it narrows.
              ...(chosenSections.length < allSections && {
                sections: [...chosenSections].sort((a, b) => a - b),
              }),
            }
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

  const toggleSection = (position: number) =>
    setChosenSections((prev) =>
      prev.includes(position)
        ? prev.filter((p) => p !== position)
        : [...prev, position],
    );

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
          <div className="flex shrink-0 items-center gap-4 rounded-2xl border border-border bg-surface-card p-4 sm:p-5">
            <Clock className="h-6 w-6 text-primary-accent" aria-hidden="true" />
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
          className="flex items-center gap-2 rounded-xl border border-danger/20 bg-danger/10 p-4 text-xs text-danger-accent sm:text-sm"
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
                  : "border-border bg-surface-card text-text hover:bg-surface-muted",
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
          <p className="rounded-2xl border border-border bg-surface-card p-12 text-center text-sm text-text-muted">
            {t("exam.hub.loading")}
          </p>
        ) : exams.isError ? (
          <p
            role="alert"
            className="rounded-2xl border border-danger/20 bg-danger/10 p-6 text-center text-sm text-danger-accent"
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
            <ExamCard
              key={exam.id}
              exam={exam}
              title={vi ? exam.title_vi : exam.title_en}
              startDisabled={startDisabled}
              onStartExam={() => void start(exam, "exam")}
              onPractice={() => openPractice(exam)}
            />
          ))
        )}
      </section>

      <section className="space-y-4 pt-4">
        <h2 className="text-base font-bold text-text sm:text-lg">
          {t("exam.hub.history")}
        </h2>
        {attempts.isLoading ? (
          <p className="rounded-xl border border-border bg-surface-card p-6 text-center text-xs text-text-muted">
            {t("exam.hub.loading")}
          </p>
        ) : (attempts.data?.items ?? []).length === 0 ? (
          <p className="rounded-xl border border-border-subtle bg-surface-muted/30 p-8 text-center text-xs text-text-muted">
            {t("exam.hub.noHistory")}
          </p>
        ) : (
          <ul className="divide-y divide-border overflow-hidden rounded-2xl border border-border bg-surface-card">
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
          className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 p-4 pb-20 md:pb-4"
        >
          <div className="max-h-[calc(90vh-5rem)] w-full max-w-lg space-y-5 overflow-y-auto rounded-2xl border border-border bg-surface-card p-5 shadow-2xl sm:p-6 md:max-h-[90vh]">
            <div className="space-y-1.5">
              <h3 id="practice-title" className="text-lg font-bold text-text">
                {t("exam.hub.practiceTitle")}
              </h3>
              <p className="text-xs leading-relaxed text-text-muted">
                {t("exam.hub.practiceBody")}
              </p>
            </div>

            <fieldset className="space-y-3">
              <div className="flex flex-wrap items-center justify-between gap-2">
                <legend className="text-sm font-semibold text-text">
                  {t("exam.parts.chooseSections")}
                </legend>
                <Button
                  type="button"
                  size="sm"
                  variant="ghost"
                  onClick={() =>
                    setChosenSections(
                      sectionsOf(practiceExam).map((s) => s.position),
                    )
                  }
                >
                  {t("exam.parts.selectAll")}
                </Button>
              </div>
              <ul className="space-y-2">
                {sectionsOf(practiceExam).map((section) => {
                  const checked = chosenSections.includes(section.position);
                  return (
                    <li key={section.position}>
                      <button
                        type="button"
                        role="checkbox"
                        aria-checked={checked}
                        onClick={() => toggleSection(section.position)}
                        className={cn(
                          "flex min-h-[44px] w-full items-start gap-3 rounded-xl border p-3 text-left",
                          checked
                            ? "border-primary bg-primary/10"
                            : "border-border-subtle bg-surface-muted/40 hover:bg-surface-muted",
                        )}
                      >
                        <span
                          className={cn(
                            "mt-0.5 flex h-5 w-5 shrink-0 items-center justify-center rounded border",
                            checked
                              ? "border-primary bg-primary text-primary-fg"
                              : "border-border bg-surface-card",
                          )}
                          aria-hidden="true"
                        >
                          {checked && <Check className="h-3.5 w-3.5" />}
                        </span>
                        <SectionParts section={section} />
                      </button>
                    </li>
                  );
                })}
              </ul>
              {chosenSections.length === 0 && (
                <p className="text-xs font-medium text-warning-accent">
                  {t("exam.parts.noneSelected")}
                </p>
              )}
            </fieldset>

            <div className="space-y-3 rounded-xl border border-border-subtle bg-surface-muted/50 p-4">
              <label
                htmlFor="practice-minutes"
                className="flex items-center justify-between text-sm font-semibold text-text"
              >
                <span>{t("exam.hub.practiceDuration")}</span>
                <span className="font-mono text-primary-accent">
                  {unlimited
                    ? t("exam.hub.practiceUnlimited")
                    : t("exam.hub.minutes", { count: minutes })}
                </span>
              </label>
              <input
                id="practice-minutes"
                type="range"
                min={10}
                max={180}
                step={5}
                value={minutes}
                disabled={unlimited}
                onChange={(e) => setMinutes(Number(e.target.value))}
                className={cn(
                  "h-11 w-full cursor-pointer accent-primary",
                  unlimited && "cursor-not-allowed opacity-40",
                )}
              />
              <button
                type="button"
                role="checkbox"
                aria-checked={unlimited}
                onClick={() => setUnlimited((v) => !v)}
                className="flex min-h-[44px] w-full items-center gap-3 rounded-lg border border-border-subtle bg-surface-card px-3 text-left"
              >
                <span
                  className={cn(
                    "flex h-5 w-5 shrink-0 items-center justify-center rounded border",
                    unlimited
                      ? "border-primary bg-primary text-primary-fg"
                      : "border-border bg-surface-card",
                  )}
                  aria-hidden="true"
                >
                  {unlimited && <Check className="h-3.5 w-3.5" />}
                </span>
                <span className="text-sm text-text">
                  {t("exam.hub.practiceUnlimited")}
                </span>
              </button>
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
                disabled={isStarting || chosenSections.length === 0}
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

const ExamCard: React.FC<{
  exam: ExamTemplate;
  title: string;
  startDisabled: boolean;
  onStartExam: () => void;
  onPractice: () => void;
}> = ({ exam, title, startDisabled, onStartExam, onPractice }) => {
  const { t } = useTranslation();
  const [showStructure, setShowStructure] = useState(false);
  const sections = sectionsOf(exam);
  const items = sections.reduce((sum, section) => sum + section.items, 0);
  const structureId = `exam-structure-${exam.id}`;

  return (
    <div className="space-y-5 rounded-3xl border border-border bg-surface-card p-5 sm:p-8">
      <div className="flex flex-col justify-between gap-6 md:flex-row md:items-center">
        <div className="max-w-xl space-y-3">
          <div className="flex flex-wrap items-center gap-2.5">
            <Badge variant="outline">{exam.level}</Badge>
          </div>
          <h3 className="text-xl font-bold text-text sm:text-2xl">{title}</h3>
          <ul className="flex flex-wrap items-center gap-x-4 gap-y-1 text-xs text-text-muted sm:text-sm">
            <li className="flex items-center gap-1.5">
              <Clock className="h-3.5 w-3.5" aria-hidden="true" />
              {t("exam.hub.minutes", { count: exam.total_minutes })}
            </li>
            <li className="flex items-center gap-1.5">
              <Layers className="h-3.5 w-3.5" aria-hidden="true" />
              {t("exam.parts.summarySections", { count: sections.length })}
            </li>
            <li className="flex items-center gap-1.5">
              <ListChecks className="h-3.5 w-3.5" aria-hidden="true" />
              {t("exam.parts.summaryItems", { count: items })}
            </li>
          </ul>
        </div>
        <div className="flex shrink-0 flex-col gap-3 sm:flex-row md:flex-col">
          <Button type="button" disabled={startDisabled} onClick={onStartExam}>
            <Play className="h-4 w-4" aria-hidden="true" />
            <span>
              {t("exam.hub.startExam", { count: exam.total_minutes })}
            </span>
          </Button>
          <Button
            type="button"
            variant="outline"
            disabled={startDisabled}
            onClick={onPractice}
          >
            <Sliders className="h-4 w-4" aria-hidden="true" />
            <span>{t("exam.hub.startPractice")}</span>
          </Button>
        </div>
      </div>

      <div className="border-t border-border-subtle pt-3">
        <Button
          type="button"
          size="sm"
          variant="ghost"
          aria-expanded={showStructure}
          aria-controls={structureId}
          onClick={() => setShowStructure((open) => !open)}
        >
          <ChevronDown
            className={cn(
              "h-4 w-4 transition-transform",
              showStructure && "rotate-180",
            )}
            aria-hidden="true"
          />
          {t("exam.parts.structure")}
        </Button>
        {showStructure && (
          <ul id={structureId} className="mt-3 grid gap-3 md:grid-cols-2">
            {sections.map((section) => (
              <li
                key={section.position}
                className="rounded-xl border border-border-subtle bg-surface-muted/40 p-3"
              >
                <SectionParts section={section} />
              </li>
            ))}
          </ul>
        )}
      </div>
    </div>
  );
};

/** A section and the parts it holds: how many items, and tags for their form. */
const SectionParts: React.FC<{ section: SectionSummary }> = ({ section }) => {
  const { t } = useTranslation();
  const Icon = SECTION_ICONS[section.skill];
  return (
    <div className="min-w-0 flex-1 space-y-2">
      <p className="flex items-center gap-2 text-sm font-semibold text-text">
        <Icon
          className="h-4 w-4 shrink-0 text-primary-accent"
          aria-hidden="true"
        />
        {t("exam.parts.sectionLabel", {
          num: section.position,
          skill: t(`exam.sections.${section.skill}`),
        })}
      </p>
      <ul className="space-y-1.5">
        {partsOfSection(section.position).map((part) => (
          <PartLine key={part.key} part={part} />
        ))}
      </ul>
    </div>
  );
};

const PartLine: React.FC<{ part: ExamPart }> = ({ part }) => {
  const { t } = useTranslation();
  return (
    <li className="space-y-1 text-xs">
      <p className="text-text">
        <span className="font-medium">{t(`exam.parts.${part.key}`)}</span>
        <span className="text-text-muted">
          {" · "}
          {t("exam.parts.summaryItems", { count: part.items })}
          {part.minQuestionsPerItem !== undefined &&
            ` · ${t("exam.parts.atLeastQuestions", {
              count: part.items * part.minQuestionsPerItem,
            })}`}
        </span>
      </p>
      <p className="flex flex-wrap gap-1.5">
        {part.tags.map((tag) => (
          <span
            key={tag}
            className="rounded-full bg-primary/10 px-2 py-0.5 text-[11px] font-medium text-primary-accent"
          >
            #{t(`exam.parts.${tag}`)}
          </span>
        ))}
      </p>
    </li>
  );
};
