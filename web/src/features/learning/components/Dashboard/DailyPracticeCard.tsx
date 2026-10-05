import React, { useState } from "react";
import type { TFunction } from "i18next";
import { Link } from "@tanstack/react-router";
import {
  ArrowRight,
  BookOpen,
  Calendar,
  CalendarDays,
  ChevronDown,
  Headphones,
  Info,
  Layers,
  Mic,
  PenTool,
  Sparkles,
  Zap,
} from "lucide-react";
import { useTranslation } from "react-i18next";

import { Button } from "@/components/ui/button";
import {
  Card,
  CardDescription,
  CardFooter,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { accountApi, replacementFor } from "@/features/account/api/accountApi";
import { useAuthStore } from "@/stores/authStore";
import { usePreferencesStore } from "@/stores/preferencesStore";

export interface DailyPracticeCardProps {
  className?: string;
  compact?: boolean;
  variant?: "default" | "practice";
}

/** The levels the practice pool holds. */
const PRACTICE_LEVELS = ["A2", "B1", "B2"] as const;
type PracticeLevel = (typeof PRACTICE_LEVELS)[number];

interface PracticeSlot {
  count: number;
  key: string;
  defaultLabel: string;
  renderLabel: (t: TFunction) => string;
  icon: typeof Layers;
  chipClass: string;
  iconClass: string;
}

const PRACTICE_SLOTS: readonly PracticeSlot[] = [
  {
    count: 3,
    key: "practice.daily.slotGrammar",
    defaultLabel: "3 Grammar",
    renderLabel: (t) => t("practice.daily.slotGrammar", "3 Grammar"),
    icon: Layers,
    chipClass:
      "bg-blue-500/10 text-blue-700 dark:text-blue-300 border-blue-500/20",
    iconClass: "text-blue-700 dark:text-blue-300",
  },
  {
    count: 2,
    key: "practice.daily.slotTransforms",
    defaultLabel: "2 Transforms",
    renderLabel: (t) => t("practice.daily.slotTransforms", "2 Transforms"),
    icon: Zap,
    chipClass:
      "bg-indigo-500/10 text-indigo-700 dark:text-indigo-300 border-indigo-500/20",
    iconClass: "text-indigo-700 dark:text-indigo-300",
  },
  {
    count: 1,
    key: "practice.daily.slotReading",
    defaultLabel: "1 Passage",
    renderLabel: (t) => t("practice.daily.slotReading", "1 Passage"),
    icon: BookOpen,
    chipClass:
      "bg-purple-500/10 text-purple-700 dark:text-purple-300 border-purple-500/20",
    iconClass: "text-purple-700 dark:text-purple-300",
  },
  {
    count: 1,
    key: "practice.daily.slotListening",
    defaultLabel: "1 Clip",
    renderLabel: (t) => t("practice.daily.slotListening", "1 Clip"),
    icon: Headphones,
    chipClass:
      "bg-green-500/10 text-green-700 dark:text-green-300 border-green-500/20",
    iconClass: "text-green-700 dark:text-green-300",
  },
  {
    count: 2,
    key: "practice.daily.slotSpeaking",
    defaultLabel: "2 Speaking",
    renderLabel: (t) => t("practice.daily.slotSpeaking", "2 Speaking"),
    icon: Mic,
    chipClass:
      "bg-orange-500/10 text-orange-700 dark:text-orange-300 border-orange-500/20",
    iconClass: "text-orange-700 dark:text-orange-300",
  },
  {
    count: 1,
    key: "practice.daily.slotWriting",
    defaultLabel: "1 Writing",
    renderLabel: (t) => t("practice.daily.slotWriting", "1 Writing"),
    icon: PenTool,
    chipClass:
      "bg-pink-500/10 text-pink-700 dark:text-pink-300 border-pink-500/20",
    iconClass: "text-pink-700 dark:text-pink-300",
  },
] as const;

const TOTAL_PRACTICE_EXERCISES = PRACTICE_SLOTS.reduce(
  (acc, slot) => acc + slot.count,
  0,
);

export const DailyPracticeCard: React.FC<DailyPracticeCardProps> = ({
  className = "",
  compact = false,
  variant = "default",
}) => {
  const { t } = useTranslation();
  const isPractice = variant === "practice";
  const EyebrowIcon = isPractice ? CalendarDays : Calendar;
  const signedIn = useAuthStore((state) => state.status === "authenticated");
  // The level lives in preferences, so the one chosen on a phone is the one on
  // a laptop. Null until the learner picks: that is when the card asks.
  const storedLevel = usePreferencesStore(
    (state) => state.preferences?.practice_level ?? null,
  );
  // A pick made here counts even when saving it fails or preferences never
  // loaded: a failed write costs the level on the next device, not today's set.
  const [pickedLevel, setPickedLevel] = useState<PracticeLevel | null>(null);
  const [saving, setSaving] = useState(false);
  const [saveFailed, setSaveFailed] = useState(false);

  const level = pickedLevel ?? storedLevel;

  const chooseLevel = async (next: PracticeLevel) => {
    if (next === level || saving) return;
    setPickedLevel(next);
    setSaveFailed(false);
    const current = usePreferencesStore.getState().preferences;
    if (!current) return;
    setSaving(true);
    try {
      const updated = await accountApi.replacePreferences(
        replacementFor(current, { practice_level: next }),
      );
      usePreferencesStore.getState().set(updated);
    } catch {
      setSaveFailed(true);
    } finally {
      setSaving(false);
    }
  };

  return (
    <Card className={`relative overflow-hidden ${className}`}>
      <CardHeader className="space-y-2 p-4 pb-3 md:p-5 md:pb-3">
        <div className="flex items-center justify-between gap-2 flex-wrap">
          <div className="flex items-center gap-2 text-primary">
            <EyebrowIcon className="h-5 w-5" aria-hidden="true" />
            <span className="text-xs font-semibold uppercase tracking-wider">
              {t("practice.daily.badge", "Daily Practice Set")}
            </span>
          </div>
          {signedIn && (
            <div className="flex items-center gap-1.5">
              <div className="relative inline-flex items-center">
                <select
                  aria-label={t("practice.daily.levelGroup", "Practice level")}
                  value={level ?? ""}
                  onChange={(e) => {
                    const next = e.target.value as PracticeLevel;
                    if (next) void chooseLevel(next);
                  }}
                  disabled={saving}
                  className="appearance-none cursor-pointer rounded-full border border-border-subtle bg-surface-muted pl-3 pr-7 py-1 text-base font-semibold text-text min-h-[44px] min-w-[44px] focus:outline-none focus:ring-2 focus:ring-primary disabled:opacity-60"
                >
                  {level === null && (
                    <option value="" disabled>
                      {t("practice.daily.selectLevelPlaceholder", "Level")}
                    </option>
                  )}
                  {PRACTICE_LEVELS.map((lvl) => (
                    <option key={lvl} value={lvl}>
                      {lvl}
                    </option>
                  ))}
                </select>
                <ChevronDown
                  className="pointer-events-none absolute right-2.5 h-4 w-4 text-text-muted"
                  aria-hidden="true"
                />
              </div>

              {isPractice && level && (
                <details className="relative inline-flex items-center">
                  <summary
                    aria-label={t(
                      "practice.daily.levelInfo",
                      "About this level",
                    )}
                    title={t("practice.daily.levelInfo", "About this level")}
                    className="flex items-center justify-center min-h-[44px] min-w-[44px] rounded-full text-text-muted hover:text-text hover:bg-surface-muted cursor-pointer focus:outline-none focus:ring-2 focus:ring-primary list-none [&::-webkit-details-marker]:hidden transition-colors motion-reduce:transition-none"
                  >
                    <Info className="h-4 w-4" aria-hidden="true" />
                  </summary>
                  <div className="absolute right-0 top-full mt-1.5 z-20 w-64 max-w-[calc(100vw-3rem)] rounded-lg border border-border-subtle bg-surface-card p-3 text-xs text-text shadow-lg">
                    <p role="status">{t(`practice.daily.levelHint${level}`)}</p>
                  </div>
                </details>
              )}
            </div>
          )}
        </div>

        <CardTitle className="text-base md:text-lg font-bold tracking-tight text-text">
          {t("practice.daily.cardTitle", "Today's exercises")}
        </CardTitle>

        {isPractice ? (
          <CardDescription className="text-sm text-text-muted">
            {t("practice.daily.setCount", {
              count: TOTAL_PRACTICE_EXERCISES,
            })}
          </CardDescription>
        ) : (
          <CardDescription className="text-sm text-text-muted">
            {t(
              "practice.daily.desc",
              "10 verified exercises freshly drawn for today, across all five skills: grammar, reading, listening, speaking and writing.",
            )}
          </CardDescription>
        )}

        {!isPractice && level && (
          <p className="text-sm text-text-muted" role="status">
            {t(`practice.daily.levelHint${level}`)}
          </p>
        )}

        {signedIn && level === null && (
          <p className="text-sm font-medium text-text">
            {t(
              "practice.daily.pickLevel",
              "Choose your level to start. You can change it any time.",
            )}
          </p>
        )}

        {saveFailed && (
          <p role="alert" className="text-xs text-danger-accent">
            {t(
              "practice.daily.levelSaveFailed",
              "Your level could not be saved. Today's set still uses it.",
            )}
          </p>
        )}

        {/*
          What the set is made of, rendered with low-saturation tints that work in dark mode.
          Transforms uses indigo.
        */}
        {!compact && (
          <div className="flex items-center gap-2 pt-2 flex-wrap">
            {PRACTICE_SLOTS.map((slot) => {
              const Icon = slot.icon;
              return (
                <span
                  key={slot.key}
                  className={`inline-flex items-center gap-1.5 py-1 px-2.5 rounded-lg border text-xs font-medium ${slot.chipClass}`}
                >
                  <Icon
                    className={`h-3.5 w-3.5 ${slot.iconClass}`}
                    aria-hidden="true"
                  />
                  <span>{slot.renderLabel(t)}</span>
                </span>
              );
            })}
          </div>
        )}
      </CardHeader>

      <CardFooter className="p-4 pt-0 md:p-5 md:pt-0 flex flex-col sm:flex-row items-stretch sm:items-center gap-3">
        {!signedIn ? (
          <div className="w-full flex items-center justify-between gap-3 flex-wrap">
            <span className="text-xs text-text-muted">
              {t(
                "practice.daily.guestNote",
                "Sign in to save streaks and track exposure history.",
              )}
            </span>
            <Link to="/login" className="w-full sm:w-auto">
              <Button variant="secondary" className="w-full sm:w-auto gap-2">
                <Sparkles className="h-4 w-4" aria-hidden="true" />
                {t("practice.daily.signInBtn", "Sign in to start")}
              </Button>
            </Link>
          </div>
        ) : level === null ? (
          <Button disabled className="w-full sm:w-auto gap-2 font-semibold">
            {t("practice.daily.continueBtn", "Continue practice")}
            <ArrowRight className="h-4 w-4" aria-hidden="true" />
          </Button>
        ) : (
          <Link
            to="/practice/daily"
            search={{ level }}
            className="w-full sm:w-auto"
          >
            <Button className="w-full sm:w-auto gap-2 shadow-sm font-semibold">
              {t("practice.daily.continueBtn", "Continue practice")}
              <ArrowRight className="h-4 w-4" aria-hidden="true" />
            </Button>
          </Link>
        )}
      </CardFooter>
    </Card>
  );
};
