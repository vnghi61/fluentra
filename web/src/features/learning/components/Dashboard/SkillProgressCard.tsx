import React from "react";
import { BarChart3 } from "lucide-react";
import { useTranslation } from "react-i18next";

import { Badge } from "@/components/ui/badge";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Progress } from "@/components/ui/progress";
import type { SkillMastery } from "../../api/learningApi";

export interface SkillProgressCardProps {
  skillMastery: SkillMastery[];
}

export const SkillProgressCard: React.FC<SkillProgressCardProps> = ({
  skillMastery,
}) => {
  const { t } = useTranslation();

  return (
    <Card className="h-full flex flex-col">
      <CardHeader>
        <div className="flex items-center gap-2 text-primary-accent mb-1">
          <BarChart3 className="h-5 w-5" aria-hidden="true" />
          <span className="text-xs font-semibold uppercase tracking-wider">
            {t("dashboard.skills.title", "Skill Progress")}
          </span>
        </div>
        <CardTitle className="text-lg font-bold">
          {t("dashboard.skills.heading", "CEFR Competency Breakdown")}
        </CardTitle>
        <CardDescription>
          {t(
            "dashboard.skills.description",
            "Continuous proficiency and confidence scores updated from your exercises.",
          )}
        </CardDescription>
      </CardHeader>

      <CardContent className="flex-1">
        {skillMastery.length === 0 ? (
          <div className="flex flex-col items-center justify-center py-6 text-center text-text-muted">
            <p className="text-sm">
              {t(
                "dashboard.skills.empty",
                "No skill data yet. Complete lessons and exercises to build your mastery profile.",
              )}
            </p>
          </div>
        ) : (
          <div className="space-y-4">
            {skillMastery.map((item) => {
              const confidencePercent = Math.round(item.confidence * 100);
              const skillLabel = t(
                `dashboard.skills.${item.skill}`,
                item.skill.charAt(0).toUpperCase() + item.skill.slice(1),
              );

              return (
                <div key={item.skill} className="space-y-1.5">
                  <div className="flex items-center justify-between text-sm">
                    <div className="flex items-center gap-2">
                      <span className="font-medium text-text">
                        {skillLabel}
                      </span>
                      <Badge
                        variant="secondary"
                        className="font-mono text-xs px-1.5 py-0"
                      >
                        {item.level}
                      </Badge>
                    </div>
                    <span className="text-xs font-medium text-text-muted">
                      {confidencePercent}%
                    </span>
                  </div>
                  <Progress
                    value={confidencePercent}
                    aria-label={`${skillLabel} progress`}
                  />
                </div>
              );
            })}
          </div>
        )}
      </CardContent>
    </Card>
  );
};
