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

  const isRegular = !locked && !isNext;

  return (
    <div
      className={cn(
        "relative flex justify-between p-3.5 sm:px-4 sm:py-3 transition-colors gap-3",
        isRegular
          ? "flex-row items-center"
          : "flex-col sm:flex-row sm:items-center",
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
            <span className="min-w-0 line-clamp-2 break-words text-base font-semibold text-text">
              {title}
            </span>
            {isCompleted ? (
              <Badge
                variant="success"
                className="text-xs uppercase font-semibold py-0"
              >
                {t("learn.statusCompleted", "Completed")}
              </Badge>
            ) : locked ? (
              <Badge
                variant="secondary"
                className="text-xs uppercase font-semibold py-0"
              >
                {t("learn.statusLocked", "Locked")}
              </Badge>
            ) : isNext ? (
              <Badge
                variant="primary"
                className="text-xs uppercase font-semibold py-0"
              >
                {t("learn.statusNext", "Next")}
              </Badge>
            ) : (
              <Badge
                variant="outline"
                className="text-xs uppercase font-semibold py-0 text-text-muted"
              >
                {t("learn.statusNotStarted", "Not started")}
              </Badge>
            )}
          </div>

          <div className="flex flex-wrap items-center gap-x-3 gap-y-1 text-sm text-text-muted">
            <span className="capitalize font-medium text-text-muted">
              {skill_focus}
            </span>
            <span aria-hidden="true">•</span>
            <span className="inline-flex items-center gap-1.5">
              <Clock className="h-4 w-4" aria-hidden="true" />
              {estimated_minutes} {t("learn.mins", "mins")}
            </span>
          </div>

          {locked && lock_reason && (
            <p className="text-sm text-danger-accent mt-0.5 flex items-start gap-1.5 font-medium">
              <Lock className="mt-0.5 h-4 w-4 shrink-0" aria-hidden="true" />
              <span className="min-w-0 break-words">{lock_reason}</span>
            </p>
          )}
        </div>
      </div>

      {/* Action CTA */}
      <div
        className={cn(
          "flex items-center gap-2 shrink-0",
          isRegular
            ? "self-center"
            : "self-end sm:self-center w-full sm:w-auto",
        )}
      >
        {locked ? (
          <Button
            variant="secondary"
            disabled
            className="w-full sm:w-auto cursor-not-allowed opacity-60"
            aria-label={`${title} (${t("learn.locked", "Locked")}: ${lock_reason || ""})`}
          >
            <Lock className="h-4 w-4" aria-hidden="true" />
            {t("learn.lockedBtn", "Locked")}
          </Button>
        ) : isNext ? (
          <Link
            to={`/learn/lesson/${lesson.id}`}
            onClick={handleClick}
            className="w-full sm:w-auto inline-flex items-center justify-center font-semibold rounded-lg min-h-[44px] px-4 text-sm bg-primary text-primary-fg hover:bg-primary-hover active:bg-primary-hover shadow-sm gap-2 transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary"
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
            className="inline-flex items-center justify-center font-semibold rounded-lg h-11 w-11 min-h-[44px] min-w-[44px] text-sm text-text hover:bg-surface-muted gap-2 transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary after:absolute after:inset-0 md:after:hidden md:h-auto md:w-auto md:px-3 md:border md:border-border md:bg-transparent"
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
              className="h-5 w-5 shrink-0 text-text-muted md:hidden"
              aria-hidden="true"
            />
          </Link>
        )}
      </div>
    </div>
  );
};
