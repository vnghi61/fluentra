import React, { useState } from "react";
import {
  AlertTriangle,
  BookOpen,
  CheckCircle2,
  Flag,
  Headphones,
  Info,
  Loader2,
  Mic,
  PenTool,
  ShieldAlert,
  Trash2,
  XCircle,
} from "lucide-react";
import { useTranslation } from "react-i18next";
import { Link } from "@tanstack/react-router";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { ReportDialog } from "@/features/learning/components/Runner/ReportDialog";
import { useWritingFeedback } from "@/features/writing/api/writingApi";
import { cn } from "@/lib/utils";
import { examApi, problemCode, useSpeakingFeedback } from "../api/examApi";
import { EXAM_PARTS, partOf, type PartKey } from "../parts";
import type {
  ExamItemOutcome,
  ExamSectionOutcome,
  ExamSkill,
  IntegrityKind,
  ItemStatus,
  ScoreReport,
} from "../types";
import {
  ExamItemReview,
  reviewStates,
  type ReviewState,
} from "./ExamItemReview";

export interface ExamReportProps {
  report: ScoreReport;
  className?: string | undefined;
}

const SECTION_ICONS: Record<ExamSkill, typeof Headphones> = {
  listening: Headphones,
  reading: BookOpen,
  writing: PenTool,
  speaking: Mic,
};

interface NumberedItem {
  item: ExamItemOutcome;
  firstNumber: number;
  states: ReviewState[];
}

/** Numbers every question across the report, in the order the sitting drew them. */
function numberItems(report: ScoreReport): NumberedItem[][] {
  let next = 1;
  return report.per_section.map((section) =>
    section.items.map((item) => {
      const states = reviewStates(item);
      const numbered = { item, firstNumber: next, states };
      next += states.length;
      return numbered;
    }),
  );
}

