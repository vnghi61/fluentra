import React from "react";
import { useTranslation } from "react-i18next";
import { Link } from "@tanstack/react-router";
import type { LucideIcon } from "lucide-react";
import { BookMarked, ChevronRight, Folder } from "lucide-react";

import { PageHeader } from "@/components/layout/PageHeader";
import { Card } from "@/components/ui/card";
import { ErrorState } from "@/components/ui/error-state";
import { Skeleton } from "@/components/ui/skeleton";
import {
  cardLinkClass,
  DailyPracticeCard,
  GuestNotice,
  SKILL_TONE,
} from "@/features/learning";
import { useCourse } from "@/features/lesson";
import {
  ForecastStrip,
  ReviewQueueCard,
  useDueCount,
  useForecast,
} from "@/features/review";
import { cn } from "@/lib/utils";
import { useAuthStore } from "@/stores/authStore";

const generatedPracticeSlug = "generated-vocabulary-practice";

interface PracticeTileProps {
  icon: LucideIcon;
  /** Tint of the icon tile. */
  tone: string;
  title: string;
  description: string;
  /** What the exercises are, e.g. "Passages · Comprehension questions". */
  meta: string;
  /** The action, read by screen readers with the card's name. */
  action: string;
  to: "/learn" | "/practice/my-words" | "/my-resources";
  search?: { course: string };
  /** A full-row card laid out horizontally from `md` (the odd one out). */
  wide?: boolean;
}

/**
 * One practice type: the whole card is the link. Icon tile, title and
 * description, then a divider, the kinds of exercise inside and a round arrow.
 */
function PracticeTile({
  icon: Icon,
  tone,
  title,
  description,
  meta,
  action,
  to,
  search,
  wide = false,
}: PracticeTileProps): React.JSX.Element {
  return (
    <Link
      to={to}
      search={search}
      className={cn(
        "group flex min-h-11 min-w-0 flex-col rounded-[var(--radius-card)] border border-border-subtle bg-surface-card p-4 shadow-[var(--shadow-card)] transition duration-150 hover:-translate-y-px hover:border-primary/40 hover:shadow-md focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary motion-reduce:transition-none motion-reduce:hover:translate-y-0 md:p-5",
        wide &&
          "md:col-span-2 md:flex-row md:items-center md:gap-5 xl:col-span-3",
      )}
    >
      <div
        className={cn(
          "flex min-w-0 flex-1 items-start gap-3.5",
          !wide && "md:flex-col md:gap-4",
        )}
      >
        <span
          className={cn(
            "flex h-11 w-11 shrink-0 items-center justify-center rounded-xl md:h-12 md:w-12",
            tone,
          )}
          aria-hidden="true"
        >
          <Icon className="h-5 w-5 md:h-6 md:w-6" />
        </span>
        <span className="min-w-0 space-y-1">
          <span className="block text-base font-semibold text-text group-hover:text-primary-accent">
            {title}
          </span>
          <span className="block text-sm text-text-muted">{description}</span>
        </span>
      </div>
      <div
        className={cn(
          "mt-4 flex items-center gap-3 border-t border-border-subtle pt-3.5",
          wide
            ? "md:mt-0 md:w-72 md:shrink-0 md:border-l md:border-t-0 md:pl-5 md:pt-0"
            : "md:mt-auto",
        )}
      >
        <span className="min-w-0 flex-1 truncate text-sm font-medium text-text-muted">
          {meta}
        </span>
        <span
          className="flex h-9 w-9 shrink-0 items-center justify-center rounded-full bg-primary/10 text-primary-accent transition-colors group-hover:bg-primary group-hover:text-primary-fg motion-reduce:transition-none"
          aria-hidden="true"
        >
          <ChevronRight className="h-4 w-4" />
        </span>
        <span className="sr-only">{action}</span>
      </div>
    </Link>
  );
}

