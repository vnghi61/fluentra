import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { renditionsPending, resourceApi, type Resource } from "../api/resourceApi";
import { resourceKeys } from "../api/keys";

/**
 * The learner's own material: the list, one resource, and the two mutations a
 * page needs. Query keys live in `api/keys.ts` so invalidation is greppable.
 */

/** Every resource the caller owns. The quota is 50, so one page holds them all. */
export function useResources(enabled = true) {
  return useQuery({
    queryKey: resourceKeys.list(),
    queryFn: () => resourceApi.listResources(1, 100),
    enabled,
  });
}

/**
 * One resource, polled while the pipeline is still working on it.
 *
 * A file is validated synchronously but its renditions, extraction and
 * classification run in GitHub Actions, so minutes of "processing" are normal
 * and a screen that stopped asking would show a preview that never arrives.
 * Polling stops at the first settled answer.
 */
export function useResource(id: string) {
  return useQuery({
    queryKey: resourceKeys.detail(id),
    queryFn: () => resourceApi.getResource(id),
    enabled: id !== "",
    refetchInterval: (query) => {
      const resource = query.state.data;
      if (!resource) return false;
      if (resource.status === "pending" || resource.status === "uploaded") {
        return 3000;
      }
      if (renditionsPending(resource)) return 5000;
      return false;
    },
  });
}

/**
 * Intent → direct-to-storage PUT → confirm, in one mutation.
 *
 * The bytes never pass through the API: the intent pins the type and size, the
 * PUT goes to the object store, and confirming is what queues validation.
 */
export function useUploadResource() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (file: File) => {
      const intent = await resourceApi.createUploadIntent(
        file.name,
        file.type || "application/octet-stream",
      );
      await resourceApi.uploadDirect(intent, file);
      await resourceApi.confirmUpload(intent.id);
      return intent.id;
    },
    onSuccess: (id) => {
      void queryClient.invalidateQueries({ queryKey: resourceKeys.all });
      void queryClient.invalidateQueries({
        queryKey: resourceKeys.detail(id),
      });
    },
  });
}

/** Delete a resource and its stored object. */
export function useDeleteResource() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => resourceApi.deleteResource(id),
    onSuccess: (_result, id) => {
      queryClient.removeQueries({ queryKey: resourceKeys.detail(id) });
      void queryClient.invalidateQueries({ queryKey: resourceKeys.list() });
    },
  });
}

/**
 * The practice generated from one resource, polled while it is being built.
 *
 * Generation is ten model calls, which outlives an HTTP request, so the POST
 * only queues it and this is what watches it finish.
 */
export function useResourcePractice(resourceId: string, enabled = true) {
  return useQuery({
    queryKey: resourceKeys.practice(resourceId),
    queryFn: () => resourceApi.getResourcePractice(resourceId),
    enabled: enabled && resourceId !== "",
    refetchInterval: (query) => {
      const set = query.state.data;
      if (!set) return false;
      return set.status === "generating" ? 3000 : false;
    },
  });
}

/** Ask for a set; the caller navigates to the runner when it accepts. */
export function useStartResourcePractice() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => resourceApi.startResourcePractice(id),
    onSuccess: (set, id) => {
      queryClient.setQueryData(resourceKeys.practice(id), set);
    },
  });
}

/** How much of the per-user quota the list uses. */
export function quotaUsage(resources: Resource[]): {
  count: number;
  bytes: number;
} {
  let count = 0;
  let bytes = 0;
  for (const resource of resources) {
    // Rejected and failed resources do not count against the quota
    // (BR-RESOURCE-08).
    if (resource.status === "rejected" || resource.status === "failed") {
      continue;
    }
    count++;
    bytes += resource.byte_size ?? 0;
  }
  return { count, bytes };
}
