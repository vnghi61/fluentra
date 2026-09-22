import { apiFetch } from "@/api/client";
import { reachableStorageUrl } from "@/lib/storage-url";
import type { components } from "@/types/api";

/**
 * The learner-resource intake surface, which Studio uses to attach a document
 * or video to a lesson.
 *
 * This lives in `features/resource` rather than inside Studio because the flow
 * — intent, direct-to-storage PUT, confirm, poll — is the resource module's, and
 * a second copy of it would drift from the first.
 */
export type ResourceUploadIntent =
  components["schemas"]["ResourceUploadIntentResponse"];
export type Resource = components["schemas"]["Resource"];
export type ResourceRendition = components["schemas"]["ResourceRendition"];
export type ResourceSubmit = components["schemas"]["ResourceSubmitResponse"];
export type ResourceListResponse = components["schemas"]["ResourceList"];
export type ResourceExtraction = components["schemas"]["ResourceExtraction"];
export type ResourceClassification =
  components["schemas"]["ResourceClassification"];
export type ClassificationNode = components["schemas"]["ClassificationNode"];
export type ResourcePracticeSet = components["schemas"]["ResourcePracticeSet"];
export type ResourcePracticeActivity = components["schemas"]["LessonActivity"];

/**
 * The types a learner may upload: `resource/domain/mime.go`'s allow-list,
 * written out because the browser needs it before the request is made. A file
 * outside it would be rejected by the server after a 50 MB upload, which is a
 * slow way to learn.
 */
export const RESOURCE_ACCEPT =
  ".pdf,.doc,.docx,.ppt,.pptx,.png,.jpg,.jpeg,.webp,.mp3,.wav,.m4a,.mp4,.webm";

export const RESOURCE_MAX_BYTES = 50 * 1024 * 1024;

/** The per-user quotas from `resource/AGENT.md` §9 (BR-RESOURCE-08). */
export const RESOURCE_QUOTA_COUNT = 50;
export const RESOURCE_QUOTA_BYTES = 250 * 1024 * 1024;

/** The rendition kinds a material must have ready before it can be published. */
const REQUIRED_RENDITIONS: Record<"document" | "video", string[]> = {
  video: ["video_360p"],
  document: ["preview"],
};

export const resourceApi = {
  /** Reserve a resource id and a signed upload target in fluentra-uploads. */
  createUploadIntent(
    filename: string,
    contentType: string,
  ): Promise<ResourceUploadIntent> {
    return apiFetch<ResourceUploadIntent>(
      "/api/v1/me/resources/upload-intent",
      {
        method: "POST",
        body: JSON.stringify({ filename, content_type: contentType }),
      },
    );
  },

  /**
   * Upload the bytes straight to storage. The API never sees them, and the
   * signed URL is reached through `reachableStorageUrl` so a phone on the LAN
   * can upload in development.
   */
  async uploadDirect(intent: ResourceUploadIntent, file: File): Promise<void> {
    const response = await fetch(reachableStorageUrl(intent.upload_url), {
      method: "PUT",
      body: file,
      headers: { "Content-Type": file.type },
    });
    if (!response.ok) {
      throw new Error(
        `Direct storage upload failed with status ${response.status}`,
      );
    }
  },

  /** Confirm the upload; validation and rendition planning are queued. */
  confirmUpload(resourceId: string): Promise<ResourceSubmit> {
    return apiFetch<ResourceSubmit>("/api/v1/me/resources", {
      method: "POST",
      body: JSON.stringify({ resource_id: resourceId }),
    });
  },

  /** One resource, with its renditions and their statuses. */
  getResource(id: string): Promise<Resource> {
    return apiFetch<Resource>(`/api/v1/me/resources/${id}`);
  },

  /** The caller's resources, newest first. */
  listResources(page = 1, pageSize = 100): Promise<ResourceListResponse> {
    return apiFetch<ResourceListResponse>(
      `/api/v1/me/resources?page=${page}&page_size=${pageSize}`,
    );
  },

  /** Delete a resource and its stored object. */
  deleteResource(id: string): Promise<void> {
    return apiFetch<void>(`/api/v1/me/resources/${id}`, { method: "DELETE" });
  },

  /**
   * Ask for practice from this file. Answers 202 with the set, which may still
   * be generating; the runner polls until it is ready.
   */
  startResourcePractice(id: string): Promise<ResourcePracticeSet> {
    return apiFetch<ResourcePracticeSet>(
      `/api/v1/me/resources/${id}/practice`,
      { method: "POST" },
    );
  },

  /** The practice generated from this file. */
  getResourcePractice(id: string): Promise<ResourcePracticeSet> {
    return apiFetch<ResourcePracticeSet>(`/api/v1/me/resources/${id}/practice`);
  },
};

/** The kinds of derived form a file resource may have. */
export function resourceFamily(mime: string): string {
  const base = mime.split(";")[0]?.trim().toLowerCase() ?? "";
  if (base === "application/pdf") return "document";
  if (base.startsWith("image/")) return "image";
  if (base.startsWith("audio/")) return "audio";
  if (base.startsWith("video/")) return "video";
  return "document";
}

/**
 * Whether a validated file is still having its derived forms built.
 *
 * The API only attaches renditions that are ready, so an empty list on a
 * validated file is either "the pipeline has not finished" or "this kind needs
 * none". Every family except a URL gets at least one, which is what makes the
 * inference safe.
 */
export function renditionsPending(resource: Resource): boolean {
  if (resource.status !== "validated" || resource.kind !== "file") {
    return false;
  }
  return (resource.renditions?.length ?? 0) === 0;
}

/** Whether a resource is validated and every rendition the runner needs is ready. */
export function materialReady(
  resource: Resource,
  materialKind: "document" | "video",
): boolean {
  if (resource.status !== "validated") return false;
  // The API only attaches renditions that are ready, each with a presigned URL,
  // so a present URL is what "ready" means here.
  const ready = new Set<string>(
    (resource.renditions ?? [])
      .filter((r) => Boolean(r.url))
      .map((r) => r.kind),
  );
  return REQUIRED_RENDITIONS[materialKind].every((kind) => ready.has(kind));
}

/** A learner-facing state for the upload status line. */
export type MaterialUploadState =
  "idle" | "uploading" | "processing" | "ready" | "failed";

/**
 * Polls a resource until its renditions settle, reporting each state. Returns a
 * stop function; the caller clears it on unmount.
 */
export function pollMaterial(
  resourceId: string,
  materialKind: "document" | "video",
  onState: (state: MaterialUploadState) => void,
  intervalMs = 3000,
): () => void {
  let stopped = false;
  let timer: ReturnType<typeof setTimeout> | null = null;

  const tick = async () => {
    if (stopped) return;
    try {
      const resource = await resourceApi.getResource(resourceId);
      if (stopped) return;
      if (resource.status === "rejected" || resource.status === "failed") {
        onState("failed");
        return;
      }
      if (materialReady(resource, materialKind)) {
        onState("ready");
        return;
      }
      onState("processing");
    } catch {
      if (stopped) return;
      onState("failed");
      return;
    }
    timer = setTimeout(() => void tick(), intervalMs);
  };

  void tick();
  return () => {
    stopped = true;
    if (timer) clearTimeout(timer);
  };
}
