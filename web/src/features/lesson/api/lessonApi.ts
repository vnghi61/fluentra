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

  /** Get full syllabus for a specific course */
  async getCourse(id: string): Promise<CourseDetail> {
    return apiFetch<CourseDetail>(`/api/v1/courses/${id}`);
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

export function useCourse(id?: string) {
  return useQuery({
    queryKey: id ? lessonKeys.course(id) : lessonKeys.courses(),
    queryFn: () => (id ? lessonApi.getCourse(id) : Promise.reject(new Error("No course id"))),
    enabled: Boolean(id),
  });
}

export function useLesson(id?: string) {
  return useQuery({
    queryKey: id ? lessonKeys.lesson(id) : lessonKeys.all,
    queryFn: () => (id ? lessonApi.getLesson(id) : Promise.reject(new Error("No lesson id"))),
    enabled: Boolean(id),
  });
}
