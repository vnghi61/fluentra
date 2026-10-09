import React from "react";
import { useTranslation } from "react-i18next";

import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
} from "@/components/ui/card";
import type { SkillMastery } from "../../api/learningApi";
import { SkillBarList } from "./SkillBarList";

export interface SkillMasteryListProps {
  skills: SkillMastery[];
}

/** Progress page: the six skills, one row each (shared SkillBarList). */
export const SkillMasteryList: React.FC<SkillMasteryListProps> = ({
  skills,
}) => {
  const { t } = useTranslation();

  return (
    <Card className="border-border-subtle bg-surface-card">
      <CardHeader className="p-5 pb-3 md:p-6 md:pb-3">
        <h2 className="text-lg font-semibold leading-snug text-text md:text-xl">
          {t("progress.skillsTitle", "Skill Mastery (CEFR)")}
        </h2>
        <CardDescription>
          {t(
            "progress.skills.desc",
            "CEFR level and confidence, updated from your exercises.",
          )}
        </CardDescription>
      </CardHeader>

      <CardContent className="px-5 pb-5 md:px-6 md:pb-6">
        <SkillBarList
          skills={skills}
          label={t("progress.skillsTitle", "Skill Mastery (CEFR)")}
        />
      </CardContent>
    </Card>
  );
};
