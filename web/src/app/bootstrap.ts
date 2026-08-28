import { wakeUp } from "@/api/wake";
import { authApi, initAuthInterceptor } from "@/features/auth/api/authApi";
import { initI18n } from "@/i18n";
import { useAuthStore } from "@/stores/authStore";

/**
 * How long the boot-time refresh may hold up the first paint.
 *
 * On a warm host it returns in tens of milliseconds and this never fires. On a
 * cold Render instance it cannot return until the process has booted, which is
 * thirty to sixty seconds — and the original code awaited it unconditionally,
 * so the whole app was a blank page for the length of a cold start.
 */
const REFRESH_PAINT_BUDGET_MS = 2_500;

/**
 * Initializes app singletons, i18n and auth interceptors, and tries a silent
 * refresh before the first render so returning learners never see a login
 * screen.
 *
 * The refresh is raced against a budget rather than awaited outright. Losing
 * the race is not an error and does not clear the session: the request is still
 * in flight, and it updates the store when it lands. Until then `status` stays
 * `idle`, which the route guards deliberately do not treat as signed out — so a
 * returning learner on a cold host sees the app, not a login screen and not a
 * blank one.
 */
export async function initApp(): Promise<void> {
  initI18n();
  initAuthInterceptor();

  // Not awaited. The point is to start the host booting now, while i18n and the
  // first paint happen, instead of at the moment something needs data.
  void wakeUp();

  const refreshed = authApi.refresh().catch(() => {
    // A refresh that genuinely fails means there is no session to resume. That
    // is the ordinary state for a first-time visitor, and it is now also the
    // state a signed-out guest browses in.
    useAuthStore.getState().clearAuth();
  });

  await Promise.race([
    refreshed,
    new Promise((resolve) => setTimeout(resolve, REFRESH_PAINT_BUDGET_MS)),
  ]);
}
