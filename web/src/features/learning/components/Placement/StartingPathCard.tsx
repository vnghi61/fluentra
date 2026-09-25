import React from "react";
import { Link } from "@tanstack/react-router";
import { BookOpen } from "lucide-react";
import { useTranslation } from "react-i18next";

import {
  Card,
  CardContent,
  CardDescription,
  CardFooter,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { useStartingPath } from "../../api/placement";
import { courseTitle } from "@/features/lesson/model/courseText";

const linkClass =
  "inline-flex min-h-[44px] items-center justify-center rounded-lg bg-primary px-6 text-base font-medium text-primary-fg hover:bg-primary-hover";

/** The recommended course for the learner's level and the lesson to start at. */
export const StartingPathCard: React.FC = () => {
  const { t } = useTranslation();
  const { data } = useStartingPath();
  const course = data?.courses[0];
  if (!data || !course) return null;

  const source =
    data.level_source === "placement"
      ? t("placement.path.fromPlacement", { level: data.level })
      : data.level_source === "declared"
        ? t("placement.path.fromDeclared", { level: data.level })
        : t("placement.path.fromDefault", { level: data.level });

  return (
    <Card className="border-border bg-surface-card">
      <CardHeader className="space-y-1">
        <div className="flex items-center gap-2">
          <BookOpen className="h-5 w-5 text-primary" aria-hidden="true" />
          <CardTitle className="text-lg font-bold text-text">
            {t("placement.path.title")}
          </CardTitle>
        </div>
        <CardDescription className="text-sm text-text-muted">
          {source}
        </CardDescription>
      </CardHeader>
      <CardContent className="space-y-1">
        <p className="text-base font-semibold text-text">
          {courseTitle(t, course)}
        </p>
        <p className="text-sm text-text-muted">
          {t("placement.path.range", {
            from: course.cefr_from,
            to: course.cefr_to,
          })}
        </p>
        {course.start_lesson && (
          <p className="text-sm text-text">
            {t("placement.path.startAt", { lesson: course.start_lesson.title })}
          </p>
        )}
      </CardContent>
      <CardFooter>
        {course.start_lesson ? (
          <Link
            to="/learn/lesson/$lessonId"
            params={{ lessonId: course.start_lesson.id }}
            className={linkClass}
          >
            {t("placement.path.start")}
          </Link>
        ) : (
          <Link to="/learn" className={linkClass}>
            {t("placement.path.browse")}
          </Link>
        )}
      </CardFooter>
    </Card>
  );
};
