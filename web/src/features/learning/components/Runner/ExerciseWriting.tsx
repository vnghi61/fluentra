import React, { useEffect, useRef, useState } from "react";
import { AlertCircle, Loader2, PenTool, Sparkles } from "lucide-react";
import { useTranslation } from "react-i18next";

import { Button } from "@/components/ui/button";
import {
  getWritingDraft,
  saveWritingDraft,
  type WritingFeedback,
  WritingFeedbackView,
} from "@/features/writing";
import { writingApi } from "@/features/writing/api/writingApi";
import { pickLocalized } from "@/lib/localized";
import { cn } from "@/lib/utils";

import { GuestNotice } from "../GuestNotice";
import {
  type AnswerExplanation,
  ExerciseActions,
  ExercisePrompt,
} from "./ExerciseShell";
import { Textarea } from "@/components/ui/textarea";

export interface ExerciseWritingProps {
  /** The writing prompt / task description. */
  prompt: string;
  /** Guidelines, constraints, or evaluation rubric. */
  rubric?: string | undefined;
  /** Minimum required word count. */
  minWords?: number | undefined;
  sampleAnswer?: string | undefined;
  feedback?: string | null | undefined;
  score?: number | null | undefined;
  isSubmitted: boolean;
  isCorrect?: boolean | null | undefined;
  isLoading?: boolean | undefined;
  isGuest?: boolean | undefined;
  isMarking?: boolean | undefined;
  markingTimedOut?: boolean | undefined;
  writingFeedback?: WritingFeedback | null | undefined;
  attemptId?: string | null | undefined;
  userId?: string | undefined;
  activityId?: string | undefined;
  onNavigateToMyWriting?: (() => void) | undefined;
  explanation?: AnswerExplanation | null | undefined;
  onSubmit: (answerText: string) => void;
  onContinue: () => void;
}

function countWords(text: string): number {
  const trimmed = text.trim();
  if (!trimmed) return 0;
  return trimmed.split(/\s+/).length;
}

/**
 * `writing_prompt` — compose open-ended written text evaluated via AI.
 *
 * Provides a rich writing prompt, optional rubric/minimum word guidelines,
 * live word-counter, draft autosave in localStorage, marking progress, and interactive grading feedback.
 */
