import React, { useEffect, useRef, useState } from "react";
import { useQueryClient } from "@tanstack/react-query";
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
import { lessonKeys } from "@/features/lesson";
import { reviewKeys } from "@/features/review";
import {
  CompletionScreen,
  SaveProgressPrompt,
  ExerciseContextChoice,
  ExerciseFlashcard,
  ExerciseGapFill,
  ExerciseListenType,
  ExerciseMatch,
  ExerciseMultipleChoice,
  ExerciseReorder,
  ActivityUnavailable,
  ExitDialog,
  learningApi,
  learningKeys,
  RunnerHeader,
} from "@/features/learning";
import { useLesson } from "@/features/lesson";
import { readExampleSentences } from "@/lib/examples";

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

interface ListenTypeConfig {
  prompt?: string;
  // The word the browser speaks. It is the answer, and it reaches the client
  // because synthesis happens there — see ExerciseListenType for why that is a
  // deliberate trade rather than an oversight.
  audio_text?: string;
  audio_url?: string;
  ipa?: string;
  hint?: string;
}

interface MatchConfig {
  prompt?: string;
  words?: { id: string; text: string }[];
  definitions?: { id: string; text: string }[];
  // No correct_pairs: the server redacts the matching key out of the body, the
  // same way it redacts correct_option_id.
}

interface ReorderConfig {
  prompt?: string;
  tokens?: string[];
  target_word?: string;
}

interface ContextChoiceConfig {
  prompt?: string;
  sentence?: string;
  target_word?: string;
  options?: { id: string; text: string }[];
}

interface FlashcardConfig {
  prompt?: string;
  target_word?: string;
  ipa?: string;
  definition?: string;
  definition_vi?: string;
  example_sentence?: string;
  example_sentences?: string[];
  audio_url?: string;
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
  // Matching is the one kind that can be partly right, and "incorrect" is a
  // poor description of three pairs out of four.
  score?: number | null | undefined;
  explanation?:
    | {
        text: string;
        text_vi: string;
      }
    | null
    | undefined;
}

