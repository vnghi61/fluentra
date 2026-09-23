import React, { useCallback, useEffect, useRef, useState } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { useNavigate, useParams } from "@tanstack/react-router";
import { AlertCircle, Flag, Loader2, RotateCcw } from "lucide-react";
import { useTranslation } from "react-i18next";

import { useAuthStore } from "@/stores/authStore";
import { usePreferencesStore } from "@/stores/preferencesStore";

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
  ExerciseListening,
  ExerciseReading,
  type ItemResult,
  type ReadingQuestionItem,
  ExerciseReorder,
  ExerciseWriting,
  ExerciseSpeaking,
  ExerciseSentenceTransform,
  ExerciseMaterial,
  ExerciseTopic,
  ActivityUnavailable,
  ExitDialog,
  ReportDialog,
  learningApi,
  learningKeys,
  useDailyPracticeSet,
  RunnerHeader,
} from "@/features/learning";
import { type FoundationTopicBodyShape } from "@/features/learning/components/Foundation/FoundationTopicBody";
import { useLesson } from "@/features/lesson";
import { useResourcePractice } from "@/features/resource";
import {
  clearWritingDraft,
  type WritingFeedback,
  writingApi,
} from "@/features/writing";
import { type SpeakingFeedback, speakingApi } from "@/features/speaking";
import { readExampleSentences } from "@/lib/examples";
import type { components } from "@/types/api";

/**
 * The activity shape the runner renders. A daily-set item can carry `weak_node`
 * (why it is in today's list); a lesson's items cannot, so the union is the
 * daily type with that field optional.
 */
type DailyPracticeActivity = components["schemas"]["DailyPracticeActivity"];

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
}

interface SentenceTransformConfig {
  prompt?: string;
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
  /** The page crediting the recording; without it the recording does not play. */
  audio_attribution?: string;
  audio_licence?: string;
}

// The listening body the browser receives. `script` is absent by design: the
// server redacts it, because an item whose script you can read is a reading
// item. It comes back from the transcript route after the attempt is graded.
interface ListeningConfig {
  title?: string;
  questions?: ReadingQuestionItem[];
}

interface ReadingConfig {
  passage_title?: string;
  passage?: string;
  prompt?: string;
  options?: { id: string; text: string }[];
  questions?: ReadingQuestionItem[];
}

interface WritingConfig {
  prompt?: string;
  rubric?: string;
  min_words?: number;
  sample_answer?: string;
}

interface SpeakingConfig {
  task_type?: "read_aloud" | "respond";
  prompt?: string;
  reference_text?: string;
  speaking_time_seconds?: number;
}

// A non-graded material. `sources` is issued at read time, after the paywall,
// and expires; the keys it was built from are never URLs.
interface MaterialConfig {
  material_kind?: "document" | "video";
  title?: string;
  description?: string;
  sources?: {
    poster_url?: string;
    video?: { url: string; height: number }[];
    document?: { url: string; preview_url?: string; page_count?: number };
  };
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
  status?: string | null | undefined;
  correct?: boolean | null | undefined;
  feedback?: string | null | undefined;
  correct_answer?: string | null | undefined;
  // Matching is the one kind that can be partly right, and "incorrect" is a
  // poor description of three pairs out of four.
  score?: number | null | undefined;
  item_results?: ItemResult[] | null | undefined;
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
  const user = useAuthStore((state) => state.user);
  const userId = user?.userId;

  const isDaily =
    lessonId === "daily" ||
    (typeof window !== "undefined" &&
      window.location.pathname.includes("/practice/daily"));

  // Private practice generated from one of the learner's own uploads. It is a
  // list of activities like the daily set, so it runs through the same runner;
  // only where it comes from and where it exits to differ.
  const isResourcePractice =
    typeof window !== "undefined" &&
    window.location.pathname.includes("/practice/resource/");
  const resourceId = params["resourceId"] ?? "";

