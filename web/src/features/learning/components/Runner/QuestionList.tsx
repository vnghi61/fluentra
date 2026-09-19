import React from "react";
import { Check, X } from "lucide-react";
import { useTranslation } from "react-i18next";

import { Badge } from "@/components/ui/badge";
import { Input } from "@/components/ui/input";
import { cn } from "@/lib/utils";

import { type AnswerExplanation } from "./ExerciseShell";
import type { OptionItem } from "./ExerciseMultipleChoice";

/**
 * The comprehension question set, shared by reading and listening.
 *
 * It lived inside ExerciseReading, which is a passage plus these questions.
 * Listening is a clip plus the same questions — same shapes, same per-question
 * verdicts, same revealed answer and explanation — and the second copy would
 * have been the one that missed the next fix. The two exercises differ in what
 * comes before the questions, which is what each of them still owns.
 */
export interface ReadingQuestionItem {
  id: string;
  type: "multiple_choice" | "true_false_not_given" | "gap_fill" | (string & {});
  prompt: string;
  options?: OptionItem[] | undefined;
}

export interface ItemResult {
  id: string;
  correct: boolean;
  correct_answer?: string | null | undefined;
  /** Why the correct answer is correct, returned with the grade. */
  explanation?: AnswerExplanation | null | undefined;
}

/** "B · 6 p.m." for a choice, the answer itself for anything typed. */
export function answerLabel(
  options: OptionItem[] | undefined,
  answer: string,
): string {
  const index = options?.findIndex((option) => option.id === answer) ?? -1;
  if (index < 0 || !options) return answer;
  return `${String.fromCharCode(65 + index)} · ${options[index]?.text ?? ""}`;
}

export const ItemExplanation: React.FC<{ explanation: AnswerExplanation }> = ({
  explanation,
}) => {
  const { t, i18n } = useTranslation();
  const vi = i18n.language.startsWith("vi");
  const lead = vi
    ? explanation.text_vi || explanation.text
    : explanation.text || explanation.text_vi;
  const second = vi ? explanation.text : explanation.text_vi;
  return (
    <div className="mt-3 space-y-1 rounded-md border border-border/40 bg-surface/60 p-3">
      <p className="text-[11px] font-semibold uppercase tracking-wider text-text-muted">
        {t("runner.explanationLabel", "Explanation")}
      </p>
      <p className="text-sm leading-relaxed text-text">{lead}</p>
      {second && second !== lead && (
        <p className="text-xs leading-relaxed text-text-muted">{second}</p>
      )}
    </div>
  );
};

export interface QuestionListProps {
  questions: ReadingQuestionItem[];
  /** questionId -> the learner's answer. */
  answers: Record<string, string>;
  onAnswerChange: (questionId: string, value: string) => void;
  /** Per-question grading outcomes from item_results. */
  itemResults?: ItemResult[] | null | undefined;
  isSubmitted: boolean;
  isLoading?: boolean;
  /** Heading above the list. Defaults to "Comprehension Questions". */
  heading?: string | undefined;
}