export const ExamReport: React.FC<ExamReportProps> = ({
  report,
  className,
}) => {
  const { t } = useTranslation();
  const [reportVersionId, setReportVersionId] = useState<string | null>(null);

  const sectionLabels: Record<ExamSkill, string> = {
    listening: t("exam.sections.listening"),
    reading: t("exam.sections.reading"),
    writing: t("exam.sections.writing"),
    speaking: t("exam.sections.speaking"),
  };
  const signalLabels: Record<IntegrityKind, string> = {
    tab_hidden: t("exam.report.signalTabHidden"),
    window_blurred: t("exam.report.signalWindowBlurred"),
    paste: t("exam.report.signalPaste"),
  };
  const numbered = numberItems(report);

  return (
    <div
      className={cn("mx-auto max-w-4xl space-y-8 p-4 sm:p-6 md:p-8", className)}
    >
      {report.status === "pending" && (
        <div
          role="status"
          className="flex items-center gap-3 rounded-xl border border-primary/20 bg-primary/10 p-4 text-primary-accent"
        >
          <Loader2
            className="h-5 w-5 shrink-0 animate-spin"
            aria-hidden="true"
          />
          <div>
            <h2 className="text-sm font-semibold">
              {t("exam.report.pendingTitle")}
            </h2>
            <p className="text-xs">{t("exam.report.pendingBody")}</p>
          </div>
        </div>
      )}
      {report.status === "partial" && (
        <div
          role="status"
          className="flex items-center gap-3 rounded-xl border border-warning/30 bg-warning/10 p-4 text-warning-accent"
        >
          <AlertTriangle className="h-5 w-5 shrink-0" aria-hidden="true" />
          <div>
            <h2 className="text-sm font-semibold">
              {t("exam.report.partialTitle")}
            </h2>
            <p className="text-xs">{t("exam.report.partialBody")}</p>
          </div>
        </div>
      )}

      <div className="space-y-6 rounded-3xl border border-border-subtle bg-surface-card p-6 text-center sm:p-8">
        <h1 className="text-2xl font-extrabold text-text sm:text-3xl">
          {t("exam.report.heading")}
        </h1>
        {report.submitted_by === "expiry" && (
          <p className="text-xs text-text-muted">
            {t("exam.report.submittedByExpiry")}
          </p>
        )}
        <div className="flex flex-wrap items-center justify-center gap-6 py-2 sm:gap-12">
          <div className="flex flex-col items-center">
            <p className="font-mono text-5xl font-extrabold text-primary-accent sm:text-6xl">
              {Math.round(report.overall_score)}
              <span className="text-lg font-normal text-text-muted">/100</span>
            </p>
            <span className="mt-1 text-xs font-semibold uppercase tracking-wider text-text-muted">
              {t("exam.report.overall")}
            </span>
          </div>
          <div className="flex flex-col items-center">
            <p className="font-mono text-5xl font-extrabold text-brand sm:text-6xl">
              {report.overall_band || "—"}
            </p>
            <span className="mt-1 text-xs font-semibold uppercase tracking-wider text-text-muted">
              {t("exam.report.band")}
            </span>
          </div>
        </div>
        <ul className="space-y-1 rounded-2xl border border-border-subtle bg-surface-muted/70 p-4 text-left text-xs leading-relaxed text-text-muted sm:p-5">
          <li className="flex gap-2 font-semibold text-text">
            <Info
              className="h-4 w-4 shrink-0 text-primary-accent"
              aria-hidden="true"
            />
            {t("exam.report.notOfficial")}
          </li>
          <li>{t("exam.report.pronunciation")}</li>
          <li>{t("exam.report.sittingsDiffer")}</li>
        </ul>
      </div>

      <section className="space-y-4">
        <h2 className="text-lg font-bold text-text">
          {t("exam.report.bySection")}
        </h2>
        <div className="grid grid-cols-1 gap-4 md:grid-cols-2">
          {report.per_section.map((section) => (
            <SectionCard
              key={section.position}
              section={section}
              label={sectionLabels[section.skill]}
            />
          ))}
        </div>
      </section>

      <AnswerSheet
        report={report}
        numbered={numbered}
        sectionLabels={sectionLabels}
      />

      <PartAnalysis report={report} numbered={numbered} />

      <section className="space-y-4 rounded-2xl border border-border-subtle bg-surface-card p-4 sm:p-6">
        <h2 className="text-lg font-bold text-text">
          {t("exam.report.itemReview")}
        </h2>
        {report.per_section.map((section, sectionIndex) => (
          <div key={section.position} className="space-y-3">
            <h3 className="text-sm font-semibold text-text">
              {sectionLabels[section.skill]}
            </h3>
            <ol className="space-y-3">
              {(numbered[sectionIndex] ?? []).map((entry, index) => (
                <ItemRow
                  key={entry.item.activity_id}
                  item={entry.item}
                  index={index + 1}
                  firstNumber={entry.firstNumber}
                  onReport={() =>
                    setReportVersionId(entry.item.content_version_id)
                  }
                />
              ))}
            </ol>
          </div>
        ))}
      </section>

      {report.integrity_signals.length > 0 && (
        <section className="space-y-3 rounded-2xl border border-border-subtle bg-surface-card p-6">
          <h2 className="flex items-center gap-2 text-sm font-bold text-text">
            <ShieldAlert
              className="h-4 w-4 text-warning-accent"
              aria-hidden="true"
            />
            {t("exam.report.integrityTitle")}
          </h2>
          <p className="text-xs text-text-muted">
            {t("exam.report.integrityBody")}
          </p>
          <ul className="flex flex-wrap gap-3">
            {report.integrity_signals.map((signal) => (
              <li key={signal.kind}>
                <Badge variant="outline">
                  {signalLabels[signal.kind]}: {signal.count}
                </Badge>
              </li>
            ))}
          </ul>
        </section>
      )}

      <div className="flex justify-center pt-4">
        <Link
          to="/exams"
          className="inline-flex min-h-[44px] items-center rounded-lg bg-primary px-6 text-sm font-semibold text-primary-fg"
        >
          {t("exam.report.back")}
        </Link>
      </div>

      <ReportDialog
        isOpen={reportVersionId !== null}
        contentVersionId={reportVersionId}
        onClose={() => setReportVersionId(null)}
      />
    </div>
  );
};

