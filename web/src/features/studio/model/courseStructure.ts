/**
 * The course outline the editor holds, the rules a submission must meet, and
 * the pricing bounds. Pure functions: the editor component only wires them up.
 *
 * The rules mirror Gate 1 (internal/modules/studio/service): a lesson holds at
 * least one material or 3-30 exercises, a course holds three lessons and twenty
 * exercises with materials not counted (BR-STUDIO-11), and a price is 0 or
 * inside the configured bounds (BR-STUDIO-01).
 */
import {
  emptyFields,
  readActivity,
  serialiseActivity,
  type ActivityKind,
  type DraftActivity,
} from "./activityKinds";

export interface LessonItem {
  id: string;
  title: string;
  description?: string | undefined;
  activities: DraftActivity[];
}

export interface UnitItem {
  id: string;
  title: string;
  description?: string | undefined;
  lessons: LessonItem[];
}

export const LIMITS = {
  units: 12,
  lessonsPerUnit: 20,
  activitiesPerLesson: 30,
  minLessons: 3,
  minExercises: 20,
  minExercisesPerLesson: 3,
} as const;

/** BR-STUDIO-01, as the server's defaults (`studio.min_price_vnd`). */
export const PRICE = {
  min: 5_000,
  max: 5_000_000,
  step: 1_000,
  creatorShare: 0.7,
  suggestions: [19_000, 49_000, 99_000, 199_000],
} as const;

export const CEFR_LEVELS = ["A1", "A2", "B1", "B2", "C1", "C2"] as const;

export const newId = (prefix: string): string =>
  `${prefix}-${crypto.randomUUID()}`;

export function newActivity(kind: ActivityKind, title: string): DraftActivity {
  return { id: newId("a"), kind, title, fields: emptyFields() };
}

export function newLesson(title: string): LessonItem {
  return { id: newId("l"), title, activities: [] };
}

export function newUnit(title: string): UnitItem {
  return { id: newId("u"), title, lessons: [] };
}

/**
 * A URL slug from a title, Vietnamese included: "Giao tiếp cơ bản" becomes
 * "giao-tiep-co-ban" rather than losing every accented letter.
 */
export function slugify(text: string): string {
  return text
    .normalize("NFD")
    .replace(/[̀-ͯ]/g, "")
    .replace(/đ/gi, "d")
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/(^-|-$)/g, "")
    .slice(0, 80);
}

export const isMaterial = (a: DraftActivity): boolean =>
  a.kind === "lesson_material";

export function lessonCounts(lesson: LessonItem): {
  exercises: number;
  materials: number;
} {
  const materials = lesson.activities.filter(isMaterial).length;
  return { exercises: lesson.activities.length - materials, materials };
}

export function lessonValid(lesson: LessonItem): boolean {
  const { exercises, materials } = lessonCounts(lesson);
  return (
    materials > 0 ||
    (exercises >= LIMITS.minExercisesPerLesson &&
      exercises <= LIMITS.activitiesPerLesson)
  );
}

export interface CourseMetrics {
  units: number;
  lessons: number;
  exercises: number;
  materials: number;
  invalidLessons: number;
  materialsNotReady: number;
}

export function courseMetrics(units: UnitItem[]): CourseMetrics {
  const m: CourseMetrics = {
    units: units.length,
    lessons: 0,
    exercises: 0,
    materials: 0,
    invalidLessons: 0,
    materialsNotReady: 0,
  };
  for (const unit of units) {
    for (const lesson of unit.lessons) {
      m.lessons += 1;
      const c = lessonCounts(lesson);
      m.exercises += c.exercises;
      m.materials += c.materials;
      if (!lessonValid(lesson)) m.invalidLessons += 1;
      // A material may be saved while it processes, but not submitted: Gate 1's
      // material_ready would reject it (WO 20 Stage B.1).
      m.materialsNotReady += lesson.activities.filter(
        (a) =>
          isMaterial(a) &&
          (!a.fields.resourceId.trim() || a.fields.materialStatus !== "ready"),
      ).length;
    }
  }
  return m;
}

export function priceInBounds(isFree: boolean, priceVnd: number): boolean {
  return (
    isFree ||
    (Number.isInteger(priceVnd) &&
      priceVnd >= PRICE.min &&
      priceVnd <= PRICE.max)
  );
}

/** Reads a saved draft's structure, tolerating every shape the editor ever wrote. */
export function readUnits(structure: unknown): UnitItem[] | null {
  const units = (structure as { units?: unknown } | undefined)?.units;
  if (!Array.isArray(units) || units.length === 0) return null;
  return (units as UnitItem[]).map((unit) => ({
    ...unit,
    lessons: (unit.lessons ?? []).map((lesson) => ({
      ...lesson,
      activities: (lesson.activities ?? []).map((raw, i) =>
        readActivity(raw, `${lesson.id}-a${i}`),
      ),
    })),
  }));
}

export function serialiseUnits(units: UnitItem[]): Record<string, unknown> {
  return {
    units: units.map((unit) => ({
      ...unit,
      lessons: unit.lessons.map((lesson) => ({
        ...lesson,
        activities: lesson.activities.map(serialiseActivity),
      })),
    })),
  };
}

/** Moves the item at `from` one step up or down, in place. */
export function moveItem<T>(list: T[], from: number, delta: -1 | 1): void {
  const to = from + delta;
  if (to < 0 || to >= list.length) return;
  const [item] = list.splice(from, 1);
  if (item !== undefined) list.splice(to, 0, item);
}
