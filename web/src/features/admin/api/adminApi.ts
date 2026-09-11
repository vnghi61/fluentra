import { apiFetch } from "@/api/client";
import type { components } from "@/types/api";

export type AdminUserSummary = components["schemas"]["AdminUserSummary"];
export type AdminUserPage = components["schemas"]["AdminUserPage"];
export type AdminUserDetail = components["schemas"]["AdminUserDetail"];
export type AdminActionRequest = components["schemas"]["AdminActionRequest"];
export type AdminUserStatusChanged =
  components["schemas"]["AdminUserStatusChanged"];
export type AdminSessionsRevoked =
  components["schemas"]["AdminSessionsRevoked"];
export type FeatureFlag = components["schemas"]["FeatureFlag"];
export type FeatureFlagList = components["schemas"]["FeatureFlagList"];
export type CreateFeatureFlagRequest =
  components["schemas"]["CreateFeatureFlagRequest"];
export type UpdateFeatureFlagRequest =
  components["schemas"]["UpdateFeatureFlagRequest"];
export type AdminAIUsageResponse =
  components["schemas"]["AdminAIUsageResponse"];
export type AdminAIUsageItem = components["schemas"]["AdminAIUsageItem"];

export type AdminContentItemList =
  components["schemas"]["AdminContentItemList"];
export type AdminContentItemDetail =
  components["schemas"]["AdminContentItemDetail"];
export type ContentItem = components["schemas"]["ContentItem"];
export type ContentVersion = components["schemas"]["ContentVersion"];
export type AuthoringStatus = components["schemas"]["AuthoringStatus"];
export type ReportedContentVersion =
  components["schemas"]["ReportedContentVersion"];
export type ReportedContentList = components["schemas"]["ReportedContentList"];

export type AdminWordList = components["schemas"]["AdminWordList"];
export type AdminWordSummary = components["schemas"]["AdminWordSummary"];
export type LearnerWordQueueList =
  components["schemas"]["LearnerWordQueueList"];
export type LearnerWordQueueItem =
  components["schemas"]["LearnerWordQueueItem"];
export type UpdateWordSenseRequest =
  components["schemas"]["UpdateWordSenseRequest"];
export type ExampleSentence = components["schemas"]["ExampleSentence"];

/**
 * The parameters `adminSearchUsers` actually takes.
 *
 * It previously declared `query` and `role`, neither of which is in the
 * operation: the server matched nothing and the table showed "No learners
 * found" for every search. `email_prefix` and `display_name` are separate
 * parameters and they are ANDed, so a caller picks one — see `searchUsers`.
 */
export interface SearchUsersParams {
  email_prefix?: string | undefined;
  display_name?: string | undefined;
  status?: components["schemas"]["UserStatus"] | undefined;
  cursor?: string | undefined;
  limit?: number | undefined;
}