/** Every question as a numbered chip, coloured by result, like a paper answer sheet. */
const AnswerSheet: React.FC<{
  report: ScoreReport;
  numbered: NumberedItem[][];
  sectionLabels: Record<ExamSkill, string>;
}> = ({ report, numbered, sectionLabels }) => {
  const { t } = useTranslation();
  return (
    <section className="space-y-4 rounded-2xl border border-border-subtle bg-surface-card p-4 sm:p-6">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <h2 className="text-lg font-bold text-text">
          {t("exam.report.answerSheet")}
        </h2>
        <ul className="flex flex-wrap gap-3 text-xs text-text-muted">
          <li className="flex items-center gap-1.5">
            <span
              className="h-3 w-3 rounded-sm bg-success"
              aria-hidden="true"
            />
            {t("exam.report.legendCorrect")}
          </li>
          <li className="flex items-center gap-1.5">
            <span className="h-3 w-3 rounded-sm bg-danger" aria-hidden="true" />
            {t("exam.report.legendWrong")}
          </li>
          <li className="flex items-center gap-1.5">
            <span
              className="h-3 w-3 rounded-sm border border-border bg-surface-muted"
              aria-hidden="true"
            />
            {t("exam.report.legendOther")}
          </li>
        </ul>
      </div>
      {report.per_section.map((section, sectionIndex) => (
        <div key={section.position} className="space-y-2">
          <h3 className="text-xs font-semibold uppercase tracking-wider text-text-muted">
            {sectionLabels[section.skill]}
          </h3>
          <div className="flex flex-wrap gap-2">
            {(numbered[sectionIndex] ?? []).flatMap((entry) =>
              entry.states.map((state, offset) => {
                const number = entry.firstNumber + offset;
                return (
                  <a
                    key={number}
                    href={`#review-q-${number}`}
                    className={cn(
                      "flex h-11 min-w-11 items-center justify-center rounded-lg border px-2 font-mono text-sm font-semibold",
                      state === "correct" &&
                        "border-success bg-success/20 text-success-accent",
                      state === "wrong" &&
                        "border-danger bg-danger/15 text-danger-accent",
                      state === "other" &&
                        "border-border-subtle bg-surface-muted text-text-muted",
                    )}
                  >
                    {number}
                  </a>
                );
              }),
            )}
          </div>
        </div>
      ))}
    </section>
  );
};

interface PartRow {
  key: PartKey;
  correct: number;
  wrong: number;
  other: number;
  numbers: { number: number; state: ReviewState }[];
}

/** Tallies the answer sheet by question type, in the order the exam lists them. */
function analyseParts(
  report: ScoreReport,
  numbered: NumberedItem[][],
): PartRow[] {
  const rows = new Map<PartKey, PartRow>();
  report.per_section.forEach((_, sectionIndex) => {
    for (const entry of numbered[sectionIndex] ?? []) {
      const key = partOf(entry.item.kind, entry.item.content);
      if (!key) continue;
      const row = rows.get(key) ?? {
        key,
        correct: 0,
        wrong: 0,
        other: 0,
        numbers: [],
      };
      entry.states.forEach((state, offset) => {
        row[
          state === "correct"
            ? "correct"
            : state === "wrong"
              ? "wrong"
              : "other"
        ]++;
        row.numbers.push({ number: entry.firstNumber + offset, state });
      });
      rows.set(key, row);
    }
  });
  return EXAM_PARTS.flatMap((part) => {
    const row = rows.get(part.key);
    return row ? [row] : [];
  });
}

