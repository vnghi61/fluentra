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
import { useAuthStore } from "@/stores/authStore";

export interface DailyPracticeCardProps {
  className?: string;
  compact?: boolean;
}

const STORAGE_KEY = "fluentra.practice_level";
type PracticeLevel = "A2" | "B1" | "B2";

export const DailyPracticeCard: React.FC<DailyPracticeCardProps> = ({
  className = "",
  compact = false,
}) => {
  const { t } = useTranslation();
  const signedIn = useAuthStore((state) => state.status === "authenticated");

  const [selectedLevel, setSelectedLevel] = useState<PracticeLevel>(() => {
    try {
      const stored = localStorage.getItem(STORAGE_KEY);
      if (stored === "A2" || stored === "B1" || stored === "B2") {
        return stored;
      }
    } catch {
      // Storage unavailable
    }
    return "B1";
  });

  const handleLevelChange = (level: PracticeLevel) => {
    setSelectedLevel(level);
    try {
      localStorage.setItem(STORAGE_KEY, level);
    } catch {
      // Ignore
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
          <div className="flex items-center gap-1 bg-surface-muted/60 p-0.5 rounded-lg border border-border/50 text-xs">
            {(["A2", "B1", "B2"] as const).map((lvl) => (
              <button
                key={lvl}
                type="button"
                onClick={() => handleLevelChange(lvl)}
                className={`px-2.5 py-1 rounded-md font-semibold transition-all ${
                  selectedLevel === lvl
                    ? "bg-primary text-primary-foreground shadow-sm"
                    : "text-text-muted hover:text-text hover:bg-surface/50"
                }`}
                aria-pressed={selectedLevel === lvl}
              >
                {t("practice.daily.levelLabel", "Level {{lvl}}", { lvl })}
              </button>
            ))}
          </div>
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
        ) : (
          <Link
            to="/practice/daily"
            search={{ level: selectedLevel }}
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
