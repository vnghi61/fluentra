import React from "react";
import { useNavigate } from "@tanstack/react-router";
import { BookOpen } from "lucide-react";
import { useTranslation } from "react-i18next";

import { useAuthStore } from "@/stores/authStore";

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
import { GuestNotice, learningApi } from "@/features/learning";

export function LearnPage(): React.JSX.Element {
  const { t } = useTranslation();
  const signedIn = useAuthStore((state) => state.status === "authenticated");
  const navigate = useNavigate();
  const [enrolErr, setEnrolErr] = React.useState<Error | null>(null);
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
  const selectedCourseSlug: string | undefined = undefined;

  // Default to first course if none explicitly selected. Addressed by slug: the
  // detail endpoint is `/courses/{slug}`, so passing `courses[0].id` here made
  // every syllabus a 404 the moment the catalogue stopped being empty.
  const activeCourseSlug = selectedCourseSlug ?? courses[0]?.slug;

  const {
    data: activeCourse,
    isLoading: activeCourseLoading,
    isError: activeCourseError,
    error: activeCourseErr,
    refetch: refetchActiveCourse,
  } = useCourse(activeCourseSlug);

  // Enrolment is the learner's half of `StartAttempt`'s precondition, and this
  // is the only screen that holds the course id it needs. Doing it here rather
  // than in the runner keeps the lesson URL free of a course parameter, which
  // is what the route declares and what the journeys assert.
  const activeCourseId = activeCourse?.id;
  const startLesson = React.useCallback(
    (lessonId: string): void => {
      void (async () => {
        // A guest enrols in nothing. Enrolment is a row against a person, so
        // the call 401s for them — and because a failed enrolment stops the
        // navigation, every "Start Lesson" button on this page did nothing at
        // all for a signed-out visitor. The lesson itself is public; only the
        // bookkeeping needs an account.
        if (signedIn) {
          try {
            if (activeCourseId) await learningApi.enrollCourse(activeCourseId);
            setEnrolErr(null);
          } catch (err) {
            setEnrolErr(err instanceof Error ? err : new Error(String(err)));
            return;
          }
        }
        await navigate({ to: `/learn/lesson/${lessonId}` });
      })();
    },
    [activeCourseId, navigate, signedIn],
  );

  if (coursesLoading || (activeCourseSlug && activeCourseLoading)) {
    return <LearnSkeleton />;
  }

  if (coursesError || activeCourseError || enrolErr) {
    return (
      <LearnError
        onRetry={() => {
          setEnrolErr(null);
          void refetchCourses();
          if (activeCourseSlug) void refetchActiveCourse();
        }}
        error={courseListErr || activeCourseErr || enrolErr}
      />
    );
  }

  if (courses.length === 0) {
    return (
      <div className="py-12 max-w-xl mx-auto">
        <Card className="text-center p-8">
          <CardHeader>
            <div className="flex justify-center mb-3">
              <BookOpen
                className="h-12 w-12 text-primary-accent"
                aria-hidden="true"
              />
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
        error={
          new Error(
            t(
              "page.courseSyllabusCouldNotBe",
              "Course syllabus could not be found.",
            ),
          )
        }
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
      {/*
        The page's own heading, which it did not have.

        The course title is rendered by CourseHeader through CardTitle, and
        CardTitle is an <h3>. So this page opened at h3, put the unit names at
        h2 *below* it, and had no h1 anywhere — a broken outline that mattered
        little while /learn sat behind a login form and matters a lot now that
        it is the first screen every visitor sees.

        Visually quiet on purpose: the course title is still the thing being
        looked at. This is the page's name, in the shape the other four routes
        already use.
      */}
      <h1 className="text-sm font-semibold uppercase tracking-wider text-text-muted">
        {t("learn.pageTitle", "Course catalogue")}
      </h1>

      {!signedIn && <GuestNotice />}

      {/* Course Header & Info */}
      <CourseHeader
        course={activeCourse}
        totalLessons={totalLessons}
        completedLessons={0}
      />

      {/* Syllabus Unit & Lesson List */}
      <UnitList units={activeCourse.units} onStartLesson={startLesson} />
    </div>
  );
}

export default LearnPage;
