import { describe, it, expect, beforeEach, vi } from "vitest";
import {
  render,
  screen,
  waitFor,
  fireEvent,
  within,
} from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { ReactElement } from "react";
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
import { DeviceSessionsList } from "@/features/account/components/DeviceSessionsList";
import { GoogleAccountLink } from "@/features/account/components/GoogleAccountLink";
import { SecurityOverview } from "@/features/account/components/SecurityOverview";
import { ChangePasswordModal } from "@/features/account/components/ChangePasswordModal";
import { DataPrivacySettings } from "@/features/account/components/DataPrivacySettings";
import type {
  UserProfile,
  UserPreferences,
} from "@/features/account/api/accountApi";
import { useAuthStore, registerQueryClient } from "@/stores/authStore";
import { usePreferencesStore } from "@/stores/preferencesStore";

// The report writers are loaded with import() on click; the tests only need to
// know they were loaded and called. Their output is covered in
// account-report.test.ts.
const { exportAccountXlsx, exportAccountPdf } = vi.hoisted(() => ({
  exportAccountXlsx: vi.fn(() => Promise.resolve()),
  exportAccountPdf: vi.fn(() => Promise.resolve()),
}));
vi.mock("@/features/account/export/reportXlsx", () => ({ exportAccountXlsx }));
vi.mock("@/features/account/export/reportPdf", () => ({ exportAccountPdf }));

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

