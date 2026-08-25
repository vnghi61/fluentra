import React from "react";
import { BookOpen } from "lucide-react";
import { useTranslation } from "react-i18next";

import {
  Card,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import {
  CourseHeader,
  LearnError,
  LearnSkeleton,
  UnitList,
  useCourse,
  useCourses,
} from "@/features/lesson";

export function LearnPage(): React.JSX.Element {
  const { t } = useTranslation();
  const {
    data: courseListData,
    isLoading: coursesLoading,
    isError: coursesError,
    error: courseListErr,
    refetch: refetchCourses,
  } = useCourses();

  // `courses` is required on the catalogue response; it is absent only before
  // the query resolves, which the loading branch handles.
  const courses = courseListData?.courses ?? [];
  const selectedCourseId: string | undefined = undefined;

  // Default to first course if none explicitly selected
  const activeCourseId = selectedCourseId ?? courses[0]?.id;

  const {
    data: activeCourse,
    isLoading: activeCourseLoading,
    isError: activeCourseError,
    error: activeCourseErr,
    refetch: refetchActiveCourse,
  } = useCourse(activeCourseId);

  if (coursesLoading || (activeCourseId && activeCourseLoading)) {
    return <LearnSkeleton />;
  }

  if (coursesError || activeCourseError) {
    return (
      <LearnError
        onRetry={() => {
          void refetchCourses();
          if (activeCourseId) void refetchActiveCourse();
        }}
        error={courseListErr || activeCourseErr}
      />
    );
  }

  if (courses.length === 0) {
    return (
      <div className="py-12 max-w-xl mx-auto">
        <Card className="text-center p-8">
          <CardHeader>
            <div className="flex justify-center mb-3">
              <BookOpen className="h-12 w-12 text-primary-accent" aria-hidden="true" />
            </div>
            <CardTitle className="text-xl font-bold">
              {t("learn.emptyTitle", "No Courses Available Yet")}
            </CardTitle>
            <CardDescription className="mt-2">
              {t(
                "learn.emptyDesc",
                "Curriculum content is currently being prepared and published. Please check back shortly.",
              )}
            </CardDescription>
          </CardHeader>
        </Card>
      </div>
    );
  }

  if (!activeCourse) {
    return (
      <LearnError
        onRetry={() => void refetchCourses()}
        error={new Error("Course syllabus could not be found.")}
      />
    );
  }

  // Calculate total lessons across units
  const totalLessons = activeCourse.units.reduce(
    (acc, unit) => acc + (unit.lessons?.length || 0),
    0,
  );

  return (
    <div className="space-y-8 animate-in fade-in duration-200">
      {/* Course Header & Info */}
      <CourseHeader
        course={activeCourse}
        totalLessons={totalLessons}
        completedLessons={0}
      />

      {/* Syllabus Unit & Lesson List */}
      <UnitList units={activeCourse.units} />
    </div>
  );
}

export default LearnPage;