export const adminApi = {
  /** Search users with cursor pagination */
  async searchUsers(params: SearchUsersParams = {}): Promise<AdminUserPage> {
    const searchParams = new URLSearchParams();
    if (params.email_prefix) {
      searchParams.set("email_prefix", params.email_prefix);
    }
    if (params.display_name) {
      searchParams.set("display_name", params.display_name);
    }
    if (params.status) searchParams.set("status", params.status);
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
  async suspendUser(
    id: string,
    reason: string,
  ): Promise<AdminUserStatusChanged> {
    return apiFetch<AdminUserStatusChanged>(
      `/api/v1/admin/users/${id}/suspend`,
      {
        method: "POST",
        body: JSON.stringify({ reason }),
      },
    );
  },

  /** Reinstate suspended user */
  async reinstateUser(
    id: string,
    reason: string,
  ): Promise<AdminUserStatusChanged> {
    return apiFetch<AdminUserStatusChanged>(
      `/api/v1/admin/users/${id}/reinstate`,
      {
        method: "POST",
        body: JSON.stringify({ reason }),
      },
    );
  },

  /** Revoke all active sessions for user */
  /** Soft-delete an account: 30-day grace period, sessions ended immediately. */
  async softDeleteUser(
    id: string,
    reason: string,
  ): Promise<AdminUserStatusChanged> {
    return apiFetch<AdminUserStatusChanged>(
      `/api/v1/admin/users/${id}/delete`,
      {
        method: "POST",
        body: JSON.stringify({ reason }),
      },
    );
  },

  async revokeUserSessions(
    id: string,
    reason: string,
  ): Promise<AdminSessionsRevoked> {
    return apiFetch<AdminSessionsRevoked>(
      `/api/v1/admin/users/${id}/sessions/revoke`,
      {
        method: "POST",
        body: JSON.stringify({ reason }),
      },
    );
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
  async updateFlag(
    key: string,
    data: UpdateFeatureFlagRequest,
  ): Promise<FeatureFlag> {
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

  /** Get today's AI usage and budget status across providers */
  async getAIUsage(): Promise<AdminAIUsageResponse> {
    return apiFetch<AdminAIUsageResponse>("/api/v1/admin/ai/usage");
  },

  /** List content items */
  async listContent(
    params: SearchContentParams = {},
  ): Promise<AdminContentItemList> {
    const sp = new URLSearchParams();
    if (params.status) sp.set("status", params.status);
    if (params.kind) sp.set("kind", params.kind);
    if (params.q) sp.set("q", params.q);
    if (params.limit !== undefined) sp.set("limit", params.limit.toString());
    if (params.offset !== undefined) sp.set("offset", params.offset.toString());
    const qs = sp.toString();
    return apiFetch<AdminContentItemList>(
      `/api/v1/admin/content${qs ? `?${qs}` : ""}`,
    );
  },

  /** Get single content item detail with all versions */
  async getContent(id: string): Promise<AdminContentItemDetail> {
    return apiFetch<AdminContentItemDetail>(`/api/v1/admin/content/${id}`);
  },

  /** Update draft body and metadata */
  async updateDraft(
    id: string,
    body: unknown,
    cefrLevel?: string,
    tags?: string[],
  ): Promise<ContentVersion> {
    return apiFetch<ContentVersion>(`/api/v1/admin/content/${id}/draft`, {
      method: "PUT",
      body: JSON.stringify({ body, cefr_level: cefrLevel, tags }),
    });
  },

  /** Submit draft for editorial review */
  async submitContent(id: string): Promise<ContentVersion> {
    return apiFetch<ContentVersion>(`/api/v1/admin/content/${id}/submit`, {
      method: "POST",
    });
  },

  /** Review content: approve or request changes */
  async reviewContent(
    id: string,
    decision: "approved" | "changes_requested",
    comments?: string,
  ): Promise<ContentVersion> {
    return apiFetch<ContentVersion>(`/api/v1/admin/content/${id}/review`, {
      method: "POST",
      body: JSON.stringify({ decision, comments }),
    });
  },

  /** Publish approved content version */
  async publishContent(id: string): Promise<ContentVersion> {
    return apiFetch<ContentVersion>(`/api/v1/admin/content/${id}/publish`, {
      method: "POST",
    });
  },

  /** Archive content item */
  async archiveContent(id: string): Promise<void> {
    return apiFetch<void>(`/api/v1/admin/content/${id}/archive`, {
      method: "POST",
    });
  },

  /** List words for vocabulary dictionary administration */
  async listWords(params: SearchWordsParams = {}): Promise<AdminWordList> {
    const sp = new URLSearchParams();
    if (params.q) sp.set("q", params.q);
    if (params.source) sp.set("source", params.source);
    if (params.limit !== undefined) sp.set("limit", params.limit.toString());
    if (params.offset !== undefined) sp.set("offset", params.offset.toString());
    const qs = sp.toString();
    return apiFetch<AdminWordList>(
      `/api/v1/admin/vocabulary/words${qs ? `?${qs}` : ""}`,
    );
  },

  /** Withdraw/delete a word */
  async deleteWord(id: string): Promise<void> {
    return apiFetch<void>(`/api/v1/admin/vocabulary/words/${id}`, {
      method: "DELETE",
    });
  },

  /** List learner words contribution queue */
  async listLearnerWordsQueue(
    params: SearchQueueParams = {},
  ): Promise<LearnerWordQueueList> {
    const sp = new URLSearchParams();
    if (params.status) sp.set("status", params.status);
    if (params.q) sp.set("q", params.q);
    if (params.limit !== undefined) sp.set("limit", params.limit.toString());
    if (params.offset !== undefined) sp.set("offset", params.offset.toString());
    const qs = sp.toString();
    return apiFetch<LearnerWordQueueList>(
      `/api/v1/admin/vocabulary/queue${qs ? `?${qs}` : ""}`,
    );
  },

  /** Update word sense (definition, gloss, topic, examples) */
  async updateWordSense(
    id: string,
    data: UpdateWordSenseRequest,
  ): Promise<unknown> {
    return apiFetch<unknown>(`/api/v1/admin/vocabulary/senses/${id}`, {
      method: "PATCH",
      body: JSON.stringify(data),
    });
  },

  /** Delete word sense */
  async deleteWordSense(id: string): Promise<void> {
    return apiFetch<void>(`/api/v1/admin/vocabulary/senses/${id}`, {
      method: "DELETE",
    });
  },

  /** List reported content versions ordered by distinct reporters */
  async listReportedContent(
    params: { limit?: number; offset?: number } = {},
  ): Promise<ReportedContentList> {
    const sp = new URLSearchParams();
    if (params.limit !== undefined) sp.set("limit", params.limit.toString());
    if (params.offset !== undefined) sp.set("offset", params.offset.toString());
    const qs = sp.toString();
    return apiFetch<ReportedContentList>(
      `/api/v1/admin/content/reports${qs ? `?${qs}` : ""}`,
    );
  },
};

export interface SearchContentParams {
  status?: string | undefined;
  kind?: string | undefined;
  q?: string | undefined;
  limit?: number | undefined;
  offset?: number | undefined;
}

export interface SearchWordsParams {
  q?: string | undefined;
  source?: string | undefined;
  limit?: number | undefined;
  offset?: number | undefined;
}

export interface SearchQueueParams {
  status?: string | undefined;
  q?: string | undefined;
  limit?: number | undefined;
  offset?: number | undefined;
}
