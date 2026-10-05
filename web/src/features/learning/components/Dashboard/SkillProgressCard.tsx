import React, { useState } from "react";
import { Link } from "@tanstack/react-router";
import { BarChart3, ChevronDown } from "lucide-react";
import { useTranslation } from "react-i18next";

import { Badge } from "@/components/ui/badge";
import { Card } from "@/components/ui/card";
import { Progress } from "@/components/ui/progress";
import { cn } from "@/lib/utils";
import type { SkillMastery } from "../../api/learningApi";

export interface SkillProgressCardProps {
  skillMastery: SkillMastery[];
}

export const SkillProgressCard: React.FC<SkillProgressCardProps> = ({
  skillMastery,
}) => {
  const { t } = useTranslation();
  const [expanded, setExpanded] = useState(false);

  return (
    <Card className="bg-surface-card border border-border-subtle rounded-2xl shadow-[0_2px_8px_rgba(15,23,42,0.04)] p-4 md:p-5">
      <div className="space-y-4">
        {/* Header with Title and Details Link */}
        <div className="flex items-start justify-between gap-3">
          <div className="space-y-1 min-w-0">
            <div className="flex items-center gap-2 text-primary-accent">
              <BarChart3 className="h-4 w-4" aria-hidden="true" />
              <span className="text-xs font-semibold uppercase tracking-wider text-text-muted">
                {t("dashboard.skills.title", "Skill Progress")}
              </span>
            </div>
            <h3 className="text-base md:text-lg font-bold text-text">
              {t("dashboard.skills.heading", "CEFR Competency Breakdown")}
            </h3>
            {skillMastery.length > 0 && (
              <p className="text-xs md:text-sm text-text-muted line-clamp-2">
                {t(
                  "dashboard.skills.description",
                  "Continuous proficiency and confidence scores updated from your exercises.",
                )}
              </p>
            )}
          </div>
          <Link
            to="/progress"
            className="shrink-0 text-xs md:text-sm font-medium text-primary-accent hover:underline inline-flex items-center min-h-[44px] min-w-[44px] px-1"
          >
            {t("dashboard.skills.detailsLink", "View details →")}
          </Link>
        </div>

        {/* Content */}
        {skillMastery.length === 0 ? (
          <div className="space-y-3 text-text-muted pt-1">
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
          <div className="space-y-3">
            {/* Mobile accordion toggle button */}
            <button
              type="button"
              aria-expanded={expanded}
              onClick={() => setExpanded((prev) => !prev)}
              className="flex md:hidden w-full items-center justify-between min-h-[44px] py-2.5 px-1 text-xs font-medium text-text-muted hover:text-text border-t border-border-subtle"
            >
              <span>
                {expanded
                  ? t("dashboard.skills.hideToggle", "Hide skills")
                  : t("dashboard.skills.showToggle", "Show skills")}
              </span>
              <ChevronDown
                className={cn(
                  "h-4 w-4 transition-transform duration-200 motion-reduce:transition-none",
                  expanded && "rotate-180",
                )}
                aria-hidden="true"
              />
            </button>

            {/* Skill list: hidden on mobile if !expanded, always visible on md+ */}
            <div className={cn("space-y-3.5", !expanded && "hidden md:block")}>
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
                        <span className="font-medium text-text text-sm">
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
                      className="h-2"
                    />
                  </div>
                );
              })}
            </div>
          </div>
        )}
      </div>
    </Card>
  );
};