/** Correct, wrong and skipped per question type, with the questions of each. */
const PartAnalysis: React.FC<{
  report: ScoreReport;
  numbered: NumberedItem[][];
}> = ({ report, numbered }) => {
  const { t } = useTranslation();
  const rows = analyseParts(report, numbered);
  if (rows.length === 0) return null;
  return (
    <section className="space-y-4 rounded-2xl border border-border-subtle bg-surface-card p-4 sm:p-6">
      <h2 className="text-lg font-bold text-text">
        {t("exam.parts.analysisTitle")}
      </h2>
      <div className="overflow-x-auto">
        <table className="w-full min-w-[36rem] text-left text-sm">
          <thead>
            <tr className="border-b border-border text-xs uppercase tracking-wider text-text-muted">
              <th scope="col" className="py-2 pr-3 font-semibold">
                {t("exam.parts.colPart")}
              </th>
              <th scope="col" className="px-2 py-2 text-center font-semibold">
                {t("exam.parts.colCorrect")}
              </th>
              <th scope="col" className="px-2 py-2 text-center font-semibold">
                {t("exam.parts.colWrong")}
              </th>
              <th scope="col" className="px-2 py-2 text-center font-semibold">
                {t("exam.parts.colOther")}
              </th>
              <th scope="col" className="px-2 py-2 text-center font-semibold">
                {t("exam.parts.colAccuracy")}
              </th>
              <th scope="col" className="py-2 pl-3 font-semibold">
                {t("exam.parts.colQuestions")}
              </th>
            </tr>
          </thead>
          <tbody className="divide-y divide-border-subtle">
            {rows.map((row) => {
              const marked = row.correct + row.wrong;
              return (
                <tr key={row.key} className="align-top">
                  <th scope="row" className="py-3 pr-3 font-semibold text-text">
                    #{t(`exam.parts.${row.key}`)}
                  </th>
                  <td className="px-2 py-3 text-center font-mono tabular-nums text-success-accent">
                    {row.correct}
                  </td>
                  <td className="px-2 py-3 text-center font-mono tabular-nums text-danger-accent">
                    {row.wrong}
                  </td>
                  <td className="px-2 py-3 text-center font-mono tabular-nums text-text-muted">
                    {row.other}
                  </td>
                  <td className="px-2 py-3 text-center font-mono tabular-nums text-text">
                    {marked > 0
                      ? `${Math.round((row.correct / (marked + row.other)) * 100)}%`
                      : "—"}
                  </td>
                  <td className="py-2 pl-3">
                    <div className="flex flex-wrap gap-1.5">
                      {row.numbers.map(({ number, state }) => (
                        <a
                          key={number}
                          href={`#review-q-${number}`}
                          className={cn(
                            "flex h-11 min-w-11 items-center justify-center rounded-lg border px-2 font-mono text-xs font-semibold",
                            state === "correct" &&
                              "border-success bg-success/20 text-success-accent",
                            state === "wrong" &&
                              "border-danger bg-danger/15 text-danger-accent",
                            state === "other" &&
                              "border-border-subtle bg-surface-muted text-text-muted",
                          )}
                        >
                          {number}
                        </a>
                      ))}
                    </div>
                  </td>
                </tr>
              );
            })}
          </tbody>
        </table>
      </div>
    </section>
  );
};

const SectionCard: React.FC<{ section: ExamSectionOutcome; label: string }> = ({
  section,
  label,
}) => {
  const { t } = useTranslation();
  const Icon = SECTION_ICONS[section.skill];
  return (
    <div className="space-y-3 rounded-2xl border border-border-subtle bg-surface-card p-5">
      <div className="flex items-center justify-between gap-3">
        <div className="flex items-center gap-2.5">
          <div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-xl bg-primary/10 text-primary-accent">
            <Icon className="h-5 w-5" aria-hidden="true" />
          </div>
          <h3 className="text-base font-bold text-text">{label}</h3>
        </div>
        {section.status === "scored" && section.score !== undefined ? (
          <p className="font-mono text-2xl font-bold text-text">
            {section.score}
            <span className="text-xs font-normal text-text-muted">
              /{section.max_score}
            </span>
          </p>
        ) : (
          <Badge variant="outline">
            {section.status === "pending"
              ? t("exam.report.sectionPending")
              : t("exam.report.notScored")}
          </Badge>
        )}
      </div>
      {section.status === "scored" && section.score !== undefined && (
        <div className="h-2 w-full overflow-hidden rounded-full bg-border-subtle">
          <div
            className="h-full rounded-full bg-primary"
            style={{ width: `${section.score}%` }}
          />
        </div>
      )}
    </div>
  );
};

