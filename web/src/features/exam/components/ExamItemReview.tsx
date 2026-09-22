import React, { useState } from "react";
import { Check, X } from "lucide-react";
import { useTranslation } from "react-i18next";

import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";
import type {
  AnswerExplanation,
  ExamItemOutcome,
  QuestionResult,
} from "../types";

/**
 * The answer review of one exam item: what was asked, what the learner chose,
 * what was right, and why.
 *
 * Built from the item as authored (`content`) and the learner's saved answer
 * (`response`), which the report carries only for a submitted sitting.
 */

export interface ReviewOption {
  id: string;
  text: string;
}

export interface ReviewQuestion {
  id: string;
  prompt: string;
  options: ReviewOption[];
  correctAnswer?: string | undefined;
  explanation: AnswerExplanation | null;
}

export type ReviewState = "correct" | "wrong" | "other";

function asRecord(value: unknown): Record<string, unknown> | undefined {
  return value && typeof value === "object" && !Array.isArray(value)
    ? (value as Record<string, unknown>)
    : undefined;
}

function asString(value: unknown): string {
  return typeof value === "string" ? value : "";
}

/** An authored explanation, in either spelling the content uses. */
export function readExplanation(raw: unknown): AnswerExplanation | null {
  const record = asRecord(raw);
  if (!record) return null;
  const text = asString(record.text) || asString(record.explanation_en);
  const textVi = asString(record.text_vi) || asString(record.explanation_vi);
  return text || textVi ? { text, text_vi: textVi } : null;
}

export function reviewQuestions(item: ExamItemOutcome): ReviewQuestion[] {
  const raw = item.content?.questions;
  if (!Array.isArray(raw)) return [];
  return raw.flatMap((entry): ReviewQuestion[] => {
    const question = asRecord(entry);
    if (!question) return [];
    const options = Array.isArray(question.options)
      ? question.options.flatMap((option): ReviewOption[] => {
          const record = asRecord(option);
          return record
            ? [{ id: asString(record.id), text: asString(record.text) }]
            : [];
        })
      : [];
    const correct =
      asString(question.correct_option_id) ||
      asString(question.correct_answer) ||
      asString(question.answer);
    return [
      {
        id: asString(question.id),
        prompt: asString(question.prompt),
        options,
        correctAnswer: correct || undefined,
        explanation: readExplanation(question.explanation),
      },
    ];
  });
}

function learnerAnswers(item: ExamItemOutcome): Record<string, string> {
  const answers = asRecord(item.response?.answers);
  if (!answers) return {};
  return Object.fromEntries(
    Object.entries(answers).map(([id, value]) => [id, asString(value)]),
  );
}

function questionState(
  question: ReviewQuestion,
  chosen: string,
  verdict: QuestionResult | undefined,
): ReviewState {
  if (!chosen) return "other";
  if (verdict) return verdict.correct ? "correct" : "wrong";
  return question.correctAnswer !== undefined &&
    chosen.trim().toLowerCase() === question.correctAnswer.trim().toLowerCase()
    ? "correct"
    : "wrong";
}

/**
 * One answer-sheet entry per question of a comprehension set, or one per item
 * for everything scored as a whole.
 */
export function reviewStates(item: ExamItemOutcome): ReviewState[] {
  const questions = reviewQuestions(item);
  const results = item.item_results ?? [];
  if (questions.length > 0) {
    const answers = learnerAnswers(item);
    return questions.map((question) =>
      questionState(
        question,
        answers[question.id] ?? "",
        results.find((result) => result.id === question.id),
      ),
    );
  }
  if (results.length > 0) {
    return results.map((result) => (result.correct ? "correct" : "wrong"));
  }
  if (item.status !== "graded" || item.max_score <= 0) return ["other"];
  if (item.kind === "writing_prompt" || item.kind === "speaking_task") {
    return ["other"];
  }
  return [item.score === item.max_score ? "correct" : "wrong"];
}

const Explanation: React.FC<{ explanation: AnswerExplanation }> = ({
  explanation,
}) => {
  const { t, i18n } = useTranslation();
  const vi = i18n.language.startsWith("vi");
  const lead = vi
    ? explanation.text_vi || explanation.text
    : explanation.text || explanation.text_vi;
  const second = vi ? explanation.text : explanation.text_vi;
  return (
    <div className="space-y-1 rounded-lg border border-border-subtle bg-surface-muted/60 p-3">
      <p className="text-[11px] font-semibold uppercase tracking-wider text-text-muted">
        {t("exam.report.explanationLabel")}
      </p>
      <p className="text-sm leading-relaxed text-text">{lead}</p>
      {second && second !== lead && (
        <p className="text-xs leading-relaxed text-text-muted">{second}</p>
      )}
    </div>
  );
};

