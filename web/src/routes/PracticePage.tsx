import React from "react";
import { useTranslation } from "react-i18next";
import { AlertCircle, BookMarked } from "lucide-react";

import { Button } from "@/components/ui/button";
import {
  Card,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
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
  const due = useDueCount();
  const forecast = useForecast();

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

      {due.isLoading ? (
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
      {forecast.isLoading ? (
        <Skeleton className="h-40 w-full rounded-xl" />
      ) : forecast.data ? (
        <ForecastStrip days={forecast.data.days} />
      ) : null}

      {/*
        Named, not hidden. Vocabulary has a backend — lookup, search and decks —
        and no screens yet, and a learner who came here from a button labelled
        "Practice Vocabulary" is owed an answer rather than a blank space. This
        card makes no promise about when.
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
            {t("practice.vocabulary.title", "Word lists are not here yet")}
          </CardTitle>
          <CardDescription>
            {t(
              "practice.vocabulary.desc",
              "Browsing and building decks is still being built. Words you meet in lessons are already scheduled for review above.",
            )}
          </CardDescription>
        </CardHeader>
      </Card>
    </div>
  );
}

export default PracticePage;
