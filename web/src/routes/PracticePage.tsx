import React from "react";
import { useTranslation } from "react-i18next";
import { Link } from "@tanstack/react-router";
import type { LucideIcon } from "lucide-react";
import {
  AlertCircle,
  BookMarked,
  BookOpen,
  ChevronRight,
  Folder,
  Headphones,
  Layers,
  Mic,
  PencilLine,
  Sparkles,
} from "lucide-react";

import { Button } from "@/components/ui/button";
import {
  Card,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { DailyPracticeCard, GuestNotice } from "@/features/learning";
import { useCourse } from "@/features/lesson";
import {
  ForecastStrip,
  ReviewQueueCard,
  useDueCount,
  useForecast,
} from "@/features/review";
import { useAuthStore } from "@/stores/authStore";

const generatedPracticeSlug = "generated-vocabulary-practice";

interface PracticeTileProps {
  icon: LucideIcon;
  title: string;
  description: string;
  action: string;
  to: "/learn" | "/practice/my-words" | "/my-resources";
  search?: { course: string };
}

function PracticeTile({
  icon: Icon,
  title,
  description,
  action,
  to,
  search,
}: PracticeTileProps): React.JSX.Element {
  return (
    <Link
      to={to}
      search={search}
      className="group flex min-h-[44px] min-w-0 items-center gap-3 rounded-2xl border border-border-subtle bg-surface-card p-4 shadow-sm transition duration-150 hover:-translate-y-px hover:bg-surface-muted/60 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary motion-reduce:transition-none motion-reduce:hover:translate-y-0 md:p-5"
    >
      <span className="flex h-10 w-10 shrink-0 items-center justify-center rounded-xl bg-primary/10 text-primary">
        <Icon className="h-5 w-5" aria-hidden="true" />
      </span>
      <span className="min-w-0 flex-1">
        <span className="block truncate text-base font-semibold text-text">
          {title}
        </span>
        <span className="hidden text-sm text-text-muted md:block md:line-clamp-2">
          {description}
        </span>
      </span>
      <span className="hidden rounded-lg border border-border-subtle px-3 py-2 text-sm font-medium text-text md:inline">
        {action}
      </span>
      <ChevronRight
        className="h-5 w-5 shrink-0 text-text-muted"
        aria-hidden="true"
      />
    </Link>
  );
}

export function PracticePage(): React.JSX.Element {
  const { t } = useTranslation();
  const signedIn = useAuthStore((state) => state.status === "authenticated");
  const generatedPractice = useCourse(generatedPracticeSlug);
  const due = useDueCount(signedIn);
  const forecast = useForecast(signedIn);

  return (
    <div className="mx-auto w-full max-w-[1200px] space-y-6 animate-in fade-in duration-200">
      <header className="space-y-1">
        <h1 className="text-2xl font-extrabold tracking-tight text-text md:text-3xl">
          {t("practice.title", "Practice")}
        </h1>
        <p className="text-sm text-text-muted">
          {t(
            "practice.tagline",
            "Keep what you have learned. Reviews are scheduled for the day you are most likely to forget.",
          )}
        </p>
      </header>

      <DailyPracticeCard variant="practice" />

      <section className="space-y-3" aria-labelledby="practice-types-title">
        <div className="space-y-1">
          <h2 id="practice-types-title" className="text-lg font-bold text-text">
            {t("practice.sections.title", "Practice types")}
          </h2>
          <p className="text-sm text-text-muted">
            {t(
              "practice.sections.subtitle",
              "Choose a type of exercise to practise what you need.",
            )}
          </p>
        </div>

        {!signedIn ? <GuestNotice /> : null}

        <div className="grid grid-cols-1 gap-3 md:grid-cols-2 md:gap-4">
          {!signedIn ? (
            <Link
              to="/learn"
              className="flex min-h-[44px] min-w-0 items-center gap-3 rounded-2xl border border-border-subtle bg-surface-card p-4 shadow-sm transition duration-150 hover:-translate-y-px hover:bg-surface-muted/60 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary motion-reduce:transition-none motion-reduce:hover:translate-y-0 md:p-5"
            >
              <span className="flex h-10 w-10 shrink-0 items-center justify-center rounded-xl bg-primary/10 text-primary">
                <Layers className="h-5 w-5" aria-hidden="true" />
              </span>
              <span className="min-w-0 flex-1">
                <span className="block truncate text-base font-semibold text-text">
                  {t("guest.practice.title", "Reviews need an account")}
                </span>
                <span className="hidden text-sm text-text-muted md:block md:line-clamp-2">
                  {t(
                    "guest.practice.desc",
                    "Sign in and the lessons you finish start filling this queue.",
                  )}
                </span>
              </span>
              <span className="hidden rounded-lg border border-border-subtle px-3 py-2 text-sm font-medium text-text md:inline">
                {t("practice.review.emptyLearnBtn", "Go to a lesson")}
              </span>
              <ChevronRight
                className="h-5 w-5 shrink-0 text-text-muted"
                aria-hidden="true"
              />
            </Link>
          ) : due.isLoading ? (
            <Skeleton className="h-28 w-full rounded-2xl" />
          ) : due.isError ? (
            <Card className="border-danger/30">
              <CardHeader>
                <div className="mb-1 flex items-center gap-2 text-danger-accent">
                  <AlertCircle className="h-5 w-5" aria-hidden="true" />
                  <CardTitle className="text-base font-semibold">
                    {t(
                      "practice.error.title",
                      "Unable to load your review queue",
                    )}
                  </CardTitle>
                </div>
                <CardDescription>
                  {t(
                    "practice.error.desc",
                    "The review schedule could not be reached. Your progress is safe.",
                  )}
                </CardDescription>
                <Button variant="secondary" onClick={() => void due.refetch()}>
                  {t("action.retry", "Try again")}
                </Button>
              </CardHeader>
            </Card>
          ) : (
            <ReviewQueueCard dueCount={due.data?.due_count ?? 0} />
          )}

          {generatedPractice.data ? (
            <PracticeTile
              icon={Sparkles}
              title={generatedPractice.data.title}
              description={t(
                "practice.generated.desc",
                "Practise vocabulary in context with varied exercises.",
              )}
              action={t("practice.generated.openBtn", "Open drills")}
              to="/learn"
              search={{ course: generatedPracticeSlug }}
            />
          ) : null}
          <PracticeTile
            icon={BookMarked}
            title={t("practice.myWords.title", "My words")}
            description={t(
              "practice.myWords.desc",
              "Create a personal vocabulary list and schedule reviews automatically.",
            )}
            action={t("uploads.openLink", "My words")}
            to="/practice/my-words"
          />
          <PracticeTile
            icon={Folder}
            title={t("practice.resources.title", "My resources")}
            description={t(
              "practice.resources.desc",
              "Upload a PDF, audio or video to create exercises from your content.",
            )}
            action={t("practice.resources.openBtn", "Open my resources")}
            to="/my-resources"
          />
          <PracticeTile
            icon={BookOpen}
            title={t("practice.reading.title", "Reading practice")}
            description={t(
              "practice.reading.desc",
              "A2–B2 reading passages with questions and reading-speed tracking.",
            )}
            action={t("practice.reading.openBtn", "Start reading")}
            to="/learn"
            search={{ course: "reading-foundations" }}
          />
          <PracticeTile
            icon={Headphones}
            title={t("practice.listening.title", "Listening practice")}
            description={t(
              "practice.listening.desc",
              "Listen to announcements and conversations, then answer questions.",
            )}
            action={t("practice.listening.openBtn", "Start listening")}
            to="/learn"
            search={{ course: "listening-foundations" }}
          />
          <PracticeTile
            icon={PencilLine}
            title={t("practice.writing.title", "Writing practice")}
            description={t(
              "practice.writing.desc",
              "Practise emails through IELTS essays with feedback after writing.",
            )}
            action={t("practice.writing.openBtn", "Start writing")}
            to="/learn"
            search={{ course: "writing-foundations" }}
          />
          <PracticeTile
            icon={Mic}
            title={t("practice.speaking.title", "Speaking practice")}
            description={t(
              "practice.speaking.desc",
              "Read aloud, answer by voice and receive feedback when you finish.",
            )}
            action={t("practice.speaking.openBtn", "Start speaking")}
            to="/learn"
            search={{ course: "speaking-foundations" }}
          />
        </div>
      </section>

      {!signedIn ? null : forecast.isLoading ? (
        <Skeleton className="h-40 w-full rounded-2xl" />
      ) : forecast.data ? (
        <ForecastStrip days={forecast.data.days} />
      ) : null}
    </div>
  );
}

export default PracticePage;