export const ExerciseWriting: React.FC<ExerciseWritingProps> = ({
  prompt,
  rubric,
  minWords = 0,
  sampleAnswer,
  feedback,
  score,
  isSubmitted,
  isCorrect,
  isLoading = false,
  isGuest = false,
  isMarking = false,
  markingTimedOut = false,
  writingFeedback,
  attemptId,
  userId,
  activityId,
  onNavigateToMyWriting,
  explanation,
  onSubmit,
  onContinue,
}) => {
  const { t, i18n } = useTranslation();
  const [fetchedFeedback, setFetchedFeedback] =
    useState<WritingFeedback | null>(null);
  // The attempt whose feedback request has settled, successfully or not.
  // Loading is derived from it rather than stored: setting a loading flag
  // synchronously in the effect body below is a cascading render, and the
  // React Compiler lint rule rejects it.
  const [settledAttemptId, setSettledAttemptId] = useState<string | null>(null);

  const shouldFetchFeedback = Boolean(
    isSubmitted && !isMarking && !writingFeedback && attemptId,
  );
  const isFeedbackLoading =
    shouldFetchFeedback && settledAttemptId !== attemptId;

  useEffect(() => {
    if (!shouldFetchFeedback || !attemptId) return;
    let isMounted = true;
    writingApi
      .getFeedback(attemptId)
      .then((fb) => {
        if (isMounted) setFetchedFeedback(fb);
      })
      .catch(() => {})
      .finally(() => {
        if (isMounted) setSettledAttemptId(attemptId);
      });
    return () => {
      isMounted = false;
    };
  }, [shouldFetchFeedback, attemptId]);

  const activeFeedback = writingFeedback ?? fetchedFeedback ?? null;
  const [text, setText] = useState(() => {
    if (userId && activityId) {
      return getWritingDraft(userId, activityId);
    }
    return "";
  });

  // Autosave draft into localStorage while working
  useEffect(() => {
    if (!isSubmitted && userId && activityId) {
      saveWritingDraft(userId, activityId, text);
    }
  }, [text, isSubmitted, userId, activityId]);

  const [isPendingSubmit, setIsPendingSubmit] = useState(false);
  const hasSubmittedRef = useRef(false);
  const submitTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  useEffect(() => {
    return () => {
      if (submitTimerRef.current !== null) {
        clearTimeout(submitTimerRef.current);
      }
    };
  }, []);

  // A change in the parent's submit state settles the pending guard. The state
  // half is adjusted during render; the ref and timer half stays in the effect.
  const submitFlags = `${isLoading}|${isMarking}|${isSubmitted}`;
  const [prevSubmitFlags, setPrevSubmitFlags] = useState(submitFlags);
  if (prevSubmitFlags !== submitFlags) {
    setPrevSubmitFlags(submitFlags);
    setIsPendingSubmit(false);
  }

  useEffect(() => {
    if (submitTimerRef.current !== null) {
      clearTimeout(submitTimerRef.current);
    }
    hasSubmittedRef.current = false;
  }, [isLoading, isMarking, isSubmitted]);

  const words = countWords(text);
  const meetsMinWords = minWords <= 0 || words >= minWords;
  const canSubmit =
    text.trim().length > 0 &&
    meetsMinWords &&
    !isMarking &&
    !isLoading &&
    !isPendingSubmit;

  const handleSubmit = () => {
    if (hasSubmittedRef.current) return;
    if (canSubmit) {
      hasSubmittedRef.current = true;
      setIsPendingSubmit(true);
      onSubmit(text.trim());
      submitTimerRef.current = setTimeout(() => {
        hasSubmittedRef.current = false;
        setIsPendingSubmit(false);
      }, 0);
    }
  };

  return (
    <div className="space-y-6 max-w-2xl mx-auto py-4">
      {/* Prompt Heading */}
      <ExercisePrompt>{prompt}</ExercisePrompt>

      {/* Rubric / Instructions Card */}
      {rubric && (
        <div className="rounded-2xl border border-border bg-surface-card/70 shadow-sm p-4 sm:p-5 space-y-2">
          <div className="flex items-center gap-2 text-xs font-semibold uppercase tracking-wider text-primary-accent">
            <PenTool className="h-4 w-4" aria-hidden="true" />
            <span>
              {t("runner.writingInstructions", "Instructions & Rubric")}
            </span>
          </div>
          <p className="text-base text-text-muted leading-relaxed whitespace-pre-line">
            {rubric}
          </p>
        </div>
      )}

      {/* Writing Textarea or Guest Notice */}
      {isGuest ? (
        <GuestNotice />
      ) : (
        <div className="space-y-2">
          <label
            htmlFor="writing-response"
            className="block text-sm font-semibold text-text"
          >
            {t("runner.writingLabel", "Your writing")}
          </label>
          <Textarea
            id="writing-response"
            value={text}
            disabled={isSubmitted || isLoading || isMarking || isPendingSubmit}
            onChange={(e) => setText(e.target.value)}
            placeholder={t(
              "runner.writingPlaceholder",
              "Write your response here...",
            )}
            rows={8}
            className={cn(
              "min-h-[220px] resize-y rounded-xl bg-surface-card p-4 shadow-sm md:min-h-[320px]",
              isSubmitted && isCorrect && "border-success/50 bg-success/5",
              isSubmitted &&
                isCorrect === false &&
                "border-danger/50 bg-danger/5",
            )}
          />

          {/* Word Counter */}
          <div className="flex flex-wrap items-center justify-between gap-x-3 gap-y-1 text-sm text-text-muted px-1">
            <span>
              {minWords > 0 ? (
                <span
                  className={cn(
                    "font-medium",
                    meetsMinWords ? "text-success-accent" : "text-text-muted",
                  )}
                >
                  {t(
                    "runner.wordCountWithMin",
                    `${words} words · minimum ${minWords} words`,
                    {
                      count: words,
                      min: minWords,
                    },
                  )}
                </span>
              ) : (
                <span>
                  {t("runner.wordCount", `${words} words`, { count: words })}
                </span>
              )}
            </span>
            {minWords > 0 && !meetsMinWords && (
              <span className="text-warning-accent">
                {t(
                  "runner.wordsRemaining",
                  `Write ${Math.max(0, minWords - words)} more words to check`,
                  {
                    remaining: Math.max(0, minWords - words),
                  },
                )}
              </span>
            )}
          </div>
        </div>
      )}

      {/* Marking Progress UI */}
      {isMarking && (
        <div className="rounded-2xl border border-primary/30 bg-primary/5 p-6 text-center space-y-4 shadow-sm animate-in fade-in duration-200">
          <div className="flex justify-center">
            <div className="relative flex items-center justify-center">
              <div className="h-12 w-12 rounded-full border-4 border-primary/20 border-t-primary animate-spin" />
              <Sparkles
                className="h-5 w-5 text-primary absolute"
                aria-hidden="true"
              />
            </div>
          </div>
          <div className="space-y-1">
            <h3 className="text-lg font-semibold leading-snug text-text">
              {t("runner.markingTitle", "Marking your essay...")}
            </h3>
            <p className="text-base text-text-muted max-w-md mx-auto leading-relaxed">
              {t(
                "runner.markingDesc",
                "Our AI examiner is evaluating your writing against IELTS criteria. This usually takes 15-30 seconds.",
              )}
            </p>
          </div>
        </div>
      )}

      {/* Marking Timed Out Notice */}
      {markingTimedOut && (
        <div className="rounded-2xl border border-amber-500/40 bg-amber-500/10 p-6 space-y-4 shadow-sm animate-in fade-in duration-200">
          <div className="flex items-start gap-3">
            <AlertCircle
              className="h-5 w-5 text-amber-500 shrink-0 mt-0.5"
              aria-hidden="true"
            />
            <div className="min-w-0 space-y-1">
              <h3 className="text-base font-semibold leading-snug text-text">
                {t(
                  "runner.markingTimeoutTitle",
                  "Marking continues in the background",
                )}
              </h3>
              <p className="text-base text-text-muted leading-relaxed">
                {t(
                  "runner.markingTimeoutDesc",
                  "Grading is taking longer than usual. You can continue your lesson, and your score and detailed feedback will appear in My Writing.",
                )}
              </p>
            </div>
          </div>
          <div className="flex flex-wrap items-center justify-end gap-3 pt-2">
            {onNavigateToMyWriting && (
              <Button
                variant="outline"
                onClick={onNavigateToMyWriting}
                className="gap-2"
              >
                <PenTool className="h-4 w-4" aria-hidden="true" />
                <span>{t("runner.goToMyWriting", "View My Writing")}</span>
              </Button>
            )}
            <Button onClick={onContinue}>
              {t("runner.continueBtn", "Continue")}
            </Button>
          </div>
        </div>
      )}

      {/* Detailed Writing Feedback View */}
      {isSubmitted && !isMarking && !markingTimedOut && activeFeedback && (
        <WritingFeedbackView
          feedback={activeFeedback}
          essayText={text}
          sampleAnswer={sampleAnswer}
          onContinue={onContinue}
        />
      )}

      {/* Writing Assessment & Feedback Panel when full interactive view is loading or unavailable */}
      {isSubmitted && !isMarking && !markingTimedOut && !activeFeedback && (
        <div className="rounded-2xl border border-border bg-surface-card p-6 space-y-4 shadow-sm animate-in fade-in duration-200">
          <div className="flex flex-wrap items-center justify-between gap-x-4 gap-y-2">
            <div className="flex items-center gap-2">
              <Sparkles className="h-5 w-5 text-primary" aria-hidden="true" />
              <h3 className="text-base font-semibold leading-snug text-text">
                {t("runner.writingAssessmentTitle", "Đánh giá bài viết")}
              </h3>
            </div>
            {typeof score === "number" && (
              <span className="font-mono text-sm font-bold tabular-nums px-3 py-1 rounded-lg border bg-primary/10 text-primary-accent border-primary/30">
                {t("writing.score", "Điểm")}: {score} / 100
              </span>
            )}
          </div>

          {isFeedbackLoading && (
            <div className="flex items-center gap-2 text-sm text-text-muted">
              <Loader2
                className="h-4 w-4 animate-spin text-primary"
                aria-hidden="true"
              />
              <span>
                {t("runner.loadingFeedback", "Đang tải chi tiết nhận xét...")}
              </span>
            </div>
          )}

          {feedback && (
            <div className="space-y-1.5 pt-2 border-t border-border">
              <div className="text-xs font-semibold uppercase tracking-wider text-text-muted">
                {t("writing.summary", "Nhận xét của giám khảo AI")}
              </div>
              <p className="text-base text-text leading-relaxed bg-surface-muted/60 rounded-xl p-4 border border-border/60 whitespace-pre-line">
                {feedback}
              </p>
            </div>
          )}

          {explanation && (
            <div className="space-y-1.5 pt-2 border-t border-border">
              <div className="text-xs font-semibold uppercase tracking-wider text-text-muted">
                {t("runner.explanationLabel", "Giải thích")}
              </div>
              <p className="text-base text-text leading-relaxed bg-surface-muted/60 rounded-xl p-4 border border-border/60 whitespace-pre-line">
                {pickLocalized(
                  explanation.text,
                  explanation.text_vi,
                  i18n.language,
                )}
              </p>
            </div>
          )}

          {sampleAnswer && (
            <div className="space-y-1.5 pt-2 border-t border-border">
              <div className="text-xs font-semibold uppercase tracking-wider text-text-muted">
                {t("writing.modelAnswer", "Bài mẫu tham khảo")}
              </div>
              <div className="p-4 rounded-xl border border-border/80 bg-surface-muted/40 text-base text-text leading-relaxed whitespace-pre-line">
                {sampleAnswer}
              </div>
            </div>
          )}
        </div>
      )}

      {/* Action Bar (hidden when WritingFeedbackView or markingTimedOut renders its own continue button) */}
      {!activeFeedback && !markingTimedOut && (
        <ExerciseActions
          isSubmitted={isSubmitted || isGuest}
          canSubmit={canSubmit}
          isLoading={isLoading || isMarking || isPendingSubmit}
          onSubmit={handleSubmit}
          onContinue={onContinue}
        />
      )}
    </div>
  );
};