function renderWithQuery(ui: ReactElement) {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return render(
    <QueryClientProvider client={client}>{ui}</QueryClientProvider>,
  );
}

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
    // A save writes the shared store, and the next save builds on it; each
    // test starts from what it renders.
    beforeEach(() => usePreferencesStore.getState().clear());

    it("has no notification, quiet-hours or AI grading controls", () => {
      render(<PreferencesSettings initialPreferences={mockPreferences} />);

      // AI grading is always on now, and notifications are not edited here.
      expect(screen.queryByRole("switch")).not.toBeInTheDocument();
      expect(screen.queryByRole("checkbox")).not.toBeInTheDocument();
      expect(screen.queryByText(/AI Grading/i)).not.toBeInTheDocument();
      expect(screen.queryByText(/Quiet Hours/i)).not.toBeInTheDocument();
      expect(
        screen.queryByText(/Notification Channels/i),
      ).not.toBeInTheDocument();
      expect(screen.getByLabelText(/Daily goal/i)).toBeInTheDocument();
    });

    it("sends back the stored notification, quiet-hours and AI values unchanged", async () => {
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

      // An opted-out learner stays opted out: removing the switch must not
      // flip what the server stored.
      const stored: UserPreferences = {
        ...mockPreferences,
        notification_channels: ["push"],
        quiet_hours: { start: "23:00", end: "06:30" },
        ai_processing_opt_out: true,
      };
      const user = userEvent.setup();
      render(<PreferencesSettings initialPreferences={stored} />);

      await user.selectOptions(screen.getByLabelText(/Daily goal/i), "45");
      await user.click(
        screen.getByRole("button", { name: /Save Preferences/i }),
      );

      await waitFor(() =>
        expect(replacedPreferences).toEqual({
          locale: "vi",
          theme: "dark",
          daily_goal_minutes: 45,
          notification_channels: ["push"],
          quiet_hours: { start: "23:00", end: "06:30" },
          ai_processing_opt_out: true,
          practice_level: "B1",
        }),
      );
    });

    it("updates the study goal", async () => {
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

    it("clears the practice level when set back to not chosen", async () => {
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

      const level = screen.getByLabelText(/Daily practice level/i);
      expect(level).toHaveValue("B1");
      await user.selectOptions(level, "");
      await user.click(
        screen.getByRole("button", { name: /Save Preferences/i }),
      );

      await waitFor(() =>
        expect(replacedPreferences).toMatchObject({
          practice_level: null,
          daily_goal_minutes: 30,
        }),
      );
    });

    it("saves the appearance card without touching the learning fields", async () => {
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

      // Each card saves on its own: a goal changed but not saved stays out.
      await user.selectOptions(screen.getByLabelText(/Daily goal/i), "60");
      const save = screen.getByRole("button", { name: /Save appearance/i });
      expect(save).toBeDisabled();
      await user.click(screen.getByRole("radio", { name: /System/i }));
      await user.click(save);

      await waitFor(() =>
        expect(replacedPreferences).toMatchObject({
          theme: "system",
          locale: "vi",
          daily_goal_minutes: 30,
          practice_level: "B1",
        }),
      );
    });
  });

  describe("DeviceSessionsList (devices & sign-ins)", () => {
    function renderDevices(onLoggedOut?: () => void) {
      const client = new QueryClient({
        defaultOptions: { queries: { retry: false } },
      });
      return render(
        <QueryClientProvider client={client}>
          <DeviceSessionsList onLoggedOut={onLoggedOut} />
        </QueryClientProvider>,
      );
    }

    const session = (
      id: string,
      label: string | null,
      lastSeen: string,
      current = false,
    ) => ({
      id,
      current,
      device_label: label,
      created_at: "2026-08-15T12:00:00Z",
      last_seen_at: lastSeen,
    });

    const trusted = (id: string, label: string, current = false) => ({
      id,
      current,
      label,
      trusted_at: "2026-08-01T10:00:00Z",
      last_seen_at: "2026-08-18T10:00:00Z",
      idle_expires_at: "2026-11-18T10:00:00Z",
      absolute_expires_at: "2027-02-18T10:00:00Z",
    });

    it("shows each device once, with its sign-ins and trust grouped under it", async () => {
      // The duplication learners saw: three sign-ins from one laptop plus its
      // trust record used to render as four rows across two cards.
      server.use(
        http.get("/api/v1/auth/sessions", () =>
          HttpResponse.json({
            sessions: [
              session("s-1", "Chrome on Windows", "2026-08-18T09:00:00Z", true),
              session("s-2", "Chrome on Windows", "2026-08-17T09:00:00Z"),
              session("s-3", "Chrome on Windows", "2026-08-10T09:00:00Z"),
              session("s-4", "Safari on iPhone", "2026-08-16T09:00:00Z"),
            ],
          }),
        ),
        http.get("/api/v1/auth/devices", () =>
          HttpResponse.json({
            devices: [trusted("d-1", "Chrome on Windows", true)],
          }),
        ),
      );

      const user = userEvent.setup();
      renderDevices();

      const rows = await screen.findAllByTestId("device-row");
      expect(rows).toHaveLength(2);
      expect(rows[0]).toHaveTextContent("Chrome on Windows");
      expect(rows[0]).toHaveTextContent("This device");
      expect(rows[0]).toHaveTextContent("Trusted");
      expect(rows[0]).toHaveTextContent("3 sign-ins");
      expect(rows[1]).toHaveTextContent("Safari on iPhone");
      expect(rows[1]).toHaveTextContent("1 sign-in");
      expect(screen.getAllByText("Chrome on Windows")).toHaveLength(1);

      // The current device cannot be signed out from here; another can.
      expect(
        screen.queryByRole("button", {
          name: /Sign out of Chrome on Windows/i,
        }),
      ).not.toBeInTheDocument();
      expect(
        screen.getByRole("button", { name: /Sign out of Safari on iPhone/i }),
      ).toBeInTheDocument();

      // The card's menu lists the device's sign-ins; this one cannot be
      // ended here.
      await user.click(
        screen.getByRole("button", {
          name: /More actions for Chrome on Windows/i,
        }),
      );
      await user.click(
        await screen.findByRole("menuitem", { name: /Show sign-ins/i }),
      );
      const signIns = screen.getByRole("list", {
        name: /Sign-ins on Chrome on Windows/i,
      });
      expect(within(signIns).getAllByRole("listitem")).toHaveLength(3);
      expect(within(signIns).getByText("This session")).toBeInTheDocument();
      expect(
        within(signIns).getAllByRole("button", { name: /End the sign-in/i }),
      ).toHaveLength(2);
    });

    it("signs a whole device out: its trust and every one of its sessions", async () => {
      const untrusted: string[] = [];
      const revoked: string[] = [];
      let sessions = [
        session("s-1", "Firefox on Linux", "2026-08-18T09:00:00Z", true),
        session("s-2", "Safari on iPhone", "2026-08-17T09:00:00Z"),
        session("s-3", "Safari on iPhone", "2026-08-12T09:00:00Z"),
      ];
      let devices = [trusted("d-2", "Safari on iPhone")];
      server.use(
        http.get("/api/v1/auth/sessions", () =>
          HttpResponse.json({ sessions }),
        ),
        http.get("/api/v1/auth/devices", () => HttpResponse.json({ devices })),
        http.delete("/api/v1/auth/devices/:id", ({ params }) => {
          untrusted.push(params.id as string);
          devices = [];
          // Untrusting revokes the device's trusted session server-side.
          sessions = sessions.filter((s) => s.id !== "s-2");
          return new HttpResponse(null, { status: 204 });
        }),
        http.delete("/api/v1/auth/sessions/:id", ({ params }) => {
          const id = params.id as string;
          if (!sessions.some((s) => s.id === id)) {
            return HttpResponse.json(
              { status: 404, title: "Not Found", code: "RESOURCE_NOT_FOUND" },
              { status: 404 },
            );
          }
          revoked.push(id);
          sessions = sessions.filter((s) => s.id !== id);
          return new HttpResponse(null, { status: 204 });
        }),
      );

      const user = userEvent.setup();
      renderDevices();

      await user.click(
        await screen.findByRole("button", {
          name: /Sign out of Safari on iPhone/i,
        }),
      );
      const dialog = await screen.findByRole("alertdialog");
      expect(dialog).toHaveTextContent("Sign out of Safari on iPhone?");
      expect(dialog).toHaveTextContent("It will also stop being trusted.");
      await user.click(
        within(dialog).getByRole("button", { name: "Sign out" }),
      );

      await waitFor(() =>
        expect(screen.getAllByTestId("device-row")).toHaveLength(1),
      );
      expect(untrusted).toEqual(["d-2"]);
      // s-2 went with the trust; s-3 was signed in without "stay signed in".
      expect(revoked).toEqual(["s-3"]);
      expect(screen.queryByText("Safari on iPhone")).not.toBeInTheDocument();
      expect(screen.queryByRole("alert")).not.toBeInTheDocument();
    });

    it("ends a single sign-in from the device details", async () => {
      let revokedId: string | null = null;
      server.use(
        http.get("/api/v1/auth/sessions", () =>
          HttpResponse.json({
            sessions: [
              session("session-1", "Safari on iPhone", "2026-08-18T09:00:00Z"),
            ],
          }),
        ),
        http.get("/api/v1/auth/devices", () =>
          HttpResponse.json({ devices: [] }),
        ),
        http.delete("/api/v1/auth/sessions/:id", ({ params }) => {
          revokedId = params.id as string;
          return new HttpResponse(null, { status: 204 });
        }),
      );

      const user = userEvent.setup();
      renderDevices();

      await user.click(
        await screen.findByRole("button", { name: /More actions for/i }),
      );
      await user.click(
        await screen.findByRole("menuitem", { name: /Show sign-ins/i }),
      );
      await user.click(
        screen.getByRole("button", { name: /End the sign-in/i }),
      );

      await waitFor(() => expect(revokedId).toBe("session-1"));
    });

    it("warns before untrusting the device in hand and then signs out", async () => {
      const onLoggedOut = vi.fn();
      let untrustedId: string | null = null;
      server.use(
        http.get("/api/v1/auth/sessions", () =>
          HttpResponse.json({
            sessions: [
              session("s-1", "Chrome on macOS", "2026-08-18T09:00:00Z", true),
            ],
          }),
        ),
        http.get("/api/v1/auth/devices", () =>
          HttpResponse.json({
            devices: [trusted("device-1", "Chrome on macOS", true)],
          }),
        ),
        http.delete("/api/v1/auth/devices/:id", ({ params }) => {
          untrustedId = params.id as string;
          return new HttpResponse(null, { status: 204 });
        }),
      );

      const user = userEvent.setup();
      renderDevices(onLoggedOut);

      await user.click(
        await screen.findByRole("button", {
          name: /More actions for Chrome on macOS/i,
        }),
      );
      await user.click(
        await screen.findByRole("menuitem", { name: /Stop trusting/i }),
      );

      const dialog = await screen.findByRole("alertdialog");
      expect(dialog).toHaveTextContent(/Stop trusting this device\?/i);
      expect(dialog).toHaveTextContent(/device you are using right now/i);
      await user.click(
        within(dialog).getByRole("button", { name: /Untrust & Sign Out/i }),
      );

      await waitFor(() => expect(untrustedId).toBe("device-1"));
      await waitFor(() => expect(onLoggedOut).toHaveBeenCalledTimes(1));
    });

    it("says so when the list cannot be read instead of showing no devices", async () => {
      server.use(
        http.get("/api/v1/auth/sessions", () =>
          HttpResponse.json(
            { status: 500, title: "Internal Server Error" },
            { status: 500 },
          ),
        ),
        http.get("/api/v1/auth/devices", () =>
          HttpResponse.json({ devices: [] }),
        ),
      );

      renderDevices();

      expect(await screen.findByRole("alert")).toBeInTheDocument();
      expect(
        screen.queryByText(/No devices are signed in/i),
      ).not.toBeInTheDocument();
    });

    it("signs out every other device and session, never this one", async () => {
      const untrusted: string[] = [];
      const revoked: string[] = [];
      let sessions = [
        session("s-1", "Chrome on Windows", "2026-08-18T09:00:00Z", true),
        // Same label as this browser, but another sign-in: it goes too.
        session("s-2", "Chrome on Windows", "2026-08-17T09:00:00Z"),
        session("s-3", "Safari on iPhone", "2026-08-16T09:00:00Z"),
      ];
      let devices = [
        trusted("d-1", "Chrome on Windows", true),
        trusted("d-2", "Safari on iPhone"),
      ];
      server.use(
        http.get("/api/v1/auth/sessions", () =>
          HttpResponse.json({ sessions }),
        ),
        http.get("/api/v1/auth/devices", () => HttpResponse.json({ devices })),
        http.delete("/api/v1/auth/devices/:id", ({ params }) => {
          untrusted.push(params.id as string);
          devices = devices.filter((d) => d.id !== params.id);
          // Untrusting revokes that device's session server-side.
          sessions = sessions.filter((s) => s.id !== "s-3");
          return new HttpResponse(null, { status: 204 });
        }),
        http.delete("/api/v1/auth/sessions/:id", ({ params }) => {
          const id = params.id as string;
          if (!sessions.some((s) => s.id === id)) {
            return HttpResponse.json(
              { status: 404, title: "Not Found", code: "RESOURCE_NOT_FOUND" },
              { status: 404 },
            );
          }
          revoked.push(id);
          sessions = sessions.filter((s) => s.id !== id);
          return new HttpResponse(null, { status: 204 });
        }),
      );

      const user = userEvent.setup();
      renderDevices();

      await user.click(
        await screen.findByRole("button", {
          name: /Sign out all other devices/i,
        }),
      );
      const dialog = await screen.findByRole("alertdialog");
      expect(dialog).toHaveTextContent("Every other active sign-in (2)");
      await user.click(
        within(dialog).getByRole("button", {
          name: /Sign out all other devices/i,
        }),
      );

      await waitFor(() =>
        expect(screen.getAllByTestId("device-row")).toHaveLength(1),
      );
      expect(untrusted).toEqual(["d-2"]);
      expect(revoked).toEqual(["s-2"]);
      expect(screen.queryByRole("alert")).not.toBeInTheDocument();
      // Only this device is left, so there is nothing more to sign out.
      expect(
        screen.getByRole("button", { name: /Sign out all other devices/i }),
      ).toBeDisabled();
    });

    it("shows the current device as in use, with when others were last active", async () => {
      server.use(
        http.get("/api/v1/auth/sessions", () =>
          HttpResponse.json({
            sessions: [
              session("s-1", "Chrome on Windows", "2026-08-18T09:00:00Z", true),
              session("s-2", "Safari on iPhone", "2026-08-16T09:00:00Z"),
            ],
          }),
        ),
        http.get("/api/v1/auth/devices", () =>
          HttpResponse.json({ devices: [] }),
        ),
      );

      renderDevices();

      const [current, other] = await screen.findAllByTestId("device-row");
      expect(current).toHaveTextContent("Active now");
      expect(
        within(current!).getByRole("button", { name: /In use/i }),
      ).toBeDisabled();
      // Browser and OS are split out of the server's coarse label.
      expect(other).toHaveTextContent(/Safari\s*•\s*,?\s*iPhone/);
      // A relative time ("2 months ago"), machine-readable underneath.
      const lastActive = other!.querySelector(
        'time[datetime="2026-08-16T09:00:00Z"]',
      );
      expect(lastActive?.textContent).toMatch(/\d/);
      // The API stores no IP address or location, so neither is shown.
      expect(screen.queryByText(/IP address/i)).not.toBeInTheDocument();
      expect(screen.queryByText(/location/i)).not.toBeInTheDocument();
    });
  });

  describe("SecurityOverview", () => {
    function renderOverview(google: { linked: boolean; can_unlink: boolean }) {
      server.use(
        http.get("/api/v1/auth/oauth/google", () => HttpResponse.json(google)),
        http.get("/api/v1/auth/sessions", () =>
          HttpResponse.json({
            sessions: [
              {
                id: "s-1",
                current: true,
                device_label: "Chrome on Windows",
                created_at: "2026-08-15T12:00:00Z",
                last_seen_at: "2026-08-18T09:00:00Z",
              },
              {
                id: "s-2",
                current: false,
                device_label: "Safari on iPhone",
                created_at: "2026-08-15T12:00:00Z",
                last_seen_at: "2026-08-17T09:00:00Z",
              },
            ],
          }),
        ),
        http.get("/api/v1/auth/devices", () =>
          HttpResponse.json({ devices: [] }),
        ),
      );
      return renderWithQuery(<SecurityOverview />);
    }

    it("suggests connecting Google when only a password is set", async () => {
      renderOverview({ linked: false, can_unlink: false });

      expect(
        await screen.findByRole("heading", {
          name: /protected, with 1 suggestion/i,
        }),
      ).toBeInTheDocument();
      expect(screen.getByText("Password set")).toBeInTheDocument();
      expect(screen.getByText("Google not connected")).toBeInTheDocument();
      expect(await screen.findByText("2 active sessions")).toBeInTheDocument();
      expect(screen.getByText("on 2 devices")).toBeInTheDocument();
      // No claims the API cannot back.
      expect(screen.queryByText(/two-factor|2FA/i)).not.toBeInTheDocument();
      expect(screen.queryByText(/strong/i)).not.toBeInTheDocument();
    });

    it("reads a Google-only account as having no password", async () => {
      renderOverview({ linked: true, can_unlink: false });

      expect(await screen.findByText("No password yet")).toBeInTheDocument();
      expect(screen.getByText("Google connected")).toBeInTheDocument();
    });

    it("says all is in place when both sign-in methods are set up", async () => {
      renderOverview({ linked: true, can_unlink: true });

      expect(
        await screen.findByRole("heading", {
          name: /All your sign-in protections are in place/i,
        }),
      ).toBeInTheDocument();
    });
  });

  describe("SecuritySettings", () => {
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

      const user = userEvent.setup();
      renderWithQuery(<GoogleAccountLink />);

      const unlinkBtn = await screen.findByRole("button", { name: /Unlink/i });
      await user.click(unlinkBtn);
      const dialog = await screen.findByRole("alertdialog");
      expect(dialog).toHaveTextContent(/disconnect your Google account/i);
      await user.click(within(dialog).getByRole("button", { name: /Unlink/i }));

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

      renderWithQuery(<GoogleAccountLink />);

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

    it("offers Excel and PDF reports, loading each writer only when pressed", async () => {
      exportAccountXlsx.mockClear();
      exportAccountPdf.mockClear();
      const user = userEvent.setup();
      render(<DataPrivacySettings initialProfile={mockProfile} />);

      expect(exportAccountXlsx).not.toHaveBeenCalled();
      expect(exportAccountPdf).not.toHaveBeenCalled();

      await user.click(
        screen.getByRole("button", { name: /Download Excel \(\.xlsx\)/i }),
      );
      await waitFor(() => expect(exportAccountXlsx).toHaveBeenCalledTimes(1));
      expect(
        await screen.findByText(/Your report has been downloaded/i),
      ).toBeInTheDocument();

      await user.click(screen.getByRole("button", { name: /Download PDF/i }));
      await waitFor(() => expect(exportAccountPdf).toHaveBeenCalledTimes(1));

      // The server-built GDPR archive stays, as the secondary option.
      expect(
        screen.getByRole("button", { name: /Request Data Export/i }),
      ).toBeInTheDocument();
    });

    it("says so when a report cannot be created", async () => {
      exportAccountPdf.mockClear();
      exportAccountPdf.mockRejectedValueOnce(new Error("font failed"));
      const user = userEvent.setup();
      render(<DataPrivacySettings initialProfile={mockProfile} />);

      await user.click(screen.getByRole("button", { name: /Download PDF/i }));

      expect(
        await screen.findByText(/Could not create the report/i),
      ).toBeInTheDocument();
      expect(
        screen.queryByText(/Your report has been downloaded/i),
      ).not.toBeInTheDocument();
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
        http.get("/api/v1/auth/sessions", () =>
          HttpResponse.json({ sessions: [] }),
        ),
        http.get("/api/v1/auth/devices", () =>
          HttpResponse.json({ devices: [] }),
        ),
        http.get("/api/v1/auth/oauth/google", () =>
          HttpResponse.json({ linked: false, can_unlink: false }),
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

    it("opens on the profile section with the section nav beside it", async () => {
      await renderSettings("/settings");

      const nav = await screen.findByRole("navigation", {
        name: /Account Settings/i,
      });
      // One nav at every width: a strip of pills below lg, a side card above.
      const tabs = within(nav).getAllByRole("button");
      expect(tabs).toHaveLength(4);
      for (const tab of tabs) expect(tab).toHaveClass("min-h-[44px]");
      expect(
        within(nav).getByRole("button", { name: /Profile & Preferences/i }),
      ).toHaveAttribute("aria-current", "page");
      expect(
        within(nav).getByRole("button", { name: /Learning Profile/i }),
      ).toBeInTheDocument();
      expect(
        within(nav).getByRole("button", { name: /Security & Devices/i }),
      ).toBeInTheDocument();
      expect(
        within(nav).getByRole("button", { name: /Data & Privacy/i }),
      ).toBeInTheDocument();

      // The page title is the open section; the old mobile hub and its Back
      // button are gone, so the content shows at every width.
      expect(
        screen.getByRole("heading", {
          level: 1,
          name: /Profile & Preferences/i,
        }),
      ).toBeInTheDocument();
      expect(
        screen.queryByRole("button", { name: /Back/i }),
      ).not.toBeInTheDocument();
      const panel = screen.getByRole("region", {
        name: /Profile & Preferences/i,
      });
      expect(panel).not.toHaveClass("hidden");

      // Hero, completeness, then the three cards.
      expect(
        screen.getByRole("heading", { level: 2, name: "Nghi Nguyen" }),
      ).toBeInTheDocument();
      expect(screen.getByText("Personal account")).toBeInTheDocument();
      expect(
        screen.getByRole("button", { name: /Change avatar/i }),
      ).toBeInTheDocument();
      // No remove-avatar operation exists, so there is no Remove button.
      expect(
        screen.queryByRole("button", { name: /Remove photo|Delete photo/i }),
      ).not.toBeInTheDocument();
      // Name, country and birth date, photo, verified email and practice
      // level are set; the learning profile has no goal (no exam, no target).
      const progress = await screen.findByRole("progressbar", {
        name: /Personalised experience/i,
      });
      expect(progress).toHaveAttribute("aria-valuenow", "4");
      expect(progress).toHaveAttribute("aria-valuemax", "5");
      expect(screen.getByDisplayValue("Nghi Nguyen")).toBeInTheDocument();
      expect(screen.getByLabelText(/Daily goal/i)).toBeInTheDocument();
      expect(
        screen.getByRole("radiogroup", { name: /Theme Preference/i }),
      ).toBeInTheDocument();
      // No density control: no preference stores one.
      expect(screen.queryByText(/Density|Compact/i)).not.toBeInTheDocument();
    });

    it("switches sections from the nav and keeps them in the URL", async () => {
      const user = userEvent.setup();
      const { router } = await renderSettings("/settings");

      await user.click(
        await screen.findByRole("button", { name: /Security & Devices/i }),
      );
      await waitFor(() => {
        expect(router.state.location.search).toEqual({ section: "security" });
      });
      expect(
        await screen.findByRole("heading", {
          level: 1,
          name: /Security & Devices/i,
        }),
      ).toBeInTheDocument();
      expect(
        await screen.findByText(/Password & Authentication/i),
      ).toBeInTheDocument();
      expect(
        await screen.findByText("Google not connected"),
      ).toBeInTheDocument();
      expect(screen.getByText(/Remote sign-out/i)).toBeInTheDocument();
      // No two-factor action: the API has no such operation.
      expect(
        screen.queryByRole("button", { name: /two-factor|2FA/i }),
      ).not.toBeInTheDocument();

      await user.click(
        screen.getByRole("button", { name: /Learning Profile/i }),
      );
      await waitFor(() => {
        expect(router.state.location.search).toEqual({ section: "learning" });
      });
      expect(
        await screen.findByRole("heading", { name: /Weekly study time/i }),
      ).toBeInTheDocument();
    });

    it("opens the password dialog from the password row", async () => {
      const user = userEvent.setup();
      await renderSettings("/settings?section=security");

      await user.click(
        await screen.findByRole("button", { name: /^Password/i }),
      );
      expect(
        await screen.findByRole("dialog", { name: /Change Password/i }),
      ).toBeInTheDocument();
    });

    it("covers direct-URL deep link", async () => {
      const { router } = await renderSettings("/settings?section=security");

      expect(router.state.location.search).toEqual({ section: "security" });
      expect(
        await screen.findByText(/Password & Authentication/i),
      ).toBeInTheDocument();
      expect(
        screen.getByRole("button", { name: /Security & Devices/i }),
      ).toHaveAttribute("aria-current", "page");
    });

    it("sends an old link to the removed preferences tab to the profile tab", async () => {
      const { router } = await renderSettings("/settings?section=preferences");

      expect(router.state.location.search).toEqual({ section: "preferences" });

      // The learning preferences now live on the profile tab, beside the profile.
      expect(await screen.findByLabelText(/Daily goal/i)).toBeInTheDocument();
      expect(document.getElementById("learning-preferences")).toContainElement(
        screen.getByLabelText(/Daily goal/i),
      );
      expect(screen.getByDisplayValue("Nghi Nguyen")).toBeInTheDocument();
      expect(
        screen.getByRole("button", { name: /Profile & Preferences/i }),
      ).toHaveAttribute("aria-current", "page");
      expect(
        screen.getByRole("heading", {
          level: 1,
          name: /Profile & Preferences/i,
        }),
      ).toBeInTheDocument();
    });

    it("no longer shows notification, quiet-hours or AI grading controls", async () => {
      await renderSettings("/settings?section=profile");

      expect(await screen.findByLabelText(/Daily goal/i)).toBeInTheDocument();
      expect(screen.queryByRole("switch")).not.toBeInTheDocument();
      expect(screen.queryByText(/AI Grading/i)).not.toBeInTheDocument();
      expect(screen.queryByText(/Quiet Hours/i)).not.toBeInTheDocument();
      expect(
        screen.queryByText(/Notification Channels/i),
      ).not.toBeInTheDocument();
      expect(
        screen.queryByText(/Notifications & Quiet Hours/i),
      ).not.toBeInTheDocument();
    });

    it("covers unknown section fallback", async () => {
      await renderSettings("/settings?section=unknown_junk");

      // An unknown section opens the first one, profile.
      const profileTab = await screen.findByRole("button", {
        name: /Profile & Preferences/i,
      });
      expect(profileTab).toHaveAttribute("aria-current", "page");
      expect(
        screen.getByRole("region", { name: /Profile & Preferences/i }),
      ).toBeInTheDocument();
      expect(screen.getByDisplayValue("Nghi Nguyen")).toBeInTheDocument();
    });
  });
});
