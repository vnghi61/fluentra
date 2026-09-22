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
};

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
