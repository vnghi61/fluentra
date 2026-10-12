import React from "react";
import { useTranslation } from "react-i18next";

import { cn } from "@/lib/utils";
import type { SkillMastery } from "../../api/learningApi";
import { ProgressBar } from "./ProgressBar";
import { SKILL_ORDER, SKILL_TONE } from "./skillTone";

export interface SkillBarListProps {
  skills: SkillMastery[];
  /** Accessible name of the list, e.g. the card's title. */
  label?: string;
  className?: string;
}

/**
 * The six skills in a fixed order, one compact row each at every width: icon
 * tile, full skill name, CEFR badge, a coloured bar taking the remaining width
 * and the percentage. A skill with no estimate yet shows 0% and no badge:
 * nothing is guessed. Shared by the Dashboard and Progress cards.
 *
 * Columns are fixed per row so the bars line up; the name column is wide
 * enough for the longest label in either locale at 360px, so nothing truncates.
 */
export function SkillBarList({
  skills,
  label,
  className,
}: SkillBarListProps): React.JSX.Element {
  const { t } = useTranslation();
  const bySkill = new Map(skills.map((s) => [s.skill, s]));

  return (
    <ul aria-label={label} className={cn("space-y-0.5", className)}>
      {SKILL_ORDER.map((skill) => {
        const tone = SKILL_TONE[skill];
        const Icon = tone.icon;
        const mastery = bySkill.get(skill);
        const name = t(
          `dashboard.skills.${skill}`,
          skill.charAt(0).toUpperCase() + skill.slice(1),
        );
        const percent = mastery ? Math.round(mastery.confidence * 100) : 0;

        return (
          <li
            key={skill}
            className="grid min-h-[44px] grid-cols-[2rem_5.25rem_2.25rem_minmax(0,1fr)_2.5rem] items-center gap-x-2 py-1 sm:grid-cols-[2.25rem_8rem_2.75rem_minmax(0,1fr)_3rem] sm:gap-x-3"
          >
            <span
              className={cn(
                "flex h-8 w-8 items-center justify-center rounded-xl sm:h-9 sm:w-9",
                tone.chip,
              )}
              aria-hidden="true"
            >
              <Icon className="h-4 w-4" />
            </span>
            <span className="whitespace-nowrap text-sm font-medium text-text">
              {name}
            </span>
            <span>
              {mastery ? (
                <span className="inline-flex rounded-md border border-border-subtle bg-surface-muted px-1.5 font-mono text-xs font-semibold text-text-muted">
                  {mastery.level}
                </span>
              ) : null}
            </span>
            <ProgressBar
              value={percent}
              label={t("progress.skills.progressLabel", { skill: name })}
              barClassName={tone.bar}
              className="h-2"
            />
            <span className="text-right text-xs font-semibold tabular-nums text-text-muted sm:text-sm">
              {percent}%
            </span>
          </li>
        );
      })}
    </ul>
  );
}
