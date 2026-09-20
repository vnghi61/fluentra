import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import {
  studioApi,
  type CreateCourseDraftRequest,
  type UpdateCourseDraftRequest,
  type UpsertCreatorProfileRequest,
  type CreatePayoutAccountRequest,
  type RequestPayoutBody,
} from "../api/studioApi";

export const studioKeys = {
  all: ["studio"] as const,
  profile: () => [...studioKeys.all, "profile"] as const,
  payoutAccount: () => [...studioKeys.all, "payout-account"] as const,
  drafts: () => [...studioKeys.all, "drafts"] as const,
  draft: (id: string) => [...studioKeys.all, "draft", id] as const,
  submissions: (draftId: string) =>
    [...studioKeys.all, "draft", draftId, "submissions"] as const,
  earnings: () => [...studioKeys.all, "earnings"] as const,
  purchases: () => [...studioKeys.all, "purchases"] as const,
  order: (orderId: string) => [...studioKeys.all, "order", orderId] as const,
};

export function useCreatorProfile() {
  return useQuery({
    queryKey: studioKeys.profile(),
    queryFn: () => studioApi.getCreatorProfile(),
    retry: false,
  });
}

export function useUpsertCreatorProfile() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (req: UpsertCreatorProfileRequest) =>
      studioApi.upsertCreatorProfile(req),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: studioKeys.profile() });
    },
  });
}

export function usePayoutAccount() {
  return useQuery({
    queryKey: studioKeys.payoutAccount(),
    queryFn: () => studioApi.getPayoutAccount(),
    retry: false,
  });
}

export function useUpsertPayoutAccount() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (req: CreatePayoutAccountRequest) =>
      studioApi.upsertPayoutAccount(req),
    onSuccess: () => {
      void queryClient.invalidateQueries({
        queryKey: studioKeys.payoutAccount(),
      });
      void queryClient.invalidateQueries({ queryKey: studioKeys.earnings() });
    },
  });
}

export function useCreatorDrafts(params?: { limit?: number; offset?: number }) {
  return useQuery({
    queryKey: [...studioKeys.drafts(), params],
    queryFn: () => studioApi.listDrafts(params),
  });
}

export function useCourseDraft(id?: string) {
  return useQuery({
    queryKey: studioKeys.draft(id ?? "__none__"),
    queryFn: () =>
      id ? studioApi.getDraft(id) : Promise.reject(new Error("No draft id")),
    enabled: Boolean(id),
  });
}

export function useCreateDraft() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (req: CreateCourseDraftRequest) => studioApi.createDraft(req),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: studioKeys.drafts() });
    },
  });
}

export function useUpdateDraft(id: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (req: UpdateCourseDraftRequest) =>
      studioApi.updateDraft(id, req),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: studioKeys.draft(id) });
      void queryClient.invalidateQueries({ queryKey: studioKeys.drafts() });
    },
  });
}

export function useSubmitDraft(id: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: () => studioApi.submitDraft(id),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: studioKeys.draft(id) });
      void queryClient.invalidateQueries({ queryKey: studioKeys.drafts() });
      void queryClient.invalidateQueries({
        queryKey: studioKeys.submissions(id),
      });
    },
  });
}

export function useDraftSubmissions(draftId?: string) {
  return useQuery({
    queryKey: studioKeys.submissions(draftId ?? "__none__"),
    queryFn: () =>
      draftId
        ? studioApi.getDraftSubmissions(draftId)
        : Promise.reject(new Error("No draft id")),
    enabled: Boolean(draftId),
  });
}

export function useCreatorEarnings() {
  return useQuery({
    queryKey: studioKeys.earnings(),
    queryFn: () => studioApi.getEarnings(),
  });
}

export function useRequestPayout() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (req?: RequestPayoutBody) => studioApi.requestPayout(req),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: studioKeys.earnings() });
    },
  });
}

export function useUserPurchases() {
  return useQuery({
    queryKey: studioKeys.purchases(),
    queryFn: () => studioApi.listUserPurchases(),
  });
}

export function useBillingOrder(orderId?: string, pollIntervalMs?: number) {
  return useQuery({
    queryKey: studioKeys.order(orderId ?? "__none__"),
    queryFn: () =>
      orderId
        ? studioApi.getOrder(orderId)
        : Promise.reject(new Error("No order id")),
    enabled: Boolean(orderId),
    refetchInterval: (query) => {
      // Stop polling once paid or expired or cancelled
      const status = query.state.data?.status;
      if (status === "paid" || status === "expired" || status === "cancelled") {
        return false;
      }
      return pollIntervalMs ?? 3000;
    },
  });
}
