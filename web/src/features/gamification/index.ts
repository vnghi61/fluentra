export {
  gamificationApi,
  useGamificationSummary,
  useStreak,
  useUseStreakFreeze,
  useLeaderboard,
  useSetLeaderboardOptIn,
  useSetDailyGoal,
} from "./api/gamificationApi";
export type {
  GamificationSummary,
  Streak,
  Badge,
  Quest,
  LeaderboardResponse,
  LeaderboardEntry,
  SetDailyGoalRequest,
  SetLeaderboardOptInRequest,
} from "./api/gamificationApi";

export { XPProgressBar } from "./components/XPProgressBar";
export type { XPProgressBarProps } from "./components/XPProgressBar";

export { StreakWidget } from "./components/StreakWidget";
export type { StreakWidgetProps } from "./components/StreakWidget";

export { LeaderboardWidget } from "./components/LeaderboardWidget";
export type { LeaderboardWidgetProps } from "./components/LeaderboardWidget";

export { BadgesWidget } from "./components/BadgesWidget";
export type { BadgesWidgetProps } from "./components/BadgesWidget";

export { QuestsWidget } from "./components/QuestsWidget";
export type { QuestsWidgetProps } from "./components/QuestsWidget";

export { GamificationSummarySection } from "./components/GamificationSummarySection";
export type { GamificationSummarySectionProps } from "./components/GamificationSummarySection";
