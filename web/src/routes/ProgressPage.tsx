import React from "react";
import { useTranslation } from "react-i18next";
import { Link } from "@tanstack/react-router";
import { TrendingUp } from "lucide-react";

import { EmptyState } from "@/components/ui/empty-state";
import {
  type CourseMeta,
  CourseProgressList,
  DashboardError,
  OverallStats,
  ProgressSkeleton,
  SkillMasteryList,
  useProgress,
} from "@/features/learning";
import { useCourses } from "@/features/lesson";
import { courseTitle } from "@/features/lesson/model/courseText";

export function ProgressPage(): React.JSX.Element {
  const { t } = useTranslation();
  const { data, isLoading, isError, error, refetch } = useProgress();
  const { data: catalogueData } = useCourses();

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
    return (
      <DashboardError
        title={t("progress.error.title", "Unable to Load Progress")}
        onRetry={() => void refetch()}
        error={error}
      />
    );
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
    <div className="mx-auto w-full max-w-[1240px] space-y-4 md:space-y-5 animate-in fade-in duration-200 motion-reduce:animate-none">
      {/* Top Header */}
      <header className="space-y-1">
        <h1 className="text-2xl md:text-3xl font-extrabold tracking-tight text-text">
          {t("progress.title", "Learning Progress")}
        </h1>
        <p className="text-sm text-text-muted">
          {t(
            "progress.tagline",
            "Track your study time, skill competencies, and course completion.",
          )}
        </p>
      </header>

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
              className="inline-flex min-h-[44px] items-center justify-center gap-2 rounded-lg bg-primary px-4 text-sm font-medium text-primary-fg shadow-sm transition-colors hover:bg-primary-hover focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary focus-visible:ring-offset-2"
            >
              {t("progress.startLearning", "Start learning")}
            </Link>
          }
        />
      )}

      {/*
        Only the real figures GET /me/progress actually returns:
        activities completed, courses in progress, and courses completed.

        Total study time and words mastered were previously derived here from the
        activity count — three minutes each and four words each — and rendered as
        measured facts beside a real figure. ProgressResponse carries neither.
        srs records real minutes in learn.review_daily_stats but does not expose
        them on this endpoint yet; until it does, the honest count is three cards
        grounded in real data, not fabricated measurements.
        See docs/development/phase-2/P10-learner-web.md §1.
      */}
      <OverallStats
        totalActivitiesCompleted={totalActivities}
        coursesInProgress={coursesInProgress}
        coursesCompleted={coursesCompleted}
      />

      {/* Two Column Grid: CEFR Skills & Course Progress */}
      <div className="grid grid-cols-1 gap-4 lg:gap-5 lg:grid-cols-[minmax(0,1.27fr)_minmax(0,1fr)] items-start">
        <SkillMasteryList skills={skills} />
        <CourseProgressList courses={courses} courseMeta={courseMeta} />
      </div>
    </div>
  );
}

export default ProgressPage;
