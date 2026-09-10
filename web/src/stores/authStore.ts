import { create } from "zustand";
import type { components } from "@/types/api";

export type AuthSession = components["schemas"]["AuthSession"];

export interface User {
  userId: string;
  role: "admin" | "user";
}

export type AuthStatus = "idle" | "authenticated" | "unauthenticated";

export interface AuthState {
  accessToken: string | null;
  user: User | null;
  status: AuthStatus;
  deviceId: string;
  setAuthSession: (session: AuthSession) => void;
  clearAuth: () => void;
  setStatus: (status: AuthStatus) => void;
}

const DEVICE_ID_STORAGE_KEY = "fluentra_device_id";

export function getOrCreateDeviceId(): string {
  if (typeof window === "undefined") {
    return "server-device-id";
  }
  try {
    let deviceId = window.localStorage.getItem(DEVICE_ID_STORAGE_KEY);
    if (!deviceId) {
      deviceId = crypto.randomUUID();
      window.localStorage.setItem(DEVICE_ID_STORAGE_KEY, deviceId);
    }
    return deviceId;
  } catch {
    return crypto.randomUUID();
  }
}

let registeredQueryClient: { clear: () => void } | null = null;

export function registerQueryClient(client: { clear: () => void }): () => void {
  registeredQueryClient = client;
  return () => {
    if (registeredQueryClient === client) {
      registeredQueryClient = null;
    }
  };
}

export function clearQueryCache(): void {
  registeredQueryClient?.clear();
}

export const useAuthStore = create<AuthState>((set) => ({
  accessToken: null,
  user: null,
  status: "idle",
  deviceId: getOrCreateDeviceId(),

  setAuthSession: (session: AuthSession) => {
    set({
      accessToken: session.access_token,
      user: {
        userId: session.user_id,
        role: session.role,
      },
      status: "authenticated",
    });
  },

  clearAuth: () => {
    set({
      accessToken: null,
      user: null,
      status: "unauthenticated",
    });
  },

  setStatus: (status: AuthStatus) => set({ status }),
}));

/**
 * Empties the query cache whenever the identity behind it changes.
 *
 * The cache is keyed by query, not by account, so everything in it belongs to
 * whoever was signed in when it was fetched. Signing out and signing in as
 * somebody else left `["account", "me"]` warm for five minutes: the header
 * greeted the new person by the previous person's name and drew their avatar.
 *
 * On `userId` alone, never on `accessToken`. The token rotates on every silent
 * refresh — boot, and each time a 401 is retried — for the same person, and
 * clearing there would throw the whole cache away on a routine event and
 * refetch every screen behind it.
 */
useAuthStore.subscribe((state, prevState) => {
  if (state.user?.userId !== prevState.user?.userId) {
    registeredQueryClient?.clear();
  }
});

export function getInMemAccessToken(): string | null {
  return useAuthStore.getState().accessToken;
}
