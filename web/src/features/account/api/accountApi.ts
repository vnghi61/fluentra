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

      const response = await fetch(uploadUrl, {
        method: "POST",
        body: formData,
      });

      if (!response.ok) {
        throw new Error(
          `Direct storage upload failed with status ${response.status}`,
        );
      }
    } else {
      // Default / direct PUT upload
      const response = await fetch(uploadUrl, {
        method: "PUT",
        body: file,
        headers: {
          "Content-Type": intent.content_type || file.type,
        },
      });

      if (!response.ok) {
        throw new Error(
          `Direct storage upload failed with status ${response.status}`,
        );
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
