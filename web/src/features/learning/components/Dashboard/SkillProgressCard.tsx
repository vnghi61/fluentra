import React from "react";
import { Link } from "@tanstack/react-router";
import { useTranslation } from "react-i18next";

import { Card } from "@/components/ui/card";
import type { SkillMastery } from "../../api/learningApi";
import { SkillBarList } from "../Progress/SkillBarList";
import { CardHeading, cardLinkClass } from "./CardHeading";

export interface SkillProgressCardProps {
  skillMastery: SkillMastery[];
}

/** Dashboard: the CEFR estimate per skill, one compact row each. */
export const SkillProgressCard: React.FC<SkillProgressCardProps> = ({
  skillMastery,
}) => {
  const { t } = useTranslation();

  return (
    <Card className="h-full p-4 md:p-5">
      <CardHeading
        title={t("overview.dashboard.skillsTitle")}
        description={
          skillMastery.length > 0
            ? t(
                "dashboard.skills.description",
                "Continuous proficiency and confidence scores updated from your exercises.",
              )
            : undefined
        }
        action={
          <Link to="/progress" className={cardLinkClass}>
            {t("dashboard.skills.detailsLink", "View details →")}
          </Link>
        }
      />

      {skillMastery.length === 0 ? (
        <div className="space-y-2 pt-3 text-text-muted">
          <p className="text-sm">
            {t(
              "dashboard.skills.empty",
              "No skill data yet. Complete lessons and exercises to build your mastery profile.",
            )}
          </p>
          <Link
            to="/learn"
            className="inline-flex min-h-[44px] items-center text-sm font-medium text-primary-accent hover:underline"
          >
            {t("dashboard.continue.exploreBtn", "Explore Syllabus")}
          </Link>
        </div>
      ) : (
        <SkillBarList
          skills={skillMastery}
          label={t("overview.dashboard.skillsTitle")}
          className="pt-3"
        />
      )}
    </Card>
  );
};
