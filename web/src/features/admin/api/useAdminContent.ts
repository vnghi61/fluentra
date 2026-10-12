import { useQuery } from "@tanstack/react-query";
import { adminApi, type SearchContentParams } from "./adminApi";
import { adminContentKeys } from "./contentKeys";

export function useAdminContent(params: SearchContentParams) {
  return useQuery({
    queryKey: adminContentKeys.list(params),
    queryFn: () => adminApi.listContent(params),
  });
}

export function useAdminContentSummary(
  params: Pick<SearchContentParams, "kind" | "q">,
) {
  return useQuery({
    queryKey: adminContentKeys.summary(params),
    queryFn: async () => {
      const [all, published, review] = await Promise.all([
        adminApi.listContent({ ...params, limit: 1, offset: 0 }),
        adminApi.listContent({
          ...params,
          status: "published",
          limit: 1,
          offset: 0,
        }),
        adminApi.listContent({
          ...params,
          status: "in_review",
          limit: 1,
          offset: 0,
        }),
      ]);
      const other = all.total - published.total - review.total;
      return {
        total: all.total,
        published: published.total,
        review: review.total,
        other: other >= 0 ? other : null,
      };
    },
    staleTime: 60_000,
  });
}