  const [practiceLevel] = useState<string>(() => {
    if (typeof window !== "undefined") {
      const urlParams = new URLSearchParams(window.location.search);
      return (
        urlParams.get("level") ||
        usePreferencesStore.getState().preferences?.practice_level ||
        "B1"
      );
    }
    return "B1";
  });

  const {
    data: dailySet,
    isLoading: dailyLoading,
    isError: isDailyError,
    error: dailyError,
    refetch: refetchDaily,
  } = useDailyPracticeSet(practiceLevel, isDaily);

  const {
    data: resourcePractice,
    isLoading: resourcePracticeLoading,
    isError: isResourcePracticeError,
    error: resourcePracticeError,
    refetch: refetchResourcePractice,
  } = useResourcePractice(resourceId, isResourcePractice);

  const {
    data: lesson,
    isLoading: lessonLoading,
    isError: isLessonError,
    error: lessonError,
    refetch: refetchLesson,
  } = useLesson(isDaily || isResourcePractice ? undefined : lessonId);

  const activities: DailyPracticeActivity[] = isDaily
    ? (dailySet?.activities ?? [])
    : isResourcePractice
      ? (resourcePractice?.activities ?? [])
      : (lesson?.activities ?? []);
  const isLoading = isDaily
    ? dailyLoading
    : isResourcePractice
      ? resourcePracticeLoading
      : lessonLoading;
  const isError = isDaily
    ? isDailyError
    : isResourcePractice
      ? isResourcePracticeError
      : isLessonError;
  const error = isDaily
    ? dailyError
    : isResourcePractice
      ? resourcePracticeError
      : lessonError;
  const refetch = isDaily
    ? refetchDaily
    : isResourcePractice
      ? refetchResourcePractice
      : refetchLesson;

  const [currentIndex, setCurrentIndex] = useState(0);
  const [currentAttemptId, setCurrentAttemptId] = useState<string | null>(null);
  const [attemptStartFailed, setAttemptStartFailed] = useState(false);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [isSubmitted, setIsSubmitted] = useState(false);
  const [isMarking, setIsMarking] = useState(false);
  const [markingTimedOut, setMarkingTimedOut] = useState(false);
  const [pollingAttemptId, setPollingAttemptId] = useState<string | null>(null);
  const [writingFeedback, setWritingFeedback] =
    useState<WritingFeedback | null>(null);
  const [speakingFeedback, setSpeakingFeedback] =
    useState<SpeakingFeedback | null>(null);
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
  const [isReportOpen, setIsReportOpen] = useState(false);
  const [reportInitialNote, setReportInitialNote] = useState("");
  const [startTime, setStartTime] = useState(() => Date.now());
  const [elapsedSeconds, setElapsedSeconds] = useState(0);

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
  const invalidateProgress = useCallback(() => {
    if (!signedIn) return;
    // Fire-and-forget: a refetch that fails must not fail the answer, which is
    // already committed on the server.
    void queryClient.invalidateQueries({ queryKey: learningKeys.all });
    void queryClient.invalidateQueries({ queryKey: lessonKeys.all });
    // Grading schedules review cards, so the due count on the dashboard and the
    // review queue itself are both stale the moment an answer lands.
    void queryClient.invalidateQueries({ queryKey: reviewKeys.all });
  }, [signedIn, queryClient]);

