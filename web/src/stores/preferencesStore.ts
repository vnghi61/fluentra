import { create } from "zustand";

import type { components } from "@/types/api";

/**
 * Typed from the generated schema rather than from `features/account`, because
 * `stores` may depend on `types` and not on a feature — the boundary is what
 * keeps a store from becoming a second home for feature logic.
 */
export type Preferences = components["schemas"]["Preferences"];

interface PreferencesState {
  /** Null until the first load, and for a signed-out visitor. */
  preferences: Preferences | null;
  loaded: boolean;
  set: (preferences: Preferences) => void;
  clear: () => void;
}

/**
 * The learner's stored preferences, held once.
 *
 * `PUT /me/preferences` replaces the whole object, so anything that changes one
 * field has to send the other six back unchanged. Loading them once and keeping
 * them here is what lets a theme toggle in the header do that without a read
 * before every write — and what stops the header and the settings screen from
 * becoming two sources of truth for the same row.
 */
export const usePreferencesStore = create<PreferencesState>((set) => ({
  preferences: null,
  loaded: false,
  set: (preferences) => set({ preferences, loaded: true }),
  clear: () => set({ preferences: null, loaded: false }),
}));
