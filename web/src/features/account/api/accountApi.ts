import { apiFetch } from "@/api/client";
import type { components } from "@/types/api";

export type UserProfile = components["schemas"]["Me"];
export type UpdateProfileRequest = components["schemas"]["UpdateMeRequest"];
export type UserPreferences = components["schemas"]["Preferences"];
export type ReplacePreferencesRequest =
  components["schemas"]["ReplacePreferencesRequest"];
export type AvatarUploadIntent = components["schemas"]["AvatarUploadIntent"];
export type ConfirmAvatarRequest =
  components["schemas"]["ConfirmAvatarRequest"];
export type TrustedDeviceList = components["schemas"]["TrustedDeviceList"];
export type TrustedDevice = components["schemas"]["TrustedDevice"];
export type SessionList = components["schemas"]["SessionList"];
export type SessionSummary = components["schemas"]["SessionSummary"];
export type ChangePasswordRequest =
  components["schemas"]["ChangePasswordRequest"];
export type PasswordChanged = components["schemas"]["PasswordChanged"];
export type OAuthIdentity = components["schemas"]["OAuthIdentity"];
export type GoogleLinkStatus = components["schemas"]["GoogleLinkStatus"];
export type OAuthStart = components["schemas"]["OAuthStart"];
export type ExportResponse = components["schemas"]["ExportResponse"];
export type DeletionResponse = components["schemas"]["DeletionResponse"];

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

export const accountApi = {
  /** Read caller's own account */
  async getMe(): Promise<UserProfile> {
    return apiFetch<UserProfile>("/api/v1/me");
  },

  /** Update caller's own profile */
  async updateProfile(data: UpdateProfileRequest): Promise<UserProfile> {
    return apiFetch<UserProfile>("/api/v1/me", {
      method: "PATCH",
      body: JSON.stringify(data),
    });
  },

  /** Read caller's preferences */
  async getPreferences(): Promise<UserPreferences> {
    return apiFetch<UserPreferences>("/api/v1/me/preferences");
  },

  /** Replace caller's preferences */
  async replacePreferences(
    data: ReplacePreferencesRequest,
  ): Promise<UserPreferences> {
    return apiFetch<UserPreferences>("/api/v1/me/preferences", {
      method: "PUT",
      body: JSON.stringify(data),
    });
  },

  /** Request presigned upload intent for avatar */
  async requestAvatarUploadIntent(
    contentType?: string,
  ): Promise<AvatarUploadIntent> {
    return apiFetch<AvatarUploadIntent>(
      "/api/v1/me/avatar/upload-intent",
      contentType
        ? {
            method: "POST",
            body: JSON.stringify({ content_type: contentType }),
          }
        : {
            method: "POST",
          },
    );
  },

  /**
   * Upload avatar bytes directly to storage (MinIO/S3).
   * The image bytes NEVER pass through the API.
   */
  async uploadAvatarDirect(
    intent: AvatarUploadIntent,
    file: File,
  ): Promise<void> {
    let uploadUrl = intent.upload_url;
    // Map internal docker host to localhost if running in local dev browser
    if (
      typeof window !== "undefined" &&
      (window.location.hostname === "localhost" ||
        window.location.hostname === "127.0.0.1") &&
      uploadUrl.includes("//minio:9000")
    ) {
      uploadUrl = uploadUrl.replace("//minio:9000", "//localhost:9000");
    }

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

      let response: Response;
      try {
        response = await fetch(uploadUrl, { method: "POST", body: formData });
      } catch (error) {
        throw storageNetworkError(error);
      }

      if (!response.ok) {
        throw await storageUploadError(response);
      }
    } else {
      // Direct PUT, for stores with no POST policy (Cloudflare R2).
      //
      // The header is `intent.content_type` and nothing else. The server signs
      // that exact string into the URL, and a presigned request has to arrive
      // carrying the headers the signature covers, byte for byte — falling back
      // to `file.type` here would send a string the signature does not describe
      // and earn a 403 from the store.
      let response: Response;
      try {
        response = await fetch(uploadUrl, {
          method: "PUT",
          body: file,
          headers: {
            "Content-Type": intent.content_type,
          },
        });
      } catch (error) {
        throw storageNetworkError(error);
      }

      if (!response.ok) {
        throw await storageUploadError(response);
      }
    }
  },

  /** Confirm uploaded avatar in storage */
  async confirmAvatar(data: ConfirmAvatarRequest): Promise<UserProfile> {
    return apiFetch<UserProfile>("/api/v1/me/avatar", {
      method: "PUT",
      body: JSON.stringify(data),
    });
  },

  /** Whether Google is linked, and whether it may be unlinked */
  async getGoogleLinkStatus(): Promise<GoogleLinkStatus> {
    return apiFetch<GoogleLinkStatus>("/api/v1/auth/oauth/google");
  },

  /** List trusted devices */
  async listDevices(): Promise<TrustedDeviceList> {
    return apiFetch<TrustedDeviceList>("/api/v1/auth/devices");
  },

  /** Untrust a device */
  async untrustDevice(id: string): Promise<void> {
    return apiFetch<void>(`/api/v1/auth/devices/${id}`, {
      method: "DELETE",
    });
  },

  /** List active sessions */
  async listSessions(): Promise<SessionList> {
    return apiFetch<SessionList>("/api/v1/auth/sessions");
  },

  /** Revoke an active session */
  async revokeSession(id: string): Promise<void> {
    return apiFetch<void>(`/api/v1/auth/sessions/${id}`, {
      method: "DELETE",
    });
  },

  /** Change password while signed in */
  async changePassword(data: ChangePasswordRequest): Promise<PasswordChanged> {
    return apiFetch<PasswordChanged>("/api/v1/auth/change-password", {
      method: "POST",
      body: JSON.stringify(data),
    });
  },

  /** Start Google link flow */
  async startGoogleLink(): Promise<OAuthStart> {
    return apiFetch<OAuthStart>(
      "/api/v1/auth/oauth/google/start?redirect_to=/settings",
    );
  },

  /** Link Google account */
  async linkGoogle(data: {
    code: string;
    state: string;
  }): Promise<OAuthIdentity> {
    return apiFetch<OAuthIdentity>("/api/v1/auth/oauth/google/link", {
      method: "POST",
      body: JSON.stringify(data),
    });
  },

  /** Unlink Google account */
  async unlinkGoogle(): Promise<void> {
    return apiFetch<void>("/api/v1/auth/oauth/google", {
      method: "DELETE",
    });
  },

  /** Request full data export */
  async requestExport(): Promise<ExportResponse> {
    return apiFetch<ExportResponse>("/api/v1/me/export", {
      method: "POST",
    });
  },

  /** Get data export status */
  async getExport(id: string): Promise<ExportResponse> {
    return apiFetch<ExportResponse>(`/api/v1/me/export/${id}`);
  },

  /** Request account deletion */
  async requestDeletion(): Promise<DeletionResponse> {
    return apiFetch<DeletionResponse>("/api/v1/me", {
      method: "DELETE",
    });
  },

  /** Cancel pending account deletion */
  async cancelDeletion(): Promise<DeletionResponse> {
    return apiFetch<DeletionResponse>("/api/v1/me/deletion/cancel", {
      method: "POST",
    });
  },

  /** Get account deletion status */
  async getDeletion(id: string): Promise<DeletionResponse> {
    return apiFetch<DeletionResponse>(`/api/v1/me/deletion/${id}`);
  },
};
