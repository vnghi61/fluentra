import { reachableStorageUrl } from "@/lib/storage-url";

/**
 * A presigned browser upload as the API hands it out (AvatarUploadIntent,
 * BannerImageUploadIntent): either an S3 POST policy or a presigned PUT.
 */
export interface PresignedUpload {
  upload_url: string;
  method: "POST" | "PUT";
  form_data?: Record<string, string>;
  file_field?: string;
  content_type: string;
}

/**
 * Turns a rejected upload request into an error that names the likely cause.
 *
 * A cross-origin `PUT` is never a CORS "simple request", so the browser sends an
 * `OPTIONS` preflight first. A bucket with no CORS policy refuses it and `fetch`
 * rejects with a bare `TypeError: Failed to fetch` — no status, no body, nothing
 * `storageUploadError` can read, because there is no response at all.
 *
 * That message is what a learner saw while the actual answer sat in R2's own
 * reply to the preflight: "CORS not configured for this bucket". The fix is a
 * bucket setting, not code — see deploy/r2/README.md — so the least this can do
 * is say where to look.
 */
function storageNetworkError(error: unknown): Error {
  if (error instanceof TypeError) {
    return new Error(
      "The upload never reached storage. This is usually the bucket's CORS " +
        "policy refusing the browser's preflight — see deploy/r2/README.md.",
    );
  }
  return error instanceof Error ? error : new Error(String(error));
}

/**
 * Turns a failed direct-to-storage upload into an error that says why.
 *
 * The store is not our API: it answers with an S3 XML body, not a Problem
 * Details document, so `apiFetch` never sees it and nothing here used to read
 * it. The thrown message was the bare status, which is how an avatar upload
 * against R2 could fail for a week reported only as "403" — and 403 covers a
 * signature the store would not accept, a token without write permission, and
 * a bucket that is not there, which are three different fixes.
 *
 * S3 and R2 both put a machine-readable `<Code>` in that body. Surfacing it
 * costs one read and turns the next failure into a diagnosis.
 */
async function storageUploadError(response: Response): Promise<Error> {
  let detail = "";
  try {
    const body = await response.text();
    const code = /<Code>([^<]+)<\/Code>/.exec(body)?.[1];
    const message = /<Message>([^<]+)<\/Message>/.exec(body)?.[1];
    detail = [code, message].filter(Boolean).join(": ") || body.slice(0, 200);
  } catch {
    // A body that cannot be read leaves the status, which is what we had before.
  }
  return new Error(
    detail
      ? `Direct storage upload failed with status ${response.status} (${detail})`
      : `Direct storage upload failed with status ${response.status}`,
  );
}

/**
 * Uploads `file` straight to storage. The bytes never pass through the API.
 */
export async function uploadToPresignedStorage(
  intent: PresignedUpload,
  file: File,
): Promise<void> {
  // A development store is reached through the dev server from other devices.
  const uploadUrl = reachableStorageUrl(intent.upload_url);

  let request: RequestInit;
  if (intent.method === "POST" && intent.form_data) {
    const formData = new FormData();
    let hasContentType = false;
    Object.entries(intent.form_data).forEach(([key, value]) => {
      if (key.toLowerCase() === "content-type") {
        hasContentType = true;
      }
      formData.append(key, value);
    });
    if (!hasContentType && (intent.content_type || file.type)) {
      formData.append("Content-Type", intent.content_type || file.type);
    }
    formData.append(intent.file_field || "file", file);
    request = { method: "POST", body: formData };
  } else {
    // Direct PUT, for stores with no POST policy (Cloudflare R2).
    //
    // The header is `intent.content_type` and nothing else. The server signs
    // that exact string into the URL, and a presigned request has to arrive
    // carrying the headers the signature covers, byte for byte — falling back
    // to `file.type` here would send a string the signature does not describe
    // and earn a 403 from the store.
    request = {
      method: "PUT",
      body: file,
      headers: { "Content-Type": intent.content_type },
    };
  }

  let response: Response;
  try {
    response = await fetch(uploadUrl, request);
  } catch (error) {
    throw storageNetworkError(error);
  }
  if (!response.ok) {
    throw await storageUploadError(response);
  }
}