  // Adaptive polling on 202 async grading:
  // every 2s for 30s, then every 5s, stopping on unmount or after 3 minutes (180s)
  useEffect(() => {
    if (!pollingAttemptId) return;

    let isMounted = true;
    let timerId: ReturnType<typeof setTimeout> | null = null;
    const startTime = Date.now();

    const poll = async () => {
      const elapsedSec = (Date.now() - startTime) / 1000;
      if (elapsedSec >= 180) {
        if (isMounted) {
          setIsMarking(false);
          setMarkingTimedOut(true);
          setIsSubmitted(true);
          setPollingAttemptId(null);
        }
        return;
      }

      try {
        const attempt = await learningApi.getAttempt(pollingAttemptId);
        if (!isMounted) return;

        if (attempt.status === "graded") {
          setIsMarking(false);
          setPollingAttemptId(null);
          setIsSubmitted(true);

          if (userId && currentActivity) {
            clearWritingDraft(userId, currentActivity.id);
          }
          invalidateProgress();

          if (currentActivity?.kind === "speaking_task") {
            try {
              const fb = await speakingApi.getFeedback(pollingAttemptId);
              if (isMounted) {
                setSpeakingFeedback(fb);
                const score =
                  fb.read_aloud_accuracy !== null &&
                  fb.read_aloud_accuracy !== undefined
                    ? Math.round(fb.read_aloud_accuracy)
                    : Math.round(
                        ((fb.criteria?.reduce((acc, c) => acc + c.band, 0) ??
                          0) /
                          (fb.criteria?.length || 1)) *
                          10,
                      );
                const isPassed = (attempt.score ?? score) >= 60;
                setSubmissionResult({
                  status: "graded",
                  correct: isPassed,
                  score: attempt.score ?? score,
                  feedback: fb.feedback_vi || fb.feedback_en,
                });
                if (isPassed) {
                  setScoreCount((prev) => prev + 1);
                }
              }
            } catch {
              if (isMounted) {
                setSubmissionResult({
                  status: "graded",
                  correct: (attempt.score ?? 0) >= 60,
                  score: attempt.score ?? undefined,
                  feedback: attempt.feedback ?? undefined,
                });
              }
            }
            return;
          }

          try {
            const fb = await writingApi.getFeedback(pollingAttemptId);
            if (isMounted) {
              setWritingFeedback(fb);
              const isPassed = fb.overall_band >= 6.0;
              setSubmissionResult({
                status: "graded",
                correct: isPassed,
                score: fb.score,
                feedback: fb.feedback_vi || fb.feedback_en,
              });
              if (isPassed) {
                setScoreCount((prev) => prev + 1);
              }
            }
          } catch {
            if (isMounted) {
              setSubmissionResult({
                status: "graded",
                correct: (attempt.score ?? 0) >= 60,
                score: attempt.score ?? undefined,
                feedback: attempt.feedback ?? undefined,
              });
            }
          }
          return;
        }

        if (attempt.status === "failed") {
          if (isMounted) {
            setIsMarking(false);
            setPollingAttemptId(null);
            setSubmissionError(
              t(
                "runner.markingFailedDesc",
                "We could not grade your essay. Your answer is preserved. Please retry.",
              ),
            );
          }
          return;
        }

        const nextDelay = elapsedSec < 30 ? 2000 : 5000;
        timerId = setTimeout(() => {
          void poll();
        }, nextDelay);
      } catch {
        const nextDelay = elapsedSec < 30 ? 2000 : 5000;
        timerId = setTimeout(() => {
          void poll();
        }, nextDelay);
      }
    };

    timerId = setTimeout(() => {
      void poll();
    }, 2000);

    return () => {
      isMounted = false;
      if (timerId) clearTimeout(timerId);
    };
  }, [pollingAttemptId, userId, currentActivity, t, invalidateProgress]);

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

      if (result.status === "grading" && signedIn && currentAttemptId) {
        setIsMarking(true);
        setMarkingTimedOut(false);
        setPollingAttemptId(currentAttemptId);
        return;
      }

      if (
        currentActivity.kind === "writing_prompt" &&
        signedIn &&
        currentAttemptId
      ) {
        try {
          const fb = await writingApi.getFeedback(currentAttemptId);
          setWritingFeedback(fb);
          const isPassed = fb.overall_band >= 6.0;
          result.correct = isPassed;
          result.score = fb.score;
          result.feedback = fb.feedback_vi || fb.feedback_en;
        } catch {
          // If feedback cannot be loaded directly, keep result
        }
      }

