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
import { cn } from "@/lib/utils";
import type { SkillMastery } from "../../api/learningApi";
import { ProgressBar } from "./ProgressBar";
import { SKILL_ORDER, SKILL_TONE } from "./skillTone";

export interface SkillMasteryListProps {
  skills: SkillMastery[];
}

export const SkillMasteryList: React.FC<SkillMasteryListProps> = ({
  skills,
}) => {
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
            "progress.skills.desc",
            "CEFR level and confidence, updated from your exercises.",
          )}
        </CardDescription>
      </CardHeader>

      <CardContent className="space-y-1">
        {SKILL_ORDER.map((skill) => {
          const tone = SKILL_TONE[skill];
          const Icon = tone.icon;
          const mastery = skillMap.get(skill);
          const skillLabel = t(
            `dashboard.skills.${skill}`,
            skill.charAt(0).toUpperCase() + skill.slice(1),
          );
          const confidencePercent = mastery
            ? Math.round(mastery.confidence * 100)
            : 0;

          return (
            <div key={skill} className="flex items-center gap-3 py-2">
              <div
                className={cn(
                  "h-9 w-9 rounded-xl flex items-center justify-center shrink-0",
                  tone.chip,
                )}
              >
                <Icon className="h-4 w-4" aria-hidden="true" />
              </div>

              <div className="min-w-0 flex-1 flex items-center gap-2">
                <span className="font-medium text-text text-sm truncate">
                  {skillLabel}
                </span>
                {mastery && (
                  <Badge
                    variant="secondary"
                    className="font-mono text-xs px-1.5 py-0 shrink-0"
                  >
                    {mastery.level}
                  </Badge>
                )}
              </div>

              <ProgressBar
                value={confidencePercent}
                label={t("progress.skills.progressLabel", {
                  skill: skillLabel,
                })}
                barClassName={tone.bar}
                className="w-24 sm:w-32 shrink-0"
              />

              <span className="w-10 text-right text-sm font-semibold tabular-nums text-text-muted">
                {confidencePercent}%
              </span>
            </div>
          );
        })}
      </CardContent>
    </Card>
  );
};
