import { apiFetch } from "@/api/client";
import type { components } from "@/types/api";

export type CreatorProfile = components["schemas"]["CreatorProfile"];
export type UpsertCreatorProfileRequest =
  components["schemas"]["UpsertCreatorProfileRequest"];
export type PayoutAccount = components["schemas"]["PayoutAccount"];
export type CreatePayoutAccountRequest =
  components["schemas"]["CreatePayoutAccountRequest"];
export type CourseDraft = Omit<components["schemas"]["CourseDraft"], "structure"> & {
  structure?: Record<string, unknown> | undefined;
};
export type CreateCourseDraftRequest = Omit<
  components["schemas"]["CreateCourseDraftRequest"],
  "structure"
> & {
  structure?: Record<string, unknown> | undefined;
};
export type UpdateCourseDraftRequest = Omit<
  components["schemas"]["UpdateCourseDraftRequest"],
  "structure"
> & {
  structure?: Record<string, unknown> | undefined;
};
export type CourseDraftList = components["schemas"]["CourseDraftList"];
export type CourseSubmission = components["schemas"]["CourseSubmission"];
export type CreatorEarningsSummary =
  components["schemas"]["CreatorEarningsSummary"];
export type CreatorLedgerEntry = components["schemas"]["CreatorLedgerEntry"];
export type CoursePurchase = components["schemas"]["CoursePurchase"];
export type UserPurchaseList = components["schemas"]["UserPurchaseList"];
export type PurchaseOrderResponse =
  components["schemas"]["PurchaseOrderResponse"];
export type BillingOrder = components["schemas"]["BillingOrder"];
export type RefundPurchaseResponse =
  components["schemas"]["RefundPurchaseResponse"];

export interface ListDraftsParams {
  limit?: number | undefined;
  offset?: number | undefined;
}

export interface RequestPayoutBody {
  amount_vnd?: number | undefined;
}

export interface ClaimCourseResponse {
  message: string;
  purchase_id: string;
}

export const studioApi = {
  /** Get current user's creator profile */
  async getCreatorProfile(): Promise<CreatorProfile> {
    return apiFetch<CreatorProfile>("/api/v1/studio/creator/profile");
  },

  /** Create or update current user's creator profile */
  async upsertCreatorProfile(
    req: UpsertCreatorProfileRequest,
  ): Promise<CreatorProfile> {
    return apiFetch<CreatorProfile>("/api/v1/studio/creator/profile", {
      method: "POST",
      body: JSON.stringify(req),
    });
  },

  /** Get current user's payout account (masked per BR-STUDIO-09) */
  async getPayoutAccount(): Promise<PayoutAccount> {
    return apiFetch<PayoutAccount>("/api/v1/studio/creator/payout-account");
  },

  /** Configure or update payout bank account */
  async upsertPayoutAccount(
    req: CreatePayoutAccountRequest,
  ): Promise<PayoutAccount> {
    return apiFetch<PayoutAccount>("/api/v1/studio/creator/payout-account", {
      method: "POST",
      body: JSON.stringify(req),
    });
  },

  /** List courses authored by current user */
  async listDrafts(params?: ListDraftsParams): Promise<CourseDraftList> {
    const q = new URLSearchParams();
    if (params?.limit !== undefined) q.set("limit", String(params.limit));
    if (params?.offset !== undefined) q.set("offset", String(params.offset));
    const qs = q.toString();
    return apiFetch<CourseDraftList>(
      `/api/v1/studio/courses${qs ? `?${qs}` : ""}`,
    );
  },

  /** Create a new course draft */
  async createDraft(req: CreateCourseDraftRequest): Promise<CourseDraft> {
    return apiFetch<CourseDraft>("/api/v1/studio/courses", {
      method: "POST",
      body: JSON.stringify(req),
    });
  },

  /** Get a specific course draft by ID */
  async getDraft(id: string): Promise<CourseDraft> {
    return apiFetch<CourseDraft>(`/api/v1/studio/courses/${id}`);
  },

  /** Update a course draft outline & details */
  async updateDraft(
    id: string,
    req: UpdateCourseDraftRequest,
  ): Promise<CourseDraft> {
    return apiFetch<CourseDraft>(`/api/v1/studio/courses/${id}`, {
      method: "PUT",
      body: JSON.stringify(req),
    });
  },

  /** Submit a course draft for Gate 1 and Gate 2 verification */
  async submitDraft(id: string): Promise<CourseSubmission> {
    return apiFetch<CourseSubmission>(`/api/v1/studio/courses/${id}/submit`, {
      method: "POST",
    });
  },

  /** Get submission history for a draft */
  async getDraftSubmissions(id: string): Promise<CourseSubmission[]> {
    return apiFetch<CourseSubmission[]>(
      `/api/v1/studio/courses/${id}/submissions`,
    );
  },

  /** Get creator earnings dashboard statistics and recent ledger */
  async getEarnings(): Promise<CreatorEarningsSummary> {
    return apiFetch<CreatorEarningsSummary>("/api/v1/me/studio/earnings");
  },

  /** Request a payout of creator earnings (minimum ₫500,000) */
  async requestPayout(req?: RequestPayoutBody): Promise<{ message: string }> {
    return apiFetch<{ message: string }>("/api/v1/me/studio/payouts", {
      method: "POST",
      body: JSON.stringify(req ?? {}),
    });
  },

  /** List user's purchased / claimed courses */
  async listUserPurchases(): Promise<UserPurchaseList> {
    return apiFetch<UserPurchaseList>("/api/v1/me/purchases");
  },

  /** Claim a free community course */
  async claimCourse(courseId: string): Promise<ClaimCourseResponse> {
    return apiFetch<ClaimCourseResponse>(`/api/v1/courses/${courseId}/claim`, {
      method: "POST",
    });
  },

  /** Purchase a paid course and get VietQR transfer instructions */
  async purchaseCourse(courseId: string): Promise<PurchaseOrderResponse> {
    return apiFetch<PurchaseOrderResponse>(
      `/api/v1/courses/${courseId}/purchase`,
      {
        method: "POST",
      },
    );
  },

  /** Poll order status while awaiting bank transfer matching */
  async getOrder(orderId: string): Promise<BillingOrder> {
    return apiFetch<BillingOrder>(`/api/v1/me/orders/${orderId}`);
  },

  /** Self-service refund for a purchased course (< 7 days, < 20% completion) */
  async refundPurchase(purchaseId: string): Promise<RefundPurchaseResponse> {
    return apiFetch<RefundPurchaseResponse>(
      `/api/v1/me/purchases/${purchaseId}/refund`,
      {
        method: "POST",
      },
    );
  },
};
