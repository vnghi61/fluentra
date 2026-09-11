import React, { useCallback, useEffect, useRef, useState } from "react";
import { ArrowRight, BookOpen, Check, Clock, X } from "lucide-react";
import { useTranslation } from "react-i18next";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { cn } from "@/lib/utils";

import {
  type AnswerExplanation,
  ExerciseActions,
  ExerciseFeedback,
  ExercisePrompt,
} from "./ExerciseShell";
import type { OptionItem } from "./ExerciseMultipleChoice";

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
}

export type ReadingSubmissionPayload =
  | string
  | {
      selectedOptionId?: string | undefined;
      answers?: Record<string, string> | undefined;
      reading_ms?: number | undefined;
    };

export interface ExerciseReadingProps {
  passageTitle?: string | undefined;
  /** The reading passage the learner reads. */
  passage: string;
  /** Legacy single-question prompt. */
  prompt?: string | undefined;
  /** Legacy single-question options. */
  options?: OptionItem[] | undefined;
  /** Multi-question comprehension set. */
  questions?: ReadingQuestionItem[] | undefined;
  /** Per-question grading outcomes from item_results. */
  itemResults?: ItemResult[] | null | undefined;
  correctOptionId?: string | null | undefined;
  feedback?: string | null | undefined;
  isSubmitted: boolean;
  isCorrect?: boolean | null | undefined;
  isLoading?: boolean;
  explanation?: AnswerExplanation | null | undefined;
  onSubmit: (payload: ReadingSubmissionPayload) => void;
  onContinue: () => void;
}

/**
 * `reading_comprehension` — answer questions based on an authored passage.
 *
 * Supports two-phase reading flow (Phase 1: reading passage with reading speed measurement;
 * Phase 2: answering comprehension questions), multi-question sets with mixed types
 * (multiple_choice, true_false_not_given, gap_fill), per-question verdicts (item_results),
 * and backward compatibility with single-question activities (BR-CONTENT-01).
 */
