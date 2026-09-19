import React, { useCallback, useEffect, useRef, useState } from "react";
import { ArrowRight, BookOpen, Check, Clock, X } from "lucide-react";
import { useTranslation } from "react-i18next";

import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";

import {
  type AnswerExplanation,
  ExerciseActions,
  ExerciseFeedback,
  ExercisePrompt,
} from "./ExerciseShell";
import type { OptionItem } from "./ExerciseMultipleChoice";
import {
  type ItemResult,
  QuestionList,
  type ReadingQuestionItem,
} from "./QuestionList";

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
        <QuestionList
          questions={questions}
          answers={answers}
          onAnswerChange={handleAnswerChange}
          itemResults={itemResults}
          isSubmitted={isSubmitted}
          isLoading={isLoading}
        />
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
