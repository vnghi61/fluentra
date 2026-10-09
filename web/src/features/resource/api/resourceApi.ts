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
  ".pdf,.doc,.docx,.xls,.xlsx,.ppt,.pptx,.png,.jpg,.jpeg,.webp,.mp3,.wav,.m4a,.mp4,.webm";

export const RESOURCE_MAX_BYTES = 50 * 1024 * 1024;

/** What a course material is, derived from the file and never from the creator. */
export type MaterialKind = "document" | "image" | "video";

const MB = 1024 * 1024;
const DOC_MAX = 20 * MB;
const IMAGE_MAX = 10 * MB;

/**
 * The files a course material may be, by extension, with the MIME the server
 * expects to be declared and a per-kind ceiling under the server's 50 MB. The
 * server re-checks every byte (magic bytes, allow-list, quota); this table only
 * spares a creator a slow upload that would be refused. Some browsers report an
 * empty `File.type` for Office files, so the declared MIME comes from here.
 */
export const MATERIAL_TYPES: Record<
  string,
  { mime: string; kind: MaterialKind; maxBytes: number }
> = {
  pdf: { mime: "application/pdf", kind: "document", maxBytes: DOC_MAX },
  doc: { mime: "application/msword", kind: "document", maxBytes: DOC_MAX },
  docx: {
    mime: "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
    kind: "document",
    maxBytes: DOC_MAX,
  },
  xls: {
    mime: "application/vnd.ms-excel",
    kind: "document",
    maxBytes: DOC_MAX,
  },
  xlsx: {
    mime: "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
    kind: "document",
    maxBytes: DOC_MAX,
  },
  ppt: {
    mime: "application/vnd.ms-powerpoint",
    kind: "document",
    maxBytes: DOC_MAX,
  },
  pptx: {
    mime: "application/vnd.openxmlformats-officedocument.presentationml.presentation",
    kind: "document",
    maxBytes: DOC_MAX,
  },
  png: { mime: "image/png", kind: "image", maxBytes: IMAGE_MAX },
  jpg: { mime: "image/jpeg", kind: "image", maxBytes: IMAGE_MAX },
  jpeg: { mime: "image/jpeg", kind: "image", maxBytes: IMAGE_MAX },
  webp: { mime: "image/webp", kind: "image", maxBytes: IMAGE_MAX },
  mp4: { mime: "video/mp4", kind: "video", maxBytes: RESOURCE_MAX_BYTES },
  webm: { mime: "video/webm", kind: "video", maxBytes: RESOURCE_MAX_BYTES },
};

export const MATERIAL_ACCEPT = Object.keys(MATERIAL_TYPES)
  .map((ext) => `.${ext}`)
  .join(",");

export type MaterialFileCheck =
  | { ok: true; kind: MaterialKind; mime: string }
  | { ok: false; reason: "type" | "size" | "empty"; maxBytes?: number };

/** Whether a picked file may become a course material, and what kind it is. */
export function checkMaterialFile(file: File): MaterialFileCheck {
  const ext = file.name.split(".").pop()?.toLowerCase() ?? "";
  const type = MATERIAL_TYPES[ext];
  if (!type) return { ok: false, reason: "type" };
  if (file.size === 0) return { ok: false, reason: "empty" };
  if (file.size > type.maxBytes)
    return { ok: false, reason: "size", maxBytes: type.maxBytes };
  return { ok: true, kind: type.kind, mime: type.mime };
}

/** The per-user quotas from `resource/AGENT.md` §9 (BR-RESOURCE-08). */
export const RESOURCE_QUOTA_COUNT = 50;
export const RESOURCE_QUOTA_BYTES = 250 * 1024 * 1024;

/**
 * The renditions a material must have ready before it can be published, any one
 * of each group: an image no larger than a thumbnail gets no display copy.
 */
const REQUIRED_RENDITIONS: Record<MaterialKind, string[][]> = {
  video: [["video_360p"]],
  document: [["preview"]],
  image: [["display", "thumbnail"]],
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
  async uploadDirect(
    intent: ResourceUploadIntent,
    file: File,
    options: {
      contentType?: string;
      /** 0-1. Given, the upload goes through XHR: fetch cannot report progress. */
      onProgress?: (fraction: number) => void;
      signal?: AbortSignal;
    } = {},
  ): Promise<void> {
    const url = reachableStorageUrl(intent.upload_url);
    const contentType = options.contentType ?? file.type;
    const { onProgress, signal } = options;
    if (!onProgress) {
      const response = await fetch(url, {
        method: "PUT",
        body: file,
        headers: { "Content-Type": contentType },
        ...(signal && { signal }),
      });
      if (!response.ok) {
        throw new Error(
          `Direct storage upload failed with status ${response.status}`,
        );
      }
      return;
    }
    await new Promise<void>((resolve, reject) => {
      const xhr = new XMLHttpRequest();
      xhr.open("PUT", url);
      xhr.setRequestHeader("Content-Type", contentType);
      xhr.upload.onprogress = (e) => {
        if (e.lengthComputable) onProgress(e.loaded / e.total);
      };
      xhr.onload = () => {
        if (xhr.status >= 200 && xhr.status < 300) resolve();
        else
          reject(
            new Error(`Direct storage upload failed with status ${xhr.status}`),
          );
      };
      xhr.onerror = () => reject(new Error("Direct storage upload failed"));
      xhr.onabort = () => reject(new DOMException("Aborted", "AbortError"));
      signal?.addEventListener("abort", () => xhr.abort());
      xhr.send(file);
    });
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

  revalidateResource(id: string): Promise<void> {
    return apiFetch<void>(`/api/v1/me/resources/${id}/revalidate`, {
      method: "POST",
    });
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
 * Prefer the explicit state shared by list and detail endpoints. Fall back to
 * the older detail response shape while the server is being upgraded.
 */
export function renditionsPending(resource: Resource): boolean {
  if (resource.status !== "validated" || resource.kind !== "file") {
    return false;
  }
  if (resource.processing_status)
    return resource.processing_status === "processing";
  return (resource.renditions?.length ?? 0) === 0;
}

/** Whether a resource is validated and every rendition the runner needs is ready. */
export function materialReady(
  resource: Resource,
  materialKind: MaterialKind,
): boolean {
  if (resource.status !== "validated") return false;
  // The API only attaches renditions that are ready, each with a presigned URL,
  // so a present URL is what "ready" means here.
  const ready = new Set<string>(
    (resource.renditions ?? [])
      .filter((r) => Boolean(r.url))
      .map((r) => r.kind),
  );
  return REQUIRED_RENDITIONS[materialKind].every((group) =>
    group.some((kind) => ready.has(kind)),
  );
}

/** A learner-facing state for the upload status line. */
export type MaterialUploadState =
  "idle" | "uploading" | "processing" | "ready" | "failed";

/** The upload state a fetched resource stands for. */
export function materialState(
  resource: Resource,
  materialKind: MaterialKind,
): MaterialUploadState {
  if (resource.status === "rejected" || resource.status === "failed")
    return "failed";
  return materialReady(resource, materialKind) ? "ready" : "processing";
}
