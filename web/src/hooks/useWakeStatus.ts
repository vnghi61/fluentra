import { useSyncExternalStore } from "react";

import { onWakeStatus, wakeStatus, type WakeStatus } from "@/api/wake";

/**
 * The API host's wake state, as React state.
 *
 * The module owns the subscription because `apiFetch` needs it too and neither
 * one may depend on the other. `useSyncExternalStore` rather than an effect and
 * a `useState`: the status can change between render and commit — a cold start
 * is exactly the case where it does — and this is the hook that does not tear.
 *
 * It lives in `hooks` and not in `components` because `components` may not
 * reach into `api`. The banner takes a prop.
 */
export function useWakeStatus(): WakeStatus {
  return useSyncExternalStore(onWakeStatus, wakeStatus, () => "unknown");
}
