import React from "react";
import {
  BookMarked,
  BookOpen,
  CheckCircle2,
  Clock,
  GraduationCap,
  ShoppingBag,
} from "lucide-react";
import type { TFunction } from "i18next";
import { useTranslation } from "react-i18next";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardHeader } from "@/components/ui/card";
import type { CourseDetail } from "../api/lessonApi";
import { courseDescription, courseTitle } from "../model/courseText";

export interface CourseHeaderProps {
  course: CourseDetail;
  totalLessons?: number | undefined;
  completedLessons?: number | undefined;
  isOwned?: boolean | undefined;
  priceVnd?: number | undefined;
  onBuyOrClaim?: (() => void) | undefined;
}

export function formatDuration(t: TFunction, minutes: number): string {
  if (minutes < 60) {
    return t("learn.durationMinutes", {
      count: minutes,
      defaultValue: `${minutes} min`,
    });
  }
  const hours = Math.floor(minutes / 60);
  const remainingMins = minutes % 60;
  if (remainingMins === 0) {
    return t("learn.durationHours", {
      count: hours,
      defaultValue: `${hours}h`,
    });
  }
  return t("learn.durationHoursMinutes", {
    hours,
    mins: remainingMins,
    defaultValue: `${hours}h ${remainingMins}m`,
  });
}

export const CourseHeader: React.FC<CourseHeaderProps> = ({
  course,
  totalLessons = 0,
  completedLessons = 0,
  isOwned = false,
  priceVnd,
  onBuyOrClaim,
}) => {
  const { t } = useTranslation();

  // Price from prop or course object
  const courseAny = course as unknown as {
    price_vnd?: number;
    origin?: string;
  };
  const effectivePrice = priceVnd ?? courseAny.price_vnd ?? 0;
  const isPaid = effectivePrice > 0;
  const isCommunity = courseAny.origin === "community";

  const totalDurationMinutes = React.useMemo(() => {
    return (
      course.units?.reduce(
        (acc, unit) =>
          acc +
          (unit.lessons?.reduce(
            (lAcc, l) => lAcc + (l.estimated_minutes || 0),
            0,
          ) ?? 0),
        0,
      ) ?? 0
    );
  }, [course.units]);

  const percent =
    totalLessons > 0 ? Math.round((completedLessons / totalLessons) * 100) : 0;

  return (
    <Card className="relative overflow-hidden border-primary/30 bg-gradient-to-r from-surface-card via-surface-card to-primary/5 shadow-sm">
      <CardHeader className="space-y-4 p-6 md:p-8">
        <div className="flex flex-col md:flex-row md:items-start md:justify-between gap-6">
          <div className="space-y-3 min-w-0 flex-1">
            <div className="flex flex-wrap items-center justify-between gap-2">
              <div className="flex items-center gap-2 text-primary-accent">
                <GraduationCap className="h-5 w-5" aria-hidden="true" />
                <span className="text-xs font-bold uppercase tracking-wider">
                  {t("learn.courseBadge", "Course Syllabus")}
                </span>
              </div>

              <div className="flex items-center gap-2">
                {Boolean(course.cefr_from && course.cefr_to) && (
                  <Badge variant="primary" className="font-mono text-xs">
                    {course.cefr_from} → {course.cefr_to}
                  </Badge>
                )}
                {totalDurationMinutes > 0 && (
                  <span className="inline-flex items-center gap-1 text-xs text-text-muted">
                    <Clock className="h-3.5 w-3.5" aria-hidden="true" />
                    {formatDuration(t, totalDurationMinutes)}
                  </span>
                )}
              </div>
            </div>

            <div>
              <h1 className="text-2xl md:text-3xl font-extrabold text-text tracking-tight">
                {courseTitle(t, course)}
              </h1>
              {course.description && (
                <p className="mt-1 text-sm md:text-base text-text-muted">
                  {courseDescription(t, course)}
                </p>
              )}
            </div>

            {totalLessons > 0 && (
              <div className="space-y-2 pt-1">
                <div className="flex items-center justify-between text-xs text-text-muted">
                  <div className="flex items-center gap-2">
                    <BookMarked
                      className="h-4 w-4 text-primary-accent shrink-0"
                      aria-hidden="true"
                    />
                    <span>
                      {t("learn.syllabusProgress", {
                        completed: completedLessons,
                        total: totalLessons,
                        defaultValue: `${completedLessons} of ${totalLessons} lessons completed`,
                      })}
                    </span>
                  </div>
                  <span className="font-semibold text-text">{percent}%</span>
                </div>
                <div
                  role="progressbar"
                  aria-valuemin={0}
                  aria-valuemax={100}
                  aria-valuenow={percent}
                  aria-label={t("learn.syllabusProgress", {
                    completed: completedLessons,
                    total: totalLessons,
                    defaultValue: `${completedLessons} of ${totalLessons} lessons completed`,
                  })}
                  className="relative h-2.5 w-full overflow-hidden rounded-full bg-surface-muted border border-border-subtle"
                >
                  <div
                    className="h-full bg-primary rounded-full transition-[width] duration-300 ease-in-out motion-reduce:transition-none"
                    style={{ width: `${percent}%` }}
                  />
                </div>
              </div>
            )}
          </div>

          <div
            className="hidden md:flex shrink-0 w-[140px] h-[140px] rounded-full bg-gradient-to-br from-primary/15 via-primary/10 to-primary/5 items-center justify-center border border-primary/20 shadow-inner self-center"
            aria-hidden="true"
          >
            <BookOpen className="h-16 w-16 text-primary-accent/80" />
          </div>
        </div>

        {/* Purchase / Enrolment Bar for community or paid courses */}
        {(isPaid || isCommunity) && (
          <div className="flex flex-wrap items-center justify-between gap-3 pt-3 border-t border-border-subtle/60">
            <div className="flex items-center gap-2">
              {isPaid ? (
                <span className="text-base font-extrabold text-primary-accent font-mono">
                  ₫{effectivePrice.toLocaleString("vi-VN")}
                </span>
              ) : (
                <Badge variant="outline" className="text-xs">
                  {t("studio.courses.freeBadge", "Free Community Course")}
                </Badge>
              )}
              {isOwned && (
                <Badge variant="success" className="gap-1 text-xs">
                  <CheckCircle2 className="h-3.5 w-3.5" />
                  {t("learn.courseOwned", "Enrolled")}
                </Badge>
              )}
            </div>

            {!isOwned && onBuyOrClaim && (
              <Button
                variant="primary"
                size="sm"
                onClick={onBuyOrClaim}
                className="gap-1.5 text-xs font-bold"
              >
                {isPaid ? (
                  <>
                    <ShoppingBag className="h-3.5 w-3.5" />
                    {t("learn.buyCourseBtn", {
                      price: effectivePrice.toLocaleString("vi-VN"),
                      defaultValue: `Buy Course • ₫${effectivePrice.toLocaleString("vi-VN")}`,
                    })}
                  </>
                ) : (
                  <>
                    <CheckCircle2 className="h-3.5 w-3.5" />
                    {t("learn.claimCourseBtn", "Enroll for Free")}
                  </>
                )}
              </Button>
            )}
          </div>
        )}
      </CardHeader>
    </Card>
  );
};
