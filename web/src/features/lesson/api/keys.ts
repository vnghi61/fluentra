export const lessonKeys = {
  all: ["lesson"] as const,
  courses: () => [...lessonKeys.all, "courses"] as const,
  course: (id: string) => [...lessonKeys.courses(), id] as const,
  lesson: (id: string) => [...lessonKeys.all, "detail", id] as const,
};
