import { act, renderHook, waitFor } from "@testing-library/react";
import { http, HttpResponse } from "msw";
import { beforeEach, describe, expect, it } from "vitest";

import { usePreferencesSync } from "@/features/account/hooks/usePreferencesSync";
import { initI18n } from "@/i18n";
import {
  usePreferencesStore,
  type Preferences,
} from "@/stores/preferencesStore";
import { server } from "./msw-server";

const stored: Preferences = {
  locale: "en",
  theme: "dark",
  daily_goal_minutes: 30,
  notification_channels: ["in_app", "email"],
  quiet_hours: null,
  ai_processing_opt_out: false,
  practice_level: "B2",
  updated_at: "2026-09-12T00:00:00Z",
};

describe("usePreferencesSync", () => {
  beforeEach(async () => {
    usePreferencesStore.getState().clear();
    await initI18n("en");
  });

  /**
   * `PUT /me/preferences` replaces the whole record, and an omitted
   * `practice_level` is stored as null. A theme toggle that built its body
   * field by field would clear the level the daily practice card saved.
   */
  it("keeps the practice level when the theme is toggled", async () => {
    let replaced: Record<string, unknown> | null = null;
    server.use(
      http.get("/api/v1/me/preferences", () => HttpResponse.json(stored)),
      http.put("/api/v1/me/preferences", async ({ request }) => {
        replaced = (await request.json()) as Record<string, unknown>;
        return HttpResponse.json({ ...stored, ...replaced });
      }),
    );

    const { result } = renderHook(() => usePreferencesSync(true));
    await waitFor(() =>
      expect(usePreferencesStore.getState().loaded).toBe(true),
    );

    act(() => result.current.setThemeChoice("light"));

    await waitFor(() => expect(replaced).not.toBeNull());
    expect(replaced).toMatchObject({ theme: "light", practice_level: "B2" });
  });
});
