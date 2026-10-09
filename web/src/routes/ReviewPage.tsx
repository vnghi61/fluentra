import React, { useCallback, useEffect, useRef, useState } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { Link, useSearch } from "@tanstack/react-router";
import { AlertCircle, CheckCircle2 } from "lucide-react";
import { useTranslation } from "react-i18next";

import { PageHeader } from "@/components/layout/PageHeader";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { ErrorState } from "@/components/ui/error-state";
import { FlipCard } from "@/components/ui/flip-card";
import { Progress } from "@/components/ui/progress";
import { Skeleton } from "@/components/ui/skeleton";
import { learningKeys, ReportDialog } from "@/features/learning";
import {
  CardContentUnavailable,
  EmptyQueue,
  FlashcardBack,
  FlashcardFront,
  flashcardContent,
  GradeButtonGroup,
  isActivationTarget,
  isForeignKeyEvent,
  reviewApi,
  type ReviewGrade,
  reviewKeys,
  ReviewSummary,
  untilLabel,
  useReviewSession,
} from "@/features/review";
import { cn } from "@/lib/utils";

type GradeCounts = Record<ReviewGrade, number>;

const noGrades: GradeCounts = { again: 0, hard: 0, good: 0, easy: 0 };

const gradeLabelKey: Record<ReviewGrade, string> = {
  again: "review.again",
  hard: "review.hard",
  good: "review.good",
  easy: "review.easy",
};

const gradeTone: Record<ReviewGrade, string> = {
  again: "text-danger-accent",
  hard: "text-warning-accent",
  good: "text-primary-accent",
  easy: "text-success-accent",
};

/** What the server said about the last grade, shown while the next card loads in. */
interface LastResult {
  grade: ReviewGrade;
  nextDueAt: string;
  answeredAt: number;
}

