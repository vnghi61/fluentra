import React from "react";
import { BookMarked, Clock, GraduationCap } from "lucide-react";
import { useTranslation } from "react-i18next";

import { Badge } from "@/components/ui/badge";
import {
  Card,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import type { CourseDetail } from "../api/lessonApi";

export interface CourseHeaderProps {
  course: CourseDetail;
  totalLessons?: number;
  completedLessons?: number;
}

export const CourseHeader: React.FC<CourseHeaderProps> = ({
  course,
  totalLessons = 0,
  completedLessons = 0,
}) => {
  const { t } = useTranslation();

  return (
    <Card className="border-primary/30 bg-gradient-to-r from-surface-card via-surface-card to-primary/5 shadow-sm">
      <CardHeader className="space-y-3">
        <div className="flex flex-wrap items-center justify-between gap-2">
          <div className="flex items-center gap-2 text-primary-accent">
            <GraduationCap className="h-5 w-5" aria-hidden="true" />
            <span className="text-xs font-bold uppercase tracking-wider">
              {t("learn.courseBadge", "Course Syllabus")}
            </span>
          </div>

          <div className="flex items-center gap-2">
            <Badge variant="primary" className="font-mono text-xs">
              {course.cefr_from} → {course.cefr_to}
            </Badge>
            {course.estimated_hours > 0 && (
              <span className="inline-flex items-center gap-1 text-xs text-text-muted">
                <Clock className="h-3.5 w-3.5" aria-hidden="true" />
                {course.estimated_hours}h
              </span>
            )}
          </div>
        </div>

        <div>
          <CardTitle className="text-2xl md:text-3xl font-extrabold text-text">
            {course.title}
          </CardTitle>
          {course.description && (
            <CardDescription className="mt-1 text-sm md:text-base">
              {course.description}
            </CardDescription>
          )}
        </div>

        {totalLessons > 0 && (
          <div className="flex items-center gap-2 text-xs text-text-muted pt-1">
            <BookMarked
              className="h-4 w-4 text-primary-accent"
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
        )}
      </CardHeader>
    </Card>
  );
};
