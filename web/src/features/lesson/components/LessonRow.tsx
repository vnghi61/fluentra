import React from "react";
import { Link } from "@tanstack/react-router";
import { ArrowRight, CheckCircle2, Clock, Lock, PlayCircle, Sparkles } from "lucide-react";
import { useTranslation } from "react-i18next";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";
import type { LessonSummary } from "../api/lessonApi";

export interface LessonRowProps {
  lesson: LessonSummary;
  isCompleted?: boolean;
  isNext?: boolean;
}

export const LessonRow: React.FC<LessonRowProps> = ({
  lesson,
  isCompleted = false,
  isNext = false,
}) => {
  const { t } = useTranslation();
  const { locked, lock_reason, title, skill_focus, estimated_minutes } = lesson;

  return (
    <div
      className={cn(
        "flex flex-col sm:flex-row sm:items-center justify-between p-4 rounded-xl border transition-all gap-4",
        locked
          ? "border-border-subtle bg-surface-muted/40 opacity-75"
          : isNext
            ? "border-primary/50 bg-primary/5 shadow-sm ring-1 ring-primary/30"
            : isCompleted
              ? "border-success/30 bg-surface-card"
              : "border-border-subtle bg-surface-card hover:border-border",
      )}
    >
      {/* Lesson Info */}
      <div className="flex items-start gap-3 min-w-0 flex-1">
        <div className="mt-0.5 shrink-0" aria-hidden="true">
          {locked ? (
            <Lock className="h-5 w-5 text-text-muted" />
          ) : isCompleted ? (
            <CheckCircle2 className="h-5 w-5 text-success" />
          ) : isNext ? (
            <PlayCircle className="h-5 w-5 text-primary-accent" />
          ) : (
            <Sparkles className="h-5 w-5 text-text-muted" />
          )}
        </div>

        <div className="space-y-1 min-w-0 flex-1">
          <div className="flex flex-wrap items-center gap-2">
            <span className="font-semibold text-text truncate text-base">
              {title}
            </span>
            {isNext && (
              <Badge variant="primary" className="text-[10px] uppercase font-bold py-0">
                {t("learn.nextBadge", "Next Up")}
              </Badge>
            )}
            {isCompleted && (
              <Badge variant="success" className="text-[10px] uppercase font-bold py-0">
                {t("learn.completedBadge", "Completed")}
              </Badge>
            )}
          </div>

          <div className="flex flex-wrap items-center gap-3 text-xs text-text-muted">
            <span className="capitalize font-medium text-text-muted">{skill_focus}</span>
            <span>•</span>
            <span className="inline-flex items-center gap-1">
              <Clock className="h-3.5 w-3.5" aria-hidden="true" />
              {estimated_minutes} {t("learn.mins", "mins")}
            </span>
          </div>

          {locked && lock_reason && (
            <p className="text-xs text-danger-accent mt-1 flex items-center gap-1 font-medium">
              <span aria-hidden="true">🔒</span>
              <span>{lock_reason}</span>
            </p>
          )}
        </div>
      </div>

      {/* Action Button */}
      <div className="flex items-center gap-2 self-end sm:self-center shrink-0 w-full sm:w-auto">
        {locked ? (
          <Button
            variant="secondary"
            disabled
            className="w-full sm:w-auto min-h-[44px] text-xs font-medium cursor-not-allowed opacity-60"
            aria-label={`${title} (${t("learn.locked", "Locked")}: ${lock_reason || ""})`}
          >
            <Lock className="h-3.5 w-3.5 mr-1.5" aria-hidden="true" />
            {t("learn.lockedBtn", "Locked")}
          </Button>
        ) : (
          <Link
            to={`/learn/lesson/${lesson.id}`}
            className="w-full sm:w-auto"
          >
            <Button
              variant={isNext ? "primary" : isCompleted ? "secondary" : "outline"}
              className="w-full sm:w-auto min-h-[44px] gap-1.5 text-sm font-semibold"
            >
              {isCompleted
                ? t("learn.reviewBtn", "Review")
                : t("learn.startBtn", "Start Lesson")}
              <ArrowRight className="h-4 w-4" aria-hidden="true" />
            </Button>
          </Link>
        )}
      </div>
    </div>
  );
};
