import React from "react";
import { Link } from "@tanstack/react-router";
import { ArrowRight, CheckCircle2, RotateCcw } from "lucide-react";
import { useTranslation } from "react-i18next";

import { Button } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardFooter,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";

export interface ReviewSummaryProps {
  totalReviewed: number;
  gradeCounts: {
    again: number;
    hard: number;
    good: number;
    easy: number;
  };
  onRestart?: () => void;
}

export const ReviewSummary: React.FC<ReviewSummaryProps> = ({
  totalReviewed,
  gradeCounts,
  onRestart,
}) => {
  const { t } = useTranslation();
  const successfulReviews = gradeCounts.good + gradeCounts.easy;
  // Nothing reviewed is not perfect retention.
  const retentionRate =
    totalReviewed > 0
      ? Math.round((successfulReviews / totalReviewed) * 100)
      : 0;

  return (
    <div className="max-w-md mx-auto py-12 px-4 animate-in fade-in">
      <Card className="text-center border-success/40 bg-gradient-to-b from-surface-card to-success/5 shadow-xl">
        <CardHeader className="space-y-3 pb-4">
          <div className="mx-auto flex h-16 w-16 items-center justify-center rounded-full bg-success/15 text-success">
            <CheckCircle2 className="h-10 w-10" aria-hidden="true" />
          </div>
          <CardTitle className="text-2xl font-bold text-text">
            {t("review.summaryTitle", "Review Session Summary")}
          </CardTitle>
          <CardDescription>
            {t(
              "review.emptyDesc",
              "You have cleared all pending review cards for today. Great job!",
            )}
          </CardDescription>
        </CardHeader>

        <CardContent className="space-y-4">
          {/* Summary Metrics */}
          <div className="grid grid-cols-2 gap-3 pt-2">
            <div className="p-4 rounded-xl border border-border bg-surface-muted/50 text-center space-y-1">
              <span className="text-xs text-text-muted font-medium">
                {t("review.cardsReviewed", "Cards Reviewed")}
              </span>
              <p className="text-2xl font-extrabold text-text">
                {totalReviewed}
              </p>
            </div>

            <div className="p-4 rounded-xl border border-border bg-surface-muted/50 text-center space-y-1">
              <span className="text-xs text-text-muted font-medium">
                {t("review.retentionRate", "Retention Rate")}
              </span>
              <p className="text-2xl font-extrabold text-text">
                {retentionRate}%
              </p>
            </div>
          </div>
        </CardContent>

        <CardFooter className="flex flex-col gap-3 pt-2">
          <Link to="/" className="w-full">
            <Button size="lg" className="w-full font-bold gap-2 min-h-[48px]">
              {t("review.backToDashboardBtn", "Back to Dashboard")}
              <ArrowRight className="h-4 w-4" aria-hidden="true" />
            </Button>
          </Link>

          {onRestart && (
            <Button
              variant="outline"
              onClick={onRestart}
              className="w-full gap-2 min-h-[44px]"
            >
              <RotateCcw className="h-4 w-4" aria-hidden="true" />
              {t("action.retry", "Practice Again")}
            </Button>
          )}
        </CardFooter>
      </Card>
    </div>
  );
};
