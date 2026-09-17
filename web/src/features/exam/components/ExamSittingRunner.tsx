import React, {
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
} from "react";
import {
  AlertCircle,
  ArrowRight,
  BookOpen,
  CheckCircle2,
  Clock,
  Headphones,
  ListOrdered,
  Mic,
  PenTool,
  Send,
} from "lucide-react";
import { useTranslation } from "react-i18next";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";
import { examApi, problemCode } from "../api/examApi";
import { partOf, questionSlots, slotAnchor, type QuestionSlot } from "../parts";
import type {
  ChoiceQuestion,
  DraftAnswers,
  ExamAttempt,
  ExamMode,
  ExamSkill,
  IntegrityEvent,
  IntegrityKind,
  SaveAnswersRequest,
  SectionActivities,
  SittingActivity,
  SittingAnswer,
} from "../types";
import { ListeningPlayer } from "./ListeningPlayer";
import { SpeakingRecorder } from "./SpeakingRecorder";

const AUTOSAVE_MS = 15_000;
const DEFAULT_SPEAKING_SECONDS = 45;
// The server closes a section five seconds after its end; ask again after that.
const SECTION_RESYNC_DELAY_MS = 6_000;

const SECTION_ICONS: Record<ExamSkill, typeof Headphones> = {
  listening: Headphones,
  reading: BookOpen,
  writing: PenTool,
  speaking: Mic,
};

export interface ExamSittingRunnerProps {
  attempt: ExamAttempt;
  onSubmitted: () => void;
}

export function isAnswered(answer: SittingAnswer | undefined): boolean {
  if (!answer) return false;
  if ("answers" in answer) return Object.keys(answer.answers).length > 0;
  if ("text_answer" in answer) return answer.text_answer.trim() !== "";
  if ("answer" in answer) return answer.answer.trim() !== "";
  return answer.audio_object_key !== "";
}

export function formatClock(totalSeconds: number): string {
  const safe = Math.max(0, Math.floor(totalSeconds));
  const hours = Math.floor(safe / 3600);
  const minutes = Math.floor((safe % 3600) / 60)
    .toString()
    .padStart(2, "0");
  const seconds = (safe % 60).toString().padStart(2, "0");
  return hours > 0 ? `${hours}:${minutes}:${seconds}` : `${minutes}:${seconds}`;
}

/** Scrolls to a question once the section holding it has rendered. */
function scrollToSlot(slot: QuestionSlot) {
  requestAnimationFrame(() =>
    requestAnimationFrame(() => {
      document
        .getElementById(slotAnchor(slot))
        ?.scrollIntoView?.({ behavior: "smooth", block: "center" });
    }),
  );
}

