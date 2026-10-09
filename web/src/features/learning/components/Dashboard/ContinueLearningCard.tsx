import React from "react";
import { Link } from "@tanstack/react-router";
import {
  ArrowRight,
  BookOpen,
  CheckCircle2,
  Clock,
  Sparkles,
  type LucideIcon,
} from "lucide-react";
import { useTranslation } from "react-i18next";

import { Card } from "@/components/ui/card";
import { Progress } from "@/components/ui/progress";
import { StackedCardsArt } from "@/components/ui/stacked-cards-art";
import { cn } from "@/lib/utils";
import { useProgress, type DashboardResponse } from "../../api/learningApi";
import { SKILL_TONE, type Skill } from "../Progress/skillTone";

export interface ContinueLearningCardProps {
  dashboard: DashboardResponse;
}

const primaryCtaStyles =
  "inline-flex w-full items-center justify-center gap-2 rounded-xl bg-primary px-5 py-2.5 text-sm font-semibold text-primary-fg shadow-sm min-h-[44px] transition-[transform,background-color] duration-150 hover:bg-primary-hover active:scale-[0.98] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary focus-visible:ring-offset-2 motion-reduce:transition-none motion-reduce:active:scale-100 sm:w-auto";

interface HeroShellProps {
  tone: "primary" | "success";
  eyebrowIcon: LucideIcon;
  eyebrow: string;
  artIcon: LucideIcon;
  children: React.ReactNode;
}

/**
 * The hero's frame: eyebrow, content column and the stacked-card illustration
 * (small in the corner on a phone, large beside the content from `sm`).
 */
function HeroShell({
  tone,
  eyebrowIcon: EyebrowIcon,
  eyebrow,
  artIcon,
  children,
}: HeroShellProps): React.JSX.Element {
  return (
    <Card
      className={cn(
        "relative h-full overflow-hidden p-5 md:p-6 lg:p-7",
        tone === "success"
          ? "border-success/30 bg-gradient-to-br from-success/10 via-surface-card to-surface-card"
          : "border-primary/25 bg-gradient-to-br from-primary/10 via-surface-card to-surface-card",
      )}
    >
      <div className="grid h-full grid-cols-1 items-center gap-5 sm:grid-cols-[minmax(0,1fr)_auto] md:gap-8">
        <div className="min-w-0 space-y-3">
          <p
            className={cn(
              "flex items-center gap-1.5 pr-16 text-xs font-semibold uppercase tracking-wider sm:pr-0",
              tone === "success"
                ? "text-success-accent"
                : "text-primary-accent",
            )}
          >
            <EyebrowIcon className="h-4 w-4" aria-hidden="true" />
            {eyebrow}
          </p>
          {children}
        </div>
        <StackedCardsArt
          icon={artIcon}
          size="lg"
          className="mr-3 hidden sm:block lg:mr-6"
        />
      </div>
      <StackedCardsArt
        icon={artIcon}
        size="sm"
        className="absolute right-5 top-5 sm:hidden"
      />
    </Card>
  );
}

export const ContinueLearningCard: React.FC<ContinueLearningCardProps> = ({
  dashboard,
}) => {
  const { t } = useTranslation();
  const { data: progressData } = useProgress();
  const { state, next_activity } = dashboard;

  if (state === "completed") {
    return (
      <HeroShell
        tone="success"
        eyebrowIcon={CheckCircle2}
        eyebrow={t("dashboard.continue.completedBadge", "Course Completed")}
        artIcon={CheckCircle2}
      >
        <h2 className="text-xl font-bold tracking-tight text-text md:text-2xl">
          {t("dashboard.continue.completedTitle", "All Caught Up!")}
        </h2>
        <p className="max-w-xl text-sm text-text-muted">
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
      </HeroShell>
    );
  }

  if (state === "in_progress" && next_activity) {
    // The course's done/total comes from /me/progress; until it arrives (or
    // when the course is not in it) the card says "pick up where you left off"
    // instead of drawing an empty bar.
    const course = progressData?.courses.find(
      (c) => c.course_id === next_activity.course_id,
    );
    const total = course?.total_activities ?? 0;
    const done = course?.completed_activities ?? 0;
    const hasProgress = Boolean(course) && total > 0 && done >= 0;
    const percent = Math.min(
      100,
      Math.max(
        0,
        course?.percentage ??
          (hasProgress ? Math.round((done / total) * 100) : 0),
      ),
    );
    const SkillIcon =
      next_activity.skill in SKILL_TONE
        ? SKILL_TONE[next_activity.skill as Skill].icon
        : BookOpen;
    const skillLabel = t(
      `dashboard.continue.skillLabel.${next_activity.skill}`,
      {
        defaultValue: t(
          `dashboard.skills.${next_activity.skill}`,
          next_activity.skill,
        ),
      },
    );

    return (
      <HeroShell
        tone="primary"
        eyebrowIcon={BookOpen}
        eyebrow={t("dashboard.continue.eyebrow", "Continue learning")}
        artIcon={SkillIcon}
      >
        <div className="flex flex-wrap items-center gap-2">
          <span className="inline-flex items-center gap-1.5 rounded-full bg-primary/10 px-2.5 py-1 text-xs font-semibold text-primary-accent">
            <SkillIcon className="h-4 w-4" aria-hidden="true" />
            {skillLabel}
          </span>
          {next_activity.estimated_minutes ? (
            <span className="inline-flex items-center gap-1 rounded-full bg-surface-muted px-2.5 py-1 text-xs font-medium text-text-muted">
              <Clock className="h-4 w-4" aria-hidden="true" />
              {t("dashboard.continue.minutes", {
                minutes: next_activity.estimated_minutes,
                defaultValue: `${next_activity.estimated_minutes} min`,
              })}
            </span>
          ) : null}
        </div>

        <h2 className="line-clamp-2 text-xl font-bold tracking-tight text-text md:text-2xl">
          {next_activity.title}
        </h2>

        {hasProgress ? (
          <div className="max-w-md space-y-1.5 pt-0.5">
            <div className="flex items-center justify-between gap-3 text-xs font-medium text-text-muted">
              <span className="text-sm font-bold text-text">{percent}%</span>
              <span>
                {t("dashboard.continue.activitiesCount", {
                  completed: done,
                  total,
                  defaultValue: `${done} / ${total} activities`,
                })}
              </span>
            </div>
            <Progress
              value={percent}
              className="h-2 bg-surface-muted"
              aria-label={t(
                "dashboard.continue.activitiesProgress",
                "{{completed}} of {{total}} activities completed",
                { completed: done, total },
              )}
            />
          </div>
        ) : (
          <p className="line-clamp-2 max-w-xl text-sm text-text-muted">
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
      </HeroShell>
    );
  }

  // not_started
  return (
    <HeroShell
      tone="primary"
      eyebrowIcon={Sparkles}
      eyebrow={t("dashboard.continue.notStartedBadge", "Get Started")}
      artIcon={BookOpen}
    >
      <h2 className="text-xl font-bold tracking-tight text-text md:text-2xl">
        {t("dashboard.continue.notStartedTitle", "Start Your English Journey")}
      </h2>
      <p className="max-w-xl text-sm text-text-muted">
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
            {t("dashboard.continue.startFirstLessonBtn", "Start First Lesson")}
            <ArrowRight className="h-4 w-4" aria-hidden="true" />
          </Link>
        ) : (
          <Link to="/learn" className={primaryCtaStyles}>
            {t("dashboard.continue.exploreBtn", "Explore Syllabus")}
            <ArrowRight className="h-4 w-4" aria-hidden="true" />
          </Link>
        )}
      </div>
    </HeroShell>
  );
};
