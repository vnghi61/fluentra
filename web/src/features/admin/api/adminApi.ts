import { apiFetch } from "@/api/client";
import type { components } from "@/types/api";

export type AdminUserSummary = components["schemas"]["AdminUserSummary"];
export type AdminUserPage = components["schemas"]["AdminUserPage"];
export type AdminUserDetail = components["schemas"]["AdminUserDetail"];
export type AdminActionRequest = components["schemas"]["AdminActionRequest"];
export type AdminUserStatusChanged = components["schemas"]["AdminUserStatusChanged"];
export type AdminSessionsRevoked = components["schemas"]["AdminSessionsRevoked"];
export type FeatureFlag = components["schemas"]["FeatureFlag"];
export type FeatureFlagList = components["schemas"]["FeatureFlagList"];
export type CreateFeatureFlagRequest = components["schemas"]["CreateFeatureFlagRequest"];
export type UpdateFeatureFlagRequest = components["schemas"]["UpdateFeatureFlagRequest"];

export interface SearchUsersParams {
  query?: string | undefined;
  status?: components["schemas"]["UserStatus"] | undefined;
  role?: components["schemas"]["RoleName"] | undefined;
  cursor?: string | undefined;
  limit?: number | undefined;
}

export const adminApi = {
  /** Search users with cursor pagination */
  async searchUsers(params: SearchUsersParams = {}): Promise<AdminUserPage> {
    const searchParams = new URLSearchParams();
    if (params.query) searchParams.set("query", params.query);
    if (params.status) searchParams.set("status", params.status);
    if (params.role) searchParams.set("role", params.role);
    if (params.cursor) searchParams.set("cursor", params.cursor);
    if (params.limit) searchParams.set("limit", params.limit.toString());

    const qs = searchParams.toString();
    const endpoint = `/api/v1/admin/users${qs ? `?${qs}` : ""}`;
    return apiFetch<AdminUserPage>(endpoint);
  },

  /** Get detailed user profile (Audited on read) */
  async getUser(id: string): Promise<AdminUserDetail> {
    return apiFetch<AdminUserDetail>(`/api/v1/admin/users/${id}`);
  },

  /** Suspend user and revoke active sessions */
  async suspendUser(id: string, reason: string): Promise<AdminUserStatusChanged> {
    return apiFetch<AdminUserStatusChanged>(`/api/v1/admin/users/${id}/suspend`, {
      method: "POST",
      body: JSON.stringify({ reason }),
    });
  },

  /** Reinstate suspended user */
  async reinstateUser(id: string, reason: string): Promise<AdminUserStatusChanged> {
    return apiFetch<AdminUserStatusChanged>(`/api/v1/admin/users/${id}/reinstate`, {
      method: "POST",
      body: JSON.stringify({ reason }),
    });
  },

  /** Revoke all active sessions for user */
  async revokeUserSessions(id: string, reason: string): Promise<AdminSessionsRevoked> {
    return apiFetch<AdminSessionsRevoked>(`/api/v1/admin/users/${id}/sessions/revoke`, {
      method: "POST",
      body: JSON.stringify({ reason }),
    });
  },

  /** List all feature flags */
  async listFlags(): Promise<FeatureFlagList> {
    return apiFetch<FeatureFlagList>("/api/v1/admin/flags");
  },

  /** Create feature flag */
  async createFlag(data: CreateFeatureFlagRequest): Promise<FeatureFlag> {
    return apiFetch<FeatureFlag>("/api/v1/admin/flags", {
      method: "POST",
      body: JSON.stringify(data),
    });
  },

  /** Update feature flag */
  async updateFlag(key: string, data: UpdateFeatureFlagRequest): Promise<FeatureFlag> {
    return apiFetch<FeatureFlag>(`/api/v1/admin/flags/${key}`, {
      method: "PATCH",
      body: JSON.stringify(data),
    });
  },

  /** Delete feature flag */
  async deleteFlag(key: string): Promise<void> {
    return apiFetch<void>(`/api/v1/admin/flags/${key}`, {
      method: "DELETE",
    });
  },
};
