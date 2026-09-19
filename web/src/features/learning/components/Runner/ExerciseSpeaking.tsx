import React, { useEffect, useState } from "react";
import { AlertCircle, Loader2, Mic, Sparkles } from "lucide-react";
import { useTranslation } from "react-i18next";

import { Button } from "@/components/ui/button";
import {
  AudioRecorder,
  speakingApi,
  type SpeakingFeedback,
  SpeakingFeedbackView,
} from "@/features/speaking";

import { GuestNotice } from "../GuestNotice";
import {
  type AnswerExplanation,
  ExerciseActions,
  ExercisePrompt,
} from "./ExerciseShell";

export interface ExerciseSpeakingProps {
  prompt?: string | undefined;
  referenceText?: string | undefined;
  taskType?: "read_aloud" | "respond" | undefined;
  speakingTimeSeconds?: number | undefined;
  feedback?: string | null | undefined;
  score?: number | null | undefined;
  isSubmitted: boolean;
  isCorrect?: boolean | null | undefined;
  isLoading?: boolean | undefined;
  isGuest?: boolean | undefined;
  isMarking?: boolean | undefined;
  markingTimedOut?: boolean | undefined;
  speakingFeedback?: SpeakingFeedback | null | undefined;
  attemptId?: string | null | undefined;
  userId?: string | undefined;
  activityId?: string | undefined;
  onNavigateToMySpeaking?: (() => void) | undefined;
  explanation?: AnswerExplanation | null | undefined;
  onSubmit: (audioObjectKey: string) => void;
  onContinue: () => void;
}