const Reveal: React.FC<{ label: string; children: React.ReactNode }> = ({
  label,
  children,
}) => {
  const { t } = useTranslation();
  const [open, setOpen] = useState(false);
  return (
    <div className="space-y-2">
      <Button
        type="button"
        size="sm"
        variant="outline"
        aria-expanded={open}
        onClick={() => setOpen((value) => !value)}
      >
        {open ? t("exam.report.hideText") : label}
      </Button>
      {open && (
        <div className="whitespace-pre-line rounded-lg border border-border-subtle bg-surface-muted/60 p-3 text-sm leading-relaxed text-text">
          {children}
        </div>
      )}
    </div>
  );
};

const StateBadge: React.FC<{ state: ReviewState }> = ({ state }) => {
  const { t } = useTranslation();
  if (state === "correct") {
    return (
      <span className="inline-flex shrink-0 items-center gap-1 rounded-full bg-success/15 px-2 py-0.5 text-xs font-semibold text-success-accent">
        <Check className="h-3.5 w-3.5" aria-hidden="true" />
        {t("exam.report.legendCorrect")}
      </span>
    );
  }
  if (state === "wrong") {
    return (
      <span className="inline-flex shrink-0 items-center gap-1 rounded-full bg-danger/15 px-2 py-0.5 text-xs font-semibold text-danger-accent">
        <X className="h-3.5 w-3.5" aria-hidden="true" />
        {t("exam.report.legendWrong")}
      </span>
    );
  }
  return (
    <span className="inline-flex shrink-0 items-center rounded-full bg-surface-muted px-2 py-0.5 text-xs font-semibold text-text-muted">
      {t("exam.report.noAnswer")}
    </span>
  );
};

function optionLabel(options: ReviewOption[], id: string | undefined): string {
  if (!id) return "";
  const index = options.findIndex((option) => option.id === id);
  if (index < 0) return id;
  return `${String.fromCharCode(65 + index)} · ${options[index]?.text ?? ""}`;
}

const ComprehensionReview: React.FC<{
  item: ExamItemOutcome;
  firstNumber: number;
}> = ({ item, firstNumber }) => {
  const { t } = useTranslation();
  const questions = reviewQuestions(item);
  const answers = learnerAnswers(item);
  const results = item.item_results ?? [];
  const source =
    item.kind === "listening_comprehension"
      ? asString(item.content?.script)
      : asString(item.content?.passage);

  return (
    <div className="space-y-3">
      {source && (
        <Reveal
          label={
            item.kind === "listening_comprehension"
              ? t("exam.report.showScript")
              : t("exam.report.showPassage")
          }
        >
          {source}
        </Reveal>
      )}
      {questions.map((question, index) => {
        const number = firstNumber + index;
        const chosen = answers[question.id] ?? "";
        const verdict = results.find((result) => result.id === question.id);
        const state = questionState(question, chosen, verdict);
        const correctId = verdict?.correct_answer ?? question.correctAnswer;
        const explanation =
          question.explanation ?? verdict?.explanation ?? null;
        return (
          <div
            key={question.id || index}
            id={`review-q-${number}`}
            className={cn(
              "space-y-3 rounded-xl border p-4",
              state === "correct" && "border-success/50 bg-success/5",
              state === "wrong" && "border-danger/50 bg-danger/5",
              state === "other" && "border-border-subtle",
            )}
          >
            <div className="flex items-start justify-between gap-3">
              <p className="text-sm font-medium leading-relaxed text-text">
                <span className="mr-2 font-mono text-primary-accent">
                  {number}.
                </span>
                {question.prompt}
              </p>
              <StateBadge state={state} />
            </div>
            {question.options.length > 0 && (
              <ul className="grid grid-cols-1 gap-2 sm:grid-cols-2">
                {question.options.map((option, optionIndex) => {
                  const isCorrect = option.id === correctId;
                  const isChosen = option.id === chosen;
                  return (
                    <li
                      key={option.id || optionIndex}
                      className={cn(
                        "flex items-center justify-between gap-2 rounded-lg border px-3 py-2 text-sm",
                        isCorrect &&
                          "border-success bg-success/15 font-semibold text-text",
                        isChosen &&
                          !isCorrect &&
                          "border-danger bg-danger/10 text-text",
                        !isCorrect &&
                          !isChosen &&
                          "border-border-subtle text-text-muted",
                      )}
                    >
                      <span>
                        <span className="mr-2 font-mono text-xs">
                          {String.fromCharCode(65 + optionIndex)}
                        </span>
                        {option.text}
                      </span>
                      {isCorrect && (
                        <Check
                          className="h-4 w-4 shrink-0 text-success-accent"
                          aria-hidden="true"
                        />
                      )}
                      {isChosen && !isCorrect && (
                        <X
                          className="h-4 w-4 shrink-0 text-danger-accent"
                          aria-hidden="true"
                        />
                      )}
                    </li>
                  );
                })}
              </ul>
            )}
            <dl className="grid grid-cols-1 gap-1 text-xs sm:grid-cols-2">
              <div>
                <dt className="inline text-text-muted">
                  {t("exam.report.yourAnswer")}:{" "}
                </dt>
                <dd
                  className={cn(
                    "inline font-semibold",
                    state === "wrong" ? "text-danger-accent" : "text-text",
                  )}
                >
                  {chosen
                    ? optionLabel(question.options, chosen)
                    : t("exam.report.noAnswer")}
                </dd>
              </div>
              {correctId && (
                <div>
                  <dt className="inline text-text-muted">
                    {t("exam.report.correctAnswer")}:{" "}
                  </dt>
                  <dd className="inline font-semibold text-success-accent">
                    {optionLabel(question.options, correctId)}
                  </dd>
                </div>
              )}
            </dl>
            {explanation && <Explanation explanation={explanation} />}
          </div>
        );
      })}
    </div>
  );
};