export const ExamSittingRunner: React.FC<ExamSittingRunnerProps> = ({
  attempt,
  onSubmitted,
}) => {
  const { t } = useTranslation();
  const isExamMode = attempt.mode === "exam";
  const inProgress = attempt.status === "in_progress";
  const sections = useMemo(
    () => attempt.section_activities ?? [],
    [attempt.section_activities],
  );

  const [currentSection, setCurrentSection] = useState(
    attempt.current_section || 1,
  );
  const [answers, setAnswers] = useState<DraftAnswers>(
    attempt.draft_answers ?? {},
  );
  const [remaining, setRemaining] = useState(attempt.remaining_seconds);
  const [sectionRemaining, setSectionRemaining] = useState<number | undefined>(
    attempt.section_remaining_seconds,
  );
  const [saveState, setSaveState] = useState<"saved" | "saving" | "failed">(
    "saved",
  );
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [timeUp, setTimeUp] = useState(false);
  const [confirmOpen, setConfirmOpen] = useState(false);
  const [listOpen, setListOpen] = useState(false);

  const answersRef = useRef<DraftAnswers>(attempt.draft_answers ?? {});
  const dirtyRef = useRef(new Set<string>());
  const eventsRef = useRef<IntegrityEvent[]>([]);
  const sectionRef = useRef(attempt.current_section || 1);
  const remainingRef = useRef(attempt.remaining_seconds);
  const sectionRemainingRef = useRef(attempt.section_remaining_seconds);
  const finishedRef = useRef(false);

  const sectionOf = useMemo(() => {
    const positions = new Map<string, number>();
    for (const section of sections) {
      for (const activity of section.activities) {
        positions.set(activity.id, section.section_position);
      }
    }
    return positions;
  }, [sections]);

  const setClock = useCallback((seconds: number) => {
    remainingRef.current = seconds;
    setRemaining(seconds);
  }, []);

  const setSection = useCallback(
    (section: number, secondsLeft: number | undefined) => {
      sectionRef.current = section;
      sectionRemainingRef.current = secondsLeft;
      setCurrentSection(section);
      setSectionRemaining(secondsLeft);
    },
    [],
  );

  const finish = useCallback(() => {
    if (finishedRef.current) return;
    finishedRef.current = true;
    onSubmitted();
  }, [onSubmitted]);

  const resync = useCallback(async () => {
    try {
      const fresh = await examApi.getAttempt(attempt.id);
      if (fresh.status !== "in_progress") {
        finish();
        return;
      }
      setClock(fresh.remaining_seconds);
      if (isExamMode) {
        setSection(fresh.current_section, fresh.section_remaining_seconds);
      }
    } catch {
      // The next autosave or tick asks again.
    }
  }, [attempt.id, finish, isExamMode, setClock, setSection]);

  const save = useCallback(async (): Promise<boolean> => {
    // In exam mode only the open section's answers are sent; the server refuses the rest.
    const ids = [...dirtyRef.current].filter(
      (id) => !isExamMode || sectionOf.get(id) === sectionRef.current,
    );
    const events = eventsRef.current;
    if (ids.length === 0 && events.length === 0) return true;

    const payload: DraftAnswers = {};
    for (const id of ids) {
      const answer = answersRef.current[id];
      if (answer) payload[id] = answer;
    }
    eventsRef.current = [];
    setSaveState("saving");

    const request: SaveAnswersRequest = {
      answers: payload,
      integrity_events: events,
    };
    if (isExamMode) request.section_number = sectionRef.current;

    try {
      const res = await examApi.saveAnswers(attempt.id, request);
      for (const id of ids) {
        if (answersRef.current[id] === payload[id]) dirtyRef.current.delete(id);
      }
      setClock(res.remaining_seconds);
      if (isExamMode) {
        setSection(res.current_section, res.section_remaining_seconds);
      }
      setSaveState("saved");
      return true;
    } catch (err: unknown) {
      const code = problemCode(err);
      if (code === "ATTEMPT_EXPIRED" || code === "EXAM_ALREADY_SUBMITTED") {
        finish();
        return false;
      }
      if (
        code === "SECTION_ALREADY_COMPLETED" ||
        code === "INVALID_SECTION_PROGRESSION"
      ) {
        for (const id of ids) dirtyRef.current.delete(id);
        await resync();
        setSaveState("saved");
        return false;
      }
      eventsRef.current = [...events, ...eventsRef.current];
      setSaveState("failed");
      return false;
    }
  }, [attempt.id, finish, isExamMode, resync, sectionOf, setClock, setSection]);

  const handleTimeUp = useCallback(async () => {
    setTimeUp(true);
    await save();
    try {
      await examApi.submitExam(attempt.id);
    } catch {
      // The server submits an expired sitting whether or not this call arrives.
    }
    finish();
  }, [attempt.id, finish, save]);

  const saveRef = useRef(save);
  const resyncRef = useRef(resync);
  const timeUpRef = useRef(handleTimeUp);
  useEffect(() => {
    saveRef.current = save;
    resyncRef.current = resync;
    timeUpRef.current = handleTimeUp;
  }, [save, resync, handleTimeUp]);

  // One clock, counted down locally and corrected by every server response.
  useEffect(() => {
    if (!inProgress) return undefined;
    const timer = setInterval(() => {
      const next = Math.max(0, remainingRef.current - 1);
      remainingRef.current = next;
      setRemaining(next);
      if (next === 0) {
        clearInterval(timer);
        void timeUpRef.current();
        return;
      }
      const sectionLeft = sectionRemainingRef.current;
      if (sectionLeft !== undefined && sectionLeft > 0) {
        const nextSection = sectionLeft - 1;
        sectionRemainingRef.current = nextSection;
        setSectionRemaining(nextSection);
        if (nextSection === 0) {
          setTimeout(() => void resyncRef.current(), SECTION_RESYNC_DELAY_MS);
        }
      }
    }, 1000);
    return () => clearInterval(timer);
  }, [inProgress]);

  useEffect(() => {
    if (!inProgress) return undefined;
    const interval = setInterval(() => void saveRef.current(), AUTOSAVE_MS);
    return () => clearInterval(interval);
  }, [inProgress]);

  // Integrity signals are recorded and shown in the report, never enforced.
  useEffect(() => {
    if (!inProgress) return undefined;
    const record = (kind: IntegrityKind) => {
      eventsRef.current.push({ kind });
    };
    const onVisibility = () => {
      if (document.visibilityState === "hidden") {
        record("tab_hidden");
      } else {
        void resyncRef.current();
      }
    };
    const onBlur = () => record("window_blurred");
    const onPaste = () => record("paste");
    document.addEventListener("visibilitychange", onVisibility);
    window.addEventListener("blur", onBlur);
    document.addEventListener("paste", onPaste);
    return () => {
      document.removeEventListener("visibilitychange", onVisibility);
      window.removeEventListener("blur", onBlur);
      document.removeEventListener("paste", onPaste);
    };
  }, [inProgress]);

  const updateAnswer = (activityId: string, answer: SittingAnswer) => {
    const next = { ...answersRef.current, [activityId]: answer };
    answersRef.current = next;
    dirtyRef.current.add(activityId);
    setAnswers(next);
  };

  const handleRecorded = (activityId: string, objectKey: string) => {
    updateAnswer(activityId, { audio_object_key: objectKey });
    void save();
  };

  const goToSection = (target: number) => {
    void save();
    setSection(target, undefined);
  };

  // A practice sitting may hold only some sections, so "next" and "last" are
  // positions in the sitting, not in the four sections an exam has.
  const sectionIndex = Math.max(
    0,
    sections.findIndex((s) => s.section_position === currentSection),
  );
  const section = sections[sectionIndex];
  const nextPosition = sections[sectionIndex + 1]?.section_position;

  const handleNext = async () => {
    await save();
    if (!isExamMode) {
      if (nextPosition !== undefined) goToSection(nextPosition);
      return;
    }
    try {
      const res = await examApi.completeSection(attempt.id, sectionRef.current);
      if (res.submitted) {
        finish();
        return;
      }
      setClock(res.remaining_seconds);
      setSection(res.current_section, res.section_remaining_seconds);
    } catch (err: unknown) {
      const code = problemCode(err);
      if (code === "ATTEMPT_EXPIRED" || code === "EXAM_ALREADY_SUBMITTED") {
        finish();
      } else {
        await resync();
      }
    }
  };

  const handleSubmit = async () => {
    setIsSubmitting(true);
    await save();
    try {
      await examApi.submitExam(attempt.id);
      finish();
    } catch (err: unknown) {
      if (problemCode(err) === "EXAM_ALREADY_SUBMITTED") {
        finish();
        return;
      }
      setSaveState("failed");
      setIsSubmitting(false);
    }
  };

  const slots = useMemo(
    () => questionSlots(sections, answers),
    [sections, answers],
  );
  const firstNumbers = useMemo(() => {
    const numbers = new Map<string, number>();
    for (const slot of slots) {
      if (!numbers.has(slot.activityId)) {
        numbers.set(slot.activityId, slot.number);
      }
    }
    return numbers;
  }, [slots]);
  const total = slots.length;
  const answered = slots.filter((slot) => slot.answered).length;

  const jumpTo = (slot: QuestionSlot) => {
    if (slot.section !== currentSection) {
      // Exam mode never reopens or skips ahead to a section.
      if (isExamMode) return;
      goToSection(slot.section);
    }
    scrollToSlot(slot);
  };

  const sectionLabels: Record<ExamSkill, string> = {
    listening: t("exam.sections.listening"),
    reading: t("exam.sections.reading"),
    writing: t("exam.sections.writing"),
    speaking: t("exam.sections.speaking"),
  };

  return (
    <div className="flex min-h-screen flex-col bg-background text-text">
      {timeUp && (
        <div
          role="alertdialog"
          aria-modal="true"
          aria-labelledby="time-up-title"
          className="fixed inset-0 z-50 flex items-center justify-center bg-black/80 p-4 text-center"
        >
          <div className="max-w-md space-y-3 text-white">
            <Clock className="mx-auto h-12 w-12" aria-hidden="true" />
            <h2 id="time-up-title" className="text-2xl font-bold">
              {t("exam.runner.timeUpTitle")}
            </h2>
            <p className="text-sm">{t("exam.runner.timeUpBody")}</p>
          </div>
        </div>
      )}

      <header className="sticky top-0 z-40 flex flex-wrap items-center justify-between gap-3 border-b border-border bg-surface-card px-4 py-3 sm:px-8">
        <div className="min-w-0">
          <h1 className="truncate text-base font-bold text-text sm:text-lg">
            {attempt.exam_title || t("exam.title")}
          </h1>
          <div className="flex flex-wrap items-center gap-2 text-xs text-text-muted">
            <Badge variant="outline">
              {isExamMode ? t("exam.mode.exam") : t("exam.mode.practice")}
            </Badge>
            <span>{t("exam.runner.answered", { answered, total })}</span>
          </div>
        </div>

        <nav
          aria-label={t("exam.runner.sectionsNav")}
          className="flex items-center gap-1 sm:gap-2"
        >
          {sections.map((s) => {
            const Icon = SECTION_ICONS[s.skill];
            const isCurrent = s.section_position === currentSection;
            const locked = isExamMode && !isCurrent;
            return (
              <button
                key={s.section_position}
                type="button"
                disabled={locked}
                aria-current={isCurrent ? "step" : undefined}
                aria-label={sectionLabels[s.skill]}
                onClick={() => goToSection(s.section_position)}
                className={cn(
                  "flex min-h-[44px] min-w-[44px] items-center justify-center gap-1.5 rounded-lg px-3 text-xs font-medium",
                  isCurrent
                    ? "bg-primary text-primary-fg"
                    : locked
                      ? "cursor-not-allowed text-text-muted opacity-50"
                      : "text-text-muted hover:bg-surface-muted hover:text-text",
                )}
              >
                <Icon className="h-4 w-4 shrink-0" aria-hidden="true" />
                <span className="hidden md:inline">
                  {sectionLabels[s.skill]}
                </span>
              </button>
            );
          })}
        </nav>

        <div className="flex flex-wrap items-center gap-3">
          <div
            className={cn(
              "flex items-center gap-2 rounded-lg border px-3 py-1.5 font-mono text-sm font-bold sm:text-base",
              remaining < 120
                ? "border-danger/30 bg-danger/10 text-danger-accent"
                : "border-border-subtle bg-surface-muted text-text",
            )}
            role="timer"
            aria-label={t("exam.runner.timeLeft")}
          >
            <Clock className="h-4 w-4 shrink-0" aria-hidden="true" />
            <span>{formatClock(remaining)}</span>
          </div>
          <Button
            type="button"
            variant="outline"
            onClick={() => setConfirmOpen(true)}
          >
            <Send className="h-4 w-4" aria-hidden="true" />
            <span>{t("exam.runner.submit")}</span>
          </Button>
        </div>
      </header>

      {saveState === "failed" && (
        <div
          role="alert"
          className="flex flex-wrap items-center justify-between gap-2 border-b border-danger/20 bg-danger/10 px-4 py-2 text-xs text-danger-accent"
        >
          <span className="flex items-center gap-2">
            <AlertCircle className="h-4 w-4 shrink-0" aria-hidden="true" />
            {t("exam.runner.saveFailed")}
          </span>
          <Button
            type="button"
            size="sm"
            variant="ghost"
            onClick={() => void save()}
          >
            {t("exam.runner.retrySave")}
          </Button>
        </div>
      )}

      <div className="mx-auto grid w-full max-w-6xl flex-1 gap-6 p-4 sm:p-6 md:p-8 lg:grid-cols-[minmax(0,1fr)_16rem]">
        <div className="space-y-3 lg:order-2">
          <Button
            type="button"
            variant="outline"
            className="w-full lg:hidden"
            aria-expanded={listOpen}
            aria-controls="sitting-question-list"
            onClick={() => setListOpen((open) => !open)}
          >
            <ListOrdered className="h-4 w-4" aria-hidden="true" />
            {listOpen
              ? t("exam.parts.hideQuestionList")
              : t("exam.parts.showQuestionList")}
          </Button>
          <aside
            id="sitting-question-list"
            className={cn(
              "rounded-2xl border border-border bg-surface-card p-4 lg:sticky lg:top-24 lg:block",
              listOpen ? "block" : "hidden",
            )}
          >
            <QuestionList
              slots={slots}
              sections={sections}
              sectionLabels={sectionLabels}
              currentSection={currentSection}
              isExamMode={isExamMode}
              onJump={jumpTo}
            />
          </aside>
        </div>

        <main className="min-w-0 space-y-6 lg:order-1">
          {section && (
            <div className="flex flex-wrap items-center justify-between gap-3 border-b border-border pb-4">
              <div>
                <p className="text-xs font-bold uppercase tracking-wider text-primary-accent">
                  {t("exam.runner.sectionOf", {
                    num: sectionIndex + 1,
                    total: sections.length,
                  })}
                </p>
                <h2 className="text-xl font-bold text-text sm:text-2xl">
                  {sectionLabels[section.skill]}
                </h2>
              </div>
              <div className="flex flex-col items-end gap-1 text-xs text-text-muted">
                {isExamMode && sectionRemaining !== undefined && (
                  <span className="font-mono">
                    {t("exam.runner.sectionEndsIn", {
                      time: formatClock(sectionRemaining),
                    })}
                  </span>
                )}
                <span
                  className={cn(
                    "flex items-center gap-1.5",
                    saveState === "saving"
                      ? "text-primary-accent"
                      : "text-success-accent",
                  )}
                >
                  <CheckCircle2 className="h-3.5 w-3.5" aria-hidden="true" />
                  {saveState === "saving"
                    ? t("exam.runner.saving")
                    : t("exam.runner.saved")}
                </span>
              </div>
            </div>
          )}

          <div className="space-y-8">
            {section?.activities.map((activity) => (
              <SittingActivityCard
                key={activity.id}
                activity={activity}
                firstNumber={firstNumbers.get(activity.id) ?? 1}
                sittingId={attempt.id}
                mode={attempt.mode}
                answer={answers[activity.id]}
                onChange={(answer) => updateAnswer(activity.id, answer)}
                onRecorded={(key) => handleRecorded(activity.id, key)}
              />
            ))}
          </div>

          <div className="flex flex-wrap items-center justify-between gap-4 border-t border-border pt-8">
            <p className="text-xs text-text-muted">
              {isExamMode
                ? t("exam.runner.forwardOnly")
                : t("exam.runner.practiceNav")}
            </p>
            {nextPosition !== undefined ? (
              <Button type="button" onClick={() => void handleNext()}>
                <span>{t("exam.runner.nextSection")}</span>
                <ArrowRight className="h-4 w-4" aria-hidden="true" />
              </Button>
            ) : (
              <Button type="button" onClick={() => setConfirmOpen(true)}>
                <span>{t("exam.runner.finish")}</span>
                <CheckCircle2 className="h-4 w-4" aria-hidden="true" />
              </Button>
            )}
          </div>
        </main>
      </div>

      {confirmOpen && (
        <div
          role="dialog"
          aria-modal="true"
          aria-labelledby="submit-title"
          className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 p-4"
        >
          <div className="w-full max-w-md space-y-4 rounded-2xl border border-border bg-surface-card p-6 shadow-2xl">
            <h3 id="submit-title" className="text-lg font-bold text-text">
              {t("exam.runner.submitTitle")}
            </h3>
            <p className="text-sm text-text">
              {t("exam.runner.answered", { answered, total })}
            </p>
            {answered < total && (
              <p className="text-xs font-medium text-warning-accent">
                {t("exam.runner.unanswered")}
              </p>
            )}
            <p className="text-xs leading-relaxed text-text-muted">
              {t("exam.runner.submitBody")}
            </p>
            <div className="flex justify-end gap-3">
              <Button
                type="button"
                variant="outline"
                onClick={() => setConfirmOpen(false)}
                disabled={isSubmitting}
              >
                {t("exam.runner.cancel")}
              </Button>
              <Button
                type="button"
                onClick={() => void handleSubmit()}
                disabled={isSubmitting}
              >
                {isSubmitting
                  ? t("exam.runner.submitting")
                  : t("exam.runner.confirmSubmit")}
              </Button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
};

// ---------------------------------------------------------------------------

/**
 * Every question of the sitting as a numbered button, grouped by section, the
 * way a paper answer sheet is laid out: what is done, what is left, and a way
 * straight to any of them.
 */
const QuestionList: React.FC<{
  slots: QuestionSlot[];
  sections: SectionActivities[];
  sectionLabels: Record<ExamSkill, string>;
  currentSection: number;
  isExamMode: boolean;
  onJump: (slot: QuestionSlot) => void;
}> = ({
  slots,
  sections,
  sectionLabels,
  currentSection,
  isExamMode,
  onJump,
}) => {
  const { t } = useTranslation();
  return (
    <div className="space-y-4">
      <div className="space-y-2">
        <h2 className="text-sm font-bold text-text">
          {t("exam.parts.questionList")}
        </h2>
        <ul className="flex flex-wrap gap-3 text-[11px] text-text-muted">
          <li className="flex items-center gap-1.5">
            <span
              className="h-3 w-3 rounded-sm bg-primary"
              aria-hidden="true"
            />
            {t("exam.parts.answeredLegend")}
          </li>
          <li className="flex items-center gap-1.5">
            <span
              className="h-3 w-3 rounded-sm border border-border bg-surface-card"
              aria-hidden="true"
            />
            {t("exam.parts.unansweredLegend")}
          </li>
        </ul>
      </div>
      {sections.map((section) => {
        const locked =
          isExamMode && section.section_position !== currentSection;
        return (
          <div key={section.section_position} className="space-y-2">
            <h3 className="text-xs font-semibold uppercase tracking-wider text-text-muted">
              {sectionLabels[section.skill]}
            </h3>
            <div className="flex flex-wrap gap-1.5">
              {slots
                .filter((slot) => slot.section === section.section_position)
                .map((slot) => (
                  <button
                    key={slot.number}
                    type="button"
                    disabled={locked}
                    aria-label={t("exam.parts.goToQuestion", {
                      num: slot.number,
                    })}
                    onClick={() => onJump(slot)}
                    className={cn(
                      "flex h-11 min-w-11 items-center justify-center rounded-lg border px-2 font-mono text-sm font-semibold",
                      slot.answered
                        ? "border-primary bg-primary text-primary-fg"
                        : "border-border bg-surface-card text-text hover:bg-surface-muted",
                      locked && "cursor-not-allowed opacity-50",
                    )}
                  >
                    {slot.number}
                  </button>
                ))}
            </div>
          </div>
        );
      })}
    </div>
  );
};

interface ChoiceQuestionsProps {
  questions: ChoiceQuestion[];
  firstNumber: number;
  selected: Record<string, string>;
  onSelect: (questionId: string, optionId: string) => void;
}

const ChoiceQuestions: React.FC<ChoiceQuestionsProps> = ({
  questions,
  firstNumber,
  selected,
  onSelect,
}) => (
  <div className="space-y-6 pt-2">
    {questions.map((question, qIndex) => (
      <fieldset
        key={question.id}
        id={slotAnchor({ number: firstNumber + qIndex })}
        className="scroll-mt-28 space-y-3 rounded-xl border border-border-subtle bg-surface-muted/30 p-4"
      >
        <legend className="text-sm font-semibold text-text">
          {firstNumber + qIndex}. {question.prompt}
        </legend>
        <div className="grid grid-cols-1 gap-2.5 sm:grid-cols-2">
          {(question.options ?? []).map((option) => {
            const isSelected = selected[question.id] === option.id;
            return (
              <button
                key={option.id}
                type="button"
                aria-pressed={isSelected}
                onClick={() => onSelect(question.id, option.id)}
                className={cn(
                  "flex min-h-[44px] items-center gap-3 rounded-lg border p-3 text-left text-sm",
                  isSelected
                    ? "border-primary bg-primary/10 font-medium text-primary-accent"
                    : "border-border bg-surface-card text-text hover:bg-surface-muted",
                )}
              >
                <span className="flex h-6 w-6 shrink-0 items-center justify-center rounded-full border border-border-subtle text-xs font-bold">
                  {option.id}
                </span>
                <span className="min-w-0 flex-1">{option.text}</span>
              </button>
            );
          })}
        </div>
      </fieldset>
    ))}
  </div>
);

interface SittingActivityCardProps {
  activity: SittingActivity;
  firstNumber: number;
  sittingId: string;
  mode: ExamMode;
  answer: SittingAnswer | undefined;
  onChange: (answer: SittingAnswer) => void;
  onRecorded: (objectKey: string) => void;
}

const SittingActivityCard: React.FC<SittingActivityCardProps> = ({
  activity,
  firstNumber,
  sittingId,
  mode,
  answer,
  onChange,
  onRecorded,
}) => {
  const { t } = useTranslation();
  const config = activity.config ?? {};
  const questionCount = config.questions?.length ?? 0;
  const part = partOf(activity.kind, config);
  const cardClass =
    "scroll-mt-28 space-y-4 rounded-2xl border border-border bg-surface-card p-5 shadow-xs sm:p-7";
  // A comprehension set anchors each question; any other item anchors itself.
  const anchor =
    questionCount > 0 ? undefined : slotAnchor({ number: firstNumber });

  const heading = (title: string) => (
    <div className="flex flex-wrap items-center gap-2">
      <Badge variant="outline">
        {questionCount > 1
          ? t("exam.parts.questionRange", {
              from: firstNumber,
              to: firstNumber + questionCount - 1,
            })
          : t("exam.parts.questionOne", { num: firstNumber })}
      </Badge>
      <h3 className="text-base font-bold text-text">{title}</h3>
      {part && (
        <span className="rounded-full bg-primary/10 px-2 py-0.5 text-[11px] font-medium text-primary-accent">
          #{t(`exam.parts.${part}`)}
        </span>
      )}
    </div>
  );

  if (
    activity.kind === "listening_comprehension" ||
    activity.kind === "reading_comprehension"
  ) {
    const selected = answer && "answers" in answer ? answer.answers : {};
    const select = (questionId: string, optionId: string) =>
      onChange({ answers: { ...selected, [questionId]: optionId } });
    const isListening = activity.kind === "listening_comprehension";
    return (
      <div className={cardClass} id={anchor}>
        {heading(
          config.title ||
            (isListening
              ? t("exam.listening.audioClip")
              : t("exam.reading.passage")),
        )}
        {isListening ? (
          <ListeningPlayer
            versionId={activity.content_version_id}
            sittingId={sittingId}
            title={config.title}
          />
        ) : (
          <div className="max-h-96 overflow-y-auto rounded-xl border border-border-subtle bg-surface-muted/60 p-5 text-sm leading-relaxed text-text sm:text-base">
            <p className="whitespace-pre-line">{config.passage}</p>
          </div>
        )}
        <ChoiceQuestions
          questions={config.questions ?? []}
          firstNumber={firstNumber}
          selected={selected}
          onSelect={select}
        />
      </div>
    );
  }

  if (activity.kind === "writing_prompt") {
    const text = answer && "text_answer" in answer ? answer.text_answer : "";
    const minWords = config.min_words ?? 150;
    const words = text.trim() ? text.trim().split(/\s+/).length : 0;
    const inputId = `essay-${activity.id}`;
    return (
      <div className={cardClass} id={anchor}>
        <div className="flex flex-wrap items-center justify-between gap-3">
          {heading(config.topic || t("exam.writing.essay"))}
          <Badge variant={words >= minWords ? "success" : "outline"}>
            {t("exam.writing.words", { count: words, min: minWords })}
          </Badge>
        </div>
        <p className="rounded-xl border border-border-subtle bg-surface-muted p-4 text-sm font-medium leading-relaxed text-text sm:text-base">
          {config.prompt}
        </p>
        <label htmlFor={inputId} className="sr-only">
          {t("exam.writing.essay")}
        </label>
        <textarea
          id={inputId}
          rows={10}
          value={text}
          onChange={(e) => onChange({ text_answer: e.target.value })}
          placeholder={t("exam.writing.essayPlaceholder")}
          className="min-h-[220px] w-full resize-y rounded-xl border border-border bg-surface-card p-4 text-base leading-relaxed text-text placeholder:text-text-muted focus:outline-hidden focus:ring-2 focus:ring-primary"
        />
      </div>
    );
  }

  if (activity.kind === "grammar_sentence_transform") {
    const value = answer && "answer" in answer ? answer.answer : "";
    const inputId = `rewrite-${activity.id}`;
    return (
      <div className={cardClass} id={anchor}>
        {heading(t("exam.writing.rewrite"))}
        <p className="rounded-xl border border-border-subtle bg-surface-muted p-4 text-sm font-medium leading-relaxed text-text sm:text-base">
          {config.prompt}
        </p>
        <label
          htmlFor={inputId}
          className="text-xs font-medium text-text-muted"
        >
          {t("exam.writing.rewriteLabel")}
        </label>
        <input
          id={inputId}
          type="text"
          value={value}
          onChange={(e) => onChange({ answer: e.target.value })}
          className="min-h-[44px] w-full rounded-xl border border-border bg-surface-card px-4 py-3 text-base text-text focus:outline-hidden focus:ring-2 focus:ring-primary"
        />
      </div>
    );
  }

  if (activity.kind === "speaking_task") {
    const taskType =
      config.task_type === "read_aloud" ? "read_aloud" : "respond";
    const key =
      answer && "audio_object_key" in answer
        ? answer.audio_object_key
        : undefined;
    return (
      <div className="scroll-mt-28 space-y-2" id={anchor}>
        {heading(
          taskType === "read_aloud"
            ? t("exam.speaking.readAloudTitle")
            : t("exam.speaking.respondTitle"),
        )}
        <SpeakingRecorder
          taskType={taskType}
          promptText={config.prompt}
          referenceText={config.reference_text}
          speakingTimeSeconds={
            config.speaking_time_seconds ?? DEFAULT_SPEAKING_SECONDS
          }
          mode={mode}
          currentRecordingKey={key}
          onRecordingComplete={onRecorded}
        />
      </div>
    );
  }

  return null;
};
