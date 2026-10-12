import { apiFetch } from "@/api/client";
import type {
  ActiveBannerList,
  Banner,
  BannerImage,
  BannerImageType,
  BannerImageUploadIntent,
  BannerList,
  BannerRequest,
} from "../model/banner";

export const bannersApi = {
  /** GET /api/v1/banners/active — any signed-in account. */
  listActive(): Promise<ActiveBannerList> {
    return apiFetch<ActiveBannerList>("/api/v1/banners/active");
  },

  /** GET /api/v1/admin/banners — requires system.flags. */
  listAll(): Promise<BannerList> {
    return apiFetch<BannerList>("/api/v1/admin/banners");
  },

  create(body: BannerRequest): Promise<Banner> {
    return apiFetch<Banner>("/api/v1/admin/banners", {
      method: "POST",
      body: JSON.stringify(body),
    });
  },

  update(id: string, body: BannerRequest): Promise<Banner> {
    return apiFetch<Banner>(`/api/v1/admin/banners/${encodeURIComponent(id)}`, {
      method: "PUT",
      body: JSON.stringify(body),
    });
  },

  /** POST /api/v1/admin/banners/image-upload-intent — a presigned upload. */
  requestImageUpload(
    contentType: BannerImageType,
  ): Promise<BannerImageUploadIntent> {
    return apiFetch<BannerImageUploadIntent>(
      "/api/v1/admin/banners/image-upload-intent",
      { method: "POST", body: JSON.stringify({ content_type: contentType }) },
    );
  },

  /** POST /api/v1/admin/banners/images — check, scale and publish an upload. */
  processImage(objectKey: string): Promise<BannerImage> {
    return apiFetch<BannerImage>("/api/v1/admin/banners/images", {
      method: "POST",
      body: JSON.stringify({ object_key: objectKey }),
    });
  },

  remove(id: string): Promise<void> {
    return apiFetch<void>(`/api/v1/admin/banners/${encodeURIComponent(id)}`, {
      method: "DELETE",
    });
  },
};