      setIsSubmitted(true);
      setSubmissionResult(result);
      if (result.correct) {
        setScoreCount((prev) => prev + 1);
      }
      if (userId && currentActivity) {
        clearWritingDraft(userId, currentActivity.id);
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
      setIsMarking(false);
      setMarkingTimedOut(false);
      setPollingAttemptId(null);
      setWritingFeedback(null);
      setSpeakingFeedback(null);
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
    if (isResourcePractice && resourceId) {
      void navigate({
        to: "/my-resources/$resourceId",
        params: { resourceId },
      });
      return;
    }
    void navigate({ to: isDaily ? "/practice" : "/learn" });
  };

  if (isLoading) {
    return (
      <div className="flex items-center justify-center min-h-[50vh]">
        <div className="animate-spin rounded-full h-8 w-8 border-4 border-border-subtle border-t-primary" />
      </div>
    );
  }

  if (
    isError ||
    (isDaily ? !dailySet : isResourcePractice ? !resourcePractice : !lesson)
  ) {
    return (
      <div className="py-12 max-w-lg mx-auto">
        <Card className="border-danger/30 text-center p-6">
          <CardHeader>
            <div className="flex justify-center mb-2">
              <AlertCircle className="h-10 w-10 text-danger-accent" />
            </div>
            <CardTitle>
              {isDaily
                ? t("practice.daily.errorTitle", "Unable to Load Practice Set")
                : isResourcePractice
                  ? t("resources.practiceErrorTitle", "Unable to Load Practice")
                  : t("learn.errorTitle", "Unable to Load Lesson")}
            </CardTitle>
            <CardDescription>
              {error?.message ||
                (isDaily
                  ? t(
                      "practice.daily.errorDesc",
                      "Could not load today's practice activities.",
                    )
                  : isResourcePractice
                    ? t(
                        "resources.practiceErrorDesc",
                        "Practice from this file could not be loaded.",
                      )
                    : t(
                        "learn.errorDesc",
                        "Could not load lesson activities.",
                      ))}
            </CardDescription>
          </CardHeader>
          <CardFooter className="justify-center gap-3">
            <Button variant="outline" onClick={() => void refetch()}>
              {t("action.retry", "Try again")}
            </Button>
            <Button
              onClick={() => {
                if (isResourcePractice && resourceId) {
                  void navigate({
                    to: "/my-resources/$resourceId",
                    params: { resourceId },
                  });
                  return;
                }
                void navigate({ to: isDaily ? "/practice" : "/learn" });
              }}
            >
              {isDaily
                ? t("practice.daily.backBtn", "Back to Practice")
                : isResourcePractice
                  ? t("resources.backToList", "All resources")
                  : t("runner.backToCourseBtn", "Back to Syllabus")}
            </Button>
          </CardFooter>
        </Card>
      </div>
    );
  }

  // A set arrives `generating` with no activities: the request only queues it
  // and a sweep turns it into ten items. Without this the runner would render
  // its header over an empty canvas, which reads as a broken screen rather
  // than as work in progress — and a set that failed would stay blank for good
  // while `failure_reason` went unread.
  if (isResourcePractice && resourcePractice?.status !== "ready") {
    const buildFailed = resourcePractice?.status === "failed";
    return (
      <div className="py-12 max-w-lg mx-auto">
        <Card
          className={`text-center p-6 ${buildFailed ? "border-danger/30" : ""}`}
        >
          <CardHeader>
            <div className="flex justify-center mb-2">
              {buildFailed ? (
                <AlertCircle className="h-10 w-10 text-danger-accent" />
              ) : (
                <Loader2
                  className="h-10 w-10 animate-spin text-primary"
                  aria-hidden="true"
                />
              )}
            </div>
            <CardTitle>
              {buildFailed
                ? t(
                    "resources.practiceFailedTitle",
                    "Practice could not be built",
                  )
                : t(
                    "resources.practiceBuildingTitle",
                    "Building your practice",
                  )}
            </CardTitle>
            <CardDescription role="status">
              {buildFailed
                ? resourcePractice?.failure_reason ||
                  t(
                    "resources.practiceFailedDesc",
                    "We could not build practice from this file. Try again later.",
                  )
                : t(
                    "resources.practiceBuildingDesc",
                    "We are writing ten exercises from this file. This takes a minute; the page opens on its own when they are ready.",
                  )}
            </CardDescription>
          </CardHeader>
          <CardFooter className="justify-center">
            <Button
              variant={buildFailed ? "primary" : "outline"}
              onClick={() =>
                void navigate({
                  to: "/my-resources/$resourceId",
                  params: { resourceId },
                })
              }
            >
              {t("resources.backToFile", "Back to the file")}
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
          {...(lesson?.next_lesson_id
            ? { nextLessonId: lesson.next_lesson_id }
            : {})}
          onRetryLesson={() => {
            setCurrentIndex(0);
            setScoreCount(0);
            setIsSubmitted(false);
            setSubmissionResult(null);
            setSubmissionError(null);
            setLastSubmittedPayload(null);
            setIsMarking(false);
            setMarkingTimedOut(false);
            setPollingAttemptId(null);
            setWritingFeedback(null);
            setSpeakingFeedback(null);
            // Same reason as handleContinue: the attempt this learner finished
            // the lesson on is not the one activity 1 is about to open.
            setCurrentAttemptId(null);
            setIsCompleted(false);
            setStartTime(Date.now());
            setElapsedSeconds(0);
          }}
        />
      </>
    );
  }

