import type { SearchContentParams } from "./adminApi";

export const adminContentKeys = {
  lists: ["admin", "content", "list"] as const,
  summaries: ["admin", "content", "summary"] as const,
  list: (params: SearchContentParams) =>
    ["admin", "content", "list", params] as const,
  summary: (params: Pick<SearchContentParams, "kind" | "q">) =>
    ["admin", "content", "summary", params] as const,
};
