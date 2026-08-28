import React from "react";
import { Link } from "@tanstack/react-router";
import { CheckCircle2, Layers, PlayCircle } from "lucide-react";
import { useTranslation } from "react-i18next";

import { Button } from "@/components/ui/button";
import {
  Card,
  CardDescription,
  CardFooter,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";

export interface ReviewQueueCardProps {
  dueCount: number;
}

/**
 * The door to the review session.
 *
 * /practice/review has existed since WP9 — the queue, the FSRS grading, the
 * 1-4 keyboard shortcuts, all of it — and nothing in the app linked to it. The
 * dashboard's "Start Review" button pointed at /practice, which was a
 * placeholder, so the only way in was to type the URL. The E2E suite did not
 * notice because its journeys navigate with page.goto rather than by clicking.
 *
 * This card is that link, and it is deliberately the first thing on the page.
 */
export const ReviewQueueCard: React.FC<ReviewQueueCardProps> = ({
  dueCount,
}) => {
  const { t } = useTranslation();

  if (dueCount === 0) {
    return (
      <Card>
        <CardHeader>
          <div className="flex items-center gap-2 text-text-muted mb-1">
            <Layers className="h-5 w-5" aria-hidden="true" />
            <span className="text-xs font-semibold uppercase tracking-wider">
              {t("practice.review.label", "Spaced repetition")}
            </span>
          </div>
          <div className="flex items-center gap-2">
            <CheckCircle2 className="h-6 w-6 text-success" aria-hidden="true" />
            <CardTitle className="text-lg font-bold">
              {t("practice.review.emptyTitle", "Nothing due right now")}
            </CardTitle>
          </div>
          <CardDescription>
            {t(
              "practice.review.emptyDesc",
              "Your queue is clear. Finish a lesson and new cards are scheduled for you automatically.",
            )}
          </CardDescription>
        </CardHeader>
        <CardFooter className="pt-0">
          <Link to="/learn">
            <Button variant="secondary">
              {t("practice.review.emptyLearnBtn", "Go to a lesson")}
            </Button>
          </Link>
        </CardFooter>
      </Card>
    );
  }

  return (
    <Card className="border-primary/30">
      <CardHeader>
        <div className="flex items-center gap-2 text-primary-accent mb-1">
          <Layers className="h-5 w-5" aria-hidden="true" />
          <span className="text-xs font-semibold uppercase tracking-wider">
            {t("practice.review.label", "Spaced repetition")}
          </span>
        </div>
        <div className="flex items-baseline gap-2">
          <span className="text-4xl font-extrabold text-text tracking-tight">
            {dueCount}
          </span>
          <CardTitle className="text-lg font-bold">
            {t("practice.review.dueTitle", "cards due today")}
          </CardTitle>
        </div>
        <CardDescription>
          {t(
            "practice.review.dueDesc",
            "Grade each card by how well you recalled it. The schedule adapts to your answers.",
          )}
        </CardDescription>
      </CardHeader>
      <CardFooter className="pt-0">
        <Link to="/practice/review">
          <Button className="gap-2">
            <PlayCircle className="h-4 w-4" aria-hidden="true" />
            {t("practice.review.startBtn", "Start review")}
          </Button>
        </Link>
      </CardFooter>
    </Card>
  );
};
