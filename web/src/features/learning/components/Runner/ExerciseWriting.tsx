import React, { useEffect, useState } from "react";
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
import { cn } from "@/lib/utils";

import { GuestNotice } from "../GuestNotice";
import {
  type AnswerExplanation,
  ExerciseActions,
  ExercisePrompt,
} from "./ExerciseShell";

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

  const words = countWords(text);
  const meetsMinWords = minWords <= 0 || words >= minWords;
  const canSubmit = text.trim().length > 0 && !isMarking;

  const handleSubmit = () => {
    if (canSubmit && !isLoading && !isMarking) {
      onSubmit(text.trim());
    }
  };

  return (
    <div className="space-y-6 max-w-2xl mx-auto py-4">
      {/* Prompt Heading */}
      <ExercisePrompt>{prompt}</ExercisePrompt>

      {/* Rubric / Instructions Card */}
      {rubric && (
        <div className="rounded-2xl border border-border bg-surface-card/70 shadow-sm p-4 sm:p-5 space-y-2">
          <div className="flex items-center gap-2 text-xs font-semibold uppercase tracking-wider text-primary">
            <PenTool className="h-3.5 w-3.5" aria-hidden="true" />
            <span>
              {t("runner.writingInstructions", "Instructions & Rubric")}
            </span>
          </div>
          <p className="text-sm text-text-muted leading-relaxed whitespace-pre-line">
            {rubric}
          </p>
        </div>
      )}

      {/* Writing Textarea or Guest Notice */}
      {isGuest ? (
        <GuestNotice />
      ) : (
        <div className="space-y-2">
          <label htmlFor="writing-response" className="sr-only">
            {prompt}
          </label>
          <textarea
            id="writing-response"
            value={text}
            disabled={isSubmitted || isLoading || isMarking}
            onChange={(e) => setText(e.target.value)}
            placeholder={t(
              "runner.writingPlaceholder",
              "Write your response here...",
            )}
            rows={6}
            className={cn(
              "w-full rounded-xl border border-border bg-surface-card p-4 text-base text-text leading-relaxed",
              "focus:outline-none focus:ring-2 focus:ring-primary focus:border-transparent transition-all shadow-sm resize-y",
              "min-h-[140px]",
              isSubmitted && isCorrect && "border-success/50 bg-success/5",
              isSubmitted &&
                isCorrect === false &&
                "border-danger/50 bg-danger/5",
            )}
          />

          {/* Word Counter */}
          <div className="flex items-center justify-between text-xs text-text-muted px-1">
            <span>
              {minWords > 0 ? (
                <span
                  className={cn(
                    "font-medium",
                    meetsMinWords ? "text-success" : "text-text-muted",
                  )}
                >
                  {t(
                    "runner.wordCountWithMin",
                    `${words} / ${minWords} words minimum`,
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
              <span className="text-warning text-xs">
                {t(
                  "runner.wordsRemaining",
                  `${Math.max(0, minWords - words)} more needed`,
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
              <Sparkles className="h-5 w-5 text-primary absolute" />
            </div>
          </div>
          <div className="space-y-1">
            <h3 className="text-lg font-bold text-text">
              {t("runner.markingTitle", "Marking your essay...")}
            </h3>
            <p className="text-sm text-text-muted max-w-md mx-auto leading-relaxed">
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
            <AlertCircle className="h-6 w-6 text-amber-500 shrink-0 mt-0.5" />
            <div className="space-y-1">
              <h3 className="text-base font-bold text-text">
                {t(
                  "runner.markingTimeoutTitle",
                  "Marking continues in the background",
                )}
              </h3>
              <p className="text-sm text-text-muted leading-relaxed">
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
                className="gap-1.5"
              >
                <PenTool className="h-4 w-4" />
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
          <div className="flex items-center justify-between gap-4">
            <div className="flex items-center gap-2">
              <Sparkles className="h-5 w-5 text-primary" />
              <h3 className="text-base font-bold text-text">
                {t("runner.writingAssessmentTitle", "Đánh giá bài viết")}
              </h3>
            </div>
            {typeof score === "number" && (
              <span className="font-mono text-sm font-bold px-3 py-1 rounded-lg border bg-primary/10 text-primary border-primary/30">
                {t("writing.score", "Điểm")}: {score} / 100
              </span>
            )}
          </div>

          {isFeedbackLoading && (
            <div className="flex items-center gap-2 text-sm text-text-muted">
              <Loader2 className="h-4 w-4 animate-spin text-primary" />
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
              <p className="text-sm text-text leading-relaxed bg-surface-muted/60 rounded-xl p-4 border border-border/60 whitespace-pre-line">
                {feedback}
              </p>
            </div>
          )}

          {explanation && (
            <div className="space-y-1.5 pt-2 border-t border-border">
              <div className="text-xs font-semibold uppercase tracking-wider text-text-muted">
                {t("runner.explanationLabel", "Giải thích")}
              </div>
              <p className="text-sm text-text leading-relaxed bg-surface-muted/60 rounded-xl p-4 border border-border/60 whitespace-pre-line">
                {Boolean(i18n?.language?.startsWith("vi")) &&
                explanation.text_vi
                  ? explanation.text_vi
                  : explanation.text}
              </p>
            </div>
          )}

          {sampleAnswer && (
            <div className="space-y-1.5 pt-2 border-t border-border">
              <div className="text-xs font-semibold uppercase tracking-wider text-text-muted">
                {t("writing.modelAnswer", "Bài mẫu tham khảo")}
              </div>
              <div className="p-4 rounded-xl border border-border/80 bg-surface-muted/40 text-sm text-text leading-relaxed whitespace-pre-line">
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
          isLoading={isLoading || isMarking}
          onSubmit={handleSubmit}
          onContinue={onContinue}
        />
      )}
    </div>
  );
};