const ItemRow: React.FC<{
  item: ExamItemOutcome;
  index: number;
  firstNumber: number;
  onReport: () => void;
}> = ({ item, index, firstNumber, onReport }) => {
  const { t } = useTranslation();
  const [showFeedback, setShowFeedback] = useState(false);

  const kindLabels: Record<string, string> = {
    listening_comprehension: t("exam.report.kindListening"),
    reading_comprehension: t("exam.report.kindReading"),
    grammar_sentence_transform: t("exam.report.kindRewrite"),
    writing_prompt: t("exam.report.kindEssay"),
    speaking_task: t("exam.report.kindSpeaking"),
  };
  const statusLabels: Record<ItemStatus, string> = {
    graded: t("exam.report.itemGraded"),
    pending: t("exam.report.itemPending"),
    failed: t("exam.report.itemFailed"),
    unanswered: t("exam.report.itemUnanswered"),
  };

  const questions = item.item_results ?? [];
  const correct = questions.filter((q) => q.correct).length;
  const hasFeedback =
    item.status === "graded" &&
    item.attempt_id !== undefined &&
    (item.kind === "writing_prompt" || item.kind === "speaking_task");
  const fullMarks =
    item.status === "graded" &&
    item.max_score > 0 &&
    item.score === item.max_score;

  return (
    <li className="space-y-3 rounded-xl border border-border-subtle bg-surface-muted/30 p-4 text-xs">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <div className="flex items-center gap-2">
          {fullMarks ? (
            <CheckCircle2
              className="h-4 w-4 shrink-0 text-success-accent"
              aria-hidden="true"
            />
          ) : (
            <XCircle
              className="h-4 w-4 shrink-0 text-text-muted"
              aria-hidden="true"
            />
          )}
          <span className="font-semibold text-text">
            {t("exam.report.item", { num: index })} ·{" "}
            {kindLabels[item.kind] ?? item.kind}
          </span>
        </div>
        <span className="text-text-muted">
          {item.status === "graded"
            ? t("exam.report.itemScore", {
                score: item.score,
                max: item.max_score,
              })
            : statusLabels[item.status]}
        </span>
      </div>
      {questions.length > 0 && (
        <p className="text-text-muted">
          {t("exam.report.questionsCorrect", {
            correct,
            total: questions.length,
          })}
        </p>
      )}

      <ExamItemReview item={item} firstNumber={firstNumber} />

      <div className="flex flex-wrap gap-2">
        {hasFeedback && (
          <Button
            type="button"
            size="sm"
            variant="outline"
            onClick={() => setShowFeedback((v) => !v)}
          >
            {showFeedback
              ? t("exam.report.hideFeedback")
              : t("exam.report.showFeedback")}
          </Button>
        )}
        <Button type="button" size="sm" variant="ghost" onClick={onReport}>
          <Flag className="h-3.5 w-3.5" aria-hidden="true" />
          {t("exam.report.reportItem")}
        </Button>
      </div>
      {showFeedback && item.attempt_id && item.kind === "writing_prompt" && (
        <WritingFeedbackPanel
          attemptId={item.attempt_id}
          graderFeedback={item.feedback}
        />
      )}
      {showFeedback && item.attempt_id && item.kind === "speaking_task" && (
        <SpeakingFeedbackPanel attemptId={item.attempt_id} />
      )}
    </li>
  );
};

