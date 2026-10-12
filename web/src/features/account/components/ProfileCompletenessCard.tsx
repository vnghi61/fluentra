import React from "react";
import { useTranslation } from "react-i18next";
import { CheckCircle2, Circle, Sparkles } from "lucide-react";

import {
  useLearningProfile,
  type UserPreferences,
  type UserProfile,
} from "../api/accountApi";
import {
  profileCompleteness,
  type CompletenessKey,
} from "../model/profileCompleteness";
import { Skeleton } from "@/components/ui/skeleton";
import { cn } from "@/lib/utils";
import { SettingsCardHeader } from "./SettingsCard";
import { settingsCard } from "./settingsStyles";

const LABELS: Record<CompletenessKey, [key: string, fallback: string]> = {
  basicInfo: ["settings.completeness.basicInfo", "Basic information"],
  photo: ["settings.completeness.photo", "Profile photo"],
  emailVerified: ["settings.completeness.emailVerified", "Verified email"],
  learningGoals: ["settings.completeness.learningGoals", "Learning goals"],
  learningPreferences: [
    "settings.completeness.learningPreferences",
    "Learning preferences",
  ],
};

const RADIUS = 34;
const CIRCUMFERENCE = 2 * Math.PI * RADIUS;

/** A ring showing done/total; the numbers inside are the accessible value. */
const ProgressRing: React.FC<{ done: number; total: number }> = ({
  done,
  total,
}) => {
  const { t } = useTranslation();
  const fraction = total > 0 ? done / total : 0;
  return (
    <div
      role="progressbar"
      aria-valuemin={0}
      aria-valuemax={total}
      aria-valuenow={done}
      aria-valuetext={t("settings.completeness.progress", {
        done,
        total,
        defaultValue: `${done} of ${total} steps complete`,
      })}
      aria-label={t("settings.completeness.title", "Personalised experience")}
      className="relative h-24 w-24 shrink-0"
    >
      <svg viewBox="0 0 80 80" className="h-full w-full -rotate-90">
        <circle
          cx="40"
          cy="40"
          r={RADIUS}
          fill="none"
          strokeWidth="8"
          className="stroke-surface-muted"
        />
        <circle
          cx="40"
          cy="40"
          r={RADIUS}
          fill="none"
          strokeWidth="8"
          strokeLinecap="round"
          strokeDasharray={CIRCUMFERENCE}
          strokeDashoffset={CIRCUMFERENCE * (1 - fraction)}
          className={cn(
            "transition-[stroke-dashoffset] duration-700 motion-reduce:transition-none",
            done === total ? "stroke-success" : "stroke-primary",
          )}
        />
      </svg>
      <span
        aria-hidden="true"
        className="absolute inset-0 flex items-center justify-center text-xl font-bold tabular-nums text-text"
      >
        {done}/{total}
      </span>
    </div>
  );
};

interface ProfileCompletenessCardProps {
  profile: UserProfile;
  preferences: UserPreferences;
}

/** "Personalised experience": how much of the profile the learner filled in. */
export const ProfileCompletenessCard: React.FC<
  ProfileCompletenessCardProps
> = ({ profile, preferences }) => {
  const { t } = useTranslation();
  const learning = useLearningProfile();
  const items = profileCompleteness(
    profile,
    preferences,
    learning.isError ? null : learning.data,
  );
  const done = items.filter((item) => item.done).length;

  return (
    <section className={`${settingsCard} @container flex flex-col gap-4`}>
      <SettingsCardHeader
        icon={Sparkles}
        title={t("settings.completeness.title", "Personalised experience")}
        description={
          done === items.length
            ? t("settings.completeness.allDone", "Your profile is complete.")
            : t(
                "settings.completeness.desc",
                "Complete your profile so lessons and plans fit you better.",
              )
        }
      />
      {learning.isPending ? (
        <Skeleton className="h-28 w-full rounded-xl" />
      ) : (
        <div className="flex flex-col items-center gap-4 @xs:flex-row @xs:items-center">
          <ProgressRing done={done} total={items.length} />
          <ul className="w-full min-w-0 flex-1 space-y-1.5">
            {items.map((item) => {
              const [key, fallback] = LABELS[item.key];
              const Icon = item.done ? CheckCircle2 : Circle;
              return (
                <li
                  key={item.key}
                  className={cn(
                    "flex items-center gap-2 text-sm",
                    item.done ? "text-text" : "text-text-muted",
                  )}
                >
                  <Icon
                    className={cn(
                      "h-4 w-4 shrink-0",
                      item.done ? "text-success-accent" : "text-text-muted",
                    )}
                    aria-hidden
                  />
                  <span className="min-w-0 flex-1">{t(key, fallback)}</span>
                  <span className="sr-only">
                    {item.done
                      ? t("settings.completeness.done", "Done")
                      : t("settings.completeness.todo", "Not yet")}
                  </span>
                </li>
              );
            })}
          </ul>
        </div>
      )}
    </section>
  );
};