export function ReviewPage(): React.JSX.Element {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const searchParams: Record<string, unknown> = useSearch({ strict: false });
  const deckId =
    typeof searchParams.deck_id === "string" && searchParams.deck_id !== ""
      ? searchParams.deck_id
      : undefined;
  const {
    data: sessionData,
    isLoading,
    isError,
    refetch,
  } = useReviewSession(deckId);

  // `cards` is required in ReviewSessionResponse; it is absent here only while the
  // query resolves, which the loading branch below handles. There is deliberately
  // no defensive default on a field the backend guarantees.
  const cards = sessionData?.cards ?? [];

  const [currentIndex, setCurrentIndex] = useState(0);
  const [isFlipped, setIsFlipped] = useState(false);
  const [pendingGrade, setPendingGrade] = useState<ReviewGrade | null>(null);
  const [isCompleted, setIsCompleted] = useState(false);
  const [gradeFailed, setGradeFailed] = useState(false);
  const [gradeCounts, setGradeCounts] = useState<GradeCounts>(noGrades);
  const [lastResult, setLastResult] = useState<LastResult | null>(null);
  const [isReportOpen, setIsReportOpen] = useState(false);
  const [reportNote, setReportNote] = useState("");

  const revealRef = useRef<HTMLButtonElement>(null);
  const answerRef = useRef<HTMLHeadingElement>(null);
  // Set when a grade moves the session on, so focus follows to the new card
  // without the page grabbing focus on its first render.
  const focusNextCard = useRef(false);

  const isSubmitting = pendingGrade !== null;
  const currentCard = cards[currentIndex];
  const content = currentCard ? flashcardContent(currentCard) : null;
  const canReveal = !isFlipped && content !== null;

  const reveal = useCallback(() => setIsFlipped(true), []);

  // Space and Enter reveal the answer. The 1–4 grade shortcuts live in
  // GradeButtonGroup, which mounts only once the card is flipped, so a digit
  // cannot grade a card whose answer the learner has not seen. A focused
  // control keeps its own Space/Enter: on the pronounce button, Space speaks.
  useEffect(() => {
    if (!canReveal) return;
    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key !== " " && event.key !== "Enter") return;
      if (isForeignKeyEvent(event) || isActivationTarget(event)) return;
      event.preventDefault();
      reveal();
    };

    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, [canReveal, reveal]);

  // The reveal button has just turned away with the front face, and focus on
  // an aria-hidden face reads nothing. The answer's heading takes it instead.
  useEffect(() => {
    if (isFlipped) answerRef.current?.focus();
  }, [isFlipped]);

  useEffect(() => {
    if (!focusNextCard.current) return;
    focusNextCard.current = false;
    revealRef.current?.focus();
  }, [currentIndex]);

  const advance = useCallback(
    (grade: ReviewGrade) => {
      setGradeCounts((prev) => ({ ...prev, [grade]: prev[grade] + 1 }));
      setIsFlipped(false);
      if (currentIndex < cards.length - 1) {
        focusNextCard.current = true;
        setCurrentIndex((prev) => prev + 1);
      } else {
        setIsCompleted(true);
      }
    },
    [cards.length, currentIndex],
  );

  // A grade that did not reach the server has not happened. The card stays on
  // screen with a retry rather than being counted and skipped: the point of the
  // queue is that the schedule gets written, and advancing past a failed write
  // loses the review silently.
  const handleGrade = useCallback(
    async (grade: ReviewGrade) => {
      if (!currentCard || isSubmitting) return;

      setPendingGrade(grade);
      setGradeFailed(false);
      try {
        const result = await reviewApi.answerCard(currentCard.id, grade);
        setLastResult({
          grade,
          nextDueAt: result.next_due_at,
          answeredAt: Date.now(),
        });
        advance(grade);
        // Answering reschedules the card, so the due count the dashboard shows
        // and the learner's mastery are both stale the moment this returns.
        // Not awaited: a refetch that fails must not fail a grade the server
        // has already written.
        void queryClient.invalidateQueries({ queryKey: reviewKeys.dueCount() });
        void queryClient.invalidateQueries({ queryKey: learningKeys.all });
      } catch {
        setGradeFailed(true);
      } finally {
        setPendingGrade(null);
      }
    },
    [advance, currentCard, isSubmitting, queryClient],
  );

  const onGrade = useCallback(
    (grade: ReviewGrade) => void handleGrade(grade),
    [handleGrade],
  );

  const header = (
    <PageHeader
      back={{ to: "/practice", label: t("review.backToPractice") }}
      title={t("review.title")}
      description={t("review.pageDesc")}
    />
  );

  let body: React.ReactNode;

  if (isLoading) {
    body = (
      <div
        role="status"
        aria-label={t("review.loading")}
        className="mx-auto w-full max-w-2xl space-y-4"
      >
        <Skeleton className="h-10 w-full rounded-xl" />
        <Skeleton className="h-[340px] w-full rounded-[var(--radius-card)]" />
      </div>
    );
  } else if (isError) {
    body = (
      <Card className="mx-auto w-full max-w-2xl">
        <ErrorState
          title={t("review.errorTitle")}
          description={t("review.errorDesc")}
          onRetry={() => void refetch()}
          retryLabel={t("action.retry")}
          action={
            <Button asChild variant="ghost">
              <Link to="/practice">{t("review.backToPractice")}</Link>
            </Button>
          }
        />
      </Card>
    );
  } else if (isCompleted) {
    body = (
      <ReviewSummary
        totalReviewed={cards.length}
        gradeCounts={gradeCounts}
        onRestart={() => {
          setCurrentIndex(0);
          setIsFlipped(false);
          setIsCompleted(false);
          setGradeFailed(false);
          setGradeCounts(noGrades);
          setLastResult(null);
          void refetch();
        }}
      />
    );
  } else if (!currentCard) {
    body = <EmptyQueue />;
  } else {
    const total = cards.length;
    const reviewed = currentIndex;
    const remaining = total - reviewed;

    body = (
      <div className="mx-auto w-full max-w-2xl space-y-4 md:space-y-5">
        <div
          className="space-y-2"
          data-testid="review-progress"
          data-current={currentIndex + 1}
          data-total={total}
          data-reviewed={reviewed}
        >
          <div className="flex items-baseline justify-between gap-3 text-sm">
            <span className="font-semibold text-text">
              {t("review.progressLabel", {
                current: currentIndex + 1,
                total,
              })}
            </span>
            <span className="text-text-muted">
              {t("review.remaining", { count: remaining })}
            </span>
          </div>
          <Progress
            value={reviewed}
            max={total}
            aria-label={t("review.sessionProgress")}
            getValueLabel={() => t("review.progressValue", { reviewed, total })}
          />
        </div>

        {/*
          Always mounted, so the announcement is heard: a live region that
          appears together with its text is often not read at all.
        */}
        <div role="status" className="min-h-6 text-sm">
          {lastResult && (
            <p
              key={lastResult.answeredAt}
              className="flex items-center gap-1.5 text-text-muted animate-in fade-in duration-200 motion-reduce:animate-none"
            >
              <CheckCircle2
                className={cn("h-4 w-4 shrink-0", gradeTone[lastResult.grade])}
                aria-hidden="true"
              />
              {t("review.lastGraded", {
                grade: t(gradeLabelKey[lastResult.grade]),
                when: untilLabel(
                  t,
                  lastResult.nextDueAt,
                  lastResult.answeredAt,
                ),
              })}
            </p>
          )}
        </div>

        {gradeFailed && (
          <div
            role="alert"
            className="flex items-center gap-2 rounded-xl border border-danger/40 bg-danger/10 p-4 text-sm font-medium text-danger-accent"
          >
            <AlertCircle className="h-5 w-5 shrink-0" aria-hidden="true" />
            <span>{t("review.gradeFailed")}</span>
          </div>
        )}

        {/*
          Keyed on the card, so the next one arrives face up instead of
          spinning back from the previous card's answer.
        */}
        <div
          key={`${currentIndex}-${currentCard.id}`}
          className="animate-in fade-in slide-in-from-bottom-2 duration-200 motion-reduce:animate-none"
        >
          {/*
            A card whose version could not be resolved carries no word to show.
            It is still gradable — the schedule is real — so the grade buttons
            stay, and the face says what is missing instead of inventing a word.
          */}
          {content === null ? (
            <CardContentUnavailable
              contentVersionId={currentCard.content_version_id}
            />
          ) : (
            // One card that turns, rather than two that swap: both faces are
            // laid out and the container rotates, so the box never changes
            // height mid-turn.
            <FlipCard
              flipped={isFlipped}
              // `inert` on the face turned away: hidden from a screen reader is
              // not enough while its buttons are still in the Tab order.
              front={
                <div inert={isFlipped} className="h-full">
                  <FlashcardFront
                    word={content.word}
                    {...(content.ipa !== undefined && { ipa: content.ipa })}
                    {...(content.audioUrl !== undefined && {
                      audioUrl: content.audioUrl,
                    })}
                    {...(content.audioAttribution !== undefined && {
                      audioAttribution: content.audioAttribution,
                    })}
                    {...(content.audioLicence !== undefined && {
                      audioLicence: content.audioLicence,
                    })}
                    onFlip={reveal}
                    revealRef={revealRef}
                  />
                </div>
              }
              back={
                <div inert={!isFlipped} className="h-full">
                  <FlashcardBack
                    word={content.word}
                    definition={content.definition}
                    {...(content.ipa !== undefined && { ipa: content.ipa })}
                    {...(content.definitionVi !== undefined && {
                      definitionVi: content.definitionVi,
                    })}
                    exampleSentences={content.exampleSentences}
                    {...(content.audioUrl !== undefined && {
                      audioUrl: content.audioUrl,
                    })}
                    {...(content.audioAttribution !== undefined && {
                      audioAttribution: content.audioAttribution,
                    })}
                    {...(content.audioLicence !== undefined && {
                      audioLicence: content.audioLicence,
                    })}
                    {...(content.pos !== undefined && {
                      partOfSpeech: content.pos,
                    })}
                    onReportSentence={(sentenceText) => {
                      setReportNote(`Example sentence: ${sentenceText}`);
                      setIsReportOpen(true);
                    }}
                    answerRef={answerRef}
                  />
                </div>
              }
            />
          )}
        </div>

        {(content === null || isFlipped) && (
          // On a phone the answer can run past the fold; the grades stay in
          // reach above the bottom navigation instead of below the examples.
          <div className="sticky bottom-[calc(4.5rem+env(safe-area-inset-bottom))] z-20 rounded-2xl border border-border-subtle bg-surface-card/95 p-3 shadow-lg backdrop-blur-sm md:static md:border-0 md:bg-transparent md:p-0 md:shadow-none md:backdrop-blur-none">
            <GradeButtonGroup
              onGrade={onGrade}
              disabled={isSubmitting}
              pendingGrade={pendingGrade}
              {...(content !== null && {
                nextDue: currentCard.next_due_by_grade,
              })}
            />
          </div>
        )}

        <p className="hidden text-center text-xs text-text-muted md:block">
          {t("review.keyboardHelp")}
        </p>
      </div>
    );
  }

  return (
    <div className="mx-auto w-full max-w-[1440px] space-y-5 animate-in fade-in duration-200 motion-reduce:animate-none md:space-y-6">
      {header}
      {body}

      <ReportDialog
        isOpen={isReportOpen}
        contentVersionId={currentCard?.content_version_id ?? null}
        initialNote={reportNote}
        onClose={() => setIsReportOpen(false)}
      />
    </div>
  );
}

export default ReviewPage;