export const QuestionList: React.FC<QuestionListProps> = ({
  questions,
  answers,
  onAnswerChange,
  itemResults,
  isSubmitted,
  isLoading = false,
  heading,
}) => {
  const { t } = useTranslation();

  return (
    <div className="space-y-6 pt-2">
      <div className="flex items-center justify-between border-b border-border pb-2">
        <h4 className="text-sm font-bold uppercase tracking-wider text-text">
          {heading ?? t("runner.questionsLabel", "Comprehension Questions")}
        </h4>
        <Badge variant="secondary" className="text-xs">
          {t("runner.questionCount", {
            count: questions.length,
            defaultValue: "{{count}} questions",
          })}
        </Badge>
      </div>

      <div className="space-y-6">
        {questions.map((q, qIndex) => {
          const itemResult = itemResults?.find((r) => r.id === q.id);
          const isItemCorrect = itemResult?.correct;
          const hasItemVerdict = isSubmitted && itemResult !== undefined;

          return (
            <div
              key={q.id}
              className={cn(
                "rounded-xl border p-4 md:p-5 transition-all bg-surface-card",
                hasItemVerdict &&
                  isItemCorrect &&
                  "border-success/60 bg-success/5",
                hasItemVerdict &&
                  !isItemCorrect &&
                  "border-danger/60 bg-danger/5",
                !hasItemVerdict && "border-border",
              )}
            >
              <div className="flex items-start justify-between gap-3 mb-3">
                <div className="space-y-1">
                  <span className="text-xs font-semibold text-primary uppercase tracking-wider">
                    {t("runner.questionNum", { num: qIndex + 1 })}
                  </span>
                  <p className="text-base font-medium text-text">{q.prompt}</p>
                </div>
                {hasItemVerdict && (
                  <div className="shrink-0 mt-1">
                    {isItemCorrect ? (
                      <span className="inline-flex items-center gap-1 text-xs font-semibold text-success bg-success/15 px-2 py-1 rounded-full">
                        <Check className="h-3.5 w-3.5" />
                        {t("runner.markCorrect", "Correct")}
                      </span>
                    ) : (
                      <span className="inline-flex items-center gap-1 text-xs font-semibold text-danger-accent bg-danger/15 px-2 py-1 rounded-full">
                        <X className="h-3.5 w-3.5" />
                        {t("runner.markIncorrect", "Incorrect")}
                      </span>
                    )}
                  </div>
                )}
              </div>

              {/* Multiple Choice Options */}
              {q.type === "multiple_choice" && q.options && (
                <div
                  className="grid grid-cols-1 sm:grid-cols-2 gap-2.5 mt-3"
                  role="radiogroup"
                  aria-label={q.prompt}
                >
                  {q.options.map((opt, optIndex) => {
                    const isSelected = answers[q.id] === opt.id;
                    const isCorrectAnswer =
                      hasItemVerdict && itemResult?.correct_answer === opt.id;

                    let optClass =
                      "border-border bg-surface hover:border-primary/50 text-text";
                    if (isSubmitted) {
                      if (isCorrectAnswer) {
                        optClass =
                          "border-success bg-success/20 text-text font-semibold";
                      } else if (isSelected && !isItemCorrect) {
                        optClass =
                          "border-danger bg-danger/15 text-text line-through opacity-80";
                      } else {
                        optClass = "border-border opacity-50 text-text-muted";
                      }
                    } else if (isSelected) {
                      optClass =
                        "border-primary bg-primary/10 text-text font-semibold ring-1 ring-primary";
                    }

                    return (
                      <button
                        key={opt.id}
                        type="button"
                        role="radio"
                        aria-checked={isSelected}
                        disabled={isSubmitted || isLoading}
                        onClick={() => onAnswerChange(q.id, opt.id)}
                        className={cn(
                          "text-left p-3 rounded-lg border text-sm transition-all flex items-center justify-between min-h-[44px] cursor-pointer",
                          optClass,
                          isSubmitted && "cursor-default",
                        )}
                      >
                        <span className="flex items-center gap-2">
                          <span className="w-5 h-5 rounded-md text-xs font-bold flex items-center justify-center border border-border/70 bg-surface">
                            {String.fromCharCode(65 + optIndex)}
                          </span>
                          <span>{opt.text}</span>
                        </span>
                        {hasItemVerdict && isCorrectAnswer && (
                          <Check className="h-4 w-4 text-success shrink-0" />
                        )}
                      </button>
                    );
                  })}
                </div>
              )}

              {/* True / False / Not Given */}
              {q.type === "true_false_not_given" && (
                <div
                  className="flex flex-wrap gap-2.5 mt-3"
                  role="radiogroup"
                  aria-label={q.prompt}
                >
                  {[
                    { val: "true", label: t("runner.true", "True") },
                    { val: "false", label: t("runner.false", "False") },
                    {
                      val: "not_given",
                      label: t("runner.notGiven", "Not Given"),
                    },
                  ].map((tf) => {
                    const isSelected = answers[q.id]?.toLowerCase() === tf.val;
                    const isCorrectAnswer =
                      hasItemVerdict &&
                      itemResult?.correct_answer?.toLowerCase() === tf.val;

                    let tfClass =
                      "border-border bg-surface hover:border-primary/50 text-text";
                    if (isSubmitted) {
                      if (isCorrectAnswer) {
                        tfClass =
                          "border-success bg-success/20 text-text font-semibold";
                      } else if (isSelected && !isItemCorrect) {
                        tfClass =
                          "border-danger bg-danger/15 text-text line-through opacity-80";
                      } else {
                        tfClass = "border-border opacity-50 text-text-muted";
                      }
                    } else if (isSelected) {
                      tfClass =
                        "border-primary bg-primary/10 text-text font-semibold ring-1 ring-primary";
                    }

                    return (
                      <button
                        key={tf.val}
                        type="button"
                        role="radio"
                        aria-checked={isSelected}
                        disabled={isSubmitted || isLoading}
                        onClick={() => onAnswerChange(q.id, tf.val)}
                        className={cn(
                          "flex-1 min-w-[100px] text-center py-2.5 px-3 rounded-lg border text-sm transition-all min-h-[44px] cursor-pointer font-medium",
                          tfClass,
                          isSubmitted && "cursor-default",
                        )}
                      >
                        {tf.label}
                      </button>
                    );
                  })}
                </div>
              )}

              {/* Gap Fill */}
              {q.type === "gap_fill" && (
                <div className="mt-3 space-y-2">
                  <Input
                    type="text"
                    value={answers[q.id] || ""}
                    disabled={isSubmitted || isLoading}
                    onChange={(e) => onAnswerChange(q.id, e.target.value)}
                    placeholder={t(
                      "runner.gapFillPlaceholder",
                      "Type your answer...",
                    )}
                    className={cn(
                      "min-h-[44px] text-base",
                      hasItemVerdict &&
                        isItemCorrect &&
                        "border-success ring-1 ring-success",
                      hasItemVerdict &&
                        !isItemCorrect &&
                        "border-danger ring-1 ring-danger",
                    )}
                  />
                </div>
              )}

              {/* Revealed Correct Answer on Failure: the option's letter
                  and text, not its id. */}
              {hasItemVerdict &&
                !isItemCorrect &&
                itemResult?.correct_answer && (
                  <div className="mt-3 text-xs text-text-muted flex items-center gap-1.5 bg-surface/60 p-2 rounded-md border border-border/40">
                    <span className="font-semibold text-text">
                      {t("runner.correctAnswerLabel", "Correct answer")}:
                    </span>
                    <span className="text-success-accent font-bold">
                      {answerLabel(q.options, itemResult.correct_answer)}
                    </span>
                  </div>
                )}

              {/* Why, for every graded question, right or wrong. */}
              {hasItemVerdict && itemResult?.explanation && (
                <ItemExplanation explanation={itemResult.explanation} />
              )}
            </div>
          );
        })}
      </div>
    </div>
  );
};