export function LessonPage(): React.JSX.Element {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
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

  /**
   * True while a signed-in learner has no attempt to submit against yet.
   *
   * Derived rather than stored, and that is the fix. It used to be a
   * `isAttemptStarting` flag initialised to false and set only by
   * handleContinue, so it covered activities 2..N and left the first one
   * uncovered: during the startAttempt round trip the Check Answer button was
   * live while handleSubmit's `if (signedIn && !currentAttemptId) return` threw
   * the answer away in silence. No request, no error, nothing on screen -- the
   * button just kept saying Check Answer.
   *
   * The window is one round trip wide. That was 81ms on the CI runner, which is
   * how [mobile-android] lost the race on main while the other four projects
   * won it, and it is the whole of the difference between a green suite and a
   * learner whose first answer of every lesson does nothing.
   *
   * A derivation cannot disagree with the id it is derived from, which is what
   * two pieces of state did. `attemptStartFailed` is excluded because that
   * renders its own screen in place of the exercise.
   */
  const isAttemptPending = signedIn && !currentAttemptId && !attemptStartFailed;

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
        }
      })
      .catch(() => {
        if (isMounted) {
          // No attempt, no submission. Inventing an id here let the learner work
          // through the whole lesson while every answer was submitted against an
          // attempt the server had never heard of, and lost.
          setCurrentAttemptId(null);
          setAttemptStartFailed(true);
        }
      });

    return () => {
      isMounted = false;
    };
  }, [currentActivity, isCompleted, signedIn]);

  /**
   * Drops every cache a graded answer has just made stale.
   *
   * Grading writes progress on the server — the activity, the lesson, the
   * course rollup — and schedules review cards. None of that reached the
   * screens that show it: TanStack Query had already cached the course and the
   * dashboard, nothing here invalidated them, and a learner who answered every
   * activity and pressed back saw the same "not started" course they had left.
   * The work was saved; only the reading of it was stale, which is the worst
   * version of the bug because it looks exactly like the work being lost.
   *
   * A guest has no progress to invalidate, and no cache entry keyed to them.
   */
  const invalidateProgress = () => {
    if (!signedIn) return;
    // Fire-and-forget: a refetch that fails must not fail the answer, which is
    // already committed on the server.
    void queryClient.invalidateQueries({ queryKey: learningKeys.all });
    void queryClient.invalidateQueries({ queryKey: lessonKeys.all });
    // Grading schedules review cards, so the due count on the dashboard and the
    // review queue itself are both stale the moment an answer lands.
    void queryClient.invalidateQueries({ queryKey: reviewKeys.all });
  };

  const handleSubmit = async (responsePayload: Record<string, unknown>) => {
    if (!currentActivity) return;

    // No attempt, no submission -- but say so. Returning quietly here is what
    // turned a lost race into an invisible one: the answer was dropped, and the
    // screen was identical to one where nothing had been clicked at all. The
    // guard should now be unreachable, since the button is disabled for exactly
    // as long as this is true. If it is ever reached again, the learner finds
    // out, and the error banner already carries a Retry.
    if (signedIn && !currentAttemptId) {
      setSubmissionError(t("runner.attemptNotReady"));
      return;
    }

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
      // On every graded answer, not only on the last one: a learner who leaves
      // a lesson half-way has still made progress, and the course screen has to
      // show it.
      invalidateProgress();
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
      // The previous activity's attempt does not belong to the next one, and
      // leaving it here is what made a second flag necessary: two values that
      // had to agree about whether an answer could be sent. Clearing it is both
      // the guard and the truth.
      setCurrentAttemptId(null);
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
          {...(lesson.next_lesson_id
            ? { nextLessonId: lesson.next_lesson_id }
            : {})}
          onRetryLesson={() => {
            setCurrentIndex(0);
            setScoreCount(0);
            setIsSubmitted(false);
            setSubmissionResult(null);
            setSubmissionError(null);
            setLastSubmittedPayload(null);
            // Same reason as handleContinue: the attempt this learner finished
            // the lesson on is not the one activity 1 is about to open.
            setCurrentAttemptId(null);
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
  const listenConfig = rawConfig as ListenTypeConfig;
  const matchConfig = rawConfig as MatchConfig;
  const reorderConfig = rawConfig as ReorderConfig;
  const contextConfig = rawConfig as ContextChoiceConfig;

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

  const canRenderListenType =
    kind === "vocab_listen_type" &&
    typeof listenConfig.audio_text === "string" &&
    listenConfig.audio_text !== "";

  const canRenderMatch =
    kind === "vocab_match" &&
    Array.isArray(matchConfig.words) &&
    Array.isArray(matchConfig.definitions) &&
    matchConfig.words.length > 0 &&
    // Unequal columns mean an authoring fault, and rendering it produces an
    // exercise that cannot be completed however well the learner knows the words.
    matchConfig.words.length === matchConfig.definitions.length;

  const canRenderReorder =
    kind === "vocab_reorder" &&
    Array.isArray(reorderConfig.tokens) &&
    reorderConfig.tokens.length > 1;

  const canRenderContextChoice =
    kind === "vocab_context_choice" &&
    typeof contextConfig.sentence === "string" &&
    contextConfig.sentence !== "" &&
    Array.isArray(contextConfig.options) &&
    contextConfig.options.length > 0;

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
          !canRenderFlashcard &&
          !canRenderListenType &&
          !canRenderMatch &&
          !canRenderReorder &&
          !canRenderContextChoice && (
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
            explanation={submissionResult?.explanation}
            isSubmitted={isSubmitted}
            isCorrect={submissionResult?.correct}
            isLoading={isSubmitting || isAttemptPending}
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
            explanation={submissionResult?.explanation}
            isSubmitted={isSubmitted}
            isCorrect={submissionResult?.correct}
            isLoading={isSubmitting || isAttemptPending}
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
            {...(fcConfig.definition_vi !== undefined && {
              definitionVi: fcConfig.definition_vi,
            })}
            exampleSentences={readExampleSentences(
              fcConfig as unknown as Record<string, unknown>,
            )}
            {...(fcConfig.audio_url !== undefined && {
              audioUrl: fcConfig.audio_url,
            })}
            isLoading={isSubmitting || isAttemptPending}
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

        {canRenderListenType && (
          <ExerciseListenType
            prompt={listenConfig.prompt ?? ""}
            audioText={listenConfig.audio_text ?? ""}
            {...(listenConfig.audio_url !== undefined && {
              audioUrl: listenConfig.audio_url,
            })}
            {...(listenConfig.ipa !== undefined && { ipa: listenConfig.ipa })}
            {...(listenConfig.hint !== undefined && {
              hint: listenConfig.hint,
            })}
            expectedAnswer={submissionResult?.correct_answer}
            feedback={submissionResult?.feedback}
            explanation={submissionResult?.explanation}
            isSubmitted={isSubmitted}
            isCorrect={submissionResult?.correct}
            isLoading={isSubmitting || isAttemptPending}
            onSubmit={(answerText) =>
              void handleSubmit({ text_answer: answerText })
            }
            onContinue={handleContinue}
          />
        )}

        {canRenderMatch && (
          <ExerciseMatch
            prompt={matchConfig.prompt ?? ""}
            words={matchConfig.words ?? []}
            definitions={matchConfig.definitions ?? []}
            feedback={submissionResult?.feedback}
            explanation={submissionResult?.explanation}
            {...(typeof submissionResult?.score === "number" && {
              score: submissionResult.score,
            })}
            isSubmitted={isSubmitted}
            isCorrect={submissionResult?.correct}
            isLoading={isSubmitting || isAttemptPending}
            // No per-pair marking: the grade response reports a score and a
            // verdict, not which pairs were right. Revealing that would mean
            // adding a structured answer to GradeResult across every skill
            // module, and the score already tells the learner how much of the
            // set they knew.
            onSubmit={(pairs) => void handleSubmit({ pairs })}
            onContinue={handleContinue}
          />
        )}

        {canRenderReorder && (
          <ExerciseReorder
            prompt={reorderConfig.prompt ?? ""}
            tokens={reorderConfig.tokens ?? []}
            {...(reorderConfig.target_word !== undefined && {
              targetWord: reorderConfig.target_word,
            })}
            expectedAnswer={submissionResult?.correct_answer}
            feedback={submissionResult?.feedback}
            explanation={submissionResult?.explanation}
            isSubmitted={isSubmitted}
            isCorrect={submissionResult?.correct}
            isLoading={isSubmitting || isAttemptPending}
            onSubmit={(sentence) =>
              void handleSubmit({ text_answer: sentence })
            }
            onContinue={handleContinue}
          />
        )}

        {canRenderContextChoice && (
          <ExerciseContextChoice
            prompt={contextConfig.prompt ?? ""}
            sentence={contextConfig.sentence ?? ""}
            {...(contextConfig.target_word !== undefined && {
              targetWord: contextConfig.target_word,
            })}
            options={contextConfig.options ?? []}
            correctOptionId={
              submissionResult?.correct
                ? selectedOptId
                : (submissionResult?.correct_answer ?? undefined)
            }
            feedback={submissionResult?.feedback}
            explanation={submissionResult?.explanation}
            isSubmitted={isSubmitted}
            isCorrect={submissionResult?.correct}
            isLoading={isSubmitting || isAttemptPending}
            onSubmit={(selectedOptionId) =>
              void handleSubmit({ selected_option_id: selectedOptionId })
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