export const ExerciseReading: React.FC<ExerciseReadingProps> = ({
  passageTitle,
  passage,
  prompt,
  options = [],
  questions,
  itemResults,
  correctOptionId,
  feedback,
  isSubmitted,
  isCorrect,
  isLoading = false,
  explanation,
  onSubmit,
  onContinue,
}) => {
  const { t } = useTranslation();
  const isMultiQuestion = Array.isArray(questions) && questions.length > 0;

  // Single-question selected option
  const [selectedId, setSelectedId] = useState<string | null>(null);

  // Multi-question answers map (questionId -> answer)
  const [answers, setAnswers] = useState<Record<string, string>>({});

  // Timing: reading_ms measured from mount to "I have finished reading"
  const startTimeRef = useRef<number | null>(null);
  const [readingMs, setReadingMs] = useState<number | null>(null);

  useEffect(() => {
    if (startTimeRef.current === null) {
      startTimeRef.current = Date.now();
    }
  }, []);

  // Two-phase flow state: starts in reading mode unless already submitted
  const [hasFinishedReading, setHasFinishedReading] = useState<boolean>(
    isSubmitted || !isMultiQuestion,
  );

  const wordCount = passage.trim() ? passage.trim().split(/\s+/).length : 0;

  const handleFinishReading = useCallback(() => {
    const start = startTimeRef.current ?? Date.now();
    const elapsed = Date.now() - start;
    setReadingMs(elapsed);
    setHasFinishedReading(true);
  }, []);

  const handleAnswerChange = (qId: string, value: string) => {
    if (isSubmitted || isLoading) return;
    setAnswers((prev) => ({ ...prev, [qId]: value }));
  };

  const handleSubmit = useCallback(() => {
    if (isLoading) return;
    if (isMultiQuestion) {
      onSubmit({
        answers,
        reading_ms: readingMs ?? undefined,
      });
    } else if (selectedId) {
      if (readingMs !== null) {
        onSubmit({
          selectedOptionId: selectedId,
          reading_ms: readingMs,
        });
      } else {
        onSubmit(selectedId);
      }
    }
  }, [isLoading, isMultiQuestion, onSubmit, answers, readingMs, selectedId]);

  // Keyboard navigation
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if (
        e.target instanceof HTMLInputElement ||
        e.target instanceof HTMLTextAreaElement
      ) {
        return;
      }

      if (!hasFinishedReading) {
        if (e.key === "Enter" || e.key === " ") {
          e.preventDefault();
          handleFinishReading();
        }
        return;
      }

      if (!isSubmitted) {
        if (!isMultiQuestion) {
          const digit = parseInt(e.key, 10);
          if (digit >= 1 && digit <= options.length) {
            e.preventDefault();
            setSelectedId(options[digit - 1]?.id || null);
          } else if (e.key === "Enter" && selectedId && !isLoading) {
            e.preventDefault();
            handleSubmit();
          }
        }
      } else if (e.key === "Enter" || e.key === " ") {
        e.preventDefault();
        onContinue();
      }
    };

    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, [
    options,
    selectedId,
    isSubmitted,
    isLoading,
    hasFinishedReading,
    isMultiQuestion,
    handleFinishReading,
    handleSubmit,
    onContinue,
  ]);

  const canSubmit = isMultiQuestion
    ? questions.length > 0 && Object.keys(answers).length > 0
    : selectedId !== null;

  return (
    <div className="space-y-6 max-w-3xl mx-auto py-4 animate-in fade-in duration-200">
      {/* Passage Card */}
      <div className="rounded-2xl border border-border bg-surface-card shadow-sm overflow-hidden">
        <div className="px-5 py-3 border-b border-border/60 bg-surface/40 flex items-center justify-between gap-2 flex-wrap">
          <div className="flex items-center gap-2">
            <BookOpen
              className="h-4 w-4 text-primary shrink-0"
              aria-hidden="true"
            />
            <h3 className="text-sm font-semibold text-text uppercase tracking-wider">
              {passageTitle || t("runner.readingPassage", "Reading Passage")}
            </h3>
          </div>
          <div className="flex items-center gap-3 text-xs text-text-muted">
            <span>{t("runner.wordCount", { count: wordCount })}</span>
            {readingMs !== null && (
              <span className="flex items-center gap-1 font-mono">
                <Clock className="h-3 w-3" />
                {Math.round(readingMs / 1000)}s
              </span>
            )}
          </div>
        </div>
        <div className="p-5 md:p-6">
          <p className="text-base md:text-lg leading-relaxed text-text font-normal whitespace-pre-line">
            {passage}
          </p>
        </div>
      </div>

      {/* Phase 1: Reading Mode Call to Action */}
      {!hasFinishedReading && (
        <div className="rounded-2xl border border-primary/20 bg-primary/5 p-6 text-center space-y-4">
          <p className="text-sm text-text-muted max-w-md mx-auto">
            {t(
              "runner.finishReadingPrompt",
              "Read the passage at your normal pace, then proceed to the questions.",
            )}
          </p>
          <Button
            size="lg"
            onClick={handleFinishReading}
            className="gap-2 min-h-[44px] px-6 text-base font-semibold shadow-sm"
          >
            <span>
              {t("runner.finishedReading", "I have finished reading")}
            </span>
            <ArrowRight className="h-4 w-4" />
          </Button>
        </div>
      )}

      {/* Phase 2: Questions List */}
      {hasFinishedReading && isMultiQuestion && (
        <div className="space-y-6 pt-2">
          <div className="flex items-center justify-between border-b border-border pb-2">
            <h4 className="text-sm font-bold uppercase tracking-wider text-text">
              {t("runner.questionsLabel", "Comprehension Questions")}
            </h4>
            <Badge variant="secondary" className="text-xs">
              {questions.length} questions
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
                      <p className="text-base font-medium text-text">
                        {q.prompt}
                      </p>
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
                          hasItemVerdict &&
                          itemResult?.correct_answer === opt.id;

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
                            optClass =
                              "border-border opacity-50 text-text-muted";
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
                            onClick={() => handleAnswerChange(q.id, opt.id)}
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
                        const isSelected =
                          answers[q.id]?.toLowerCase() === tf.val;
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
                            tfClass =
                              "border-border opacity-50 text-text-muted";
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
                            onClick={() => handleAnswerChange(q.id, tf.val)}
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
                        onChange={(e) =>
                          handleAnswerChange(q.id, e.target.value)
                        }
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

                  {/* Revealed Correct Answer on Failure */}
                  {hasItemVerdict &&
                    !isItemCorrect &&
                    itemResult?.correct_answer && (
                      <div className="mt-3 text-xs text-text-muted flex items-center gap-1.5 bg-surface/60 p-2 rounded-md border border-border/40">
                        <span className="font-semibold text-text">
                          {t("runner.correctAnswerLabel", "Correct answer")}:
                        </span>
                        <span className="font-mono text-success font-bold">
                          {itemResult.correct_answer}
                        </span>
                      </div>
                    )}
                </div>
              );
            })}
          </div>
        </div>
      )}

      {/* Phase 2: Legacy Single-Question Mode (BR-CONTENT-01) */}
      {hasFinishedReading && !isMultiQuestion && prompt && (
        <div className="space-y-4 pt-2">
          <ExercisePrompt>{prompt}</ExercisePrompt>

          <div className="space-y-3" role="radiogroup" aria-label={prompt}>
            {options.map((option, index) => {
              const isSelected = selectedId === option.id;
              const isAnswer = correctOptionId === option.id;
              const isSelectedWrong = isSubmitted && isSelected && !isCorrect;
              const isSelectedCorrect = isSubmitted && isSelected && isCorrect;

              let optionStyle =
                "border-border bg-surface-card hover:border-primary/50 text-text";
              if (isSubmitted) {
                if (isSelectedCorrect || isAnswer) {
                  optionStyle =
                    "border-success bg-success/10 text-text font-semibold";
                } else if (isSelectedWrong) {
                  optionStyle =
                    "border-danger bg-danger/10 text-text line-through opacity-80";
                } else {
                  optionStyle = "border-border opacity-50 text-text-muted";
                }
              } else if (isSelected) {
                optionStyle =
                  "border-primary bg-primary/10 text-text font-semibold ring-1 ring-primary";
              }

              return (
                <button
                  key={option.id}
                  type="button"
                  role="radio"
                  aria-checked={isSelected}
                  disabled={isSubmitted || isLoading}
                  onClick={() => setSelectedId(option.id)}
                  className={cn(
                    "w-full text-left p-4 rounded-xl border transition-all duration-150 flex items-center justify-between group cursor-pointer min-h-[44px] text-base",
                    optionStyle,
                    isSubmitted && "cursor-default",
                  )}
                >
                  <div className="flex items-center gap-3">
                    <span
                      className={cn(
                        "flex items-center justify-center w-7 h-7 rounded-lg text-xs font-bold border transition-colors shrink-0",
                        isSelected && !isSubmitted
                          ? "bg-primary text-primary-foreground border-primary"
                          : "bg-surface text-text-muted border-border group-hover:border-primary/40",
                        isSubmitted &&
                          (isSelectedCorrect || isAnswer) &&
                          "bg-success text-success-foreground border-success",
                        isSubmitted &&
                          isSelectedWrong &&
                          "bg-danger text-danger-foreground border-danger",
                      )}
                    >
                      {index + 1}
                    </span>
                    <span className="leading-snug">{option.text}</span>
                  </div>

                  {isSubmitted && (isSelectedCorrect || isAnswer) && (
                    <Check
                      className="h-5 w-5 text-success shrink-0 ml-2"
                      aria-hidden="true"
                    />
                  )}
                  {isSubmitted && isSelectedWrong && (
                    <X
                      className="h-5 w-5 text-danger shrink-0 ml-2"
                      aria-hidden="true"
                    />
                  )}
                </button>
              );
            })}
          </div>
        </div>
      )}

      {/* Feedback Panel */}
      {isSubmitted && (
        <ExerciseFeedback
          isCorrect={isCorrect}
          feedback={feedback}
          explanation={explanation}
        />
      )}

      {/* Action Bar */}
      {hasFinishedReading && (
        <ExerciseActions
          isSubmitted={isSubmitted}
          canSubmit={canSubmit}
          isLoading={isLoading}
          onSubmit={handleSubmit}
          onContinue={onContinue}
        />
      )}
    </div>
  );
};
