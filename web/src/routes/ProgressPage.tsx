import React from "react";
import { useTranslation } from "react-i18next";

import {
  CourseProgressList,
  DashboardError,
  DashboardSkeleton,
  OverallStats,
  SkillMasteryList,
  useProgress,
} from "@/features/learning";

export function ProgressPage(): React.JSX.Element {
  const { t } = useTranslation();
  const { data, isLoading, isError, error, refetch } = useProgress();

  if (isLoading) {
    return <DashboardSkeleton />;
  }

  if (isError || !data) {
    return <DashboardError onRetry={() => void refetch()} error={error} />;
  }

  // `courses` and `skills` are required in ProgressResponse and the backend ships
  // `[]`. No defensive default: if one ever arrives null that is a backend bug
  // worth failing loudly on, not one to paper over here.
  const { courses, skills } = data;

  const totalActivities = courses.reduce(
    (acc, c) => acc + c.completed_activities,
    0,
  );

  return (
    <div className="space-y-8 animate-in fade-in duration-200">
      {/* Top Header */}
      <header className="space-y-1">
        <h1 className="text-2xl md:text-3xl font-extrabold text-text tracking-tight">
          {t("progress.title", "Learning Progress")}
        </h1>
        <p className="text-sm text-text-muted">
          {t(
            "progress.tagline",
            "Track your study time, skill competencies, and course completion.",
          )}
        </p>
      </header>

      {/*
        Only the one number GET /me/progress actually returns.

        Total study time and words mastered were derived here from the activity
        count — three minutes each and four words each — and rendered as measured
        facts beside a real figure. ProgressResponse carries neither. srs records
        real minutes in learn.review_daily_stats but does not expose them on this
        endpoint yet; until it does, the honest count is one card, not three.
        See docs/development/phase-2/P10-learner-web.md §1.
      */}
      <OverallStats totalActivitiesCompleted={totalActivities} />

      {/* Two Column Grid: CEFR Skills & Course Progress */}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-8">
        <SkillMasteryList skills={skills} />
        <CourseProgressList courses={courses} />
      </div>
    </div>
  );
}

export default ProgressPage;