  const rawConfig = (currentActivity?.config ?? {}) as unknown;
  const kind = currentActivity?.kind;

  const mcConfig = rawConfig as MultipleChoiceConfig;
  const gapConfig = rawConfig as GapFillConfig;
  const transformConfig = rawConfig as SentenceTransformConfig;
  const fcConfig = rawConfig as FlashcardConfig;
  const listenConfig = rawConfig as ListenTypeConfig;
  const matchConfig = rawConfig as MatchConfig;
  const reorderConfig = rawConfig as ReorderConfig;
  const contextConfig = rawConfig as ContextChoiceConfig;
  const readingConfig = rawConfig as ReadingConfig;
  const listeningConfig = rawConfig as ListeningConfig;
  const writingConfig = rawConfig as WritingConfig;
  const speakingConfig = rawConfig as SpeakingConfig;
  const materialConfig = rawConfig as MaterialConfig;
  const topicConfig = rawConfig as FoundationTopicBodyShape & {
    title?: string;
  };
  const typedConfig = rawConfig as {
    prompt?: string;
    sentence?: string;
    max_words?: number;
  };

  // An exercise is renderable only when its config carries the fields it needs.
  // Everything else is ActivityUnavailable — there is no default question,
  // because a default question is somebody else's question.
  const canRenderMultipleChoice =
    (kind === "vocab_multiple_choice" ||
      kind === "grammar_tense_choice" ||
      kind === "mcq_gap" ||
      kind === "foundation_quiz" ||
      kind === "foundation_review") &&
    (typeof mcConfig.prompt === "string" ||
      typeof (rawConfig as Record<string, unknown>).sentence === "string") &&
    Array.isArray(mcConfig.options) &&
    mcConfig.options.length > 0;

  // A curriculum sentence transform is authored as a gap fill: the instruction in
  // `prompt`, the sentence around the blank in `sentence_before`/`sentence_after`.
  // It keeps the gap-fill renderer, which shows that sentence.
  //
  // Decided from the sentence, not from `expected_answer`: that is the answer,
  // and the server now redacts it. The runner used to require it, which is why
  // every visitor received the word for the blank before typing anything.
  const canRenderGapFill =
    (kind === "vocab_gap_fill" || kind === "grammar_sentence_transform") &&
    (Boolean(gapConfig.sentence_before?.trim()) ||
      Boolean(gapConfig.sentence_after?.trim()));

  // A generated one carries the whole task in `prompt` and nothing to fill in,
  // so it is rewritten whole.
  const canRenderSentenceTransform =
    kind === "grammar_sentence_transform" &&
    !canRenderGapFill &&
    typeof transformConfig.prompt === "string" &&
    transformConfig.prompt !== "";

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

