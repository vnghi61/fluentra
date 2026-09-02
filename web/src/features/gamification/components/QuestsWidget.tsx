import React from "react";
import { Compass, Gift } from "lucide-react";
import { useTranslation } from "react-i18next";

import { Badge } from "@/components/ui/badge";
import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Progress } from "@/components/ui/progress";
import type { Quest } from "../api/gamificationApi";

export interface QuestsWidgetProps {
  quests?: Quest[];
}

export const QuestsWidget: React.FC<QuestsWidgetProps> = ({ quests = [] }) => {
  const { t } = useTranslation();

  return (
    <Card className="border-border/60 bg-gradient-to-br from-surface-card to-surface/40 shadow-sm">
      <CardHeader className="pb-3">
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-2.5">
            <div className="flex h-9 w-9 items-center justify-center rounded-xl bg-purple-500/10 text-purple-500 shadow-inner">
              <Compass className="h-5 w-5" aria-hidden="true" />
            </div>
            <div>
              <CardTitle className="text-lg font-bold text-text">
                {t("gamification.questsTitle", "Active Quests")}
              </CardTitle>
              <p className="text-xs text-text-muted">
                {t("gamification.questsSubtitle", "Complete tasks to earn bonus XP")}
              </p>
            </div>
          </div>

          <Badge variant="secondary" className="px-2 py-0.5 text-xs font-semibold">
            {t("gamification.activeQuestsCount", "{{count}} active", {
              count: quests.length,
            })}
          </Badge>
        </div>
      </CardHeader>

      <CardContent className="space-y-3 pt-0">
        {quests.length === 0 ? (
          <p className="py-3 text-center text-xs text-text-muted">
            {t("gamification.noQuests", "No active quests right now. Check back tomorrow!")}
          </p>
        ) : (
          <div className="space-y-2.5">
            {quests.map((quest) => {
              // Quest progress calculation from steps and progress objects
              let totalTarget = 0;
              let currentProgress = 0;

              if (quest.steps && typeof quest.steps === "object") {
                for (const [key, target] of Object.entries(quest.steps)) {
                  if (typeof target === "number") {
                    totalTarget += target;
                    const prog = (quest.progress as Record<string, number> | undefined)?.[key] ?? 0;
                    currentProgress += Math.min(target, prog);
                  }
                }
              }

              const percentage =
                totalTarget > 0
                  ? Math.min(100, Math.round((currentProgress / totalTarget) * 100))
                  : 0;

              return (
                <div
                  key={quest.code}
                  className="rounded-xl border border-border/60 bg-surface/50 p-3 space-y-2"
                >
                  <div className="flex items-start justify-between gap-2">
                    <div>
                      <h4 className="text-xs font-bold text-text">{quest.name}</h4>
                      <p className="text-[11px] text-text-muted line-clamp-1">
                        {quest.description}
                      </p>
                    </div>

                    <div className="flex items-center gap-1 text-xs font-bold text-purple-600 dark:text-purple-400 shrink-0">
                      <Gift className="h-3.5 w-3.5" aria-hidden="true" />
                      <span>+{quest.reward_xp} XP</span>
                    </div>
                  </div>

                  <div className="space-y-1">
                    <div className="flex justify-between text-[10px] font-medium text-text-muted">
                      <span>
                        {currentProgress} / {totalTarget}
                      </span>
                      <span>{percentage}%</span>
                    </div>
                    <Progress
                      value={percentage}
                      className="h-1.5 bg-surface-raised"
                      aria-label={quest.name}
                    />
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