const RewriteReview: React.FC<{ item: ExamItemOutcome }> = ({ item }) => {
  const { t } = useTranslation();
  // The sitting saves a rewrite as `answer`; `text_answer` is the essay's field.
  const answer =
    asString(item.response?.answer) || asString(item.response?.text_answer);
  const correct = asString(item.content?.correct_answer);
  const accepted = Array.isArray(item.content?.acceptable)
    ? item.content.acceptable.map(asString).filter(Boolean)
    : [];
  const explanation = readExplanation(item.content?.explanation);
  return (
    <div className="space-y-2 text-sm">
      <p className="leading-relaxed text-text">
        {asString(item.content?.prompt)}
      </p>
      <p className="text-xs">
        <span className="text-text-muted">{t("exam.report.yourText")}: </span>
        <span className="font-semibold text-text">
          {answer || t("exam.report.noAnswer")}
        </span>
      </p>
      {correct && (
        <p className="text-xs">
          <span className="text-text-muted">
            {t("exam.report.correctAnswer")}:{" "}
          </span>
          <span className="font-semibold text-success-accent">{correct}</span>
        </p>
      )}
      {accepted.length > 0 && (
        <p className="text-xs text-text-muted">
          {t("exam.report.acceptedAnswers")}: {accepted.join(" / ")}
        </p>
      )}
      {explanation && <Explanation explanation={explanation} />}
    </div>
  );
};

const WritingReview: React.FC<{ item: ExamItemOutcome }> = ({ item }) => {
  const { t } = useTranslation();
  const answer = asString(item.response?.text_answer);
  const words = answer.trim() ? answer.trim().split(/\s+/).length : 0;
  const sample = asString(item.content?.model_answer);
  return (
    <div className="space-y-2 text-sm">
      <p className="leading-relaxed text-text">
        {asString(item.content?.prompt)}
      </p>
      <div className="space-y-1 rounded-lg border border-border-subtle p-3">
        <p className="text-[11px] font-semibold uppercase tracking-wider text-text-muted">
          {t("exam.report.yourText")} ·{" "}
          {t("exam.report.wordCount", { count: words })}
        </p>
        <p className="whitespace-pre-line leading-relaxed text-text">
          {answer || t("exam.report.noAnswer")}
        </p>
      </div>
      {item.feedback && (
        <p className="text-xs leading-relaxed text-text-muted">
          {item.feedback}
        </p>
      )}
      {sample && <Reveal label={t("exam.report.showSample")}>{sample}</Reveal>}
    </div>
  );
};

const SpeakingReview: React.FC<{ item: ExamItemOutcome }> = ({ item }) => {
  const { t } = useTranslation();
  const reference = asString(item.content?.reference_text);
  return (
    <div className="space-y-2 text-sm">
      <p className="leading-relaxed text-text">
        {asString(item.content?.prompt)}
      </p>
      {reference && (
        <div className="space-y-1 rounded-lg border border-border-subtle p-3">
          <p className="text-[11px] font-semibold uppercase tracking-wider text-text-muted">
            {t("exam.report.readAloudText")}
          </p>
          <p className="leading-relaxed text-text">{reference}</p>
        </div>
      )}
      {item.feedback && (
        <p className="text-xs leading-relaxed text-text-muted">
          {item.feedback}
        </p>
      )}
    </div>
  );
};

export const ExamItemReview: React.FC<{
  item: ExamItemOutcome;
  firstNumber: number;
}> = ({ item, firstNumber }) => {
  if (!item.content) return null;
  switch (item.kind) {
    case "listening_comprehension":
    case "reading_comprehension":
    case "text_completion":
    case "mcq_gap":
    case "photo_description":
    case "question_response":
      return <ComprehensionReview item={item} firstNumber={firstNumber} />;
    case "grammar_sentence_transform":
      return <RewriteReview item={item} />;
    case "writing_prompt":
      return <WritingReview item={item} />;
    case "speaking_task":
      return <SpeakingReview item={item} />;
    default:
      return null;
  }
};
