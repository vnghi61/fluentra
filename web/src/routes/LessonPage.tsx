import React, { useEffect, useRef, useState } from "react";
import { useNavigate, useParams } from "@tanstack/react-router";
import { AlertCircle, RotateCcw } from "lucide-react";
import { useTranslation } from "react-i18next";

import { useAuthStore } from "@/stores/authStore";

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
  SaveProgressPrompt,
  ExerciseFlashcard,
  ExerciseGapFill,
  ExerciseMultipleChoice,
  ActivityUnavailable,
  ExitDialog,
  learningApi,
  RunnerHeader,
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
  // No correct_option_id. The server redacts it out of the lesson body — the
  // answer used to travel with the question, so every learner held the answer
  // key before starting. It arrives on the grade response instead, which is
  // after submitting.
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

/**
 * What the screen needs out of a grading, from either path.
 *
 * A signed-in learner's answer goes through the attempt flow and comes back as
 * SubmitAttemptResult; a guest's goes to POST /activities/{id}/grade and comes
 * back as PreviewGradeResult. The two responses differ in what they say about
 * storage — one has an attempt id, the other says `saved: false` — and agree
 * exactly on the verdict, which is the only part the runner renders.
 */
interface Verdict {
  // `null` as well as `undefined`: the attempt response models these as
  // nullable, because an attempt handed to an async grader has been accepted
  // without yet having a verdict.
  correct?: boolean | null | undefined;
  feedback?: string | null | undefined;
  correct_answer?: string | null | undefined;
}

export function LessonPage(): React.JSX.Element {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const params: Record<string, string | undefined> = useParams({
    strict: false,
  });
  const lessonId = params["lessonId"] ?? "";
  // A guest works through the same lesson with the same grader; what differs is
  // that nothing they do is written down, and the completion screen says so.
  const signedIn = useAuthStore((state) => state.status === "authenticated");

  const {
    data: lesson,
    isLoading: lessonLoading,
    isError,
    error,
    refetch,
  } = useLesson(lessonId);

  const [currentIndex, setCurrentIndex] = useState(0);
  const [currentAttemptId, setCurrentAttemptId] = useState<string | null>(null);
  const [isAttemptStarting, setIsAttemptStarting] = useState(false);
  const [attemptStartFailed, setAttemptStartFailed] = useState(false);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [isSubmitted, setIsSubmitted] = useState(false);
  const [submissionResult, setSubmissionResult] = useState<Verdict | null>(
    null,
  );
  const [submissionError, setSubmissionError] = useState<string | null>(null);
  const [lastSubmittedPayload, setLastSubmittedPayload] = useState<Record<
    string,
    unknown
  > | null>(null);

  // Idempotency key per submission attempt
  const idempotencyKeyRef = useRef<string>(crypto.randomUUID());

  const [scoreCount, setScoreCount] = useState(0);
  const [isCompleted, setIsCompleted] = useState(false);
  // Shown once, at the end, and dismissible. A guest who has decided to keep
  // looking around should not be asked again on the next lesson's last screen.
  const [savePromptDismissed, setSavePromptDismissed] = useState(false);
  const [isExitDialogOpen, setIsExitDialogOpen] = useState(false);
  const [startTime] = useState(() => Date.now());
  const [elapsedSeconds, setElapsedSeconds] = useState(0);

  // `activities` is required on the lesson response; it is absent here only
  // before the query resolves, which the loading branch handles.
  const activities = lesson?.activities ?? [];
  const currentActivity = activities[currentIndex];

  // Start attempt when current activity changes.
  //
  // Skipped entirely for a guest: there is no attempt to start, because there is
  // nobody to attribute one to. Their answers go to the grading route instead,
  // and nothing about the lesson is written down.
  useEffect(() => {
    if (!currentActivity || isCompleted || !signedIn) return;

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
  }, [currentActivity, isCompleted, signedIn]);

  const handleSubmit = async (responsePayload: Record<string, unknown>) => {
    if (signedIn && !currentAttemptId) return;
    if (!currentActivity) return;

    setIsSubmitting(true);
    setSubmissionError(null);
    setLastSubmittedPayload(responsePayload);

    try {
      const body = { response: responsePayload as Record<string, never> };
      // The guest path is deliberately a different call, not the same call with
      // a flag. Nothing it sends can be mistaken for work to be saved.
      const result: Verdict =
        signedIn && currentAttemptId
          ? await learningApi.submitAttempt(
              currentAttemptId,
              body,
              idempotencyKeyRef.current, // Reuses same key on retry
            )
          : await learningApi.gradePreview(currentActivity.id, body);

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
      // Only a signed-in learner is waiting on an attempt to be opened. For a
      // guest nothing is being started, so leaving this true left every
      // activity after the first with its Check Answer button disabled — the
      // effect that would clear it returns early for them.
      setIsAttemptStarting(signedIn);
      setCurrentIndex((prev) => prev + 1);
    } else {
      setElapsedSeconds(
        Math.max(0, Math.round((Date.now() - startTime) / 1000)),
      );
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
        <div className="animate-spin rounded-full h-8 w-8 border-4 border-border-subtle border-t-primary" />
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
            <CardTitle>
              {t("learn.errorTitle", "Unable to Load Lesson")}
            </CardTitle>
            <CardDescription>
              {error?.message ||
                t("learn.errorDesc", "Could not load lesson activities.")}
            </CardDescription>
          </CardHeader>
          <CardFooter className="justify-center gap-3">
            <Button variant="outline" onClick={() => void refetch()}>
              {t("action.retry", "Try again")}
            </Button>
            <Button onClick={() => void navigate({ to: "/learn" })}>
              {t("runner.backToCourseBtn", "Back to Syllabus")}
            </Button>
          </CardFooter>
        </Card>
      </div>
    );
  }

  if (isCompleted) {
    return (
      <>
        <SaveProgressPrompt
          isOpen={!signedIn && !savePromptDismissed}
          score={scoreCount}
          total={activities.length}
          onDismiss={() => setSavePromptDismissed(true)}
        />
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
            setIsAttemptStarting(signedIn);
            setIsCompleted(false);
          }}
        />
      </>
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

  const selectedOptId =
    typeof lastSubmittedPayload?.selected_option_id === "string"
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

        {signedIn && attemptStartFailed && (
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

        {!(signedIn && attemptStartFailed) &&
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
            correctOptionId={
              submissionResult?.correct
                ? selectedOptId
                : (submissionResult?.correct_answer ?? undefined)
            }
            feedback={submissionResult?.feedback}
            isSubmitted={isSubmitted}
            isCorrect={submissionResult?.correct}
            isLoading={isSubmitting || isAttemptStarting}
            onSubmit={(selectedOptionId) =>
              void handleSubmit({ selected_option_id: selectedOptionId })
            }
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
            onSubmit={(answerText) =>
              void handleSubmit({ text_answer: answerText })
            }
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
              void handleSubmit({
                text_answer: knewIt ? (fcConfig.target_word ?? "") : "",
              })
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
