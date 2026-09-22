import React, { useEffect } from "react";
import { useTranslation } from "react-i18next";
import { useNavigate } from "@tanstack/react-router";

import { useLearningProfile } from "@/features/account";
import {
  GamificationSummarySection,
  useGamificationSummary,
} from "@/features/gamification";
import {
  ContinueLearningCard,
  DailyPracticeCard,
  DashboardError,
  DashboardSkeleton,
  FoundationNextCard,
  PlacementInviteCard,
  ReviewsDueCard,
  SkillProgressCard,
  useDashboard,
  WeeklyPlanCard,
} from "@/features/learning";

export function DashboardPage(): React.JSX.Element {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const { data, isLoading, isError, error, refetch } = useDashboard();
  const { data: gamificationData } = useGamificationSummary();
  const { data: profile, isLoading: isProfileLoading } = useLearningProfile();

  useEffect(() => {
    // If user has no learning profile yet, redirect to onboarding wizard (WO13 §8)
    if (!isProfileLoading && profile === null) {
      void navigate({ to: "/welcome" });
    }
  }, [isProfileLoading, profile, navigate]);

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

      {/* Placement Test Invitation Card (When eligible and unplaced) */}
      <section aria-label={t("placement.invite.title")}>
        <PlacementInviteCard />
      </section>

      {/* Today's Practice (Daily Set) */}
      <section aria-label={t("practice.daily.title", "Today's Practice Set")}>
        <DailyPracticeCard />
      </section>

      {/* Hero Card: 1. Continue Learning */}
      <section aria-label={t("dashboard.continue.title", "Continue Learning")}>
        <ContinueLearningCard dashboard={data} />
      </section>

      {/* The Foundation path's next node, when there are published topics. */}
      <section aria-label={t("foundation.next.label", "Foundation path")}>
        <FoundationNextCard />
      </section>

      {/* Personalized Weekly Plan */}
      <section aria-label={t("weeklyPlan.title")}>
        <WeeklyPlanCard />
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
