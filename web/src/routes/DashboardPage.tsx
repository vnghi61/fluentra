import React from "react";
import { useTranslation } from "react-i18next";

import {
  GamificationSummarySection,
  useGamificationSummary,
} from "@/features/gamification";
import {
  ContinueLearningCard,
  DashboardError,
  DashboardSkeleton,
  ReviewsDueCard,
  SkillProgressCard,
  useDashboard,
} from "@/features/learning";

export function DashboardPage(): React.JSX.Element {
  const { t } = useTranslation();
  const { data, isLoading, isError, error, refetch } = useDashboard();
  const { data: gamificationData } = useGamificationSummary();

  if (isLoading) {
    return <DashboardSkeleton />;
  }

  if (isError || !data) {
    return <DashboardError onRetry={() => void refetch()} error={error} />;
  }

  return (
    <div className="space-y-6 animate-in fade-in duration-200">
      {/* Dashboard Top Heading */}
      <header className="space-y-1">
        <h1 className="text-2xl md:text-3xl font-extrabold text-text tracking-tight">
          {t("dashboard.welcome", "Welcome to Fluentra")}
        </h1>
        <p className="text-sm text-text-muted">
          {t("dashboard.tagline", "Here is your learning summary for today.")}
        </p>
      </header>

      {/* Gamification Motivation Section (XP, Level, Streak, Quests, League) */}
      {gamificationData && (
        <section
          aria-label={t(
            "gamification.sectionAria",
            "Learning Motivation & Progress",
          )}
        >
          <GamificationSummarySection summary={gamificationData} />
        </section>
      )}

      {/* Hero Card: 1. Continue Learning */}
      <section aria-label={t("dashboard.continue.title", "Continue Learning")}>
        <ContinueLearningCard dashboard={data} />
      </section>

      {/* Two-Column Grid: 2. Reviews Due & 3. Skill Progress */}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        <section aria-label={t("dashboard.reviews.title", "Reviews Due")}>
          <ReviewsDueCard dueCount={data.due_reviews_count} />
        </section>

        <section aria-label={t("dashboard.skills.title", "Skill Progress")}>
          <SkillProgressCard skillMastery={data.skill_mastery} />
        </section>
      </div>
    </div>
  );
}

export default DashboardPage;
