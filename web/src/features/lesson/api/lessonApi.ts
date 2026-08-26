import { useQuery } from "@tanstack/react-query";

import { apiFetch } from "@/api/client";
import type { components } from "@/types/api";
import { lessonKeys } from "./keys";

export type CourseSummary = components["schemas"]["CourseSummary"];
export type CourseDetail = components["schemas"]["CourseDetail"];
export type CourseUnit = components["schemas"]["CourseUnit"];
export type LessonSummary = components["schemas"]["LessonSummary"];
export type LessonDetail = components["schemas"]["LessonDetail"];
export type LessonActivity = components["schemas"]["LessonActivity"];
export type CourseList = components["schemas"]["CourseList"];

export const lessonApi = {
  /** List all published courses */
  async listCourses(): Promise<CourseList> {
    return apiFetch<CourseList>("/api/v1/courses");
  },

  /** Get full syllabus for a specific course, addressed by slug. */
  async getCourse(slug: string): Promise<CourseDetail> {
    // The path parameter is `{slug}`, not `{id}` — openapi.yaml declares
    // `getCourseBySlug`, and the repository looks the course up by slug alone.
    // A course id here is a 404 for every course that exists.
    return apiFetch<CourseDetail>(`/api/v1/courses/${slug}`);
  },

  /** Get lesson detail with activities */
  async getLesson(id: string): Promise<LessonDetail> {
    return apiFetch<LessonDetail>(`/api/v1/lessons/${id}`);
  },
};

export function useCourses() {
  return useQuery({
    queryKey: lessonKeys.courses(),
    queryFn: () => lessonApi.listCourses(),
  });
}

// A disabled query still registers its options against its key, so a fallback
// key that collides with another hook's lets the rejecting queryFn below
// overwrite that hook's. `useCourse(undefined)` fell back to
// `lessonKeys.courses()` — the exact key `useCourses` owns — and the syllabus
// then rendered "Unable to Load Curriculum / No course id" over a catalogue it
// had already fetched successfully. A private placeholder segment cannot collide.
const noArgument = "__none__";

export function useCourse(slug?: string) {
  return useQuery({
    queryKey: lessonKeys.course(slug ?? noArgument),
    queryFn: () =>
      slug
        ? lessonApi.getCourse(slug)
        : Promise.reject(new Error("No course slug")),
    enabled: Boolean(slug),
  });
}

export function useLesson(id?: string) {
  return useQuery({
    queryKey: lessonKeys.lesson(id ?? noArgument),
    queryFn: () =>
      id ? lessonApi.getLesson(id) : Promise.reject(new Error("No lesson id")),
    enabled: Boolean(id),
  });
}
