import { authApi, initAuthInterceptor } from "@/features/auth/api/authApi";
import { initI18n } from "@/i18n";
import { useAuthStore } from "@/stores/authStore";

/**
 * Initializes app singletons, i18n, auth interceptors, and performs a boot-time
 * silent refresh before first render so returning learners never see a login screen.
 */
export async function initApp(): Promise<void> {
  initI18n();
  initAuthInterceptor();

  try {
    await authApi.refresh();
  } catch {
    useAuthStore.getState().clearAuth();
  }
}
