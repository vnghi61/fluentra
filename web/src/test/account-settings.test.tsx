import { describe, it, expect, beforeEach, vi } from "vitest";
import { render, screen, waitFor, fireEvent } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { http, HttpResponse } from "msw";
import { server } from "./msw-server";
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
import { useAuthStore } from "@/stores/authStore";

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

      const goalInput = screen.getByLabelText(/Daily Study Goal/i);
      await user.clear(goalInput);
      await user.type(goalInput, "45");

      const saveBtn = screen.getByRole("button", { name: /Save Preferences/i });
      await user.click(saveBtn);

      await waitFor(() => {
        expect(
          screen.getByText(/Preferences updated successfully/i),
        ).toBeInTheDocument();
      });

      expect(replacedPreferences).toMatchObject({
        daily_goal_minutes: 45,
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
});