const WritingFeedbackPanel: React.FC<{
  attemptId: string;
  graderFeedback?: string | undefined;
}> = ({ attemptId, graderFeedback }) => {
  const { t, i18n } = useTranslation();
  const { data, isLoading, isError, error } = useWritingFeedback(attemptId);
  if (isLoading)
    return (
      <p className="text-text-muted">{t("exam.report.feedbackLoading")}</p>
    );
  // An essay under its word count, or an empty one, is scored without a model
  // call, so there is no AI feedback to fetch. That is not a failure to load.
  if (isError && problemCode(error) === "FEEDBACK_NOT_FOUND") {
    return (
      <div className="space-y-1 rounded-lg bg-surface-card p-3 text-text">
        <p className="font-semibold">{t("exam.report.noAiFeedback")}</p>
        {graderFeedback && (
          <p className="leading-relaxed text-text-muted">{graderFeedback}</p>
        )}
      </div>
    );
  }
  if (isError || !data)
    return (
      <p className="text-danger-accent">{t("exam.report.feedbackFailed")}</p>
    );
  return (
    <div className="space-y-1 rounded-lg bg-surface-card p-3 text-text">
      <p className="font-semibold">
        {t("exam.report.writingBand", { band: data.overall_band })}
      </p>
      <p className="leading-relaxed">
        {i18n.language.startsWith("vi") ? data.feedback_vi : data.feedback_en}
      </p>
    </div>
  );
};

const SpeakingFeedbackPanel: React.FC<{ attemptId: string }> = ({
  attemptId,
}) => {
  const { t, i18n } = useTranslation();
  const { data, isLoading, isError } = useSpeakingFeedback(attemptId);
  const [deleteState, setDeleteState] = useState<
    "idle" | "confirm" | "deleting" | "deleted" | "failed"
  >("idle");

  if (isLoading)
    return (
      <p className="text-text-muted">{t("exam.report.feedbackLoading")}</p>
    );
  if (isError || !data)
    return (
      <p className="text-danger-accent">{t("exam.report.feedbackFailed")}</p>
    );

  const deleted =
    deleteState === "deleted" || data.recording_deleted_at !== undefined;

  const deleteRecording = async () => {
    setDeleteState("deleting");
    try {
      await examApi.deleteSpeakingRecording(attemptId);
      setDeleteState("deleted");
    } catch {
      setDeleteState("failed");
    }
  };

  return (
    <div className="space-y-2 rounded-lg bg-surface-card p-3 text-text">
      <p>
        <span className="font-semibold">{t("exam.report.transcript")}: </span>
        {data.transcript}
      </p>
      {data.read_aloud_accuracy !== undefined && (
        <p>
          {t("exam.report.readAloudAccuracy", {
            value: Math.round(data.read_aloud_accuracy),
          })}
        </p>
      )}
      <p className="leading-relaxed">
        {i18n.language.startsWith("vi") ? data.feedback_vi : data.feedback_en}
      </p>
      {deleted ? (
        <p className="text-text-muted">{t("exam.report.recordingDeleted")}</p>
      ) : deleteState === "confirm" || deleteState === "deleting" ? (
        <div className="flex flex-wrap items-center gap-2">
          <span>{t("exam.report.confirmDelete")}</span>
          <Button
            type="button"
            size="sm"
            variant="destructive"
            disabled={deleteState === "deleting"}
            onClick={() => void deleteRecording()}
          >
            {t("exam.report.deleteRecording")}
          </Button>
          <Button
            type="button"
            size="sm"
            variant="outline"
            onClick={() => setDeleteState("idle")}
          >
            {t("exam.report.keepRecording")}
          </Button>
        </div>
      ) : (
        <>
          <Button
            type="button"
            size="sm"
            variant="ghost"
            onClick={() => setDeleteState("confirm")}
          >
            <Trash2 className="h-3.5 w-3.5" aria-hidden="true" />
            {t("exam.report.deleteRecording")}
          </Button>
          {deleteState === "failed" && (
            <p role="alert" className="text-danger-accent">
              {t("exam.report.deleteFailed")}
            </p>
          )}
        </>
      )}
    </div>
  );
};
