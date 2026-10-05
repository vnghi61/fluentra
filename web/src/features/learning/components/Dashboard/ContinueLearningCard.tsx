import React from "react";
import { Link } from "@tanstack/react-router";
import {
  ArrowRight,
  BookOpen,
  CheckCircle2,
  Clock,
  Mic,
  Sparkles,
} from "lucide-react";
import { useTranslation } from "react-i18next";

import { Badge } from "@/components/ui/badge";
import { Card } from "@/components/ui/card";
import { Progress } from "@/components/ui/progress";
import { useProgress, type DashboardResponse } from "../../api/learningApi";

export interface ContinueLearningCardProps {
  dashboard: DashboardResponse;
}

const primaryCtaStyles =
  "inline-flex items-center justify-center font-semibold rounded-xl bg-primary text-primary-fg hover:bg-primary-hover active:bg-primary-hover shadow-sm min-h-[44px] px-5 py-2.5 text-sm gap-2 transition-[transform,background-color] duration-150 active:scale-[0.98] motion-reduce:active:scale-100 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary focus-visible:ring-offset-2 w-full sm:w-auto";

const HeroDecoration: React.FC<{ skill?: string | undefined }> = ({
  skill,
}) => {
  const Icon = skill === "speaking" ? Mic : BookOpen;
  return (
    <>
      {/* Small icon on mobile */}
      <div
        className="sm:hidden absolute right-3 top-3 flex h-8 w-8 items-center justify-center rounded-xl bg-primary/10 text-primary-accent"
        aria-hidden="true"
      >
        <Icon className="h-4 w-4" />
      </div>

      {/* Large composition on sm+ */}
      <div
        className="pointer-events-none select-none relative hidden sm:flex sm:items-center sm:justify-center sm:shrink-0 sm:pr-2 lg:pr-4"
        aria-hidden="true"
      >
        <div className="relative w-20 h-20 sm:w-24 sm:h-24 md:w-28 md:h-28">
          {/* Soft circle */}
          <div className="absolute -inset-2 rounded-full bg-primary/10 blur-xl opacity-70" />
          {/* Layer 1: Back plate tilted */}
          <div className="absolute inset-0 rounded-2xl bg-gradient-to-br from-primary/20 to-primary/5 border border-primary/25 rotate-6 transform motion-reduce:rotate-0" />
          {/* Layer 2: Middle plate reverse-tilted */}
          <div className="absolute inset-0 rounded-2xl bg-gradient-to-tr from-surface-muted/80 to-surface-card/60 dark:bg-surface-muted/30 border border-primary/20 -rotate-3 transform motion-reduce:rotate-0" />
          {/* Layer 3: Front plate with chosen Icon */}
          <div className="absolute inset-0 rounded-2xl bg-surface-card/90 dark:bg-surface-card/95 border border-primary/30 shadow-sm flex items-center justify-center">
            <Icon className="h-7 w-7 sm:h-10 sm:w-10 md:h-12 md:w-12 text-primary" />
          </div>
        </div>
      </div>
    </>
  );
};

