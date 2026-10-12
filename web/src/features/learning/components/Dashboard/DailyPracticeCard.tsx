import React, { useState } from "react";
import type { TFunction } from "i18next";
import { Link } from "@tanstack/react-router";
import {
  ArrowRight,
  BookOpen,
  Calendar,
  CalendarDays,
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
import { Card } from "@/components/ui/card";
import { accountApi, replacementFor } from "@/features/account/api/accountApi";
import { useAuthStore } from "@/stores/authStore";
import { usePreferencesStore } from "@/stores/preferencesStore";
import { NativeSelect } from "@/components/ui/native-select";
import { ProgressRing } from "@/components/ui/progress-ring";
import { StackedCardsArt } from "@/components/ui/stacked-cards-art";
import { cn } from "@/lib/utils";

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

  const levelControls = (
    <>
      {signedIn && (
        <div className="flex shrink-0 items-center gap-1.5">
          <NativeSelect
            aria-label={t("practice.daily.levelGroup", "Practice level")}
            value={level ?? ""}
            onChange={(e) => {
              const next = e.target.value as PracticeLevel;
              if (next) void chooseLevel(next);
            }}
            disabled={saving}
            className="h-auto min-h-[44px] min-w-[44px] rounded-full border-border-subtle bg-surface-muted py-1 pl-3 pr-9 font-semibold disabled:opacity-60"
            wrapperClassName="w-auto"
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
          </NativeSelect>

          {isPractice && level && (
            <details className="relative inline-flex items-center">
              <summary
                aria-label={t("practice.daily.levelInfo", "About this level")}
                title={t("practice.daily.levelInfo", "About this level")}
                className="flex items-center justify-center min-h-[44px] min-w-[44px] rounded-full text-text-muted hover:text-text hover:bg-surface-muted cursor-pointer focus:outline-none focus:ring-2 focus:ring-primary list-none [&::-webkit-details-marker]:hidden transition-colors motion-reduce:transition-none"
              >
                <Info className="h-4 w-4" aria-hidden="true" />
              </summary>
              <div className="absolute right-0 top-full mt-1.5 z-20 w-64 max-w-[calc(100vw-3rem)] rounded-lg border border-border-subtle bg-surface-card p-3 text-sm text-text shadow-lg">
                <p role="status">{t(`practice.daily.levelHint${level}`)}</p>
              </div>
            </details>
          )}
        </div>
      )}
    </>
  );

  const statusMessages = (
    <>
      {signedIn && level === null && (
        <p className="text-sm font-medium text-text">
          {t(
            "practice.daily.pickLevel",
            "Choose your level to start. You can change it any time.",
          )}
        </p>
      )}

      {saveFailed && (
        <p role="alert" className="text-sm text-danger-accent">
          {t(
            "practice.daily.levelSaveFailed",
            "Your level could not be saved. Today's set still uses it.",
          )}
        </p>
      )}
    </>
  );

  const chips = (
    <>
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
                  className={`h-4 w-4 ${slot.iconClass}`}
                  aria-hidden="true"
                />
                <span>{slot.renderLabel(t)}</span>
              </span>
            );
          })}
        </div>
      )}
    </>
  );

  const ctaLabel = isPractice
    ? t("overview.practice.startBtn")
    : t("practice.daily.continueBtn", "Continue practice");

  const cta = !signedIn ? (
    <div className="flex w-full flex-wrap items-center justify-between gap-3">
      <span className="text-xs text-text-muted">
        {t(
          "practice.daily.guestNote",
          "Sign in to save streaks and track exposure history.",
        )}
      </span>
      <Button asChild variant="secondary" className="w-full gap-2 sm:w-auto">
        <Link to="/login">
          <Sparkles className="h-4 w-4" aria-hidden="true" />
          {t("practice.daily.signInBtn", "Sign in to start")}
        </Link>
      </Button>
    </div>
  ) : level === null ? (
    <Button disabled className="w-full gap-2 font-semibold sm:w-auto">
      {ctaLabel}
      <ArrowRight className="h-4 w-4" aria-hidden="true" />
    </Button>
  ) : (
    <Button asChild className="w-full gap-2 font-semibold shadow-sm sm:w-auto">
      <Link to="/practice/daily" search={{ level }}>
        {ctaLabel}
        <ArrowRight className="h-4 w-4" aria-hidden="true" />
      </Link>
    </Button>
  );

  const countChip = (
    <span className="rounded-full bg-primary/10 px-2.5 py-0.5 text-xs font-semibold text-primary-accent">
      {t("practice.daily.setCount", { count: TOTAL_PRACTICE_EXERCISES })}
    </span>
  );

  // The practice hub's hero: what the set holds and the way in on the left,
  // the set's size on the right. DailyPracticeSet carries no done flags, so the
  // ring states the size of today's set; it is not a progress figure.
  if (isPractice) {
    return (
      <Card
        className={cn(
          "relative h-full overflow-hidden border-primary/20 bg-gradient-to-br from-primary/10 via-surface-card to-surface-card p-5 md:p-6 lg:p-7",
          className,
        )}
      >
        <div className="grid gap-6 lg:grid-cols-[minmax(0,1fr)_auto] lg:items-center">
          <div className="min-w-0 space-y-3">
            <div className="flex flex-wrap items-center justify-between gap-2">
              <div className="flex flex-wrap items-center gap-2">
                <EyebrowIcon
                  className="h-5 w-5 text-primary-accent"
                  aria-hidden="true"
                />
                <span className="text-xs font-semibold uppercase tracking-wider text-primary-accent">
                  {t("practice.daily.cardTitle", "Today's exercises")}
                </span>
                {countChip}
              </div>
              {levelControls}
            </div>
            <h2 className="text-2xl font-bold tracking-tight text-text md:text-3xl">
              {t("overview.practice.heroTitle")}
            </h2>
            <p className="max-w-xl text-sm text-text-muted md:text-base">
              {t(
                "practice.daily.practiceDesc",
                "Complete the exercises to consolidate what you know and build your skills.",
              )}
            </p>
            {statusMessages}
            {!compact && (
              <div className="[&_span.inline-flex]:rounded-xl [&_span.inline-flex]:px-3 [&_span.inline-flex]:py-1.5 [&_span.inline-flex]:text-sm">
                {chips}
              </div>
            )}
            <div className="pt-2">{cta}</div>
          </div>

          <div className="flex items-center gap-4 border-border-subtle lg:flex-col lg:gap-3 lg:border-l lg:pl-8">
            <ProgressRing
              value={0}
              decorative
              size={104}
              strokeWidth={9}
              trackClassName="text-primary/15"
              className="hidden sm:inline-flex"
            >
              <span className="text-3xl font-bold tabular-nums text-text">
                {TOTAL_PRACTICE_EXERCISES}
              </span>
            </ProgressRing>
            <span className="text-sm text-text-muted sm:hidden">
              <span className="mr-1 text-lg font-bold text-text">
                {TOTAL_PRACTICE_EXERCISES}
              </span>
              {t("practice.daily.todayTotal", "exercises today")}
            </span>
            <span className="hidden text-sm text-text-muted sm:block">
              {t("practice.daily.todayTotal", "exercises today")}
            </span>
            <StackedCardsArt
              icon={EyebrowIcon}
              size="sm"
              className="ml-auto hidden sm:block lg:ml-0 lg:mt-2"
            />
          </div>
        </div>
      </Card>
    );
  }

  return (
    <Card className={cn("flex h-full flex-col p-4 md:p-5", className)}>
      <div className="flex items-start justify-between gap-3">
        <div className="min-w-0 space-y-1">
          <div className="flex flex-wrap items-center gap-2">
            <h2 className="text-base font-bold tracking-tight text-text md:text-lg">
              {t("practice.daily.cardTitle", "Today's exercises")}
            </h2>
            {countChip}
          </div>
          <p className="text-sm text-text-muted">
            {t(
              "practice.daily.desc",
              "10 verified exercises freshly drawn for today, across all five skills: grammar, reading, listening, speaking and writing.",
            )}
          </p>
        </div>
        {levelControls}
      </div>

      {level && (
        <p className="mt-2 text-sm text-text-muted" role="status">
          {t(`practice.daily.levelHint${level}`)}
        </p>
      )}

      <div className="mt-1 space-y-1">{statusMessages}</div>

      {!compact && chips}

      <div className="mt-auto pt-4">{cta}</div>
    </Card>
  );
};
