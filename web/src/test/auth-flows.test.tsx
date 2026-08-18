import * as React from "react";
import { act, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { http, HttpResponse } from "msw";
import { beforeEach, describe, expect, it, vi } from "vitest";
import {
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
  RouterProvider,
} from "@tanstack/react-router";

import {
  ForgotPasswordForm,
  GoogleButton,
  LoginForm,
  OtpVerificationScreen,
  RegisterForm,
  ResetPasswordForm,
  type Challenge,
} from "@/features/auth";
import { useAuthStore } from "@/stores/authStore";
import type { components } from "@/types/api";
import { server } from "./msw-server";

async function renderWithRouter(ui: React.ReactElement) {
  const rootRoute = createRootRoute();
  const indexRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: "/",
    component: () => ui,
  });
  const router = createRouter({
    routeTree: rootRoute.addChildren([indexRoute]),
    history: createMemoryHistory({ initialEntries: ["/"] }),
  });
  await router.load();
  return render(<RouterProvider router={router} />);
}

describe("Auth Flows (Tasks 1, 3, 4, 5, 6, 7, 8)", () => {
  beforeEach(() => {
    useAuthStore.getState().clearAuth();
    vi.restoreAllMocks();
  });

  describe("Task 6: LoginForm", () => {
    it("submits email and password with remember_device true and device_id", async () => {
      const user = userEvent.setup();
      let requestBody: Record<string, unknown> | null = null;

      server.use(
        http.post("/api/v1/auth/login", async ({ request }) => {
          requestBody = (await request.json()) as Record<string, unknown>;
          return HttpResponse.json({
            access_token: "login-token-123",
            token_type: "Bearer",
            expires_in: 900,
            user_id: "user-uuid-1",
            role: "user",
          });
        }),
      );

      const onSuccess = vi.fn();
      await renderWithRouter(<LoginForm onSuccess={onSuccess} />);

      await user.type(
        screen.getByLabelText(/email address/i),
        "learner@fluentra.test",
      );
      await user.type(
        screen.getByLabelText(/password/i),
        "CorrectPassword123!",
      );
      await user.click(screen.getByRole("button", { name: /^sign in$/i }));

      await waitFor(() => {
        expect(onSuccess).toHaveBeenCalled();
      });

      expect(requestBody).toMatchObject({
        email: "learner@fluentra.test",
        password: "CorrectPassword123!",
        remember_device: true,
      });
      expect(
        typeof (requestBody as Record<string, unknown> | null)?.device_id,
      ).toBe("string");
      expect(useAuthStore.getState().accessToken).toBe("login-token-123");
    });

    it("displays error message mapped from error catalogue on invalid credentials", async () => {
      const user = userEvent.setup();
      server.use(
        http.post("/api/v1/auth/login", () => {
          return HttpResponse.json(
            { title: "Unauthorized", status: 401, code: "INVALID_CREDENTIALS" },
            { status: 401 },
          );
        }),
      );

      await renderWithRouter(<LoginForm />);

      await user.type(
        screen.getByLabelText(/email address/i),
        "wrong@fluentra.test",
      );
      await user.type(screen.getByLabelText(/password/i), "WrongPassword123!");
      await user.click(screen.getByRole("button", { name: /^sign in$/i }));

      const alert = await screen.findByRole("alert");
      expect(alert).toHaveTextContent(
        "The email or password you entered is incorrect.",
      );
    });
  });

  describe("Task 5 & 6: RegisterForm (Silent Collision & 1-char display name)", () => {
    it("returns 201 challenge and triggers OTP view for both fresh and existing emails", async () => {
      const user = userEvent.setup();
      const mockChallenge: Challenge = {
        challenge_id: "chal-uuid-123",
        purpose: "verify_email",
        expires_at: new Date(Date.now() + 600000).toISOString(),
        resend_after: new Date(Date.now() + 60000).toISOString(),
        attempts_remaining: 5,
      };

      server.use(
        http.post("/api/v1/auth/register", () => {
          return HttpResponse.json(mockChallenge, { status: 201 });
        }),
      );

      const onChallengeIssued = vi.fn();
      await renderWithRouter(
        <RegisterForm onChallengeIssued={onChallengeIssued} />,
      );

      await user.type(
        screen.getByLabelText(/email address/i),
        "newlearner@fluentra.test",
      );
      await user.type(screen.getByLabelText(/display name/i), "New Learner");
      await user.type(
        screen.getByLabelText(/password/i),
        "SuperStrongPassword123!",
      );
      await user.click(screen.getByRole("button", { name: /create account/i }));

      await waitFor(() => {
        expect(onChallengeIssued).toHaveBeenCalledWith(
          mockChallenge,
          "newlearner@fluentra.test",
        );
      });
    });

    it("accepts a 1-character display name matching OpenAPI schema", async () => {
      const user = userEvent.setup();
      let requestPayload: Record<string, unknown> | null = null;

      server.use(
        http.post("/api/v1/auth/register", async ({ request }) => {
          requestPayload = (await request.json()) as Record<string, unknown>;
          return HttpResponse.json(
            {
              challenge_id: "chal-1-char",
              purpose: "verify_email",
              expires_at: new Date(Date.now() + 600000).toISOString(),
              resend_after: new Date(Date.now() + 60000).toISOString(),
              attempts_remaining: 5,
            },
            { status: 201 },
          );
        }),
      );

      const onChallengeIssued = vi.fn();
      await renderWithRouter(
        <RegisterForm onChallengeIssued={onChallengeIssued} />,
      );

      await user.type(
        screen.getByLabelText(/email address/i),
        "single@fluentra.test",
      );
      await user.type(screen.getByLabelText(/display name/i), "A");
      await user.type(
        screen.getByLabelText(/password/i),
        "SuperStrongPassword123!",
      );
      await user.click(screen.getByRole("button", { name: /create account/i }));

      await waitFor(() => {
        expect(onChallengeIssued).toHaveBeenCalled();
      });

      expect(requestPayload).toMatchObject({
        display_name: "A",
        email: "single@fluentra.test",
      });
    });
  });

  describe("Task 1 & 7: GoogleButton (Popup message & lifecycle)", () => {
    it("signs the opener in when popup emits GOOGLE_AUTH_SUCCESS message", async () => {
      const user = userEvent.setup();
      server.use(
        http.get("/api/v1/auth/oauth/google/start", () => {
          return HttpResponse.json({
            authorization_url:
              "https://accounts.google.com/o/oauth2/v2/auth?client_id=123",
          });
        }),
      );

      const mockPopup = {
        focus: vi.fn(),
        closed: false,
      };
      const openSpy = vi
        .spyOn(window, "open")
        .mockReturnValue(mockPopup as unknown as Window);

      const onSuccess = vi.fn();
      await renderWithRouter(<GoogleButton onSuccess={onSuccess} />);

      const button = screen.getByRole("button", {
        name: /continue with google/i,
      });
      await user.click(button);

      await waitFor(() => {
        expect(openSpy).toHaveBeenCalled();
      });

      // Simulate popup completing authentication and posting message to opener window
      act(() => {
        window.dispatchEvent(
          new MessageEvent("message", {
            origin: window.location.origin,
            data: {
              type: "GOOGLE_AUTH_SUCCESS",
              session: {
                access_token: "google-session-token-999",
                token_type: "Bearer",
                expires_in: 900,
                user_id: "google-user-1",
                role: "user",
              },
            },
          }),
        );
      });

      await waitFor(() => {
        expect(onSuccess).toHaveBeenCalled();
      });

      expect(useAuthStore.getState().accessToken).toBe(
        "google-session-token-999",
      );
      expect(useAuthStore.getState().status).toBe("authenticated");
    });

    it("resets isLoading without hanging when popup is closed without success message", async () => {
      const user = userEvent.setup();
      vi.useFakeTimers({ shouldAdvanceTime: true });

      server.use(
        http.get("/api/v1/auth/oauth/google/start", () => {
          return HttpResponse.json({
            authorization_url:
              "https://accounts.google.com/o/oauth2/v2/auth?client_id=123",
          });
        }),
      );

      const mockPopup = {
        focus: vi.fn(),
        closed: false,
      };
      vi.spyOn(window, "open").mockReturnValue(mockPopup as unknown as Window);

      await renderWithRouter(<GoogleButton />);

      const button = screen.getByRole("button", {
        name: /continue with google/i,
      });
      await user.click(button);

      // User closed the popup window without completing login
      mockPopup.closed = true;

      // Advance interval timer to trigger popup.closed check
      act(() => {
        vi.advanceTimersByTime(600);
      });

      await waitFor(() => {
        expect(button).not.toBeDisabled();
      });

      vi.useRealTimers();
    });

    it("falls back to full page redirect if popup is blocked", async () => {
      const user = userEvent.setup();
      const authUrl =
        "https://accounts.google.com/o/oauth2/v2/auth?client_id=123";

      server.use(
        http.get("/api/v1/auth/oauth/google/start", () => {
          return HttpResponse.json({ authorization_url: authUrl });
        }),
      );

      let assignedHref = "";
      const originalLocation = window.location;
      Object.defineProperty(window, "location", {
        configurable: true,
        value: {
          ...originalLocation,
          get href() {
            return assignedHref || "http://localhost:3000/";
          },
          set href(val: string) {
            assignedHref = val;
          },
        },
      });

      // window.open returns null when blocked
      vi.spyOn(window, "open").mockReturnValue(null);

      await renderWithRouter(<GoogleButton />);

      const button = screen.getByRole("button", {
        name: /continue with google/i,
      });
      await user.click(button);

      // Verify it falls back to redirect
      await waitFor(() => {
        expect(assignedHref).toBe(authUrl);
      });

      Object.defineProperty(window, "location", {
        configurable: true,
        value: originalLocation,
      });
    });
  });

  describe("Task 3, 4, 6: OtpVerificationScreen (Real OpenAPI shape & Server Meta)", () => {
    const baseChallenge: Challenge = {
      challenge_id: "chal-uuid-456",
      purpose: "verify_email",
      expires_at: new Date(Date.now() + 600000).toISOString(),
      resend_after: new Date(Date.now() + 60000).toISOString(),
      attempts_remaining: 5,
    };

    it("automatically signs in learner using VerifiedChallenge OpenAPI schema shape", async () => {
      const user = userEvent.setup();

      const mockVerified: components["schemas"]["VerifiedChallenge"] = {
        purpose: "verify_email",
        verified_at: new Date().toISOString(),
        session: {
          access_token: "verified-session-token",
          token_type: "Bearer",
          expires_in: 900,
          user_id: "user-uuid-1",
          role: "user",
        },
      };

      server.use(
        http.post("/api/v1/auth/challenges/chal-uuid-456/verify", () => {
          return HttpResponse.json(mockVerified);
        }),
      );

      const onSuccess = vi.fn();
      await renderWithRouter(
        <OtpVerificationScreen
          challenge={baseChallenge}
          email="learner@fluentra.test"
          onSuccess={onSuccess}
        />,
      );

      const inputs = screen.getAllByRole("textbox");
      await user.click(inputs[0]!);
      await user.paste("123456");

      await waitFor(() => {
        expect(onSuccess).toHaveBeenCalledWith(mockVerified);
      });

      expect(useAuthStore.getState().accessToken).toBe(
        "verified-session-token",
      );
    });

    it("reads authoritative attempts_remaining from server Problem Details meta", async () => {
      const user = userEvent.setup();

      server.use(
        http.post("/api/v1/auth/challenges/chal-uuid-456/verify", () => {
          return HttpResponse.json(
            {
              type: "https://fluentra.dev/errors/code-invalid",
              title: "Invalid verification code",
              status: 400,
              code: "CODE_INVALID",
              meta: {
                attempts_remaining: 2,
              },
            },
            { status: 400 },
          );
        }),
      );

      await renderWithRouter(
        <OtpVerificationScreen
          challenge={baseChallenge}
          email="learner@fluentra.test"
          onSuccess={vi.fn()}
        />,
      );

      const inputs = screen.getAllByRole("textbox");
      await user.click(inputs[0]!);
      await user.paste("999999");

      await waitFor(() => {
        expect(screen.getByText(/attempts left:/i)).toHaveTextContent("2/5");
      });
    });

    it("falls back to local decrement when server Problem Details meta is absent", async () => {
      const user = userEvent.setup();

      server.use(
        http.post("/api/v1/auth/challenges/chal-uuid-456/verify", () => {
          return HttpResponse.json(
            {
              title: "Invalid code",
              status: 400,
              code: "CODE_INVALID",
            },
            { status: 400 },
          );
        }),
      );

      await renderWithRouter(
        <OtpVerificationScreen
          challenge={baseChallenge}
          email="learner@fluentra.test"
          onSuccess={vi.fn()}
        />,
      );

      const inputs = screen.getAllByRole("textbox");
      await user.click(inputs[0]!);
      await user.paste("888888");

      await waitFor(() => {
        expect(screen.getByText(/attempts left:/i)).toHaveTextContent("4/5");
      });
    });

    it("does not decrement attempts on network or 500 error", async () => {
      const user = userEvent.setup();

      server.use(
        http.post("/api/v1/auth/challenges/chal-uuid-456/verify", () => {
          return HttpResponse.error();
        }),
      );

      await renderWithRouter(
        <OtpVerificationScreen
          challenge={baseChallenge}
          email="learner@fluentra.test"
          onSuccess={vi.fn()}
        />,
      );

      const inputs = screen.getAllByRole("textbox");
      await user.click(inputs[0]!);
      await user.paste("777777");

      const alert = await screen.findByRole("alert");
      expect(alert).toHaveTextContent(/failed to verify code/i);
      expect(screen.getByText(/attempts left:/i)).toHaveTextContent("5/5");
    });
  });

  describe("Task 3 & 8: Forgot and Reset Password (PasswordChanged shape)", () => {
    it("uniform 202 Accepted response for forgot-password", async () => {
      const user = userEvent.setup();
      const mockChallenge: Challenge = {
        challenge_id: "reset-chal-789",
        purpose: "password_reset",
        expires_at: new Date(Date.now() + 600000).toISOString(),
        resend_after: new Date(Date.now() + 60000).toISOString(),
        attempts_remaining: 5,
      };

      server.use(
        http.post("/api/v1/auth/forgot-password", () => {
          return HttpResponse.json(mockChallenge, { status: 202 });
        }),
      );

      const onChallengeIssued = vi.fn();
      await renderWithRouter(
        <ForgotPasswordForm onChallengeIssued={onChallengeIssued} />,
      );

      await user.type(
        screen.getByLabelText(/email address/i),
        "anyemail@fluentra.test",
      );
      await user.click(
        screen.getByRole("button", { name: /send recovery code/i }),
      );

      await waitFor(() => {
        expect(onChallengeIssued).toHaveBeenCalledWith(
          mockChallenge,
          "anyemail@fluentra.test",
        );
      });
    });

    it("resets password using PasswordChanged OpenAPI schema and displays sessions revoked notice", async () => {
      const user = userEvent.setup();

      const mockPasswordChanged: components["schemas"]["PasswordChanged"] = {
        changed_at: new Date().toISOString(),
        sessions_revoked: 3,
      };

      server.use(
        http.post("/api/v1/auth/reset-password", () => {
          return HttpResponse.json(mockPasswordChanged);
        }),
      );

      const onSuccess = vi.fn();
      await renderWithRouter(
        <ResetPasswordForm
          challengeId="reset-chal-789"
          email="learner@fluentra.test"
          onSuccess={onSuccess}
        />,
      );

      const inputs = screen.getAllByRole("textbox");
      await user.click(inputs[0]!);
      await user.paste("654321");

      await user.type(
        screen.getByLabelText(/new password/i),
        "BrandNewSecurePassword123!",
      );
      await user.click(screen.getByRole("button", { name: /reset password/i }));

      const status = await screen.findByRole("status");
      expect(status).toHaveTextContent("3 active sessions were signed out");
      expect(onSuccess).toHaveBeenCalledWith(mockPasswordChanged);
    });
  });
});