  const canRenderReading =
    (kind === "reading_comprehension" || kind === "text_completion") &&
    typeof readingConfig.passage === "string" &&
    readingConfig.passage !== "" &&
    ((typeof readingConfig.prompt === "string" &&
      Array.isArray(readingConfig.options) &&
      readingConfig.options.length > 0) ||
      (Array.isArray(readingConfig.questions) &&
        readingConfig.questions.length > 0));

  // The clip itself is not in the config — it is fetched, play by play, against
  // the attempt — so what makes a listening item renderable is its questions
  // plus a content version to ask the play route about.
  const canRenderListening =
    kind === "listening_comprehension" &&
    Boolean(currentActivity?.content_version_id) &&
    Array.isArray(listeningConfig.questions) &&
    listeningConfig.questions.length > 0;

  const canRenderWriting =
    kind === "writing_prompt" &&
    typeof writingConfig.prompt === "string" &&
    writingConfig.prompt !== "";

  const canRenderSpeaking =
    kind === "speaking_task" &&
    (typeof speakingConfig.prompt === "string" ||
      typeof speakingConfig.reference_text === "string");

  // A material is renderable when the server issued it sources: a video with at
  // least one rendition, or a document with a URL to open.
  const canRenderMaterial =
    kind === "lesson_material" &&
    ((Array.isArray(materialConfig.sources?.video) &&
      materialConfig.sources.video.length > 0) ||
      Boolean(materialConfig.sources?.document?.url));

  // A topic teaches: objective and explanation are what make it renderable.
  const canRenderTopic =
    kind === "foundation_topic" &&
    (typeof topicConfig.objective === "string" ||
      typeof topicConfig.explanation === "object" ||
      Array.isArray(topicConfig.examples));

  // A typed completion is a gap fill with a typed answer and a word limit
  // (WO 22 D22-25); it renders with the same component.
  const typedBlankParts =
    typeof typedConfig.sentence === "string"
      ? typedConfig.sentence.split("___")
      : [];
  const canRenderTypedCompletion =
    kind === "typed_completion" &&
    (typeof typedConfig.prompt === "string" || typedBlankParts.length === 2);

  const selectedOptId =
    typeof lastSubmittedPayload?.selected_option_id === "string"
      ? lastSubmittedPayload.selected_option_id
      : undefined;

  return (
    <div className="min-h-full bg-surface flex flex-col justify-between">
      {/* Runner Header */}
      <RunnerHeader
        lessonTitle={
          isDaily
            ? t(
                "practice.daily.runnerTitle",
                "Daily Practice Set ({{level}})",
                {
                  level: dailySet?.level || practiceLevel,
                },
              )
            : isResourcePractice
              ? t("resources.practiceRunnerTitle", "Practice from your file")
              : (lesson?.title ?? "")
        }
        currentStep={currentIndex + 1}
        totalSteps={activities.length}
        onExit={() => setIsExitDialogOpen(true)}
      />

      {/* Main Exercise Canvas */}
      <main className="flex-1 max-w-4xl w-full mx-auto p-4 flex flex-col justify-center">
        {/*
          The daily set draws one item to revisit the learner's weakest spine
          node. It says so, because an exercise that looks like every other
          one reads as a mistake rather than as a plan (WO 21 Stage F).
        */}
        {currentActivity?.weak_node && (
          <div
            role="status"
            className="mx-auto mb-4 flex w-full max-w-2xl items-center gap-2 rounded-xl border border-primary/30 bg-primary/5 px-3 py-2 text-sm text-primary-accent"
          >
            <RotateCcw className="h-4 w-4 shrink-0" aria-hidden="true" />
            <span>
              {t("practice.daily.weakNode", {
                label: currentActivity.weak_node.label,
                defaultValue: `Review: ${currentActivity.weak_node.label}`,
              })}
            </span>
          </div>
        )}
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
          !canRenderSentenceTransform &&
          !canRenderFlashcard &&
          !canRenderListenType &&
          !canRenderMatch &&
          !canRenderReorder &&
          !canRenderContextChoice &&
          !canRenderReading &&
          !canRenderListening &&
          !canRenderWriting &&
          !canRenderSpeaking &&
          !canRenderMaterial &&
          !canRenderTopic &&
          !canRenderTypedCompletion && (
            <ActivityUnavailable
              {...(kind !== undefined && { kind })}
              onSkip={handleContinue}
            />
          )}

