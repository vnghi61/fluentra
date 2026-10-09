import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import {
  materialState,
  renditionsPending,
  resourceApi,
  type MaterialKind,
  type MaterialUploadState,
  type Resource,
} from "../api/resourceApi";
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
    refetchOnMount: "always",
    refetchOnWindowFocus: "always",
    refetchInterval: (query) =>
      query.state.data?.items.some(
        (resource) =>
          resource.status === "pending" ||
          resource.status === "uploaded" ||
          renditionsPending(resource),
      )
        ? 5000
        : false,
  });
}

/**
 * One resource, polled while the pipeline is still working on it.
 *
 * Validation, rendition/extraction and classification run asynchronously in
 * separate jobs. Keep asking while the server reports pending work.
 * Polling stops at the first settled answer.
 */
export function useResource(id: string) {
  return useQuery({
    queryKey: resourceKeys.detail(id),
    queryFn: () => resourceApi.getResource(id),
    enabled: id !== "",
    refetchOnMount: "always",
    refetchInterval: (query) => {
      const resource = query.state.data;
      if (!resource) return false;
      if (resource.status === "pending" || resource.status === "uploaded") {
        return 3000;
      }
      if (renditionsPending(resource)) return 5000;
      if (
        (resource.extraction?.char_count ?? 0) > 0 &&
        !resource.classification
      )
        return 5000;
      return false;
    },
  });
}

/** Recheck existing content without reuploading the original file. */
export function useRevalidateResource() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => resourceApi.revalidateResource(id),
    onSuccess: (_result, id) => {
      void queryClient.invalidateQueries({ queryKey: resourceKeys.detail(id) });
      void queryClient.invalidateQueries({ queryKey: resourceKeys.list() });
      queryClient.removeQueries({ queryKey: resourceKeys.practice(id) });
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
    retry: false,
    refetchOnMount: "always",
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

/**
 * A course material's processing state, asked every three seconds until it
 * settles. A reopened draft has nobody else asking, so the editor leans on this
 * rather than on a state it saved before the renditions finished.
 */
export function useMaterialStatus(
  resourceId: string | undefined,
  materialKind: MaterialKind,
): MaterialUploadState | undefined {
  const query = useQuery({
    queryKey: [...resourceKeys.detail(resourceId ?? ""), "material"],
    queryFn: () => resourceApi.getResource(resourceId ?? ""),
    enabled: Boolean(resourceId),
    retry: 1,
    refetchInterval: (q) =>
      q.state.data && materialState(q.state.data, materialKind) !== "processing"
        ? false
        : 3000,
  });
  if (!resourceId) return undefined;
  if (query.isError) return "failed";
  return query.data ? materialState(query.data, materialKind) : undefined;
}
