import React from "react";

import type { GamificationSummary } from "../api/gamificationApi";
import { LeaderboardWidget } from "./LeaderboardWidget";
import { QuestsWidget } from "./QuestsWidget";
import { StreakWidget } from "./StreakWidget";
import { XPProgressBar } from "./XPProgressBar";

export interface GamificationSummarySectionProps {
  summary: GamificationSummary;
}

export const GamificationSummarySection: React.FC<
  GamificationSummarySectionProps
> = ({ summary }) => {
  return (
    <div className="space-y-6">
      {/* Top Grid: XP/Level Progress + Streak Tracker */}
      <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
        <XPProgressBar summary={summary} />
        <StreakWidget streak={summary.streak} />
      </div>

      {/* Secondary Grid: Quests + Weekly Leaderboard */}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        <QuestsWidget quests={summary.quests} />
        <LeaderboardWidget currentLeague={summary.league} />
      </div>
    </div>
  );
};
