import React, { useEffect, useRef, useState } from "react";
import { useNavigate, useParams } from "@tanstack/react-router";
import { AlertCircle, RotateCcw } from "lucide-react";
import { useTranslation } from "react-i18next";

import { Button } from "@/components/ui/button";
import {
  Card,
  CardDescription,
  CardFooter,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import {
  CompletionScreen,
  ExerciseFlashcard,
  ExerciseGapFill,
  ExerciseMultipleChoice,
  ActivityUnavailable,
  ExitDialog,
  learningApi,
  RunnerHeader,
  type SubmitAttemptResult,
} from "@/features/learning";
import { useLesson } from "@/features/lesson";

// The activity `config` is a free-form object in the spec, because its shape
// belongs to whichever skill module authored the activity and the OpenAPI
// document does not describe every kind. Narrowing it is therefore the client's
// job. These are not hand-written response types: every field is optional, and
// a config missing what its exercise needs renders ActivityUnavailable rather
// than being topped up with a default.
interface MultipleChoiceConfig {
  prompt?: string;
  options?: { id: string; text: string }[];
  correct_option_id?: string;
}

interface GapFillConfig {
  prompt?: string;
  sentence_before?: string;
  sentence_after?: string;
  expected_answer?: string;
}

interface FlashcardConfig {
  prompt?: string;
  target_word?: string;
  ipa?: string;
  definition?: string;
  example_sentence?: string;
}

export function LessonPage(): React.JSX.Element {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const params: Record<string, string | undefined> = useParams({ strict: false });
  const lessonId = params["lessonId"] ?? "";

  const { data: lesson, isLoading: lessonLoading, isError, error, refetch } = useLesson(lessonId);

  const [currentIndex, setCurrentIndex] = useState(0);
  const [currentAttemptId, setCurrentAttemptId] = useState<string | null>(null);
  const [isAttemptStarting, setIsAttemptStarting] = useState(false);
  const [attemptStartFailed, setAttemptStartFailed] = useState(false);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [isSubmitted, setIsSubmitted] = useState(false);
  const [submissionResult, setSubmissionResult] = useState<SubmitAttemptResult | null>(null);
  const [submissionError, setSubmissionError] = useState<string | null>(null);
  const [lastSubmittedPayload, setLastSubmittedPayload] = useState<Record<string, unknown> | null>(null);

  // Idempotency key per submission attempt
  const idempotencyKeyRef = useRef<string>(crypto.randomUUID());

  const [scoreCount, setScoreCount] = useState(0);
  const [isCompleted, setIsCompleted] = useState(false);
  const [isExitDialogOpen, setIsExitDialogOpen] = useState(false);
  const [startTime] = useState(() => Date.now());
  const [elapsedSeconds, setElapsedSeconds] = useState(0);

  // `activities` is required on the lesson response; it is absent here only
  // before the query resolves, which the loading branch handles.
  const activities = lesson?.activities ?? [];
  const currentActivity = activities[currentIndex];

  // Start attempt when current activity changes
  useEffect(() => {
    if (!currentActivity || isCompleted) return;

    let isMounted = true;
    idempotencyKeyRef.current = crypto.randomUUID();

    learningApi
      .startAttempt(currentActivity.id)
      .then((res) => {
        if (isMounted) {
          setCurrentAttemptId(res.attempt_id);
          setAttemptStartFailed(false);
          setIsAttemptStarting(false);
        }
      })
      .catch(() => {
        if (isMounted) {
          // No attempt, no submission. Inventing an id here let the learner work
          // through the whole lesson while every answer was submitted against an
          // attempt the server had never heard of, and lost.
          setCurrentAttemptId(null);
          setAttemptStartFailed(true);
          setIsAttemptStarting(false);
        }
      });

    return () => {
      isMounted = false;
    };
  }, [currentActivity, isCompleted]);

  const handleSubmit = async (responsePayload: Record<string, unknown>) => {
    if (!currentAttemptId) return;

    setIsSubmitting(true);
    setSubmissionError(null);
    setLastSubmittedPayload(responsePayload);

    try {
      const result = await learningApi.submitAttempt(
        currentAttemptId,
        { response: responsePayload as Record<string, never> },
        idempotencyKeyRef.current, // Reuses same key on retry
      );

      setIsSubmitted(true);
      setSubmissionResult(result);
      if (result.correct) {
        setScoreCount((prev) => prev + 1);
      }
    } catch (err: unknown) {
      const message = err instanceof Error ? err.message : null;
      setSubmissionError(
        message ||
          t(
            "runner.errorDesc",
            "We could not submit your attempt due to a network error. Your answer has been saved. Please retry.",
          ),
      );
    } finally {
      setIsSubmitting(false);
    }
  };

  const handleRetrySubmit = () => {
    if (lastSubmittedPayload) {
      void handleSubmit(lastSubmittedPayload);
    }
  };

  const handleContinue = () => {
    if (currentIndex < activities.length - 1) {
      setIsSubmitted(false);
      setSubmissionResult(null);
      setSubmissionError(null);
      setLastSubmittedPayload(null);
      setAttemptStartFailed(false);
      setIsAttemptStarting(true);
      setCurrentIndex((prev) => prev + 1);
    } else {
      setElapsedSeconds(Math.max(0, Math.round((Date.now() - startTime) / 1000)));
      setIsCompleted(true);
    }
  };

  const handleConfirmExit = () => {
    setIsExitDialogOpen(false);
    void navigate({ to: "/learn" });
  };

  if (lessonLoading) {
    return (
      <div className="flex items-center justify-center min-h-[50vh]">
        <div className="animate-spin rounded-full h-8 w-8 border-4 border-slate-700 border-t-primary" />
      </div>
    );
  }

  if (isError || !lesson) {
    return (
      <div className="py-12 max-w-lg mx-auto">
        <Card className="border-danger/30 text-center p-6">
          <CardHeader>
            <div className="flex justify-center mb-2">
              <AlertCircle className="h-10 w-10 text-danger-accent" />
            </div>
            <CardTitle>{t("learn.errorTitle", "Unable to Load Lesson")}</CardTitle>
            <CardDescription>{error?.message || t("learn.errorDesc", "Could not load lesson activities.")}</CardDescription>
          </CardHeader>
          <CardFooter className="justify-center gap-3">
            <Button variant="outline" onClick={() => void refetch()}>{t("action.retry", "Try again")}</Button>
            <Button onClick={() => void navigate({ to: "/learn" })}>{t("runner.backToCourseBtn", "Back to Syllabus")}</Button>
          </CardFooter>
        </Card>
      </div>
    );
  }

  if (isCompleted) {
    return (
      <CompletionScreen
        score={scoreCount}
        totalActivities={activities.length}
        timeSpentSeconds={elapsedSeconds}
        onRetryLesson={() => {
          setCurrentIndex(0);
          setScoreCount(0);
          setIsSubmitted(false);
          setSubmissionResult(null);
          setSubmissionError(null);
          setLastSubmittedPayload(null);
          setIsAttemptStarting(true);
          setIsCompleted(false);
        }}
      />
    );
  }

  const rawConfig = (currentActivity?.config ?? {}) as unknown;
  const kind = currentActivity?.kind;

  const mcConfig = rawConfig as MultipleChoiceConfig;
  const gapConfig = rawConfig as GapFillConfig;
  const fcConfig = rawConfig as FlashcardConfig;

  // An exercise is renderable only when its config carries the fields it needs.
  // Everything else is ActivityUnavailable — there is no default question,
  // because a default question is somebody else's question.
  const canRenderMultipleChoice =
    kind === "vocab_multiple_choice" &&
    typeof mcConfig.prompt === "string" &&
    Array.isArray(mcConfig.options) &&
    mcConfig.options.length > 0;

  const canRenderGapFill =
    kind === "vocab_gap_fill" &&
    typeof gapConfig.expected_answer === "string" &&
    gapConfig.expected_answer !== "";

  const canRenderFlashcard =
    kind === "vocab_flashcard" &&
    typeof fcConfig.target_word === "string" &&
    typeof fcConfig.definition === "string";

  const selectedOptId = typeof lastSubmittedPayload?.selected_option_id === "string"
    ? lastSubmittedPayload.selected_option_id
    : undefined;

  return (
    <div className="min-h-screen bg-surface flex flex-col justify-between">
      {/* Runner Header */}
      <RunnerHeader
        lessonTitle={lesson.title}
        currentStep={currentIndex + 1}
        totalSteps={activities.length}
        onExit={() => setIsExitDialogOpen(true)}
      />

      {/* Main Exercise Canvas */}
      <main className="flex-1 max-w-4xl w-full mx-auto p-4 flex flex-col justify-center">
        {submissionError && (
          <div className="mb-6 p-4 rounded-xl border border-danger/40 bg-danger/10 max-w-2xl mx-auto w-full flex items-center justify-between gap-4">
            <div className="flex items-center gap-2 text-danger-accent text-sm font-medium">
              <AlertCircle className="h-5 w-5 shrink-0" />
              <span>{submissionError}</span>
            </div>
            <Button
              size="sm"
              variant="outline"
              onClick={handleRetrySubmit}
              isLoading={isSubmitting}
              className="shrink-0 gap-1.5"
            >
              <RotateCcw className="h-3.5 w-3.5" />
              {t("runner.retryBtn", "Retry")}
            </Button>
          </div>
        )}

        {attemptStartFailed && (
          <Card className="max-w-2xl mx-auto w-full text-center p-6 border-danger/30">
            <CardHeader>
              <CardTitle>{t("runner.attemptFailedTitle")}</CardTitle>
              <CardDescription>{t("runner.attemptFailedDesc")}</CardDescription>
            </CardHeader>
            <CardFooter className="justify-center">
              <Button onClick={() => setCurrentIndex((prev) => prev)}>
                {t("action.retry")}
              </Button>
            </CardFooter>
          </Card>
        )}

        {!attemptStartFailed &&
          !canRenderMultipleChoice &&
          !canRenderGapFill &&
          !canRenderFlashcard && (
            <ActivityUnavailable
              {...(kind !== undefined && { kind })}
              onSkip={handleContinue}
            />
          )}

        {canRenderMultipleChoice && (
          <ExerciseMultipleChoice
            prompt={mcConfig.prompt ?? ""}
            options={mcConfig.options ?? []}
            correctOptionId={submissionResult?.correct ? selectedOptId : mcConfig.correct_option_id}
            feedback={submissionResult?.feedback}
            isSubmitted={isSubmitted}
            isCorrect={submissionResult?.correct}
            isLoading={isSubmitting || isAttemptStarting}
            onSubmit={(selectedOptionId) => void handleSubmit({ selected_option_id: selectedOptionId })}
            onContinue={handleContinue}
          />
        )}

        {canRenderGapFill && (
          <ExerciseGapFill
            prompt={gapConfig.prompt ?? ""}
            sentenceBeforeBlank={gapConfig.sentence_before ?? ""}
            sentenceAfterBlank={gapConfig.sentence_after ?? ""}
            expectedAnswer={gapConfig.expected_answer ?? ""}
            feedback={submissionResult?.feedback}
            isSubmitted={isSubmitted}
            isCorrect={submissionResult?.correct}
            isLoading={isSubmitting || isAttemptStarting}
            onSubmit={(answerText) => void handleSubmit({ text_answer: answerText })}
            onContinue={handleContinue}
          />
        )}

        {canRenderFlashcard && (
          <ExerciseFlashcard
            prompt={fcConfig.prompt ?? ""}
            targetWord={fcConfig.target_word ?? ""}
            ipa={fcConfig.ipa ?? ""}
            definition={fcConfig.definition ?? ""}
            exampleSentence={fcConfig.example_sentence ?? ""}
            isLoading={isSubmitting || isAttemptStarting}
            isSubmitted={isSubmitted}
            isCorrect={submissionResult?.correct}
            // The recall verdict is the answer. "I knew it" submits the word
            // itself, which is what the grader calls a typed recall; "Not yet"
            // submits nothing, and an empty recall is not a match. Either way
            // the attempt is graded rather than abandoned, so it stops leaking
            // an `in_progress` row and starts counting towards progress.
            onSubmit={(knewIt) =>
              void handleSubmit({ text_answer: knewIt ? (fcConfig.target_word ?? "") : "" })
            }
            onContinue={handleContinue}
          />
        )}
      </main>

      {/* Exit Confirmation Dialog */}
      <ExitDialog
        isOpen={isExitDialogOpen}
        onCancel={() => setIsExitDialogOpen(false)}
        onConfirm={handleConfirmExit}
      />
    </div>
  );
}

export default LessonPage;
