import React from "react";
import { Link } from "@tanstack/react-router";
import { CheckCircle2, Layers, PlayCircle, Sparkles } from "lucide-react";
import { useTranslation } from "react-i18next";

import { Button } from "@/components/ui/button";
import {
  Card,
  CardDescription,
  CardFooter,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";

export interface ReviewsDueCardProps {
  dueCount: number;
}

export const ReviewsDueCard: React.FC<ReviewsDueCardProps> = ({ dueCount }) => {
  const { t } = useTranslation();
  const estimatedMinutes = Math.max(1, Math.ceil(dueCount * 0.5));

  if (dueCount === 0) {
    return (
      <Card className="h-full flex flex-col justify-between">
        <CardHeader>
          <div className="flex items-center gap-2 text-text-muted mb-1">
            <Layers className="h-5 w-5" aria-hidden="true" />
            <span className="text-xs font-semibold uppercase tracking-wider">
              {t("dashboard.reviews.title", "Reviews Due")}
            </span>
          </div>
          <div className="flex items-center gap-2 mt-1">
            <CheckCircle2 className="h-6 w-6 text-success" aria-hidden="true" />
            <CardTitle className="text-lg font-bold">
              {t("dashboard.reviews.emptyTitle", "All Caught Up!")}
            </CardTitle>
          </div>
          <CardDescription>
            {t(
              "dashboard.reviews.emptyDesc",
              "Nothing due right now. New cards are scheduled as you finish lessons.",
            )}
          </CardDescription>
        </CardHeader>
        <CardFooter className="pt-0">
          <Link to="/practice" className="w-full">
            <Button variant="secondary" className="w-full gap-2">
              <Sparkles className="h-4 w-4" aria-hidden="true" />
              {t("dashboard.reviews.practiceBtn", "Go to practice")}
            </Button>
          </Link>
        </CardFooter>
      </Card>
    );
  }

  return (
    <Card className="h-full flex flex-col justify-between border-warning/30">
      <CardHeader>
        <div className="flex items-center justify-between gap-2 mb-1">
          <div className="flex items-center gap-2 text-warning-accent">
            <Layers className="h-5 w-5" aria-hidden="true" />
            <span className="text-xs font-semibold uppercase tracking-wider">
              {t("dashboard.reviews.title", "Reviews Due")}
            </span>
          </div>
          <span className="text-xs text-text-muted">
            {t("dashboard.reviews.estimatedTime", {
              minutes: estimatedMinutes,
              defaultValue: `~${estimatedMinutes} mins`,
            })}
          </span>
        </div>
        <div className="mt-2">
          <div className="flex items-baseline gap-2">
            <span className="text-3xl font-extrabold text-text tracking-tight">
              {dueCount}
            </span>
            <span className="text-sm font-medium text-text-muted">
              {t("dashboard.reviews.cardsLabel", "cards to review today")}
            </span>
          </div>
          <CardDescription className="mt-2">
            {t(
              "dashboard.reviews.readyDesc",
              "Spaced repetition cards are ready for your daily memory review.",
            )}
          </CardDescription>
        </div>
      </CardHeader>
      <CardFooter className="pt-0">
        {/*
          Straight into the session. This button used to point at /practice,
          which was a placeholder page, so the one control on the dashboard
          that offered to start a review started nothing.
        */}
        <Link to="/practice/review" className="w-full">
          <Button className="w-full gap-2 bg-warning-fill hover:bg-warning-fill-hover text-surface font-semibold">
            <PlayCircle className="h-4 w-4" aria-hidden="true" />
            {t("dashboard.reviews.startBtn", "Start Review")}
          </Button>
        </Link>
      </CardFooter>
    </Card>
  );
};
