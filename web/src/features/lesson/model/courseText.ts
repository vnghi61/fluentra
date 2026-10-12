import type { TFunction } from "i18next";

/** The parts of a course its card shows. */
interface CourseText {
  slug?: string | undefined;
  title: string;
  description?: string | null | undefined;
}

/**
 * A course's title in the reader's language (WO 22 Stage H). The catalogue
 * stores one title per course; a course with a translation under
 * `courses.<slug>` is shown in the reader's locale, and any other course keeps
 * the title it was authored with.
 */
export function courseTitle(t: TFunction, course: CourseText): string {
  if (!course.slug) return course.title;
  return t(`courses.${course.slug}.title`, { defaultValue: course.title });
}

/** A course's description in the reader's language; see courseTitle. */
export function courseDescription(t: TFunction, course: CourseText): string {
  const fallback = course.description ?? "";
  if (!course.slug) return fallback;
  return t(`courses.${course.slug}.description`, { defaultValue: fallback });
}
