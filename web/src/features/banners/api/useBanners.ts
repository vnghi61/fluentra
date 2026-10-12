import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { uploadToPresignedStorage } from "@/lib/storage-upload";
import { isBannerImageType, type BannerRequest } from "../model/banner";
import { bannersApi } from "./bannersApi";
import { bannerKeys } from "./keys";

/**
 * The banners the dashboard shows. A failure is not the learner's problem:
 * no retry storm, no error UI — the caller renders nothing.
 */
export function useActiveBanners() {
  return useQuery({
    queryKey: bannerKeys.active,
    queryFn: () => bannersApi.listActive(),
    staleTime: 60_000,
    retry: false,
  });
}

export function useAdminBanners() {
  return useQuery({
    queryKey: bannerKeys.adminList,
    queryFn: () => bannersApi.listAll(),
  });
}

/** Every admin write changes what the dashboard shows, so both lists go stale. */
function useInvalidateBanners() {
  const client = useQueryClient();
  return () =>
    Promise.all([
      client.invalidateQueries({ queryKey: bannerKeys.adminList }),
      client.invalidateQueries({ queryKey: bannerKeys.active }),
    ]);
}

export function useCreateBanner() {
  const invalidate = useInvalidateBanners();
  return useMutation({
    mutationFn: (body: BannerRequest) => bannersApi.create(body),
    onSuccess: invalidate,
  });
}

export function useUpdateBanner() {
  const invalidate = useInvalidateBanners();
  return useMutation({
    mutationFn: ({ id, body }: { id: string; body: BannerRequest }) =>
      bannersApi.update(id, body),
    onSuccess: invalidate,
  });
}

/**
 * Upload intent, direct upload, then processing. The result belongs to no
 * banner until the form is saved with its `image_key`, so nothing is
 * invalidated here.
 */
export function useUploadBannerImage() {
  return useMutation({
    mutationFn: async (file: File) => {
      if (!isBannerImageType(file.type)) {
        throw new Error(`Unsupported banner image type: ${file.type}`);
      }
      const intent = await bannersApi.requestImageUpload(file.type);
      await uploadToPresignedStorage(intent, file);
      return bannersApi.processImage(intent.object_key);
    },
  });
}

export function useDeleteBanner() {
  const invalidate = useInvalidateBanners();
  return useMutation({
    mutationFn: (id: string) => bannersApi.remove(id),
    onSuccess: invalidate,
  });
}
