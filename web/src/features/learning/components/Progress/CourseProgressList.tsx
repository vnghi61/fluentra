import React from "react";
import { Link } from "@tanstack/react-router";
import { BookOpen, CheckCircle2, ChevronRight } from "lucide-react";
import { useTranslation } from "react-i18next";

import { Card, CardContent, CardHeader } from "@/components/ui/card";
import { cn } from "@/lib/utils";
import type { CourseProgress } from "../../api/learningApi";
import { ProgressBar } from "./ProgressBar";
import { NativeSelect } from "@/components/ui/native-select";

export interface CourseMeta {
  title: string;
  slug?: string;
}

export interface CourseProgressListProps {
  courses: CourseProgress[];
  courseMeta?: ReadonlyMap<string, CourseMeta>;
}

type FilterStatus = "all" | "in_progress" | "completed";

export const CourseProgressList: React.FC<CourseProgressListProps> = ({
  courses,
  courseMeta,
}) => {
  const { t } = useTranslation();
  const [filter, setFilter] = React.useState<FilterStatus>("all");

  const isCourseDone = (c: CourseProgress) =>
    c.status === "completed" || c.percentage >= 100;

  const filteredCourses = React.useMemo(() => {
    return courses.filter((course) => {
      const isDone = isCourseDone(course);
      if (filter === "completed") return isDone;
      if (filter === "in_progress") return !isDone;
      return true;
    });
  }, [courses, filter]);

  return (
    <Card className="border-border-subtle bg-surface-card">
      <CardHeader>
        <div className="flex items-center justify-between gap-3 flex-wrap">
          <h2 className="text-lg font-semibold leading-snug text-text md:text-xl">
            {t("progress.courses.title", "Your courses")}
          </h2>
          <div className="flex items-center gap-2">
            <label htmlFor="course-status-filter" className="sr-only">
              {t("progress.courses.filterLabel", "Filter courses")}
            </label>
            <NativeSelect
              id="course-status-filter"
              value={filter}
              onChange={(e) => setFilter(e.target.value as FilterStatus)}
              wrapperClassName="w-auto"
            >
              <option value="all">
                {t("progress.courses.filterAll", "All")}
              </option>
              <option value="in_progress">
                {t("progress.courses.filterInProgress", "In progress")}
              </option>
              <option value="completed">
                {t("progress.courses.filterCompleted", "Completed")}
              </option>
            </NativeSelect>
          </div>
        </div>
      </CardHeader>

      <CardContent>
        {courses.length === 0 ? (
          <div className="text-center py-8 text-text-muted space-y-4">
            <div className="h-12 w-12 rounded-xl bg-surface-muted flex items-center justify-center mx-auto text-text-muted">
              <BookOpen className="h-6 w-6" aria-hidden="true" />
            </div>
            <p className="text-base leading-relaxed">
              {t(
                "progress.courses.empty",
                "You haven't started any courses yet.",
              )}
            </p>
            <div>
              <Link
                to="/learn"
                className="inline-flex items-center justify-center gap-2 min-h-[44px] px-4 rounded-lg border border-border bg-transparent text-sm font-semibold text-text hover:bg-surface-muted transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary"
              >
                {t("progress.courses.explore", "Explore courses")}
              </Link>
            </div>
          </div>
        ) : filteredCourses.length === 0 ? (
          <div className="text-center py-8 text-text-muted">
            <p className="text-base leading-relaxed">
              {t(
                "progress.courses.filterEmpty",
                "No courses match this filter.",
              )}
            </p>
          </div>
        ) : (
          <ul
            className="space-y-3"
            aria-label={t("progress.courses.title", "Your courses")}
          >
            {filteredCourses.map((course, idx) => {
              const isDone = isCourseDone(course);
              const meta = courseMeta?.get(course.course_id);
              const originalIdx = courses.indexOf(course);
              const courseNum = (originalIdx >= 0 ? originalIdx : idx) + 1;
              const courseTitleText =
                meta?.title ||
                t("progress.courses.fallbackTitle", {
                  n: courseNum,
                  defaultValue: `Course ${courseNum}`,
                });
              const metaLineText = `${t("progress.courses.activityCount", {
                done: course.completed_activities,
                total: course.total_activities,
                defaultValue: `${course.completed_activities}/${course.total_activities} activities`,
              })} · ${course.percentage}%`;

              const rowClassName =
                "group grid grid-cols-[auto_1fr_auto] md:flex md:items-center gap-3 rounded-xl border border-border-subtle bg-surface-card p-3 min-h-[44px] transition-[transform,box-shadow] duration-200 hover:-translate-y-px hover:shadow-md motion-reduce:transition-none motion-reduce:hover:translate-y-0 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary";

              const rowContent = (
                <>
                  {/* Icon Tile */}
                  <div
                    className={cn(
                      "col-start-1 row-start-1 h-12 w-12 md:h-14 md:w-14 shrink-0 rounded-xl flex items-center justify-center",
                      isDone
                        ? "bg-success/10 text-success-accent"
                        : "bg-primary/10 text-primary-accent",
                    )}
                  >
                    {isDone ? (
                      <CheckCircle2 className="h-6 w-6" aria-hidden="true" />
                    ) : (
                      <BookOpen className="h-6 w-6" aria-hidden="true" />
                    )}
                  </div>

                  {/* Title + Meta + Bar Wrapper (contents on mobile, flex-1 min-w-0 on desktop) */}
                  <div className="contents md:flex md:flex-col md:flex-1 md:min-w-0 md:space-y-1">
                    {/* Title */}
                    <span className="col-start-2 row-start-1 self-center md:self-start font-semibold text-text text-base line-clamp-2 min-w-0">
                      {courseTitleText}
                    </span>

                    {/* Meta line */}
                    <div className="col-span-3 md:col-auto min-w-0 truncate text-sm text-text-muted tabular-nums">
                      {metaLineText}
                    </div>

                    {/* Progress Bar */}
                    <ProgressBar
                      value={course.percentage}
                      label={t("progress.courses.progressLabel", {
                        course: courseTitleText,
                        defaultValue: `${courseTitleText} progress`,
                      })}
                      barClassName={isDone ? "bg-success" : "bg-primary"}
                      className="w-full col-span-3 md:col-auto"
                    />
                  </div>

                  {/* Action */}
                  <div className="col-span-3 md:col-auto w-full md:w-auto shrink-0">
                    {isDone ? (
                      <span className="inline-flex items-center justify-center gap-1.5 whitespace-nowrap min-w-[8.5rem] w-full md:w-auto min-h-[44px] rounded-lg bg-success/10 text-success-accent text-sm font-semibold px-4">
                        <CheckCircle2 className="h-4 w-4" aria-hidden="true" />
                        {t("progress.courses.completed", "Completed")}
                      </span>
                    ) : (
                      <span className="inline-flex items-center justify-center whitespace-nowrap min-w-[8.5rem] w-full md:w-auto min-h-[44px] rounded-lg bg-primary text-primary-fg text-sm font-semibold px-4">
                        {t("progress.courses.continue", "Continue learning")}
                      </span>
                    )}
                  </div>

                  {/* Chevron Right */}
                  <ChevronRight
                    className="col-start-3 row-start-1 self-center h-5 w-5 text-text-muted shrink-0 group-hover:text-text transition-colors"
                    aria-hidden="true"
                  />
                </>
              );

              return (
                <li key={course.course_id || idx}>
                  {meta?.slug ? (
                    <Link
                      to="/learn"
                      search={{ course: meta.slug }}
                      className={rowClassName}
                    >
                      {rowContent}
                    </Link>
                  ) : (
                    <Link to="/learn" className={rowClassName}>
                      {rowContent}
                    </Link>
                  )}
                </li>
              );
            })}
          </ul>
        )}
      </CardContent>
    </Card>
  );
};