        {canRenderMultipleChoice && (
          <ExerciseMultipleChoice
            prompt={
              mcConfig.prompt ||
              ((rawConfig as Record<string, unknown>).sentence as string) ||
              ""
            }
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

        {canRenderTypedCompletion && (
          <ExerciseGapFill
            prompt={typedConfig.prompt ?? ""}
            sentenceBeforeBlank={typedBlankParts[0]?.trim() ?? ""}
            sentenceAfterBlank={typedBlankParts[1]?.trim() ?? ""}
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

        {canRenderSentenceTransform && (
          <ExerciseSentenceTransform
            prompt={transformConfig.prompt ?? ""}
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
            {...(fcConfig.audio_url !== undefined &&
              fcConfig.audio_attribution !== undefined && {
                audioUrl: fcConfig.audio_url,
                audioAttribution: fcConfig.audio_attribution,
                ...(fcConfig.audio_licence !== undefined && {
                  audioLicence: fcConfig.audio_licence,
                }),
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
            onReportSentence={(sentenceText) => {
              setReportInitialNote(`Example sentence: ${sentenceText}`);
              setIsReportOpen(true);
            }}
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

        {canRenderReading && (
          <ExerciseReading
            passageTitle={readingConfig.passage_title}
            passage={readingConfig.passage ?? ""}
            prompt={readingConfig.prompt}
            options={readingConfig.options}
            questions={readingConfig.questions}
            itemResults={submissionResult?.item_results}
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
            onSubmit={(payload) => {
              if (typeof payload === "string") {
                void handleSubmit({ selected_option_id: payload });
              } else {
                void handleSubmit({
                  ...(payload.selectedOptionId
                    ? { selected_option_id: payload.selectedOptionId }
                    : {}),
                  ...(payload.answers ? { answers: payload.answers } : {}),
                  ...(payload.reading_ms !== undefined
                    ? { reading_ms: payload.reading_ms }
                    : {}),
                });
              }
            }}
            onContinue={handleContinue}
          />
        )}

        {canRenderListening && (
          <ExerciseListening
            // Remount on the next activity, for the same reason as writing and
            // speaking below: the player holds a play it has already spent and
            // the answers hold the previous question's choices, neither of
            // which handleContinue can reach.
            key={currentActivity?.id}
            versionId={currentActivity?.content_version_id}
            attemptId={currentAttemptId ?? pollingAttemptId}
            title={listeningConfig.title}
            questions={listeningConfig.questions}
            itemResults={submissionResult?.item_results}
            feedback={submissionResult?.feedback}
            explanation={submissionResult?.explanation}
            isSubmitted={isSubmitted}
            isCorrect={submissionResult?.correct}
            isLoading={isSubmitting || isAttemptPending}
            onSubmit={(answers) => void handleSubmit({ answers })}
            onContinue={handleContinue}
          />
        )}

        {canRenderWriting && (
          <ExerciseWriting
            // Same reason as ExerciseSpeaking below: the typed answer is state
            // in the child, and the draft it restores is keyed by activity.
            key={currentActivity?.id}
            prompt={writingConfig.prompt ?? ""}
            rubric={writingConfig.rubric}
            minWords={writingConfig.min_words}
            sampleAnswer={writingConfig.sample_answer}
            feedback={submissionResult?.feedback}
            score={
              typeof submissionResult?.score === "number"
                ? submissionResult.score
                : undefined
            }
            explanation={submissionResult?.explanation}
            isSubmitted={isSubmitted}
            isCorrect={submissionResult?.correct}
            isLoading={isSubmitting || isAttemptPending}
            isGuest={!signedIn}
            isMarking={isMarking}
            markingTimedOut={markingTimedOut}
            writingFeedback={writingFeedback}
            attemptId={currentAttemptId ?? pollingAttemptId}
            userId={userId}
            activityId={currentActivity?.id}
            onNavigateToMyWriting={() => void navigate({ to: "/my-writing" })}
            onSubmit={(answerText) =>
              void handleSubmit({ text_answer: answerText })
            }
            onContinue={handleContinue}
          />
        )}

        {canRenderSpeaking && (
          <ExerciseSpeaking
            // Remount on the next activity. These two exercises hold state the
            // parent cannot reach — a recording that has been made, a draft
            // being typed — and handleContinue resets only its own. Without the
            // key, the second speaking task in a lesson opened already showing
            // "recording ready" for the previous question's audio, with no
            // action left to take.
            key={currentActivity?.id}
            prompt={speakingConfig.prompt}
            referenceText={speakingConfig.reference_text}
            taskType={speakingConfig.task_type}
            speakingTimeSeconds={speakingConfig.speaking_time_seconds}
            feedback={submissionResult?.feedback}
            score={
              typeof submissionResult?.score === "number"
                ? submissionResult.score
                : undefined
            }
            explanation={submissionResult?.explanation}
            isSubmitted={isSubmitted}
            isCorrect={submissionResult?.correct}
            isLoading={isSubmitting || isAttemptPending}
            isGuest={!signedIn}
            isMarking={isMarking}
            markingTimedOut={markingTimedOut}
            speakingFeedback={speakingFeedback}
            attemptId={currentAttemptId ?? pollingAttemptId}
            userId={userId}
            activityId={currentActivity?.id}
            onNavigateToMySpeaking={() => void navigate({ to: "/my-speaking" })}
            onSubmit={(audioObjectKey) =>
              void handleSubmit({ audio_object_key: audioObjectKey })
            }
            onContinue={handleContinue}
          />
        )}

        {canRenderMaterial && (
          <ExerciseMaterial
            key={currentActivity?.id}
            materialKind={materialConfig.material_kind}
            title={materialConfig.title}
            description={materialConfig.description}
            sources={materialConfig.sources}
            isSubmitted={isSubmitted}
            isCorrect={submissionResult?.correct}
            isLoading={isSubmitting || isAttemptPending}
            onRefetchLesson={() => void refetch()}
            onSubmit={(done) => void handleSubmit({ done })}
            onContinue={handleContinue}
          />
        )}

        {canRenderTopic && (
          <ExerciseTopic
            key={currentActivity?.id}
            title={topicConfig.title}
            body={topicConfig}
            isSubmitted={isSubmitted}
            isLoading={isSubmitting || isAttemptPending}
            onSubmit={() => void handleSubmit({ done: true })}
            onContinue={handleContinue}
          />
        )}

        {isSubmitted && signedIn && currentActivity?.content_version_id && (
          <div className="mt-4 max-w-2xl mx-auto w-full flex justify-start animate-in fade-in duration-200">
            <Button
              type="button"
              variant="ghost"
              onClick={() => {
                setReportInitialNote("");
                setIsReportOpen(true);
              }}
              className="text-xs text-text-muted hover:text-danger gap-1.5 min-h-[44px]"
              title={t("report.reportBtn", "Report issue")}
            >
              <Flag className="h-3.5 w-3.5" aria-hidden="true" />
              <span>{t("report.reportBtn", "Report issue")}</span>
            </Button>
          </div>
        )}
      </main>

      {/* Exit Confirmation Dialog */}
      <ExitDialog
        isOpen={isExitDialogOpen}
        onCancel={() => setIsExitDialogOpen(false)}
        onConfirm={handleConfirmExit}
      />

      {/* Report Bad Item / Content Dialog */}
      <ReportDialog
        isOpen={isReportOpen}
        contentVersionId={currentActivity?.content_version_id ?? null}
        initialNote={reportInitialNote}
        onClose={() => setIsReportOpen(false)}
      />
    </div>
  );
}

export default LessonPage;
