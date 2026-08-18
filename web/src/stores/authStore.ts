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

export const useAuthStore = create<AuthState>((set) => ({
  accessToken: null,
  user: null,
  status: "idle",
  deviceId: getOrCreateDeviceId(),

  setAuthSession: (session: AuthSession) =>
    set({
      accessToken: session.access_token,
      user: {
        userId: session.user_id,
        role: session.role,
      },
      status: "authenticated",
    }),

  clearAuth: () =>
    set({
      accessToken: null,
      user: null,
      status: "unauthenticated",
    }),

  setStatus: (status: AuthStatus) => set({ status }),
}));

export function getInMemAccessToken(): string | null {
  return useAuthStore.getState().accessToken;
}
