import React, { useEffect } from "react";
import { useTranslation } from "react-i18next";
import { useNavigate } from "@tanstack/react-router";

import { useLearningProfile } from "@/features/account";
import {
  TodayProgressCard,
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
import { cn } from "@/lib/utils";

function getGreetingKey(hour: number): "morning" | "afternoon" | "evening" {
  if (hour >= 5 && hour <= 11) return "morning";
  if (hour >= 12 && hour <= 17) return "afternoon";
  return "evening";
}

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

  const currentHour = new Date().getHours();
  const greetingKey = getGreetingKey(currentHour);
  const greeting = t(`dashboard.greeting.${greetingKey}`, {
    defaultValue:
      greetingKey === "morning"
        ? "Good morning! 👋"
        : greetingKey === "afternoon"
          ? "Good afternoon! 👋"
          : "Good evening! 👋",
  });

  const xpRemaining =
    gamificationData &&
    gamificationData.daily_goal_xp > 0 &&
    gamificationData.xp_today < gamificationData.daily_goal_xp
      ? gamificationData.daily_goal_xp - gamificationData.xp_today
      : null;

  const subtitle =
    xpRemaining !== null
      ? t("dashboard.subtitleXpLeft", {
          xp: xpRemaining,
          defaultValue: `${xpRemaining} XP to reach today's goal`,
        })
      : t("dashboard.tagline", "Here is your learning summary for today.");

  return (
    <div className="mx-auto w-full max-w-[1240px] space-y-3 md:space-y-4 lg:space-y-5">
      {/* Dashboard Top Heading */}
      <header className="space-y-1">
        <h1 className="text-2xl md:text-3xl font-bold text-text tracking-tight">
          {greeting}
        </h1>
        <p className="text-sm text-text-muted">{subtitle}</p>
      </header>

      {/* Row 1: ContinueLearning (2fr) | TodayProgressCard (1fr) when gamificationData exists */}
      <div
        className={cn(
          "grid grid-cols-1 gap-3 md:gap-4 lg:gap-5",
          gamificationData && "lg:grid-cols-[2fr_1fr]",
        )}
      >
        <section
          aria-label={t("dashboard.continue.title", "Continue Learning")}
          className="min-w-0"
        >
          <ContinueLearningCard dashboard={data} />
        </section>

        {gamificationData && (
          <section
            aria-label={t(
              "gamification.sectionAria",
              "Learning Motivation & Progress",
            )}
            className="min-w-0"
          >
            <TodayProgressCard summary={gamificationData} />
          </section>
        )}
      </div>

      {/* Placement Test Invitation Card (When eligible and unplaced, spans full width above row 2) */}
      <PlacementInviteCard />

      {/* Row 2, Row 3, Row 4 in a responsive grid */}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-3 md:gap-4 lg:gap-5">
        {/* Row 2: DailyPractice | Reviews */}
        <section
          aria-label={t("practice.daily.title", "Today's Practice Set")}
          className="order-1 lg:order-1 min-w-0"
        >
          <DailyPracticeCard />
        </section>

        <section
          aria-label={t("dashboard.reviews.title", "Reviews Due")}
          className="order-2 lg:order-2 min-w-0"
        >
          <ReviewsDueCard dueCount={data.due_reviews_count} />
        </section>

        {/* Row 3: FoundationNext | SkillProgress */}
        <div className="order-3 lg:order-3 min-w-0">
          <FoundationNextCard />
        </div>

        <section
          aria-label={t("dashboard.skills.title", "Skill Progress")}
          className="order-5 lg:order-4 min-w-0"
        >
          <SkillProgressCard skillMastery={data.skill_mastery} />
        </section>

        {/* Row 4: WeeklyPlan full width (lg:col-span-2) */}
        <aside
          aria-label={t("weeklyPlan.title", "This week's plan")}
          className="order-4 lg:order-5 lg:col-span-2 min-w-0"
        >
          <WeeklyPlanCard />
        </aside>
      </div>
    </div>
  );
}

export default DashboardPage;
