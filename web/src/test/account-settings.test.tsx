import { describe, it, expect, beforeEach, vi } from "vitest";
import { render, screen, waitFor, fireEvent } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { http, HttpResponse } from "msw";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import {
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
  RouterProvider,
} from "@tanstack/react-router";
import { I18nextProvider } from "react-i18next";
import i18n, { initI18n } from "@/i18n";
import { server } from "./msw-server";
import { AccountSettingsPage } from "@/pages/AccountSettingsPage";
import { ProfileSettings } from "@/features/account/components/ProfileSettings";
import { PreferencesSettings } from "@/features/account/components/PreferencesSettings";
import { AvatarUploadModal } from "@/features/account/components/AvatarUploadModal";
import { DevicesList } from "@/features/account/components/DevicesList";
import { SessionsList } from "@/features/account/components/SessionsList";
import { GoogleAccountLink } from "@/features/account/components/GoogleAccountLink";
import { ChangePasswordModal } from "@/features/account/components/ChangePasswordModal";
import { DataPrivacySettings } from "@/features/account/components/DataPrivacySettings";
import type {
  UserProfile,
  UserPreferences,
} from "@/features/account/api/accountApi";
import { useAuthStore, registerQueryClient } from "@/stores/authStore";

const mockProfile: UserProfile = {
  id: "user-123",
  email: "learner@example.com",
  status: "active",
  email_verified_at: "2026-08-10T10:00:00Z",
  created_at: "2026-08-01T10:00:00Z",
  updated_at: "2026-08-10T10:00:00Z",
  profile: {
    display_name: "Nghi Nguyen",
    avatar_url: "https://storage.example.com/avatar.jpg",
    country: "VN",
    timezone: "Asia/Ho_Chi_Minh",
    date_of_birth: "1998-05-12",
  },
};

const mockPreferences: UserPreferences = {
  locale: "vi",
  theme: "dark",
  daily_goal_minutes: 30,
  notification_channels: ["in_app", "email"],
  quiet_hours: {
    start: "22:00",
    end: "07:00",
  },
  ai_processing_opt_out: false,
  practice_level: "B1",
  updated_at: "2026-08-10T10:00:00Z",
};

