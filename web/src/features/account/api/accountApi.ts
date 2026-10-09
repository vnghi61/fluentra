import { ApiError, apiFetch } from "@/api/client";
import { uploadToPresignedStorage } from "@/lib/storage-upload";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import type { components } from "@/types/api";
import { accountKeys } from "./keys";

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
export type LearningProfile = components["schemas"]["LearningProfile"];
export type LearningProfileRequest =
  components["schemas"]["LearningProfileRequest"];

/**
 * The PUT body that stores `preferences` as they are, with `changes` applied.
 *
 * `PUT /me/preferences` replaces the whole record, and an omitted nullable
 * member is stored as null. Every caller that changes one setting builds its
 * body here, so none can forget a field: a theme toggle that left out
 * `practice_level` would clear the learner's level each time it was used.
 */
export function replacementFor(
  preferences: UserPreferences,
  changes: Partial<ReplacePreferencesRequest> = {},
): ReplacePreferencesRequest {
  return {
    locale: preferences.locale,
    theme: preferences.theme,
    daily_goal_minutes: preferences.daily_goal_minutes,
    notification_channels: preferences.notification_channels,
    quiet_hours: preferences.quiet_hours ?? null,
    ai_processing_opt_out: preferences.ai_processing_opt_out,
    practice_level: preferences.practice_level ?? null,
    ...changes,
  };
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
    await uploadToPresignedStorage(intent, file);
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

  /** Read caller's learning profile */
  async getLearningProfile(): Promise<LearningProfile | null> {
    try {
      return await apiFetch<LearningProfile>("/api/v1/me/learning-profile");
    } catch (err) {
      if (err instanceof ApiError && err.problem.status === 404) {
        return null;
      }
      throw err;
    }
  },

  /** Replace caller's learning profile */
  async replaceLearningProfile(
    data: LearningProfileRequest,
  ): Promise<LearningProfile> {
    return apiFetch<LearningProfile>("/api/v1/me/learning-profile", {
      method: "PUT",
      body: JSON.stringify(data),
    });
  },
};

/** React Query hook for the caller's learning profile */
export function useLearningProfile(enabled = true) {
  return useQuery({
    queryKey: ["account", "learningProfile"] as const,
    queryFn: () => accountApi.getLearningProfile(),
    enabled,
    staleTime: 5 * 60 * 1000,
  });
}

/** React Query hook to create or update learning profile */
export function useUpdateLearningProfile() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (data: LearningProfileRequest) =>
      accountApi.replaceLearningProfile(data),
    onSuccess: () => {
      void queryClient.invalidateQueries({
        queryKey: ["account", "learningProfile"],
      });
      void queryClient.invalidateQueries({
        queryKey: ["learning", "weeklyPlan"],
      });
      void queryClient.invalidateQueries({
        queryKey: ["learning", "startingPath"],
      });
      void queryClient.invalidateQueries({
        queryKey: ["learning", "placement"],
      });
    },
  });
}

/** The caller's live sessions (one per sign-in). */
export function useSessions() {
  return useQuery({
    queryKey: accountKeys.sessions(),
    queryFn: () => accountApi.listSessions(),
  });
}

/** Devices the caller chose to stay signed in on. */
export function useTrustedDevices() {
  return useQuery({
    queryKey: accountKeys.devices(),
    queryFn: () => accountApi.listDevices(),
  });
}
