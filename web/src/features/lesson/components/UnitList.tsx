import React from "react";
import { FolderCheck } from "lucide-react";
import { useTranslation } from "react-i18next";

import {
  Card,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import type { CourseUnit } from "../api/lessonApi";
import { LessonRow } from "./LessonRow";

export interface UnitListProps {
  units: CourseUnit[];
  completedLessonIds?: Set<string>;
  nextLessonId?: string | null;
}

export const UnitList: React.FC<UnitListProps> = ({
  units,
  completedLessonIds = new Set(),
  nextLessonId,
}) => {
  const { t } = useTranslation();

  if (units.length === 0) {
    return (
      <Card className="text-center py-12">
        <CardHeader>
          <div className="flex justify-center mb-2">
            <FolderCheck className="h-10 w-10 text-text-muted" aria-hidden="true" />
          </div>
          <CardTitle className="text-lg font-bold">
            {t("learn.noUnitsTitle", "Curriculum in Preparation")}
          </CardTitle>
          <CardDescription>
            {t(
              "learn.noUnitsDesc",
              "Units and lessons for this course are currently being authored. Please check back soon.",
            )}
          </CardDescription>
        </CardHeader>
      </Card>
    );
  }

  return (
    <div className="space-y-8">
      {units.map((unit) => {
        return (
          <section
            key={unit.id}
            aria-labelledby={`unit-heading-${unit.id}`}
            className="space-y-4"
          >
            {/* Unit Header Card */}
            <div className="border-b border-border pb-3">
              <div className="text-xs font-bold text-primary-accent uppercase tracking-wider mb-1">
                {t("learn.unitPrefix", "Unit")} {unit.position}
              </div>
              <h2
                id={`unit-heading-${unit.id}`}
                className="text-xl md:text-2xl font-bold text-text"
              >
                {unit.title}
              </h2>
              {unit.description && (
                <p className="text-sm text-text-muted mt-1">
                  {unit.description}
                </p>
              )}
            </div>

            {/* Lesson Rows */}
            <div className="space-y-3">
              {unit.lessons && unit.lessons.length > 0 ? (
                unit.lessons.map((lesson) => {
                  const isCompleted = completedLessonIds.has(lesson.id);
                  const isNext = lesson.id === nextLessonId;

                  return (
                    <LessonRow
                      key={lesson.id}
                      lesson={lesson}
                      isCompleted={isCompleted}
                      isNext={isNext}
                    />
                  );
                })
              ) : (
                <p className="text-xs text-text-muted italic py-2">
                  {t("learn.emptyUnit", "No lessons in this unit yet.")}
                </p>
              )}
            </div>
          </section>
        );
      })}
    </div>
  );
};
