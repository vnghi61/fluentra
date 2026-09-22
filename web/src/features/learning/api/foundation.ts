import { useQuery } from "@tanstack/react-query";

import { apiFetch } from "@/api/client";
import type { components } from "@/types/api";

export type FoundationTopicDetail =
  components["schemas"]["FoundationTopicDetail"];
export type LearnerFoundationPath =
  components["schemas"]["LearnerFoundationPath"];
export type FoundationPathNode = components["schemas"]["FoundationPathNode"];
export type FoundationPath = components["schemas"]["FoundationPath"];

export const foundationApi = {
  /** Fetch detailed view of a foundation topic by canonical code */
  async getTopic(code: string): Promise<FoundationTopicDetail> {
    return apiFetch<FoundationTopicDetail>(
      `/api/v1/foundation/topics/${encodeURIComponent(code)}`,
    );
  },

  /** Fetch topologically sorted learning path with learner's mastery */
  async getLearnerPath(target: string): Promise<LearnerFoundationPath> {
    return apiFetch<LearnerFoundationPath>(
      `/api/v1/me/foundation/path?target=${encodeURIComponent(target)}`,
    );
  },

  /** Fetch public topologically sorted foundation path */
  async getPublicPath(target: string): Promise<FoundationPath> {
    return apiFetch<FoundationPath>(
      `/api/v1/foundation/path?target=${encodeURIComponent(target)}`,
    );
  },

  /** Fetch the next recommended topic across strands for the signed-in learner */
  async getNextTopic(): Promise<FoundationPathNode> {
    return apiFetch<FoundationPathNode>("/api/v1/me/foundation/next");
  },
};

export function useFoundationTopic(code: string) {
  return useQuery({
    queryKey: ["foundation", "topic", code],
    queryFn: () => foundationApi.getTopic(code),
    enabled: Boolean(code),
  });
}

export function useLearnerFoundationPath(target: string, enabled = true) {
  return useQuery({
    queryKey: ["foundation", "learner-path", target],
    queryFn: () => foundationApi.getLearnerPath(target),
    enabled: Boolean(target) && enabled,
  });
}

export function usePublicFoundationPath(target: string, enabled = true) {
  return useQuery({
    queryKey: ["foundation", "public-path", target],
    queryFn: () => foundationApi.getPublicPath(target),
    enabled: Boolean(target) && enabled,
  });
}

export function useFoundationNext(enabled = true) {
  return useQuery({
    queryKey: ["foundation", "next"],
    queryFn: () => foundationApi.getNextTopic(),
    enabled,
  });
}
