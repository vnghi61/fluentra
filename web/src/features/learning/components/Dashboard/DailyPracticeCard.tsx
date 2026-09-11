import React, { useState } from "react";
import { Link } from "@tanstack/react-router";
import {
  ArrowRight,
  BookOpen,
  Calendar,
  Layers,
  Sparkles,
  Zap,
} from "lucide-react";
import { useTranslation } from "react-i18next";

import { Badge } from "@/components/ui/badge";
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
}

/** The levels the practice pool holds. */
const PRACTICE_LEVELS = ["A2", "B1", "B2"] as const;
type PracticeLevel = (typeof PRACTICE_LEVELS)[number];

export const DailyPracticeCard: React.FC<DailyPracticeCardProps> = ({
  className = "",
  compact = false,
}) => {
  const { t } = useTranslation();
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
    <Card
      className={`relative overflow-hidden border-primary/20 bg-gradient-to-br from-surface-card via-surface-card to-primary/5 shadow-md ${className}`}
    >
      <CardHeader className="space-y-2">
        <div className="flex items-center justify-between gap-2 flex-wrap">
          <div className="flex items-center gap-2 text-primary">
            <Calendar className="h-5 w-5" aria-hidden="true" />
            <span className="text-xs font-semibold uppercase tracking-wider">
              {t("practice.daily.badge", "Daily Practice Set")}
            </span>
          </div>
          {signedIn && (
            <div
              role="group"
              aria-label={t("practice.daily.levelGroup", "Practice level")}
              className="flex items-center gap-1 bg-surface-muted/60 p-0.5 rounded-lg border border-border/50 text-xs"
            >
              {PRACTICE_LEVELS.map((lvl) => (
                <button
                  key={lvl}
                  type="button"
                  onClick={() => void chooseLevel(lvl)}
                  disabled={saving}
                  className={`px-2.5 py-1 rounded-md font-semibold transition-all disabled:opacity-60 ${
                    level === lvl
                      ? "bg-primary text-primary-foreground shadow-sm"
                      : "text-text-muted hover:text-text hover:bg-surface/50"
                  }`}
                  aria-pressed={level === lvl}
                >
                  {t("practice.daily.levelLabel", "Level {{lvl}}", { lvl })}
                </button>
              ))}
            </div>
          )}
        </div>

        <CardTitle className="text-xl font-bold tracking-tight">
          {t("practice.daily.title", "Today's Practice Set")}
        </CardTitle>

        <CardDescription className="text-sm text-text-muted">
          {t(
            "practice.daily.desc",
            "9 verified exercises freshly drawn for today: 1 reading passage, 5 grammar questions, and 3 sentence rewrites.",
          )}
        </CardDescription>

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

        {!compact && (
          <div className="flex items-center gap-2 pt-2 flex-wrap">
            <Badge
              variant="outline"
              className="gap-1.5 py-1 px-2.5 text-xs bg-surface/60"
            >
              <BookOpen
                className="h-3.5 w-3.5 text-primary"
                aria-hidden="true"
              />
              <span>{t("practice.daily.slotReading", "1 Passage")}</span>
            </Badge>
            <Badge
              variant="outline"
              className="gap-1.5 py-1 px-2.5 text-xs bg-surface/60"
            >
              <Layers
                className="h-3.5 w-3.5 text-secondary-accent"
                aria-hidden="true"
              />
              <span>{t("practice.daily.slotGrammar", "5 Grammar")}</span>
            </Badge>
            <Badge
              variant="outline"
              className="gap-1.5 py-1 px-2.5 text-xs bg-surface/60"
            >
              <Zap
                className="h-3.5 w-3.5 text-warning-accent"
                aria-hidden="true"
              />
              <span>{t("practice.daily.slotTransforms", "3 Transforms")}</span>
            </Badge>
          </div>
        )}
      </CardHeader>

      <CardFooter className="pt-2 flex flex-col sm:flex-row items-stretch sm:items-center justify-between gap-3">
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
          <Button
            disabled
            className="w-full sm:w-auto ml-auto gap-2 shadow-sm font-semibold"
          >
            <Sparkles className="h-4 w-4" aria-hidden="true" />
            {t("practice.daily.startBtn", "Start Today's Set")}
            <ArrowRight className="h-4 w-4" aria-hidden="true" />
          </Button>
        ) : (
          <Link
            to="/practice/daily"
            search={{ level }}
            className="w-full sm:w-auto ml-auto"
          >
            <Button className="w-full sm:w-auto gap-2 shadow-sm font-semibold">
              <Sparkles className="h-4 w-4" aria-hidden="true" />
              {t("practice.daily.startBtn", "Start Today's Set")}
              <ArrowRight className="h-4 w-4" aria-hidden="true" />
            </Button>
          </Link>
        )}
      </CardFooter>
    </Card>
  );
};
