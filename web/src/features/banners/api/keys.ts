/** Query keys for announcement banners, in one place so invalidation is greppable. */
export const bannerKeys = {
  all: ["banners"] as const,
  active: ["banners", "active"] as const,
  adminList: ["banners", "admin", "list"] as const,
};
