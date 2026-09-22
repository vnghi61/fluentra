export const resourceKeys = {
  all: ["resources"] as const,
  list: () => [...resourceKeys.all, "list"] as const,
  detail: (id: string) => [...resourceKeys.all, "detail", id] as const,
  practice: (id: string) => [...resourceKeys.all, "practice", id] as const,
};