export function PracticePage(): React.JSX.Element {
  const { t } = useTranslation();
  const signedIn = useAuthStore((state) => state.status === "authenticated");
  const generatedPractice = useCourse(generatedPracticeSlug);
  const due = useDueCount(signedIn);
  const forecast = useForecast(signedIn);

  // The forecast sits beside the queue only when there is one to draw, so a
  // failed or empty forecast does not leave half the row blank.
  const forecastDays = forecast.data?.days ?? [];
  const showForecast =
    signedIn && (forecast.isLoading || forecastDays.length > 0);

  let queue: React.ReactNode;
  if (!signedIn) {
    queue = <ReviewQueueCard dueCount={0} guest />;
  } else if (due.isLoading) {
    queue = <Skeleton className="h-44 w-full rounded-[var(--radius-card)]" />;
  } else if (due.isError) {
    queue = (
      <Card className="h-full">
        <ErrorState
          title={t("practice.error.title", "Unable to load your review queue")}
          description={t(
            "practice.error.desc",
            "The review schedule could not be reached. Your progress is safe.",
          )}
          onRetry={() => void due.refetch()}
          retryLabel={t("action.retry", "Try again")}
        />
      </Card>
    );
  } else {
    queue = <ReviewQueueCard dueCount={due.data?.due_count ?? 0} />;
  }

  return (
    <div className="mx-auto w-full max-w-[1440px] space-y-6 animate-in fade-in duration-200 motion-reduce:animate-none md:space-y-8">
      <PageHeader
        title={t("practice.title", "Practice")}
        description={t(
          "practice.tagline",
          "Keep what you have learned. Reviews are scheduled for the day you are most likely to forget.",
        )}
      />

      {!signedIn ? <GuestNotice /> : null}

      {/*
        Today's set leads; the review queue and the week's forecast sit beside
        it from xl, and side by side under it on a tablet.
      */}
      <div className="grid gap-4 md:gap-5 xl:grid-cols-[minmax(0,1.65fr)_minmax(0,1fr)]">
        <div className="min-w-0">
          <DailyPracticeCard variant="practice" />
        </div>

        <section
          aria-label={t("practice.review.sectionTitle")}
          className={cn(
            "grid min-w-0 gap-4 md:gap-5 xl:grid-cols-1",
            showForecast && "md:grid-cols-2",
          )}
        >
          {queue}
          {!showForecast ? null : forecast.isLoading ? (
            <Skeleton className="h-44 w-full rounded-[var(--radius-card)]" />
          ) : (
            <ForecastStrip days={forecastDays} className="h-full" />
          )}
        </section>
      </div>

      <section className="space-y-4" aria-labelledby="practice-types-title">
        <div className="flex flex-wrap items-end justify-between gap-3">
          <div className="min-w-0 space-y-1">
            <h2
              id="practice-types-title"
              className="text-lg font-semibold leading-snug text-text md:text-xl"
            >
              {t("practice.sections.title", "Practice types")}
            </h2>
            <p className="text-sm text-text-muted">
              {t(
                "practice.sections.subtitle",
                "Choose a type of exercise to practise what you need.",
              )}
            </p>
          </div>
          <Link
            to="/learn"
            className={cardLinkClass}
            aria-label={t("overview.practice.seeAllLabel")}
          >
            {t("overview.practice.seeAll")}
          </Link>
        </div>

        <div className="grid grid-cols-1 gap-3 md:grid-cols-2 md:gap-4 xl:grid-cols-3">
          {generatedPractice.data ? (
            <PracticeTile
              icon={SKILL_TONE.vocabulary.icon}
              tone={SKILL_TONE.vocabulary.chip}
              title={generatedPractice.data.title}
              description={t(
                "practice.generated.desc",
                "Practise vocabulary in context with varied exercises.",
              )}
              meta={t("overview.practice.meta.vocabulary")}
              action={t("practice.generated.openBtn", "Open drills")}
              to="/learn"
              search={{ course: generatedPracticeSlug }}
            />
          ) : null}
          <PracticeTile
            icon={SKILL_TONE.reading.icon}
            tone={SKILL_TONE.reading.chip}
            title={t("practice.reading.title", "Reading practice")}
            description={t(
              "practice.reading.desc",
              "A2–B2 reading passages with questions and reading-speed tracking.",
            )}
            meta={t("overview.practice.meta.reading")}
            action={t("practice.reading.openBtn", "Start reading")}
            to="/learn"
            search={{ course: "reading-foundations" }}
          />
          <PracticeTile
            icon={SKILL_TONE.listening.icon}
            tone={SKILL_TONE.listening.chip}
            title={t("practice.listening.title", "Listening practice")}
            description={t(
              "practice.listening.desc",
              "Listen to announcements and conversations, then answer questions.",
            )}
            meta={t("overview.practice.meta.listening")}
            action={t("practice.listening.openBtn", "Start listening")}
            to="/learn"
            search={{ course: "listening-foundations" }}
          />
          <PracticeTile
            icon={SKILL_TONE.writing.icon}
            tone={SKILL_TONE.writing.chip}
            title={t("practice.writing.title", "Writing practice")}
            description={t(
              "practice.writing.desc",
              "Practise emails through IELTS essays with feedback after writing.",
            )}
            meta={t("overview.practice.meta.writing")}
            action={t("practice.writing.openBtn", "Start writing")}
            to="/learn"
            search={{ course: "writing-foundations" }}
          />
          <PracticeTile
            icon={SKILL_TONE.speaking.icon}
            tone={SKILL_TONE.speaking.chip}
            title={t("practice.speaking.title", "Speaking practice")}
            description={t(
              "practice.speaking.desc",
              "Read aloud, answer by voice and receive feedback when you finish.",
            )}
            meta={t("overview.practice.meta.speaking")}
            action={t("practice.speaking.openBtn", "Start speaking")}
            to="/learn"
            search={{ course: "speaking-foundations" }}
          />
          <PracticeTile
            icon={BookMarked}
            tone="bg-primary/10 text-primary-accent"
            title={t("practice.myWords.title", "My words")}
            description={t(
              "practice.myWords.desc",
              "Create a personal vocabulary list and schedule reviews automatically.",
            )}
            meta={t("overview.practice.meta.myWords")}
            action={t("uploads.openLink", "My words")}
            to="/practice/my-words"
          />
          <PracticeTile
            icon={Folder}
            tone="bg-surface-muted text-text-muted"
            title={t("practice.resources.title", "My resources")}
            description={t(
              "practice.resources.desc",
              "Upload a PDF, audio or video to create exercises from your content.",
            )}
            meta={t("overview.practice.meta.resources")}
            action={t("practice.resources.openBtn", "Open my resources")}
            to="/my-resources"
            wide
          />
        </div>
      </section>
    </div>
  );
}

export default PracticePage;
