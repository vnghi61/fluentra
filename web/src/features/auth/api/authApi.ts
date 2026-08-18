import { apiFetch, configureAuthInterceptor } from "@/api/client";
import { useAuthStore } from "@/stores/authStore";
import type { components } from "@/types/api";

export type LoginRequest = components["schemas"]["LoginRequest"];
export type RegisterRequest = components["schemas"]["RegisterRequest"];
export type Challenge = components["schemas"]["Challenge"];
export type VerifiedChallenge = components["schemas"]["VerifiedChallenge"];
export type VerifyChallengeRequest = components["schemas"]["VerifyChallengeRequest"];
export type ForgotPasswordRequest = components["schemas"]["ForgotPasswordRequest"];
export type ResetPasswordRequest = components["schemas"]["ResetPasswordRequest"];
export type PasswordChanged = components["schemas"]["PasswordChanged"];
export type OAuthStart = components["schemas"]["OAuthStart"];
export type OAuthCallbackRequest = components["schemas"]["OAuthCallbackRequest"];
export type AuthSession = components["schemas"]["AuthSession"];
export type Session = AuthSession;

export const authApi = {
  async login(body: LoginRequest): Promise<AuthSession> {
    const session = await apiFetch<AuthSession>("/api/v1/auth/login", {
      method: "POST",
      body: JSON.stringify(body),
    });
    useAuthStore.getState().setAuthSession(session);
    return session;
  },

  async register(body: RegisterRequest): Promise<Challenge> {
    return apiFetch<Challenge>("/api/v1/auth/register", {
      method: "POST",
      body: JSON.stringify(body),
    });
  },

  async verifyChallenge(
    challengeId: string,
    body: VerifyChallengeRequest,
  ): Promise<VerifiedChallenge> {
    const verified = await apiFetch<VerifiedChallenge>(
      `/api/v1/auth/challenges/${challengeId}/verify`,
      {
        method: "POST",
        body: JSON.stringify(body),
      },
    );
    if (verified.session) {
      useAuthStore.getState().setAuthSession(verified.session);
    }
    return verified;
  },

  async resendChallenge(challengeId: string): Promise<Challenge> {
    return apiFetch<Challenge>(
      `/api/v1/auth/challenges/${challengeId}/resend`,
      {
        method: "POST",
      },
    );
  },

  async refresh(): Promise<AuthSession> {
    const session = await apiFetch<AuthSession>("/api/v1/auth/refresh", {
      method: "POST",
    });
    useAuthStore.getState().setAuthSession(session);
    return session;
  },

  async logout(): Promise<void> {
    try {
      await apiFetch<void>("/api/v1/auth/logout", {
        method: "POST",
      });
    } finally {
      useAuthStore.getState().clearAuth();
    }
  },

  async forgotPassword(body: ForgotPasswordRequest): Promise<Challenge> {
    return apiFetch<Challenge>("/api/v1/auth/forgot-password", {
      method: "POST",
      body: JSON.stringify(body),
    });
  },

  async resetPassword(body: ResetPasswordRequest): Promise<PasswordChanged> {
    return apiFetch<PasswordChanged>("/api/v1/auth/reset-password", {
      method: "POST",
      body: JSON.stringify(body),
    });
  },

  async googleStart(redirectTo?: string): Promise<OAuthStart> {
    const query = redirectTo ? `?redirect_to=${encodeURIComponent(redirectTo)}` : "";
    return apiFetch<OAuthStart>(`/api/v1/auth/oauth/google/start${query}`, {
      method: "GET",
    });
  },

  async googleCallback(body: OAuthCallbackRequest): Promise<AuthSession> {
    const session = await apiFetch<AuthSession>(
      "/api/v1/auth/oauth/google/callback",
      {
        method: "POST",
        body: JSON.stringify(body),
      },
    );
    useAuthStore.getState().setAuthSession(session);
    return session;
  },

  async googleLink(body: OAuthCallbackRequest): Promise<AuthSession> {
    return apiFetch<AuthSession>("/api/v1/auth/oauth/google/link", {
      method: "POST",
      body: JSON.stringify(body),
    });
  },

  async googleUnlink(): Promise<void> {
    return apiFetch<void>("/api/v1/auth/oauth/google", {
      method: "DELETE",
    });
  },
};

/** Initialize the API fetch interceptor to talk to the in-memory auth store */
export function initAuthInterceptor(): void {
  configureAuthInterceptor({
    getToken: () => useAuthStore.getState().accessToken,
    onRefresh: async () => {
      try {
        const session = await authApi.refresh();
        return session.access_token;
      } catch {
        useAuthStore.getState().clearAuth();
        return null;
      }
    },
  });
}
