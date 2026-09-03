import React from "react";
import { Crown, Trophy, UserCheck, Users } from "lucide-react";
import { useTranslation } from "react-i18next";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { useLeaderboard, useSetLeaderboardOptIn } from "../api/gamificationApi";

export interface LeaderboardWidgetProps {
  currentLeague?: string;
}

const LEAGUE_COLORS: Record<
  string,
  { bg: string; text: string; border: string }
> = {
  bronze: {
    bg: "bg-amber-700/10",
    text: "text-amber-700 dark:text-amber-400",
    border: "border-amber-700/30",
  },
  silver: {
    bg: "bg-slate-400/10",
    text: "text-slate-600 dark:text-slate-300",
    border: "border-slate-400/30",
  },
  gold: {
    bg: "bg-amber-400/10",
    text: "text-amber-500 dark:text-amber-300",
    border: "border-amber-400/30",
  },
  diamond: {
    bg: "bg-cyan-500/10",
    text: "text-cyan-500 dark:text-cyan-300",
    border: "border-cyan-500/30",
  },
};

const defaultLeagueStyle = {
  bg: "bg-amber-700/10",
  text: "text-amber-700 dark:text-amber-400",
  border: "border-amber-700/30",
};

export const LeaderboardWidget: React.FC<LeaderboardWidgetProps> = ({
  currentLeague = "bronze",
}) => {
  const { t } = useTranslation();
  const { data, isLoading, error } = useLeaderboard();
  const optInMutation = useSetLeaderboardOptIn();

  const isNotOptedIn =
    typeof error === "object" &&
    error !== null &&
    "problem" in error &&
    (error as { problem: { status: number } }).problem.status === 403;

  const leagueStyle =
    (currentLeague ? LEAGUE_COLORS[currentLeague.toLowerCase()] : null) ??
    defaultLeagueStyle;

  if (isNotOptedIn) {
    return (
      <Card className="border-border/60 bg-gradient-to-br from-surface-card to-surface/40 shadow-sm">
        <CardHeader className="pb-3">
          <div className="flex items-center gap-2.5">
            <div className="flex h-9 w-9 items-center justify-center rounded-xl bg-primary/10 text-primary shadow-inner">
              <Trophy className="h-5 w-5" aria-hidden="true" />
            </div>
            <div>
              <CardTitle className="text-lg font-bold text-text">
                {t("gamification.leaderboardTitle", "Weekly League")}
              </CardTitle>
              <CardDescription>
                {t(
                  "gamification.optInDesc",
                  "Compete with learners at your skill level and climb the ranks.",
                )}
              </CardDescription>
            </div>
          </div>
        </CardHeader>
        <CardContent className="pt-0">
          <div className="rounded-xl border border-dashed border-border/80 bg-surface/50 p-4 text-center">
            <Users
              className="mx-auto mb-2 h-7 w-7 text-primary/60"
              aria-hidden="true"
            />
            <p className="mb-3 text-xs text-text-muted">
              {t(
                "gamification.optInNotice",
                "Leagues are opt-in. Only your display name and weekly XP are shown to peers.",
              )}
            </p>
            <Button
              size="sm"
              className="gap-2 font-semibold"
              onClick={() => optInMutation.mutate(true)}
              disabled={optInMutation.isPending}
            >
              <UserCheck className="h-4 w-4" aria-hidden="true" />
              {t("gamification.joinLeague", "Join {{league}} League", {
                league: currentLeague.toUpperCase(),
              })}
            </Button>
          </div>
        </CardContent>
      </Card>
    );
  }

  return (
    <Card className="border-border/60 bg-gradient-to-br from-surface-card to-surface/40 shadow-sm">
      <CardHeader className="pb-3">
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-2.5">
            <div className="flex h-9 w-9 items-center justify-center rounded-xl bg-amber-500/10 text-amber-500 shadow-inner">
              <Trophy
                className="h-5 w-5 fill-amber-500/20"
                aria-hidden="true"
              />
            </div>
            <div>
              <CardTitle className="text-lg font-bold text-text">
                {t("gamification.leaderboardTitle", "Weekly League")}
              </CardTitle>
              <p className="text-xs text-text-muted">
                {t(
                  "gamification.standingsSubtitle",
                  "Weekly standings snapshot",
                )}
              </p>
            </div>
          </div>

          <Badge
            variant="outline"
            className={`capitalize ${leagueStyle.bg} ${leagueStyle.text} ${leagueStyle.border}`}
          >
            <Crown className="mr-1 h-3 w-3" aria-hidden="true" />
            {currentLeague}
          </Badge>
        </div>
      </CardHeader>

      <CardContent className="space-y-2 pt-0">
        {isLoading ? (
          <div className="space-y-2">
            <Skeleton className="h-8 w-full" />
            <Skeleton className="h-8 w-full" />
            <Skeleton className="h-8 w-full" />
          </div>
        ) : !data || data.entries.length === 0 ? (
          <p className="py-4 text-center text-xs text-text-muted">
            {t(
              "gamification.noEntries",
              "No standings available yet for this week.",
            )}
          </p>
        ) : (
          <div className="divide-y divide-border/40 rounded-xl border border-border/60 bg-surface/40">
            {data.entries.slice(0, 5).map((entry) => (
              <div
                key={entry.user_id}
                className={`flex items-center justify-between px-3 py-2 text-xs transition-colors ${
                  entry.is_self
                    ? "bg-primary/10 font-semibold text-primary"
                    : "text-text hover:bg-surface/80"
                }`}
              >
                <div className="flex items-center gap-2.5">
                  <span
                    className={`flex h-5 w-5 items-center justify-center rounded-full text-[11px] font-bold ${
                      entry.rank === 1
                        ? "bg-amber-400 text-amber-950"
                        : entry.rank === 2
                          ? "bg-slate-300 text-slate-900"
                          : entry.rank === 3
                            ? "bg-amber-700 text-amber-100"
                            : "text-text-muted"
                    }`}
                  >
                    {entry.rank}
                  </span>
                  {entry.avatar_url ? (
                    <img
                      src={`${entry.avatar_url}${entry.avatar_url.includes("?") ? "&" : "?"}size=sm`}
                      alt={entry.display_name}
                      className="h-6 w-6 rounded-full object-cover border border-border/50"
                    />
                  ) : (
                    <div className="flex h-6 w-6 items-center justify-center rounded-full bg-primary/10 text-[10px] font-bold text-primary">
                      {entry.display_name ? entry.display_name.charAt(0).toUpperCase() : "?"}
                    </div>
                  )}
                  <span className="truncate max-w-[120px] sm:max-w-[180px]">
                    {entry.display_name}
                    {entry.is_self && (
                      <span className="ml-1 text-[10px] text-text-muted">
                        ({t("gamification.you", "You")})
                      </span>
                    )}
                  </span>
                </div>
                <span className="font-semibold tabular-nums text-text">
                  {entry.xp} XP
                </span>
              </div>
            ))}
          </div>
        )}
      </CardContent>
    </Card>
  );
};