export const ContinueLearningCard: React.FC<ContinueLearningCardProps> = ({
  dashboard,
}) => {
  const { t } = useTranslation();
  const { data: progressData } = useProgress();
  const { state, next_activity } = dashboard;

  const activeCourse =
    state === "in_progress" && next_activity?.course_id
      ? progressData?.courses?.find(
          (c) => c.course_id === next_activity.course_id,
        )
      : undefined;

  const totalActivities = activeCourse?.total_activities ?? 0;
  const completedActivities = activeCourse?.completed_activities ?? 0;
  const hasValidProgress =
    Boolean(activeCourse) &&
    typeof totalActivities === "number" &&
    totalActivities > 0 &&
    typeof completedActivities === "number" &&
    completedActivities >= 0;

  const rawPercentage =
    activeCourse?.percentage ??
    (hasValidProgress
      ? Math.round((completedActivities / totalActivities) * 100)
      : 0);
  const percentage = Math.min(100, Math.max(0, rawPercentage));

  if (state === "completed") {
    return (
      <Card className="relative overflow-hidden rounded-2xl border border-success/30 bg-gradient-to-br from-success/10 via-surface-card to-primary/5 dark:from-success/15 dark:via-surface-card dark:to-surface-card p-5 md:p-6 lg:p-7 md:min-h-[215px] flex flex-col justify-between shadow-sm hover:-translate-y-0.5 transition duration-150 motion-reduce:transform-none">
        <div className="grid grid-cols-1 sm:grid-cols-[1fr_auto] items-center justify-between gap-4 md:gap-6 relative z-10">
          <div className="flex-1 min-w-0 space-y-3">
            <div className="flex items-center gap-1.5 text-success-accent mb-0.5">
              <CheckCircle2 className="h-4 w-4" aria-hidden="true" />
              <span className="text-xs font-semibold uppercase tracking-wider">
                {t("dashboard.continue.completedBadge", "Course Completed")}
              </span>
            </div>
            <h2 className="text-xl md:text-2xl font-bold text-text tracking-tight">
              {t("dashboard.continue.completedTitle", "All Caught Up!")}
            </h2>
            <p className="text-sm text-text-muted max-w-xl">
              {t(
                "dashboard.continue.completedDesc",
                "You have completed all activities in your enrolled course. Keep practicing to maintain mastery.",
              )}
            </p>
            <div className="pt-1">
              <Link to="/learn" className={primaryCtaStyles}>
                {t("dashboard.continue.reviewBtn", "Review Lessons")}
                <ArrowRight className="h-4 w-4" aria-hidden="true" />
              </Link>
            </div>
          </div>
          <HeroDecoration />
        </div>
      </Card>
    );
  }

  if (state === "in_progress" && next_activity) {
    return (
      <Card className="relative overflow-hidden rounded-2xl border border-primary/30 bg-gradient-to-br from-primary/10 via-surface-card to-primary/5 dark:from-primary/15 dark:via-surface-card dark:to-surface-card p-5 md:p-6 lg:p-7 md:min-h-[215px] flex flex-col justify-between shadow-sm hover:-translate-y-0.5 transition duration-150 motion-reduce:transform-none">
        <div className="grid grid-cols-1 sm:grid-cols-[1fr_auto] items-center justify-between gap-4 md:gap-6 relative z-10">
          <div className="flex-1 min-w-0 space-y-3">
            <div className="flex flex-wrap items-center gap-2">
              <div className="flex items-center gap-1.5 text-primary-accent">
                <BookOpen className="h-4 w-4" aria-hidden="true" />
                <span className="text-xs font-semibold uppercase tracking-wider">
                  {t("dashboard.continue.eyebrow", "Continue learning")}
                </span>
              </div>
              <Badge
                variant="primary"
                className="capitalize text-xs font-medium py-0.5"
              >
                {t(`dashboard.continue.skillLabel.${next_activity.skill}`, {
                  defaultValue: t(
                    `dashboard.skills.${next_activity.skill}`,
                    next_activity.skill,
                  ),
                })}
              </Badge>
              {next_activity.estimated_minutes && (
                <span className="inline-flex items-center gap-1 text-xs text-text-muted">
                  <Clock className="h-3.5 w-3.5" aria-hidden="true" />
                  {t("dashboard.continue.minutes", {
                    minutes: next_activity.estimated_minutes,
                    defaultValue: `${next_activity.estimated_minutes} min`,
                  })}
                </span>
              )}
            </div>

            <h2 className="text-xl md:text-2xl font-bold text-text tracking-tight line-clamp-2">
              {next_activity.title}
            </h2>

            {hasValidProgress ? (
              <div className="space-y-1.5 max-w-md pt-0.5">
                <div className="flex items-center justify-between text-xs text-text-muted font-medium">
                  <span>{percentage}%</span>
                  <span>
                    {t("dashboard.continue.activitiesCount", {
                      completed: completedActivities,
                      total: totalActivities,
                      defaultValue: `${completedActivities} / ${totalActivities} activities`,
                    })}
                  </span>
                </div>
                <Progress
                  value={percentage}
                  className="h-2 bg-surface-muted border-border-subtle transition-[width] duration-300 motion-reduce:transition-none"
                  aria-label={t(
                    "dashboard.continue.activitiesProgress",
                    "{{completed}} of {{total}} activities completed",
                    {
                      completed: completedActivities,
                      total: totalActivities,
                    },
                  )}
                />
              </div>
            ) : (
              <p className="text-sm text-text-muted line-clamp-2 max-w-xl">
                {t(
                  "dashboard.continue.resumePrompt",
                  "Pick up right where you left off in your study plan.",
                )}
              </p>
            )}

            <div className="pt-1">
              <Link
                to="/learn/lesson/$lessonId"
                params={{ lessonId: next_activity.lesson_id }}
                className={primaryCtaStyles}
              >
                {t("dashboard.continue.continueBtn", "Continue learning")}
                <ArrowRight className="h-4 w-4" aria-hidden="true" />
              </Link>
            </div>
          </div>

          <HeroDecoration skill={next_activity.skill} />
        </div>
      </Card>
    );
  }

  // Default: not_started
  return (
    <Card className="relative overflow-hidden rounded-2xl border border-primary/20 bg-gradient-to-br from-primary/10 via-surface-card to-primary/5 dark:from-primary/15 dark:via-surface-card dark:to-surface-card p-5 md:p-6 lg:p-7 md:min-h-[215px] flex flex-col justify-between shadow-sm hover:-translate-y-0.5 transition duration-150 motion-reduce:transform-none">
      <div className="grid grid-cols-1 sm:grid-cols-[1fr_auto] items-center justify-between gap-4 md:gap-6 relative z-10">
        <div className="flex-1 min-w-0 space-y-3">
          <div className="flex items-center gap-1.5 text-primary-accent mb-0.5">
            <Sparkles className="h-4 w-4" aria-hidden="true" />
            <span className="text-xs font-semibold uppercase tracking-wider">
              {t("dashboard.continue.notStartedBadge", "Get Started")}
            </span>
          </div>
          <h2 className="text-xl md:text-2xl font-bold text-text tracking-tight">
            {t(
              "dashboard.continue.notStartedTitle",
              "Start Your English Journey",
            )}
          </h2>
          <p className="text-sm text-text-muted max-w-xl">
            {next_activity?.title
              ? next_activity.title
              : t(
                  "dashboard.continue.notStartedDesc",
                  "You haven't enrolled in any courses yet. Explore our structured curriculum to begin.",
                )}
          </p>
          <div className="pt-1">
            {next_activity?.lesson_id ? (
              <Link
                to="/learn/lesson/$lessonId"
                params={{ lessonId: next_activity.lesson_id }}
                className={primaryCtaStyles}
              >
                {t(
                  "dashboard.continue.startFirstLessonBtn",
                  "Start First Lesson",
                )}
                <ArrowRight className="h-4 w-4" aria-hidden="true" />
              </Link>
            ) : (
              <Link to="/learn" className={primaryCtaStyles}>
                {t("dashboard.continue.exploreBtn", "Explore Syllabus")}
                <ArrowRight className="h-4 w-4" aria-hidden="true" />
              </Link>
            )}
          </div>
        </div>
        <HeroDecoration />
      </div>
    </Card>
  );
};