export const ExerciseSpeaking: React.FC<ExerciseSpeakingProps> = ({
  prompt = "",
  referenceText,
  taskType = "respond",
  speakingTimeSeconds = 45,
  feedback,
  score,
  isSubmitted,
  isLoading = false,
  isGuest = false,
  isMarking = false,
  markingTimedOut = false,
  speakingFeedback,
  attemptId,
  onNavigateToMySpeaking,
  explanation,
  onSubmit,
  onContinue,
}) => {
  const { t, i18n } = useTranslation();
  const [recordedKey, setRecordedKey] = useState<string>("");
  const [dailyUsed, setDailyUsed] = useState<number | undefined>(undefined);
  const [dailyLimit, setDailyLimit] = useState<number | undefined>(undefined);

  const [fetchedFeedback, setFetchedFeedback] =
    useState<SpeakingFeedback | null>(null);
  const [settledAttemptId, setSettledAttemptId] = useState<string | null>(null);

  // Load quota stats on mount
  useEffect(() => {
    if (isGuest || isSubmitted) return;
    let isMounted = true;
    speakingApi
      .getUploadIntent("audio/webm")
      .then((intent) => {
        if (isMounted) {
          setDailyUsed(intent.daily_recordings_used);
          setDailyLimit(intent.daily_recordings_limit);
        }
      })
      .catch(() => {
        // Quota check failure will be handled at recording time
      });
    return () => {
      isMounted = false;
    };
  }, [isGuest, isSubmitted]);

  const shouldFetchFeedback = Boolean(
    isSubmitted && !isMarking && !speakingFeedback && attemptId,
  );
  const isFeedbackLoading =
    shouldFetchFeedback && settledAttemptId !== attemptId;

  useEffect(() => {
    if (!shouldFetchFeedback || !attemptId) return;
    let isMounted = true;
    speakingApi
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

  const activeFeedback = speakingFeedback ?? fetchedFeedback ?? null;
  const canSubmit = recordedKey.trim().length > 0 && !isMarking && !isSubmitted;

  const handleSubmit = () => {
    if (canSubmit && !isLoading && !isMarking) {
      onSubmit(recordedKey.trim());
    }
  };

  return (
    <div className="space-y-6 max-w-2xl mx-auto py-4">
      {/* Prompt Heading */}
      {prompt && <ExercisePrompt>{prompt}</ExercisePrompt>}

      {/* Guest Notice or Audio Recorder */}
      {isGuest ? (
        <GuestNotice />
      ) : !isSubmitted && (
        <div className="space-y-4">
          <AudioRecorder
            taskType={taskType}
            promptText={prompt}
            referenceText={referenceText}
            speakingTimeSeconds={speakingTimeSeconds}
            mode="practice"
            currentRecordingKey={recordedKey}
            onRecordingComplete={(key) => setRecordedKey(key)}
            dailyRecordingsUsed={dailyUsed}
            dailyRecordingsLimit={dailyLimit}
            disabled={isMarking || isLoading}
          />
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
              {t("runner.markingSpeakingTitle", "Grading your recording...")}
            </h3>
            <p className="text-sm text-text-muted max-w-md mx-auto leading-relaxed">
              {t(
                "runner.markingSpeakingDesc",
                "Our speech engine is transcribing and analyzing your spoken practice. This usually takes 15-30 seconds.",
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
                  "runner.speakingMarkingTimeoutDesc",
                  "Grading is taking a little longer than usual. You can continue your lesson, and your evaluation and transcript will appear in My Speaking.",
                )}
              </p>
            </div>
          </div>
          <div className="flex flex-wrap items-center justify-end gap-3 pt-2">
            {onNavigateToMySpeaking && (
              <Button
                variant="outline"
                onClick={onNavigateToMySpeaking}
                className="gap-1.5"
              >
                <Mic className="h-4 w-4" />
                <span>{t("runner.goToMySpeaking", "View My Speaking")}</span>
              </Button>
            )}
            <Button onClick={onContinue}>
              {t("runner.continueBtn", "Continue")}
            </Button>
          </div>
        </div>
      )}

      {/* Detailed Speaking Feedback View */}
      {isSubmitted && !isMarking && !markingTimedOut && activeFeedback && (
        <SpeakingFeedbackView
          feedback={activeFeedback}
          taskType={taskType}
          referenceText={referenceText}
          onContinue={onContinue}
        />
      )}

      {/* Fallback Speaking Feedback Panel when active feedback is still loading */}
      {isSubmitted && !isMarking && !markingTimedOut && !activeFeedback && (
        <div className="rounded-2xl border border-border bg-surface-card p-6 space-y-4 shadow-sm animate-in fade-in duration-200">
          <div className="flex items-center justify-between gap-4">
            <div className="flex items-center gap-2">
              <Sparkles className="h-5 w-5 text-primary" />
              <h3 className="text-base font-bold text-text">
                {t("speaking.assessmentTitle", "Speaking Evaluation")}
              </h3>
            </div>
            {typeof score === "number" && (
              <span className="font-mono text-sm font-bold px-3 py-1 rounded-lg border bg-primary/10 text-primary border-primary/30">
                {t("speaking.score", "Score")}: {score} / 100
              </span>
            )}
          </div>

          {isFeedbackLoading && (
            <div className="flex items-center gap-2 text-sm text-text-muted">
              <Loader2 className="h-4 w-4 animate-spin text-primary" />
              <span>
                {t("runner.loadingFeedback", "Loading evaluation details...")}
              </span>
            </div>
          )}

          {feedback && (
            <div className="space-y-1.5 pt-2 border-t border-border">
              <div className="text-xs font-semibold uppercase tracking-wider text-text-muted">
                {t("speaking.overallFeedback", "Overall Feedback")}
              </div>
              <p className="text-sm text-text leading-relaxed bg-surface-muted/60 rounded-xl p-4 border border-border/60 whitespace-pre-line">
                {feedback}
              </p>
            </div>
          )}

          {explanation && (
            <div className="space-y-1.5 pt-2 border-t border-border">
              <div className="text-xs font-semibold uppercase tracking-wider text-text-muted">
                {t("runner.explanationLabel", "Explanation")}
              </div>
              <p className="text-sm text-text leading-relaxed bg-surface-muted/60 rounded-xl p-4 border border-border/60 whitespace-pre-line">
                {Boolean(i18n?.language?.startsWith("vi")) && explanation.text_vi
                  ? explanation.text_vi
                  : explanation.text}
              </p>
            </div>
          )}
        </div>
      )}

      {/* Action Bar */}
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

export default ExerciseSpeaking;
