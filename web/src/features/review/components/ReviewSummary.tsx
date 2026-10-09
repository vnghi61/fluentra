import React, { useEffect, useRef } from "react";
import { Link } from "@tanstack/react-router";
import { ArrowRight, CheckCircle2, RotateCcw } from "lucide-react";
import { useTranslation } from "react-i18next";

import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { cn } from "@/lib/utils";
import type { ReviewGrade } from "../api/reviewApi";

export interface ReviewSummaryProps {
  totalReviewed: number;
  gradeCounts: Record<ReviewGrade, number>;
  onRestart?: () => void;
}

const breakdown: {
  grade: ReviewGrade;
  labelKey: string;
  text: string;
  bar: string;
}[] = [
  {
    grade: "again",
    labelKey: "review.again",
    text: "text-danger-accent",
    bar: "bg-danger",
  },
  {
    grade: "hard",
    labelKey: "review.hard",
    text: "text-warning-accent",
    bar: "bg-warning",
  },
  {
    grade: "good",
    labelKey: "review.good",
    text: "text-primary-accent",
    bar: "bg-primary",
  },
  {
    grade: "easy",
    labelKey: "review.easy",
    text: "text-success-accent",
    bar: "bg-success",
  },
];

/**
 * The end of a session: what was done, how it went, and where to go next.
 *
 * Calm on purpose — a summary is a place to stop, not a reward screen. Every
 * figure is the learner's own grades from this session; nothing is estimated.
 */
export const ReviewSummary: React.FC<ReviewSummaryProps> = ({
  totalReviewed,
  gradeCounts,
  onRestart,
}) => {
  const { t } = useTranslation();
  const headingRef = useRef<HTMLHeadingElement>(null);

  // The grade button that finished the session has gone, and focus with it.
  // Landing on the heading tells a screen reader the session is over.
  useEffect(() => {
    headingRef.current?.focus();
  }, []);

  const successfulReviews = gradeCounts.good + gradeCounts.easy;
  // Nothing reviewed is not perfect retention.
  const retentionRate =
    totalReviewed > 0
      ? Math.round((successfulReviews / totalReviewed) * 100)
      : 0;

  return (
    <Card className="mx-auto w-full max-w-2xl animate-in fade-in duration-200 motion-reduce:animate-none">
      <div className="flex flex-col items-center gap-3 px-5 pb-6 pt-8 text-center sm:px-8">
        <span className="flex h-14 w-14 items-center justify-center rounded-full bg-success/10 text-success-accent">
          <CheckCircle2 className="h-8 w-8" aria-hidden="true" />
        </span>
        <h2
          ref={headingRef}
          tabIndex={-1}
          className="text-xl font-bold leading-tight tracking-tight text-text focus:outline-none md:text-2xl"
        >
          {t("review.summaryTitle", "Review Session Summary")}
        </h2>
        <p className="max-w-md text-base leading-relaxed text-text-muted">
          {t("review.summaryDesc", { count: totalReviewed })}
        </p>
      </div>

      <dl className="grid grid-cols-2 gap-3 px-5 sm:px-8">
        <div className="space-y-1 rounded-xl border border-border-subtle bg-surface-muted p-4 text-center">
          <dt className="text-sm font-medium text-text-muted">
            {t("review.cardsReviewed", "Cards Reviewed")}
          </dt>
          <dd className="text-2xl font-bold tabular-nums text-text">
            {totalReviewed}
          </dd>
        </div>
        <div className="space-y-1 rounded-xl border border-border-subtle bg-surface-muted p-4 text-center">
          <dt className="text-sm font-medium text-text-muted">
            {t("review.retentionRate", "Retention Rate")}
          </dt>
          <dd className="text-2xl font-bold tabular-nums text-text">
            {retentionRate}%
          </dd>
        </div>
      </dl>

      <section
        aria-labelledby="review-breakdown-title"
        className="space-y-3 px-5 pt-6 sm:px-8"
      >
        <h3
          id="review-breakdown-title"
          className="text-base font-semibold leading-snug text-text"
        >
          {t("review.breakdownTitle")}
        </h3>
        {totalReviewed > 0 && (
          <div
            aria-hidden="true"
            className="flex h-2 w-full overflow-hidden rounded-full bg-surface-muted"
          >
            {breakdown.map(({ grade, bar }) =>
              gradeCounts[grade] > 0 ? (
                <span
                  key={grade}
                  className={bar}
                  style={{
                    width: `${(gradeCounts[grade] / totalReviewed) * 100}%`,
                  }}
                />
              ) : null,
            )}
          </div>
        )}
        <ul className="grid grid-cols-2 gap-2 sm:grid-cols-4">
          {breakdown.map(({ grade, labelKey, text }) => (
            <li
              key={grade}
              className="flex items-center justify-between gap-2 rounded-lg border border-border-subtle px-3 py-2"
            >
              <span className={cn("text-sm font-medium", text)}>
                {t(labelKey)}
              </span>
              <span className="text-sm font-semibold tabular-nums text-text">
                {gradeCounts[grade]}
              </span>
            </li>
          ))}
        </ul>
      </section>

      <div className="mt-6 flex flex-col-reverse gap-3 border-t border-border-subtle p-5 sm:flex-row sm:flex-wrap sm:items-center sm:justify-end sm:px-8">
        {onRestart && (
          <Button variant="ghost" onClick={onRestart} className="sm:mr-auto">
            <RotateCcw className="h-4 w-4" aria-hidden="true" />
            {t("review.checkAgainBtn")}
          </Button>
        )}
        <Button asChild variant="outline">
          <Link to="/">
            {t("review.backToDashboardBtn", "Back to Dashboard")}
          </Link>
        </Button>
        <Button asChild>
          <Link to="/learn">
            {t("review.continueLearningBtn")}
            <ArrowRight className="h-4 w-4" aria-hidden="true" />
          </Link>
        </Button>
      </div>
    </Card>
  );
};
