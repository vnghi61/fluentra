import { http, HttpResponse } from "msw";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { apiFetch } from "@/api/client";
import { initAuthInterceptor } from "@/features/auth/api/authApi";
import { getErrorMessage } from "@/lib/errors/catalogue";
import { useAuthStore } from "@/stores/authStore";
import { server } from "./msw-server";

describe("Token Handling and Session Boundary (Task 5)", () => {
  beforeEach(() => {
    useAuthStore.getState().clearAuth();
    initAuthInterceptor();
  });

  describe("In-memory Token Injection", () => {
    it("attaches in-memory access token to outgoing requests", async () => {
      useAuthStore.getState().setAuthSession({
        access_token: "in-mem-test-token",
        token_type: "Bearer",
        expires_in: 900,
        user_id: "user-uuid-1",
        role: "user",
      });

      let capturedAuthHeader: string | null = null;
      server.use(
        http.get("/api/v1/user/me", ({ request }) => {
          capturedAuthHeader = request.headers.get("Authorization");
          return HttpResponse.json({ id: "user-uuid-1" });
        }),
      );

      await apiFetch("/api/v1/user/me");
      expect(capturedAuthHeader).toBe("Bearer in-mem-test-token");
    });
  });

  describe("Single-Flight 401 Refresh", () => {
    it("handles 10 concurrent 401s with exactly ONE refresh call, then retries all 10", async () => {
      let refreshCallCount = 0;
      let protectedEndpointCalls = 0;

      // Initially set an expired token
      useAuthStore.getState().setAuthSession({
        access_token: "initial-expired-token",
        token_type: "Bearer",
        expires_in: 900,
        user_id: "user-uuid-1",
        role: "user",
      });

      server.use(
        http.get("/api/v1/protected/resource", ({ request }) => {
          protectedEndpointCalls++;
          const auth = request.headers.get("Authorization");
          if (auth === "Bearer fresh-rotated-token") {
            return HttpResponse.json({
              data: "success",
              call: protectedEndpointCalls,
            });
          }
          return HttpResponse.json(
            { title: "Token expired", status: 401, code: "TOKEN_EXPIRED" },
            { status: 401 },
          );
        }),

        http.post("/api/v1/auth/refresh", async () => {
          refreshCallCount++;
          // Add a tiny delay to simulate network latency and ensure racing concurrent requests overlap
          await new Promise((resolve) => setTimeout(resolve, 50));
          return HttpResponse.json({
            access_token: "fresh-rotated-token",
            token_type: "Bearer",
            expires_in: 900,
            user_id: "user-uuid-1",
            role: "user",
          });
        }),
      );

      // Fire 10 concurrent requests that will all hit 401 initially
      const promises = Array.from({ length: 10 }, () =>
        apiFetch<{ data: string }>("/api/v1/protected/resource"),
      );

      const results = await Promise.all(promises);

      // All 10 requests must succeed after retry
      expect(results).toHaveLength(10);
      results.forEach((res) => {
        expect(res.data).toBe("success");
      });

      // Crucial security & performance invariant: exactly ONE refresh call
      expect(refreshCallCount).toBe(1);

      // In-memory store updated with fresh token
      expect(useAuthStore.getState().accessToken).toBe("fresh-rotated-token");
    });

    it("clears auth store when refresh fails with 401", async () => {
      useAuthStore.getState().setAuthSession({
        access_token: "invalid-token",
        token_type: "Bearer",
        expires_in: 900,
        user_id: "user-uuid-1",
        role: "user",
      });

      server.use(
        http.get("/api/v1/protected/resource", () => {
          return HttpResponse.json(
            { title: "Unauthorized", status: 401 },
            { status: 401 },
          );
        }),
        http.post("/api/v1/auth/refresh", () => {
          return HttpResponse.json(
            { title: "Session revoked", status: 401, code: "SESSION_REVOKED" },
            { status: 401 },
          );
        }),
      );

      await expect(apiFetch("/api/v1/protected/resource")).rejects.toThrow();
      expect(useAuthStore.getState().accessToken).toBeNull();
      expect(useAuthStore.getState().user).toBeNull();
    });
  });

  describe("Error Catalogue Distinction", () => {
    it("distinguishes SESSION_ABSOLUTE_EXPIRED from SESSION_REVOKED in copy", () => {
      const expiredMsg = getErrorMessage({
        title: "Session Expired",
        status: 401,
        code: "SESSION_ABSOLUTE_EXPIRED",
      });
      const revokedMsg = getErrorMessage({
        title: "Session Revoked",
        status: 401,
        code: "SESSION_REVOKED",
      });

      expect(expiredMsg).toContain("reached its maximum lifetime");
      expect(revokedMsg).toContain("revoked or signed out");
      expect(expiredMsg).not.toEqual(revokedMsg);
    });

    it("logs a warning when encountering uncatalogued error codes", () => {
      const warnSpy = vi.spyOn(console, "warn").mockImplementation(() => {});
      const msg = getErrorMessage({
        title: "Custom Title",
        status: 400,
        code: "SOME_UNKNOWN_CODE",
      });

      expect(msg).toBe("Custom Title");
      expect(warnSpy).toHaveBeenCalledWith(
        expect.stringContaining(
          'Uncatalogued error code received: "SOME_UNKNOWN_CODE"',
        ),
      );
      warnSpy.mockRestore();
    });
  });
});
