import { useQuery } from "@tanstack/react-query";
import { adminApi, type ReviewQueueParams } from "./adminApi";

export const reviewQueueKeys = {
  all: ["admin", "review-queue"] as const,
  list: (params: ReviewQueueParams) =>
    ["admin", "review-queue", "list", params] as const,
};

export function useAdminReviewQueue(params: ReviewQueueParams) {
  return useQuery({
    queryKey: reviewQueueKeys.list(params),
    queryFn: () => adminApi.listReviewQueue(params),
  });
}
