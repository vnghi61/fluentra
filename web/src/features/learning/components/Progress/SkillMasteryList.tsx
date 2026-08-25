import React from "react";
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

export interface SkillMasteryListProps {
  skills: SkillMastery[];
}

const ALL_SKILLS = [
  "vocabulary",
  "grammar",
  "reading",
  "listening",
  "speaking",
  "writing",
] as const;

export const SkillMasteryList: React.FC<SkillMasteryListProps> = ({ skills }) => {
  const { t } = useTranslation();

  const skillMap = new Map<string, SkillMastery>();
  skills.forEach((s) => skillMap.set(s.skill, s));

  return (
    <Card className="border-border-subtle bg-surface-card">
      <CardHeader>
        <CardTitle className="text-xl font-bold">
          {t("progress.skillsTitle", "Skill Mastery (CEFR)")}
        </CardTitle>
        <CardDescription>
          {t(
            "dashboard.skills.description",
            "Continuous proficiency and confidence scores updated from your exercises.",
          )}
        </CardDescription>
      </CardHeader>

      <CardContent className="space-y-5">
        {ALL_SKILLS.map((skillName) => {
          const mastery = skillMap.get(skillName);
          const skillLabel = t(
            `dashboard.skills.${skillName}`,
            skillName.charAt(0).toUpperCase() + skillName.slice(1),
          );

          if (!mastery) {
            return (
              <div key={skillName} className="space-y-1.5 opacity-60">
                <div className="flex items-center justify-between text-sm">
                  <span className="font-medium text-text">{skillLabel}</span>
                  <Badge variant="secondary" className="text-xs px-2 py-0">
                    {t("progress.notStarted", "Not started yet")}
                  </Badge>
                </div>
                <Progress value={0} aria-label={`${skillLabel} progress`} />
              </div>
            );
          }

          const confidencePercent = Math.round(mastery.confidence * 100);

          return (
            <div key={skillName} className="space-y-1.5">
              <div className="flex items-center justify-between text-sm">
                <div className="flex items-center gap-2">
                  <span className="font-medium text-text">{skillLabel}</span>
                  <Badge variant="primary" className="font-mono text-xs px-1.5 py-0">
                    {mastery.level}
                  </Badge>
                </div>
                <span className="text-xs font-semibold text-text-muted">
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
      </CardContent>
    </Card>
  );
};
