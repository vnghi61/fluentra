import React, { useCallback, useEffect, useState } from "react";
import { FileText } from "lucide-react";
import { useTranslation } from "react-i18next";

import { Button } from "@/components/ui/button";
import { LessonListeningPlayer, listeningApi } from "@/features/listening";

import { type AnswerExplanation, ExerciseActions } from "./ExerciseShell";
import { ExerciseFeedback } from "./ExerciseShell";
import {
  type ItemResult,
  QuestionList,
  type ReadingQuestionItem,
} from "./QuestionList";

export interface ExerciseListeningProps {
  /** Content version of the item. The play and transcript routes key off it. */
  versionId: string | undefined;
  /** The attempt plays are counted against, and the transcript is released for. */
  attemptId: string | null;
  title?: string | undefined;
  questions?: ReadingQuestionItem[] | undefined;
  itemResults?: ItemResult[] | null | undefined;
  feedback?: string | null | undefined;
  explanation?: AnswerExplanation | null | undefined;
  isSubmitted: boolean;
  isCorrect?: boolean | null | undefined;
  isLoading?: boolean;
  onSubmit: (answers: Record<string, string>) => void;
  onContinue: () => void;
}

/**
 * `listening_comprehension` — hear a clip, answer questions about it.
 *
 * The kind has been graded on the server since the listening module was
 * written, and the exam sitting has played its clips all along; the runner had
 * no renderer for it, so every listening item in the database sat in a pool no
 * learner opens, and Practice offered no listening at all.
 *
 * The script is never in the body the browser receives — the server redacts it,
 * because an item you can read is not a listening item. It arrives from the
 * transcript route after grading, and only then.
 */
export const ExerciseListening: React.FC<ExerciseListeningProps> = ({
  versionId,
  attemptId,
  title,
  questions,
  itemResults,
  feedback,
  explanation,
  isSubmitted,
  isCorrect,
  isLoading = false,
  onSubmit,
  onContinue,
}) => {
  const { t } = useTranslation();
  const [answers, setAnswers] = useState<Record<string, string>>({});
  const [transcript, setTranscript] = useState<string | null>(null);
  const [transcriptError, setTranscriptError] = useState(false);

  const items = questions ?? [];

  const handleAnswerChange = (questionId: string, value: string) => {
    if (isSubmitted || isLoading) return;
    setAnswers((prev) => ({ ...prev, [questionId]: value }));
  };

  const handleSubmit = useCallback(() => {
    if (isLoading) return;
    onSubmit(answers);
  }, [answers, isLoading, onSubmit]);

  // Continue on Enter once graded, matching every other exercise.
  useEffect(() => {
    if (!isSubmitted) return undefined;
    const onKeyDown = (event: KeyboardEvent) => {
      if (
        event.target instanceof HTMLInputElement ||
        event.target instanceof HTMLTextAreaElement
      ) {
        return;
      }
      if (event.key === "Enter" || event.key === " ") {
        event.preventDefault();
        onContinue();
      }
    };
    window.addEventListener("keydown", onKeyDown);
    return () => window.removeEventListener("keydown", onKeyDown);
  }, [isSubmitted, onContinue]);

  const showTranscript = async () => {
    if (!versionId || !attemptId) return;
    try {
      const res = await listeningApi.getTranscript(versionId, attemptId);
      setTranscript(res.transcript);
    } catch {
      // The attempt is graded asynchronously in some flows, so "not yet" is an
      // ordinary answer here rather than a failure worth a banner.
      setTranscriptError(true);
    }
  };

  return (
    <div className="space-y-6 max-w-3xl mx-auto py-4 animate-in fade-in duration-200">
      {versionId && (
        <LessonListeningPlayer
          versionId={versionId}
          attemptId={attemptId}
          title={title}
        />
      )}

      {items.length > 0 && (
        <QuestionList
          questions={items}
          answers={answers}
          onAnswerChange={handleAnswerChange}
          itemResults={itemResults}
          isSubmitted={isSubmitted}
          isLoading={isLoading}
        />
      )}

      {isSubmitted && (
        <ExerciseFeedback
          isCorrect={isCorrect}
          feedback={feedback}
          explanation={explanation}
        />
      )}

      {/*
        The script, after grading and only if asked for. It is the answer key to
        a listening item, so it is not fetched with the exercise and not shown
        by default — but once the attempt is marked, reading what was said is
        the fastest way to see what was misheard.
      */}
      {isSubmitted && versionId && attemptId && (
        <div className="space-y-2">
          {transcript === null ? (
            <Button
              type="button"
              variant="ghost"
              onClick={() => void showTranscript()}
              className="gap-2 min-h-[44px] text-sm text-text-muted"
            >
              <FileText className="h-4 w-4" aria-hidden="true" />
              {t("runner.listening.showTranscript", "Show the transcript")}
            </Button>
          ) : (
            <div className="rounded-xl border border-border bg-surface/60 p-4">
              <p className="mb-2 text-[11px] font-semibold uppercase tracking-wider text-text-muted">
                {t("runner.listening.transcriptLabel", "Transcript")}
              </p>
              <p className="whitespace-pre-line text-sm leading-relaxed text-text">
                {transcript}
              </p>
            </div>
          )}
          {transcriptError && (
            <p className="text-xs text-text-muted">
              {t(
                "runner.listening.transcriptNotReady",
                "The transcript is available once this attempt has been marked.",
              )}
            </p>
          )}
        </div>
      )}

      <ExerciseActions
        isSubmitted={isSubmitted}
        canSubmit={Object.keys(answers).length > 0}
        isLoading={isLoading}
        onSubmit={handleSubmit}
        onContinue={onContinue}
      />
    </div>
  );
};
