import React from "react";
import { useTranslation } from "react-i18next";
import { Link } from "@tanstack/react-router";
import { TrendingUp } from "lucide-react";

import { PageHeader } from "@/components/layout/PageHeader";
import { EmptyState } from "@/components/ui/empty-state";
import {
  type CourseMeta,
  CourseProgressList,
  DashboardError,
  OverallStats,
  ProgressSkeleton,
  SkillMasteryList,
  useProgress,
  useWeeklyPlan,
} from "@/features/learning";
import { useCourses } from "@/features/lesson";
import { courseTitle } from "@/features/lesson/model/courseText";

export function ProgressPage(): React.JSX.Element {
  const { t } = useTranslation();
  const { data, isLoading, isError, error, refetch } = useProgress();
  const { data: catalogueData } = useCourses();
  const { data: weeklyPlan } = useWeeklyPlan();

  const courseMeta = React.useMemo(() => {
    const map = new Map<string, CourseMeta>();
    const list = catalogueData?.courses ?? [];
    for (const c of list) {
      map.set(c.id, {
        title: courseTitle(t, c),
        slug: c.slug,
      });
    }
    return map;
  }, [catalogueData, t]);

  if (isLoading) {
    return <ProgressSkeleton />;
  }

  if (isError || !data) {
    return <DashboardError onRetry={() => refetch()} error={error} />;
  }

  // `courses` and `skills` are required in ProgressResponse and the backend ships
  // `[]`. No defensive default: if one ever arrives null that is a backend bug
  // worth failing loudly on, not one to paper over here.
  const { courses, skills } = data;

  const totalActivities = courses.reduce(
    (acc, c) => acc + c.completed_activities,
    0,
  );
  const coursesInProgress = courses.filter(
    (c) => c.status === "in_progress",
  ).length;
  const coursesCompleted = courses.filter(
    (c) => c.status === "completed" || c.percentage >= 100,
  ).length;

  const isEmpty = courses.length === 0 && skills.length === 0;

  return (
    <div className="mx-auto w-full max-w-[1440px] space-y-4 animate-in fade-in duration-200 motion-reduce:animate-none md:space-y-5">
      <PageHeader
        title={t("progress.title", "Learning Progress")}
        description={t(
          "progress.tagline",
          "Track your study time, skill competencies, and course completion.",
        )}
      />

      {isEmpty && (
        <EmptyState
          icon={TrendingUp}
          title={t("progress.emptyTitle", "No learning progress yet")}
          description={t(
            "progress.emptyDesc",
            "Start learning lessons to track your study progress, skill competencies, and course completion.",
          )}
          action={
            <Link
              to="/learn"
              className="inline-flex min-h-[44px] items-center justify-center gap-2 rounded-lg bg-primary px-4 text-sm font-semibold text-primary-fg shadow-sm transition-colors hover:bg-primary-hover focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary focus-visible:ring-offset-2"
            >
              {t("progress.startLearning", "Start learning")}
            </Link>
          }
        />
      )}

      {/*
        Only measured figures: activities completed and courses in progress /
        completed from GET /me/progress, and this week's study minutes from
        GET /me/weekly-plan (WeeklyPlanProgress.minutes). All-time study time,
        words mastered, week-over-week deltas, a date-range filter and a
        per-day activity chart have no endpoint yet and are not drawn; the
        earlier derived "3 minutes per activity" figures were removed for that
        reason (the P10 learner-web card).
      */}
      <OverallStats
        totalActivitiesCompleted={totalActivities}
        coursesInProgress={coursesInProgress}
        coursesCompleted={coursesCompleted}
        studyThisWeek={
          weeklyPlan
            ? {
                minutes: weeklyPlan.progress.minutes,
                goal: weeklyPlan.minutes_goal,
              }
            : undefined
        }
      />

      {/* Two Column Grid: CEFR Skills & Course Progress */}
      <div className="grid grid-cols-1 items-start gap-4 lg:gap-5 xl:grid-cols-[minmax(0,1fr)_minmax(0,1.15fr)]">
        <SkillMasteryList skills={skills} />
        <CourseProgressList courses={courses} courseMeta={courseMeta} />
      </div>
    </div>
  );
}

export default ProgressPage;
