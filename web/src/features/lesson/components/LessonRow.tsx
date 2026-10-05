import React from "react";
import { Link } from "@tanstack/react-router";
import {
  ArrowRight,
  CheckCircle2,
  ChevronRight,
  Clock,
  Lock,
  PlayCircle,
  Sparkles,
} from "lucide-react";
import { useTranslation } from "react-i18next";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";
import type { LessonSummary } from "../api/lessonApi";

export interface LessonRowProps {
  lesson: LessonSummary;
  isCompleted?: boolean | undefined;
  isNext?: boolean | undefined;
  /**
   * Called instead of following the link on a plain left click, so the page
   * that knows the course id can enrol before the runner opens an attempt.
   * Absent, the row stays an ordinary link.
   */
  onStartLesson?: ((lessonId: string) => void) | undefined;
}

export const LessonRow: React.FC<LessonRowProps> = ({
  lesson,
  isCompleted = false,
  isNext = false,
  onStartLesson,
}) => {
  const { t } = useTranslation();
  const { locked, lock_reason, title, skill_focus, estimated_minutes } = lesson;

  const handleClick = (event: React.MouseEvent<HTMLAnchorElement>) => {
    if (!onStartLesson) return;
    if (event.defaultPrevented) return;
    if (
      event.metaKey ||
      event.ctrlKey ||
      event.shiftKey ||
      event.altKey ||
      event.button !== 0
    ) {
      return;
    }
    event.preventDefault();
    onStartLesson(lesson.id);
  };

  return (
    <div
      className={cn(
        "relative flex flex-col sm:flex-row sm:items-center justify-between p-3.5 sm:px-4 sm:py-3 transition-colors gap-3",
        locked
          ? "bg-surface-muted/30 opacity-75"
          : isNext
            ? "bg-primary/[0.04] hover:bg-primary/[0.08]"
            : isCompleted
              ? "hover:bg-surface-muted/40"
              : "hover:bg-surface-muted/50",
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
          <div className="flex flex-wrap items-center gap-2 min-w-0">
            <span className="font-semibold text-text truncate text-sm sm:text-base">
              {title}
            </span>
            {isCompleted ? (
              <Badge
                variant="success"
                className="text-[10px] uppercase font-bold py-0"
              >
                {t("learn.statusCompleted", "Completed")}
              </Badge>
            ) : locked ? (
              <Badge
                variant="secondary"
                className="text-[10px] uppercase font-bold py-0"
              >
                {t("learn.statusLocked", "Locked")}
              </Badge>
            ) : isNext ? (
              <Badge
                variant="primary"
                className="text-[10px] uppercase font-bold py-0"
              >
                {t("learn.statusNext", "Next")}
              </Badge>
            ) : (
              <Badge
                variant="outline"
                className="text-[10px] uppercase font-bold py-0 text-text-muted"
              >
                {t("learn.statusNotStarted", "Not started")}
              </Badge>
            )}
          </div>

          <div className="flex flex-wrap items-center gap-3 text-xs text-text-muted">
            <span className="capitalize font-medium text-text-muted">
              {skill_focus}
            </span>
            <span>•</span>
            <span className="inline-flex items-center gap-1">
              <Clock className="h-3.5 w-3.5" aria-hidden="true" />
              {estimated_minutes} {t("learn.mins", "mins")}
            </span>
          </div>

          {locked && lock_reason && (
            <p className="text-xs text-danger-accent mt-0.5 flex items-center gap-1 font-medium">
              <span aria-hidden="true">🔒</span>
              <span>{lock_reason}</span>
            </p>
          )}
        </div>
      </div>

      {/* Action CTA */}
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
        ) : isNext ? (
          <Link
            to={`/learn/lesson/${lesson.id}`}
            onClick={handleClick}
            className="w-full sm:w-auto inline-flex items-center justify-center font-semibold rounded-lg min-h-[44px] px-4 text-xs bg-primary text-primary-fg hover:bg-primary-hover active:bg-primary-hover shadow-sm gap-1.5 transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary"
          >
            <span>
              {isCompleted
                ? t("learn.reviewBtn", "Review")
                : t("learn.startBtn", "Start Lesson")}
            </span>
            <ArrowRight className="h-4 w-4 shrink-0" aria-hidden="true" />
          </Link>
        ) : (
          <Link
            to={`/learn/lesson/${lesson.id}`}
            onClick={handleClick}
            className="inline-flex items-center justify-center font-medium rounded-lg min-h-[44px] px-3 text-xs border border-border bg-transparent text-text hover:bg-surface-muted gap-1.5 transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary after:absolute after:inset-0 md:after:hidden w-full sm:w-auto md:w-auto"
            aria-label={`${isCompleted ? t("learn.reviewBtn", "Review") : t("learn.startBtn", "Start Lesson")}: ${title}`}
          >
            <span className="hidden md:inline">
              {isCompleted
                ? t("learn.reviewBtn", "Review")
                : t("learn.startBtn", "Start Lesson")}
            </span>
            <ArrowRight
              className="h-4 w-4 shrink-0 hidden md:inline-block"
              aria-hidden="true"
            />
            <ChevronRight
              className="h-5 w-5 shrink-0 text-text-muted md:hidden ml-auto sm:ml-0"
              aria-hidden="true"
            />
          </Link>
        )}
      </div>
    </div>
  );
};
