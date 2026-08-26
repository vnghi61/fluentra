import React, { useCallback, useEffect, useState } from "react";
import { Link } from "@tanstack/react-router";
import { AlertCircle, ArrowLeft } from "lucide-react";
import { useTranslation } from "react-i18next";

import { Button } from "@/components/ui/button";
import {
  Card,
  CardDescription,
  CardFooter,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Progress } from "@/components/ui/progress";
import {
  CardContentUnavailable,
  EmptyQueue,
  FlashcardBack,
  FlashcardFront,
  flashcardContent,
  GradeButtonGroup,
  reviewApi,
  type ReviewGrade,
  ReviewSummary,
  useReviewSession,
} from "@/features/review";

type GradeCounts = Record<ReviewGrade, number>;

const noGrades: GradeCounts = { again: 0, hard: 0, good: 0, easy: 0 };

export function ReviewPage(): React.JSX.Element {
  const { t } = useTranslation();
  const { data: sessionData, isLoading, isError, refetch } = useReviewSession();

  // `cards` is required in ReviewSessionResponse; it is absent here only while the
  // query resolves, which the loading branch below handles. There is deliberately
  // no defensive default on a field the backend guarantees.
  const cards = sessionData?.cards ?? [];

  const [currentIndex, setCurrentIndex] = useState(0);
  const [isFlipped, setIsFlipped] = useState(false);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [isCompleted, setIsCompleted] = useState(false);
  const [gradeFailed, setGradeFailed] = useState(false);
  const [gradeCounts, setGradeCounts] = useState<GradeCounts>(noGrades);

  const currentCard = cards[currentIndex];
  const content = currentCard ? flashcardContent(currentCard) : null;

  // Space and Enter reveal the answer. The 1–4 grade shortcuts live in
  // GradeButtonGroup, which mounts only once the card is flipped, so a digit
  // cannot grade a card whose answer the learner has not seen.
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if (
        e.target instanceof HTMLInputElement ||
        e.target instanceof HTMLTextAreaElement
      ) {
        return;
      }
      if ((e.key === " " || e.key === "Enter") && !isFlipped && currentCard) {
        e.preventDefault();
        setIsFlipped(true);
      }
    };

    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, [isFlipped, currentCard]);

  const advance = useCallback(
    (grade: ReviewGrade) => {
      setGradeCounts((prev) => ({ ...prev, [grade]: prev[grade] + 1 }));
      setIsFlipped(false);
      if (currentIndex < cards.length - 1) {
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

      setIsSubmitting(true);
      setGradeFailed(false);
      try {
        await reviewApi.answerCard(currentCard.id, grade);
        advance(grade);
      } catch {
        setGradeFailed(true);
      } finally {
        setIsSubmitting(false);
      }
    },
    [advance, currentCard, isSubmitting],
  );

  if (isLoading) {
    return (
      <div
        className="flex items-center justify-center min-h-[50vh]"
        role="status"
        aria-label={t("review.loading")}
      >
        <div className="animate-spin rounded-full h-8 w-8 border-4 border-border-subtle border-t-primary" />
      </div>
    );
  }

  if (isError) {
    return (
      <div className="py-12 max-w-lg mx-auto">
        <Card className="border-danger/30 text-center p-6">
          <CardHeader>
            <CardTitle>{t("review.errorTitle")}</CardTitle>
            <CardDescription>{t("review.errorDesc")}</CardDescription>
          </CardHeader>
          <CardFooter className="justify-center gap-3">
            <Button variant="outline" onClick={() => void refetch()}>
              {t("action.retry")}
            </Button>
            <Link to="/">
              <Button>{t("review.backToDashboardBtn")}</Button>
            </Link>
          </CardFooter>
        </Card>
      </div>
    );
  }

  if (isCompleted) {
    return (
      <ReviewSummary
        totalReviewed={cards.length}
        gradeCounts={gradeCounts}
        onRestart={() => {
          setCurrentIndex(0);
          setIsFlipped(false);
          setIsCompleted(false);
          setGradeFailed(false);
          setGradeCounts(noGrades);
          void refetch();
        }}
      />
    );
  }

  if (!currentCard) {
    return <EmptyQueue />;
  }

  const progressPercent = Math.round(((currentIndex + 1) / cards.length) * 100);

  return (
    <div className="max-w-2xl mx-auto space-y-6 animate-in fade-in duration-200 py-4">
      <div className="flex items-center justify-between gap-4">
        <Link to="/">
          {/*
            The label is hidden below `sm`, which left the arrow alone in a 32 px
            button — under the 44 × 44 minimum, and only at 320 px, which is why
            no test saw it until the narrow-320 project covered this screen.
            min-w-11 keeps the target while the text goes away, and the
            accessible name comes from aria-label rather than the hidden span.
            The negative margin went with it: it pulled the button outside the
            link wrapping it, so the anchor measured 36 px however wide the
            button was.
          */}
          <Button
            variant="ghost"
            size="sm"
            aria-label={t("review.backToDashboardBtn")}
            className="gap-1.5 text-text-muted hover:text-text min-w-11 min-h-11"
          >
            <ArrowLeft className="h-4 w-4" aria-hidden="true" />
            <span className="hidden sm:inline">
              {t("review.backToDashboardBtn")}
            </span>
          </Button>
        </Link>

        <div
          className="flex-1 max-w-xs space-y-1"
          data-testid="review-progress"
          data-current={currentIndex + 1}
          data-total={cards.length}
        >
          <div className="flex justify-between text-xs text-text-muted font-medium">
            <span>{t("review.sessionTitle")}</span>
            <span>
              {currentIndex + 1} / {cards.length}
            </span>
          </div>
          <Progress
            value={progressPercent}
            aria-label={t("review.progressLabel", {
              current: currentIndex + 1,
              total: cards.length,
            })}
          />
        </div>
      </div>

      <main className="space-y-6">
        {gradeFailed && (
          <div
            role="alert"
            className="p-4 rounded-xl border border-danger/40 bg-danger/10 flex items-center gap-2 text-danger-accent text-sm font-medium"
          >
            <AlertCircle className="h-5 w-5 shrink-0" aria-hidden="true" />
            <span>{t("review.gradeFailed")}</span>
          </div>
        )}

        {/*
          A card whose version could not be resolved carries no word to show. It is
          still gradable — the schedule is real — so the grade buttons stay, and the
          face says what is missing instead of inventing a word to put there.
        */}
        {content === null ? (
          <div className="space-y-6">
            <CardContentUnavailable
              contentVersionId={currentCard.content_version_id}
            />
            <GradeButtonGroup
              onGrade={(grade) => void handleGrade(grade)}
              disabled={isSubmitting}
            />
          </div>
        ) : !isFlipped ? (
          <FlashcardFront
            word={content.word}
            {...(content.ipa !== undefined && { ipa: content.ipa })}
            {...(content.audioUrl !== undefined && { audioUrl: content.audioUrl })}
            onFlip={() => setIsFlipped(true)}
          />
        ) : (
          <div className="space-y-6 animate-in fade-in duration-200">
            <FlashcardBack
              word={content.word}
              definition={content.definition}
              {...(content.ipa !== undefined && { ipa: content.ipa })}
              {...(content.definitionVi !== undefined && { definitionVi: content.definitionVi })}
              {...(content.exampleSentence !== undefined && {
                exampleSentence: content.exampleSentence,
              })}
              {...(content.pos !== undefined && { partOfSpeech: content.pos })}
            />

            <GradeButtonGroup
              onGrade={(grade) => void handleGrade(grade)}
              disabled={isSubmitting}
            />
          </div>
        )}
      </main>
    </div>
  );
}

export default ReviewPage;
