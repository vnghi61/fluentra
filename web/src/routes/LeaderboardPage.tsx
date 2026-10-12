import React from "react";
import { useTranslation } from "react-i18next";

import { PageHeader } from "@/components/layout/PageHeader";
import {
  LeaderboardWidget,
  useGamificationSummary,
} from "@/features/gamification";

export function LeaderboardPage(): React.JSX.Element {
  const { t } = useTranslation();
  const { data: summary } = useGamificationSummary();

  return (
    <div className="mx-auto w-full max-w-3xl space-y-4 md:space-y-5">
      <PageHeader
        title={t("gamification.leaderboardPageTitle", "Leaderboard")}
        description={t(
          "gamification.leaderboardPageDescription",
          "Compare weekly learning XP in your league. Joining is optional; you can view the standings without participating.",
        )}
      />
      <LeaderboardWidget currentLeague={summary?.league ?? undefined} />
    </div>
  );
}
