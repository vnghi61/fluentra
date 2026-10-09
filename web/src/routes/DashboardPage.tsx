import React, { useEffect } from "react";
import { useTranslation } from "react-i18next";
import { useNavigate } from "@tanstack/react-router";

import { PageHeader } from "@/components/layout/PageHeader";
// The API module, not the barrel: the barrel re-exports the settings forms and
// their zod schemas, which would ride along into the Dashboard chunk.
import { useLearningProfile } from "@/features/account/api/accountApi";
import { DashboardBanners, useActiveBanners } from "@/features/banners";
import {
  DailyGoalTiles,
  TodayProgressCard,
  useGamificationSummary,
} from "@/features/gamification";
import {
  ContinueLearningCard,
  DailyPracticeCard,
  DashboardError,
  DashboardSkeleton,
  greetingKeyFor,
  PlacementInviteCard,
  ReviewPathCard,
  SkillProgressCard,
  useDashboard,
  WeeklyPlanCard,
} from "@/features/learning";
import { cn } from "@/lib/utils";

const greetingFallback = {
  morning: "Good morning! 👋",
  afternoon: "Good afternoon! 👋",
  evening: "Good evening! 👋",
} as const;

export function DashboardPage(): React.JSX.Element {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const { data, isLoading, isError, error, refetch } = useDashboard();
  const { data: gamificationData } = useGamificationSummary();
  const { data: profile, isLoading: isProfileLoading } = useLearningProfile();
  const banners = useActiveBanners();
  const hasSlides = (banners.data?.items ?? []).some(
    (banner) => banner.kind === "slide",
  );
  // With promo slides the carousel is the page's hero, as in the dashboard
  // design; without them (or while they load, or if they fail) the continue
  // and today's-progress row takes that place so the page always opens on an
  // action. Waiting for the list avoids showing the row and then swapping it.
  const showStarterRow = !banners.isPending && !hasSlides;

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

  const greetingKey = greetingKeyFor(new Date().getHours());
  const greeting = t(`dashboard.greeting.${greetingKey}`, {
    defaultValue: greetingFallback[greetingKey],
  });

  const xpRemaining =
    gamificationData &&
    gamificationData.daily_goal_xp > 0 &&
    gamificationData.xp_today < gamificationData.daily_goal_xp
      ? gamificationData.daily_goal_xp - gamificationData.xp_today
      : null;

  const subtitle =
    xpRemaining !== null
      ? t("overview.dashboard.subtitleXpLeft", { xp: xpRemaining })
      : t("dashboard.tagline", "Here is your learning summary for today.");

  return (
    <div className="mx-auto w-full space-y-4 lg:space-y-5">
      {/* An announcement opens as a dialog; it has no place in the layout. */}
      <DashboardBanners part="announcements" />

      {/* Side by side only from xl: below that the sidebar leaves the content
        column too narrow for the greeting and both tiles, and the tiles sit
        under the greeting at full width instead. */}
      <div className="flex flex-col gap-4 xl:flex-row xl:items-end xl:justify-between">
        <PageHeader title={greeting} description={subtitle} />
        {/* On a phone these are left to "Today's progress" below, unless the
          carousel has taken that row's place. */}
        {gamificationData && (
          <DailyGoalTiles
            summary={gamificationData}
            className={cn(
              "xl:w-[420px] xl:shrink-0",
              hasSlides ? "grid" : "hidden md:grid",
            )}
          />
        )}
      </div>

      {/* Promo slides from the admin screen: the hero when there are any. */}
      <DashboardBanners part="slides" />

      {/* Row 1: continue (2fr) | today's progress (1fr) */}
      {showStarterRow && (
        <div
          className={cn(
            "grid grid-cols-1 gap-4 lg:gap-5",
            gamificationData && "lg:grid-cols-[minmax(0,2fr)_minmax(0,1fr)]",
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
      )}

      {/* Placement invitation: only when eligible and unplaced. */}
      <PlacementInviteCard />

      {/* Row 2: today's practice set | reviews & learning path */}
      <div className="grid grid-cols-1 gap-4 lg:grid-cols-2 lg:gap-5">
        <section
          aria-label={t("practice.daily.title", "Today's Practice Set")}
          className="min-w-0"
        >
          <DailyPracticeCard />
        </section>

        <section
          aria-label={t("overview.dashboard.reviewPathTitle")}
          className="min-w-0"
        >
          <ReviewPathCard dueCount={data.due_reviews_count} />
        </section>
      </div>

      {/* Row 3: this week's plan | CEFR skills */}
      <div className="grid grid-cols-1 gap-4 lg:gap-5 xl:grid-cols-[minmax(0,1.35fr)_minmax(0,1fr)]">
        <aside
          aria-label={t("weeklyPlan.title", "This week's plan")}
          className="min-w-0"
        >
          <WeeklyPlanCard />
        </aside>

        <section
          aria-label={t("dashboard.skills.title", "Skill Progress")}
          className="min-w-0"
        >
          <SkillProgressCard skillMastery={data.skill_mastery} />
        </section>
      </div>
    </div>
  );
}

export default DashboardPage;
