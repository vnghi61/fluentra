import React from "react";
import { useTranslation } from "react-i18next";
import { Link } from "@tanstack/react-router";
import { AlertCircle, BookMarked, Layers } from "lucide-react";

import { Button } from "@/components/ui/button";
import {
  Card,
  CardDescription,
  CardFooter,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { GuestNotice } from "@/features/learning";
import { useAuthStore } from "@/stores/authStore";
import {
  ForecastStrip,
  ReviewQueueCard,
  useDueCount,
  useForecast,
} from "@/features/review";

/**
 * The practice hub.
 *
 * It replaces a twelve-line placeholder that existed "so the router has
 * something to route to" — while the sidebar advertised Practice, and the
 * dashboard's two review buttons both pointed here. Every one of those led to
 * a heading and a tagline.
 *
 * The page owns no learning logic. It is a door: due count and the way into
 * the session, then the week ahead so an empty queue still says something.
 */
export function PracticePage(): React.JSX.Element {
  const { t } = useTranslation();
  // Review cards belong to a person. A guest has none, and asking for them
  // would earn a 401 on a page they are allowed to be on — which reads as a
  // bug rather than as the honest "there is nothing here for you yet".
  const signedIn = useAuthStore((state) => state.status === "authenticated");
  const due = useDueCount(signedIn);
  const forecast = useForecast(signedIn);

  return (
    <div className="space-y-6 animate-in fade-in duration-200">
      <header className="space-y-1">
        <h1 className="text-2xl md:text-3xl font-extrabold text-text tracking-tight">
          {t("practice.title", "Practice")}
        </h1>
        <p className="text-sm text-text-muted">
          {t(
            "practice.tagline",
            "Keep what you have learned. Reviews are scheduled for the day you are most likely to forget.",
          )}
        </p>
      </header>

      {!signedIn ? (
        <>
          <GuestNotice />
          <Card>
            <CardHeader>
              <div className="flex items-center gap-2 text-text-muted mb-1">
                <Layers className="h-5 w-5" aria-hidden="true" />
                <span className="text-xs font-semibold uppercase tracking-wider">
                  {t("practice.review.label", "Spaced repetition")}
                </span>
              </div>
              <CardTitle className="text-base font-semibold">
                {t("guest.practice.title", "Reviews need an account")}
              </CardTitle>
              <CardDescription>
                {t(
                  "guest.practice.desc",
                  "Cards are scheduled against the day you are likely to forget, which means they belong to a person. Sign in and the lessons you finish start filling this queue.",
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
        </>
      ) : due.isLoading ? (
        <Skeleton className="h-48 w-full rounded-xl" />
      ) : due.isError ? (
        <Card className="border-danger/30">
          <CardHeader>
            <div className="flex items-center gap-2 text-danger-accent mb-1">
              <AlertCircle className="h-5 w-5" aria-hidden="true" />
              <CardTitle className="text-base font-semibold">
                {t("practice.error.title", "Unable to load your review queue")}
              </CardTitle>
            </div>
            <CardDescription>
              {t(
                "practice.error.desc",
                "The review schedule could not be reached. Your progress is safe.",
              )}
            </CardDescription>
          </CardHeader>
          <div className="px-6 pb-6">
            <Button variant="secondary" onClick={() => void due.refetch()}>
              {t("action.retry", "Try again")}
            </Button>
          </div>
        </Card>
      ) : (
        <ReviewQueueCard dueCount={due.data?.due_count ?? 0} />
      )}

      {/*
        The forecast is supporting detail, so it fails quietly: a learner whose
        queue loaded does not need an error banner because a chart did not.
      */}
      {!signedIn ? null : forecast.isLoading ? (
        <Skeleton className="h-40 w-full rounded-xl" />
      ) : forecast.data ? (
        <ForecastStrip days={forecast.data.days} />
      ) : null}

      {/*
        This card said "Word lists are not here yet" for as long as there was no
        screen behind it. There is one now, so it says what it does instead of
        apologising for what it does not.
      */}
      <Card>
        <CardHeader>
          <div className="flex items-center gap-2 text-text-muted mb-1">
            <BookMarked className="h-5 w-5" aria-hidden="true" />
            <span className="text-xs font-semibold uppercase tracking-wider">
              {t("practice.vocabulary.label", "Vocabulary")}
            </span>
          </div>
          <CardTitle className="text-base font-semibold">
            {t("practice.myWords.title", "Add your own words")}
          </CardTitle>
          <CardDescription>
            {t(
              "practice.myWords.desc",
              "Paste vocabulary from your own course. We check each word against a dictionary, write example sentences for it, and schedule it for review here.",
            )}
          </CardDescription>
        </CardHeader>
        <CardFooter className="pt-0">
          <Link to="/practice/my-words">
            <Button variant="secondary" className="gap-2">
              <BookMarked className="h-4 w-4" aria-hidden="true" />
              {t("uploads.openLink", "My words")}
            </Button>
          </Link>
        </CardFooter>
      </Card>
    </div>
  );
}

export default PracticePage;