describe("Account Management Settings (P5.2)", () => {
  beforeEach(() => {
    useAuthStore.getState().setAuthSession({
      access_token: "valid-test-token",
      token_type: "Bearer",
      expires_in: 900,
      user_id: "user-123",
      role: "user",
    });
  });

  describe("ProfileSettings", () => {
    it("renders profile details and allows updating display name and timezone", async () => {
      let patchedData: unknown = null;

      server.use(
        http.patch("/api/v1/me", async ({ request }) => {
          patchedData = await request.json();
          return HttpResponse.json({
            ...mockProfile,
            profile: {
              ...mockProfile.profile,
              ...(patchedData as object),
            },
          });
        }),
      );

      const user = userEvent.setup();
      render(<ProfileSettings initialProfile={mockProfile} />);

      expect(screen.getByDisplayValue("Nghi Nguyen")).toBeInTheDocument();
      expect(screen.getByText("learner@example.com")).toBeInTheDocument();
      expect(screen.getByText("Verified")).toBeInTheDocument();

      const nameInput = screen.getByLabelText(/Display Name/i);
      await user.clear(nameInput);
      await user.type(nameInput, "Nghi Updated");

      const saveBtn = screen.getByRole("button", { name: /Save Changes/i });
      await user.click(saveBtn);

      await waitFor(() => {
        expect(
          screen.getByText(/Profile updated successfully/i),
        ).toBeInTheDocument();
      });

      expect(patchedData).toMatchObject({
        display_name: "Nghi Updated",
      });
    });

    it("searches timezones by city, country, or UTC offset", async () => {
      const user = userEvent.setup();
      render(<ProfileSettings initialProfile={mockProfile} />);

      const tzTrigger = screen.getByRole("combobox", { name: /Timezone/i });
      await user.click(tzTrigger);

      const searchInput = screen.getByPlaceholderText(/Search timezone/i);

      // Search by city "Hanoi"
      await user.type(searchInput, "Hanoi");
      expect(
        screen.getByRole("option", { name: /Asia\/Ho Chi Minh/i }),
      ).toBeInTheDocument();

      // Clear and search by country "Vietnam"
      await user.clear(searchInput);
      await user.type(searchInput, "Vietnam");
      expect(
        screen.getByRole("option", { name: /Asia\/Ho Chi Minh/i }),
      ).toBeInTheDocument();

      // Clear and search by offset "+7"
      await user.clear(searchInput);
      await user.type(searchInput, "+7");
      expect(
        screen.getByRole("option", { name: /Asia\/Ho Chi Minh/i }),
      ).toBeInTheDocument();

      // Select it
      const option = screen.getByRole("option", {
        name: /Asia\/Ho Chi Minh/i,
      });
      await user.click(option);
      expect(tzTrigger).toHaveTextContent(/Asia\/Ho Chi Minh/i);
    });

    it("searches countries by name or code and saves the chosen code", async () => {
      let patchedData: unknown = null;
      server.use(
        http.patch("/api/v1/me", async ({ request }) => {
          patchedData = await request.json();
          return HttpResponse.json({
            ...mockProfile,
            profile: { ...mockProfile.profile, ...(patchedData as object) },
          });
        }),
      );
      const user = userEvent.setup();
      render(<ProfileSettings initialProfile={mockProfile} />);

      const countryTrigger = screen.getByRole("combobox", { name: /Country/i });
      expect(countryTrigger).toHaveTextContent(/Vietnam/i);
      await user.click(countryTrigger);

      const searchInput = screen.getByPlaceholderText(/Search country/i);
      await user.type(searchInput, "JP");
      await user.click(screen.getByRole("option", { name: /Japan/i }));
      expect(countryTrigger).toHaveTextContent(/Japan/i);

      await user.click(screen.getByRole("button", { name: /Save Changes/i }));
      await waitFor(() => {
        expect(patchedData).toMatchObject({ country: "JP" });
      });
    });
  });

  describe("Avatar Direct-to-Storage Upload", () => {
    it("uploads avatar directly to storage presigned URL and confirms with API", async () => {
      let directStorageUploadCalled = false;
      let confirmAvatarCalled = false;
      const directStorageUrl =
        "https://minio.storage.local/avatars/upload-dest";

      server.use(
        // 1. Upload Intent
        http.post("/api/v1/me/avatar/upload-intent", () => {
          return HttpResponse.json({
            upload_url: directStorageUrl,
            method: "PUT",
            object_key: "users/user-123/2026/08/avatar-raw.png",
            expires_at: "2026-08-18T21:00:00Z",
            max_bytes: 5242880,
            content_type: "image/png",
          });
        }),
        // 2. Direct Storage PUT
        http.put(directStorageUrl, () => {
          directStorageUploadCalled = true;
          return new HttpResponse(null, { status: 200 });
        }),
        // 3. Confirm Avatar
        http.put("/api/v1/me/avatar", async ({ request }) => {
          confirmAvatarCalled = true;
          const body = (await request.json()) as { object_key: string };
          expect(body.object_key).toBe("users/user-123/2026/08/avatar-raw.png");
          return HttpResponse.json({
            ...mockProfile,
            profile: {
              ...mockProfile.profile,
              avatar_url: "https://storage.example.com/new-avatar.png",
            },
          });
        }),
      );

      const handleSuccess = vi.fn();
      render(
        <AvatarUploadModal
          isOpen={true}
          onClose={() => {}}
          onSuccess={handleSuccess}
        />,
      );

      const file = new File(["dummy png content"], "test-avatar.png", {
        type: "image/png",
      });

      const fileInput = document.getElementById(
        "avatar-file-input",
      ) as HTMLInputElement;
      fireEvent.change(fileInput, { target: { files: [file] } });

      const uploadBtn = screen.getByRole("button", { name: /Save Avatar/i });
      await userEvent.click(uploadBtn);

      await waitFor(() => {
        expect(directStorageUploadCalled).toBe(true);
        expect(confirmAvatarCalled).toBe(true);
        expect(handleSuccess).toHaveBeenCalledWith(
          "https://storage.example.com/new-avatar.png",
        );
      });
    });
  });

  describe("PreferencesSettings", () => {
    it("uses switches for quiet hours and AI grading, with AI shown as on when not opted out", async () => {
      let replacedPreferences: unknown = null;
      server.use(
        http.put("/api/v1/me/preferences", async ({ request }) => {
          replacedPreferences = await request.json();
          return HttpResponse.json({
            ...mockPreferences,
            ...(replacedPreferences as object),
          });
        }),
      );

      const user = userEvent.setup();
      render(<PreferencesSettings initialPreferences={mockPreferences} />);

      const ai = screen.getByRole("switch", { name: /AI Grading/i });
      expect(ai).toHaveAttribute("aria-checked", "true");
      expect(
        screen.getByRole("switch", { name: /Quiet Hours/i }),
      ).toHaveAttribute("aria-checked", "true");

      await user.click(ai);
      expect(ai).toHaveAttribute("aria-checked", "false");
      expect(screen.getByText(/^Off:/)).toBeInTheDocument();

      await user.click(
        screen.getByRole("button", { name: /Save Preferences/i }),
      );
      await waitFor(() =>
        expect(replacedPreferences).toMatchObject({
          ai_processing_opt_out: true,
        }),
      );
    });

    it("updates study goal, notification channels and quiet hours", async () => {
      let replacedPreferences: unknown = null;

      server.use(
        http.put("/api/v1/me/preferences", async ({ request }) => {
          replacedPreferences = await request.json();
          return HttpResponse.json({
            ...mockPreferences,
            ...(replacedPreferences as object),
          });
        }),
      );

      const user = userEvent.setup();
      render(<PreferencesSettings initialPreferences={mockPreferences} />);

      // A select now, not a number box: the field offers the goals the server
      // accepts rather than trusting a learner not to type 1130.
      await user.selectOptions(screen.getByLabelText(/Daily goal/i), "45");

      const saveBtn = screen.getByRole("button", { name: /Save Preferences/i });
      await user.click(saveBtn);

      await waitFor(() => {
        expect(
          screen.getByText(/Preferences updated successfully/i),
        ).toBeInTheDocument();
      });

      expect(replacedPreferences).toMatchObject({
        daily_goal_minutes: 45,
        // Not on this form. A replacement that left it out would clear the
        // level the daily practice card saved.
        practice_level: "B1",
      });
    });
  });

  describe("SecuritySettings & DevicesList", () => {
    it("lists trusted devices and allows untrusting with confirmation", async () => {
      let untrustedId: string | null = null;

      server.use(
        http.get("/api/v1/auth/devices", () => {
          return HttpResponse.json({
            devices: [
              {
                id: "device-1",
                label: "Chrome on macOS",
                trusted_at: "2026-08-01T10:00:00Z",
                last_seen_at: "2026-08-18T10:00:00Z",
                idle_expires_at: "2026-11-18T10:00:00Z",
                absolute_expires_at: "2027-02-18T10:00:00Z",
              },
            ],
          });
        }),
        http.delete("/api/v1/auth/devices/:id", ({ params }) => {
          untrustedId = params.id as string;
          return new HttpResponse(null, { status: 204 });
        }),
      );

      const user = userEvent.setup();
      render(<DevicesList />);

      await waitFor(() => {
        expect(screen.getByText("Chrome on macOS")).toBeInTheDocument();
      });

      const untrustBtn = screen.getByRole("button", {
        name: /Untrust device/i,
      });
      await user.click(untrustBtn);

      expect(
        screen.getByText(/Stop trusting this device\?/i),
      ).toBeInTheDocument();

      const confirmBtn = screen.getByRole("button", {
        name: /Untrust & Sign Out/i,
      });
      await user.click(confirmBtn);

      await waitFor(() => {
        expect(untrustedId).toBe("device-1");
      });
    });

    it("lists sessions and revokes session", async () => {
      let revokedId: string | null = null;

      server.use(
        http.get("/api/v1/auth/sessions", () => {
          return HttpResponse.json({
            sessions: [
              {
                id: "session-1",
                current: false,
                device_label: "Safari on iPhone",
                created_at: "2026-08-15T12:00:00Z",
                last_seen_at: "2026-08-18T09:00:00Z",
              },
            ],
          });
        }),
        http.delete("/api/v1/auth/sessions/:id", ({ params }) => {
          revokedId = params.id as string;
          return new HttpResponse(null, { status: 204 });
        }),
      );

      const user = userEvent.setup();
      render(<SessionsList />);

      await waitFor(() => {
        expect(screen.getByText("Safari on iPhone")).toBeInTheDocument();
      });

      const revokeBtn = screen.getByRole("button", { name: /Revoke/i });
      await user.click(revokeBtn);

      await waitFor(() => {
        expect(revokedId).toBe("session-1");
      });
    });

    it("does not offer revoke button on current session", async () => {
      server.use(
        http.get("/api/v1/auth/sessions", () => {
          return HttpResponse.json({
            sessions: [
              {
                id: "session-curr",
                current: true,
                device_label: "This Browser",
                created_at: "2026-08-15T12:00:00Z",
                last_seen_at: "2026-08-18T09:00:00Z",
              },
            ],
          });
        }),
      );

      render(<SessionsList />);

      await waitFor(() => {
        expect(screen.getByText("This Browser")).toBeInTheDocument();
      });

      expect(
        screen.queryByRole("button", { name: /Revoke/i }),
      ).not.toBeInTheDocument();
      expect(screen.getByText(/Current Device/i)).toBeInTheDocument();
    });

    it("puts the current session first and shows five until asked for more", async () => {
      const others = Array.from({ length: 8 }, (_, i) => ({
        id: `session-${i}`,
        current: false,
        device_label: `Chrome on Windows ${i}`,
        created_at: "2026-08-15T12:00:00Z",
        last_seen_at: `2026-08-${String(10 + i).padStart(2, "0")}T09:00:00Z`,
      }));
      server.use(
        http.get("/api/v1/auth/sessions", () =>
          HttpResponse.json({
            sessions: [
              ...others,
              {
                id: "session-curr",
                current: true,
                device_label: "This Browser",
                created_at: "2026-08-15T12:00:00Z",
                last_seen_at: "2026-08-01T09:00:00Z",
              },
            ],
          }),
        ),
      );

      const user = userEvent.setup();
      render(<SessionsList />);

      await screen.findByText("This Browser");
      const rows = screen.getAllByRole("listitem");
      expect(rows).toHaveLength(5);
      expect(rows[0]).toHaveTextContent("This Browser");
      // Most recently active next.
      expect(rows[1]).toHaveTextContent("Chrome on Windows 7");

      await user.click(
        screen.getByRole("button", { name: "Show 4 more sessions" }),
      );
      expect(screen.getAllByRole("listitem")).toHaveLength(9);
      await user.click(screen.getByRole("button", { name: "Show fewer" }));
      expect(screen.getAllByRole("listitem")).toHaveLength(5);
    });

    it("refuses unlinking Google when LAST_SIGN_IN_METHOD is returned", async () => {
      server.use(
        http.delete("/api/v1/auth/oauth/google", () => {
          return HttpResponse.json(
            {
              title: "Last sign in method",
              status: 409,
              code: "LAST_SIGN_IN_METHOD",
              detail: "Cannot unlink only sign-in method",
            },
            { status: 409 },
          );
        }),
      );

      // Linked, and the server says it may be unlinked. The 409 above is the
      // crafted case: the client believed it could, and the server refused
      // anyway — which is the half of "both, always" this test covers.
      server.use(
        http.get("/api/v1/auth/oauth/google", () =>
          HttpResponse.json({
            linked: true,
            linked_at: "2026-08-01T00:00:00Z",
            can_unlink: true,
          }),
        ),
      );

      vi.spyOn(window, "confirm").mockReturnValue(true);

      const user = userEvent.setup();
      render(<GoogleAccountLink />);

      const unlinkBtn = await screen.findByRole("button", { name: /Unlink/i });
      await user.click(unlinkBtn);

      await waitFor(() => {
        expect(
          screen.getByText(
            /Cannot unlink Google account: it is your only sign-in method/i,
          ),
        ).toBeInTheDocument();
      });
    });

    it("does not offer Unlink when Google is the only sign-in method", async () => {
      // The other half: the interface prevents the common case before the
      // learner can reach the refusal.
      server.use(
        http.get("/api/v1/auth/oauth/google", () =>
          HttpResponse.json({
            linked: true,
            linked_at: "2026-08-01T00:00:00Z",
            can_unlink: false,
          }),
        ),
      );

      render(<GoogleAccountLink />);

      const unlinkBtn = await screen.findByRole("button", { name: /Unlink/i });
      expect(unlinkBtn).toBeDisabled();
      expect(unlinkBtn).toHaveAttribute(
        "title",
        expect.stringContaining("only sign-in method"),
      );
    });

    it("changes password and notifies about revoked sessions", async () => {
      server.use(
        http.post("/api/v1/auth/change-password", () => {
          return HttpResponse.json({
            changed_at: "2026-08-18T20:00:00Z",
            sessions_revoked: 3,
          });
        }),
      );

      const handleSuccess = vi.fn();
      const user = userEvent.setup();
      render(
        <ChangePasswordModal
          isOpen={true}
          onClose={() => {}}
          onSuccess={handleSuccess}
        />,
      );

      await user.type(
        screen.getByLabelText(/Current Password/i),
        "oldpassword123",
      );
      await user.type(
        screen.getByLabelText(/^New Password/i),
        "newpassword123",
      );
      await user.type(
        screen.getByLabelText(/Confirm New Password/i),
        "newpassword123",
      );

      const submitBtn = screen.getByRole("button", {
        name: /Update Password/i,
      });
      await user.click(submitBtn);

      await waitFor(() => {
        expect(handleSuccess).toHaveBeenCalledWith(3);
      });
    });
  });

  describe("DataPrivacySettings", () => {
    it("requests GDPR export", async () => {
      server.use(
        http.post("/api/v1/me/export", () => {
          return HttpResponse.json({
            id: "export-1",
            status: "pending",
            created_at: "2026-08-18T20:00:00Z",
          });
        }),
      );

      const user = userEvent.setup();
      render(<DataPrivacySettings initialProfile={mockProfile} />);

      const exportBtn = screen.getByRole("button", {
        name: /Request Data Export/i,
      });
      await user.click(exportBtn);

      await waitFor(() => {
        expect(screen.getByText(/Data export requested/i)).toBeInTheDocument();
      });
    });

    it("requests account deletion with 30-day grace period and allows cancelling", async () => {
      server.use(
        http.delete("/api/v1/me", () => {
          return HttpResponse.json({
            id: "del-1",
            user_id: "user-123",
            status: "pending",
            requested_at: "2026-08-18T20:00:00Z",
            execute_at: "2026-09-17T20:00:00Z",
          });
        }),
        http.post("/api/v1/me/deletion/cancel", () => {
          return HttpResponse.json({
            id: "del-1",
            user_id: "user-123",
            status: "cancelled",
            requested_at: "2026-08-18T20:00:00Z",
            execute_at: "2026-09-17T20:00:00Z",
            cancelled_at: "2026-08-18T20:05:00Z",
          });
        }),
      );

      const user = userEvent.setup();
      render(<DataPrivacySettings initialProfile={mockProfile} />);

      const startDeleteBtn = screen.getByRole("button", {
        name: /Delete My Account/i,
      });
      await user.click(startDeleteBtn);

      expect(
        screen.getByText(/Are you absolutely sure\?/i),
      ).toBeInTheDocument();

      const confirmInput = screen.getByPlaceholderText("DELETE");
      await user.type(confirmInput, "DELETE");

      const confirmBtn = screen.getByRole("button", {
        name: /Confirm Deletion Request/i,
      });
      await user.click(confirmBtn);

      await waitFor(() => {
        expect(
          screen.getByText(/Account deletion is currently scheduled/i),
        ).toBeInTheDocument();
      });

      const cancelBtn = screen.getByRole("button", {
        name: /Cancel Deletion & Restore Account/i,
      });
      await user.click(cancelBtn);

      await waitFor(() => {
        expect(
          screen.getByText(/Account deletion cancelled/i),
        ).toBeInTheDocument();
      });
    });
  });

  describe("Auth store query cache invalidation", () => {
    function session(userID: string, token: string) {
      return {
        access_token: token,
        token_type: "Bearer" as const,
        expires_in: 900,
        user_id: userID,
        role: "user" as const,
      };
    }

    it("clears the cache when a different person signs in", () => {
      // Store state is module-global and other cases in this file leave a
      // session behind; start from signed-out so the first transition is real.
      useAuthStore.getState().clearAuth();
      const clear = vi.fn();
      registerQueryClient({ clear });

      useAuthStore.getState().setAuthSession(session("user-123", "token-1"));
      expect(clear).toHaveBeenCalledTimes(1);

      useAuthStore.getState().clearAuth();
      expect(clear).toHaveBeenCalledTimes(2);

      useAuthStore.getState().setAuthSession(session("user-456", "token-2"));
      expect(clear).toHaveBeenCalledTimes(3);
    });

    it("keeps the cache when only the access token rotates", () => {
      // authApi.refresh() calls setAuthSession with a fresh token for the same
      // person, on boot and on every retried 401. Clearing there empties the
      // cache on a routine event and refetches every screen behind it — the
      // opposite of what the cache is for.
      useAuthStore.getState().clearAuth();
      useAuthStore.getState().setAuthSession(session("user-123", "token-1"));
      const clear = vi.fn();
      registerQueryClient({ clear });

      useAuthStore.getState().setAuthSession(session("user-123", "token-2"));
      useAuthStore.getState().setAuthSession(session("user-123", "token-3"));

      expect(clear).not.toHaveBeenCalled();
    });
  });

  describe("AccountSettingsPage routing and mobile navigation (TASK-W2-SETTINGS)", () => {
    beforeEach(async () => {
      await initI18n("en");
      await i18n.changeLanguage("en");

      server.use(
        http.get("/api/v1/me", () => HttpResponse.json(mockProfile)),
        http.get("/api/v1/me/preferences", () =>
          HttpResponse.json(mockPreferences),
        ),
        http.get("/api/v1/me/sessions", () =>
          HttpResponse.json({ sessions: [] }),
        ),
        http.get("/api/v1/me/trusted-devices", () =>
          HttpResponse.json({ devices: [] }),
        ),
        http.get("/api/v1/me/oauth/google", () =>
          HttpResponse.json({ connected: false, email: null }),
        ),
        http.get("/api/v1/me/learning-profile", () =>
          HttpResponse.json({
            declared_level: "B1",
            target_exam: "none",
            weekly_study_minutes_target: 90,
            learning_goals: [],
            interests: [],
            occupations: [],
            reminders_enabled: false,
          }),
        ),
      );
    });

    async function renderSettings(initialEntry = "/settings") {
      const client = new QueryClient({
        defaultOptions: { queries: { retry: false } },
      });
      const rootRoute = createRootRoute();
      const settingsRoute = createRoute({
        getParentRoute: () => rootRoute,
        path: "/settings",
        component: () => (
          <I18nextProvider i18n={i18n}>
            <QueryClientProvider client={client}>
              <AccountSettingsPage />
            </QueryClientProvider>
          </I18nextProvider>
        ),
      });
      const router = createRouter({
        routeTree: rootRoute.addChildren([settingsRoute]),
        history: createMemoryHistory({ initialEntries: [initialEntry] }),
      });
      await router.load();
      const rendered = render(<RouterProvider router={router} />);
      return { ...rendered, router };
    }

    it("covers index -> subpage -> back navigation", async () => {
      const user = userEvent.setup();
      const { router } = await renderSettings("/settings");

      // On /settings without a section: shows index list of all sections
      expect(
        await screen.findByRole("button", { name: /Profile & Avatar/i }),
      ).toBeInTheDocument();
      expect(
        screen.getByRole("button", { name: /Learning Profile/i }),
      ).toBeInTheDocument();
      expect(
        screen.getByRole("button", { name: /Learning Preferences/i }),
      ).toBeInTheDocument();
      expect(
        screen.getByRole("button", { name: /Security & Devices/i }),
      ).toBeInTheDocument();
      expect(
        screen.getByRole("button", { name: /Data & Privacy/i }),
      ).toBeInTheDocument();

      // Check all rows have >= 44px min-height
      const navButtons = screen.getAllByRole("button", {
        name: /Profile|Learning|Security|Data/i,
      });
      expect(navButtons.length).toBe(5);
      for (const btn of navButtons) {
        expect(btn).toHaveClass("min-h-[44px]");
      }

      // Back button is not visible on the index
      expect(
        screen.queryByRole("button", { name: /Back/i }),
      ).not.toBeInTheDocument();

      // The tab panel is hidden on mobile below md (has "hidden md:block")
      const panel = screen.getByRole("region", {
        name: /Profile & Avatar/i,
      });
      expect(panel).toHaveClass("hidden");
      expect(panel).toHaveClass("md:block");

      // Selecting one navigates to /settings?section=<key>
      const securityBtn = screen.getByRole("button", {
        name: /Security & Devices/i,
      });
      await user.click(securityBtn);

      await waitFor(() => {
        expect(router.state.location.search).toEqual({ section: "security" });
      });

      // Shows that section with a header back button that returns to /settings
      const backBtn = await screen.findByRole("button", { name: /Back/i });
      expect(backBtn).toBeInTheDocument();
      expect(backBtn).toHaveClass("min-h-[44px]");

      // Header shows the section title
      expect(
        screen.getByRole("heading", {
          level: 2,
          name: /Security & Devices/i,
        }),
      ).toBeInTheDocument();

      // Security section content is displayed
      expect(
        await screen.findByText(/Password & Authentication/i),
      ).toBeInTheDocument();

      // Navigation is hidden below md (has "hidden md:grid")
      const nav = screen.getByRole("navigation", {
        name: /Account Settings/i,
      });
      expect(nav).toHaveClass("hidden");
      expect(nav).toHaveClass("md:grid");

      // Clicking back button returns to /settings
      await user.click(backBtn);

      await waitFor(() => {
        expect(router.state.location.search).toEqual({});
      });

      // Back button is no longer rendered
      expect(
        screen.queryByRole("button", { name: /Back/i }),
      ).not.toBeInTheDocument();

      // Navigation is visible on index again
      expect(nav).not.toHaveClass("hidden");
    });

    it("covers direct-URL deep link", async () => {
      const { router } = await renderSettings("/settings?section=preferences");

      expect(router.state.location.search).toEqual({ section: "preferences" });

      // Preferences section is displayed
      expect(await screen.findByText(/Daily goal/i)).toBeInTheDocument();

      // Side-tab is selected (aria-current="page")
      const prefTab = screen.getByRole("button", {
        name: /Learning Preferences/i,
      });
      expect(prefTab).toHaveAttribute("aria-current", "page");

      // Subpage header back button is rendered
      expect(screen.getByRole("button", { name: /Back/i })).toBeInTheDocument();
    });

    it("covers unknown section fallback", async () => {
      await renderSettings("/settings?section=unknown_junk");

      // Unknown section falls back: /settings still opens
      expect(
        await screen.findByRole("button", { name: /Profile & Avatar/i }),
      ).toBeInTheDocument();

      // On mobile: falls back to index (no back button, nav visible)
      expect(
        screen.queryByRole("button", { name: /Back/i }),
      ).not.toBeInTheDocument();
      const nav = screen.getByRole("navigation", {
        name: /Account Settings/i,
      });
      expect(nav).not.toHaveClass("hidden");

      // On desktop: falls back to first tab ("profile")
      const profileTab = screen.getByRole("button", {
        name: /Profile & Avatar/i,
      });
      expect(profileTab).toHaveAttribute("aria-current", "page");

      // Tab panel has "hidden md:block" (index on mobile, first tab on desktop)
      const panel = screen.getByRole("region", {
        name: /Profile & Avatar/i,
      });
      expect(panel).toHaveClass("hidden");
      expect(panel).toHaveClass("md:block");
      expect(screen.getByDisplayValue("Nghi Nguyen")).toBeInTheDocument();
    });
  });
});
