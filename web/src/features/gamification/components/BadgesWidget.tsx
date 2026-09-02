import React from "react";
import { Award, Lock, Medal } from "lucide-react";
import { useTranslation } from "react-i18next";

import { Badge as UIBadge } from "@/components/ui/badge";
import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import type { Badge } from "../api/gamificationApi";

export interface BadgesWidgetProps {
  badges?: Badge[];
}

const TIER_COLORS: Record<string, { bg: string; text: string; border: string }> = {
  bronze: { bg: "bg-amber-700/10", text: "text-amber-700 dark:text-amber-400", border: "border-amber-700/30" },
  silver: { bg: "bg-slate-400/10", text: "text-slate-600 dark:text-slate-300", border: "border-slate-400/30" },
  gold: { bg: "bg-amber-400/10", text: "text-amber-500 dark:text-amber-300", border: "border-amber-400/30" },
  platinum: { bg: "bg-indigo-500/10", text: "text-indigo-500 dark:text-indigo-300", border: "border-indigo-500/30" },
};

const defaultTierStyle = {
  bg: "bg-amber-700/10",
  text: "text-amber-700 dark:text-amber-400",
  border: "border-amber-700/30",
};

export const BadgesWidget: React.FC<BadgesWidgetProps> = ({ badges = [] }) => {
  const { t } = useTranslation();

  return (
    <Card className="border-border/60 bg-gradient-to-br from-surface-card to-surface/40 shadow-sm">
      <CardHeader className="pb-3">
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-2.5">
            <div className="flex h-9 w-9 items-center justify-center rounded-xl bg-amber-500/10 text-amber-500 shadow-inner">
              <Award className="h-5 w-5" aria-hidden="true" />
            </div>
            <div>
              <CardTitle className="text-lg font-bold text-text">
                {t("gamification.badgesTitle", "Badges & Achievements")}
              </CardTitle>
              <p className="text-xs text-text-muted">
                {t("gamification.badgesSubtitle", "Milestones you have unlocked")}
              </p>
            </div>
          </div>

          <UIBadge variant="secondary" className="px-2 py-0.5 text-xs font-semibold">
            {t("gamification.badgesCount", "{{count}} unlocked", {
              count: badges.length,
            })}
          </UIBadge>
        </div>
      </CardHeader>

      <CardContent className="space-y-3 pt-0">
        {badges.length === 0 ? (
          <div className="rounded-xl border border-dashed border-border/80 bg-surface/40 p-4 text-center">
            <Lock className="mx-auto mb-2 h-6 w-6 text-text-muted" aria-hidden="true" />
            <p className="text-xs text-text-muted">
              {t(
                "gamification.noBadges",
                "No badges unlocked yet. Complete lessons, keep streaks and level up to earn them!",
              )}
            </p>
          </div>
        ) : (
          <div className="grid grid-cols-1 sm:grid-cols-2 gap-2">
            {badges.map((badge) => {
              const tierStyle =
                (badge.tier ? TIER_COLORS[badge.tier.toLowerCase()] : null) ??
                defaultTierStyle;

              return (
                <div
                  key={badge.code}
                  className={`flex items-start gap-2.5 rounded-xl border p-2.5 ${tierStyle.border} ${tierStyle.bg}`}
                >
                  <div className="mt-0.5 flex h-7 w-7 shrink-0 items-center justify-center rounded-lg bg-surface shadow-xs">
                    <Medal className={`h-4 w-4 ${tierStyle.text}`} aria-hidden="true" />
                  </div>
                  <div className="min-w-0 flex-1">
                    <div className="flex items-center justify-between gap-1">
                      <h4 className="text-xs font-bold text-text truncate">{badge.name}</h4>
                      <span className={`text-[10px] font-bold uppercase tracking-wider ${tierStyle.text}`}>
                        {badge.tier}
                      </span>
                    </div>
                    <p className="text-[11px] text-text-muted line-clamp-1">{badge.description}</p>
                  </div>
                </div>
              );
            })}
          </div>
        )}
      </CardContent>
    </Card>
  );
};
