import { describe, expect, it } from "vitest";

import type {
  LearningProfile,
  UserPreferences,
  UserProfile,
} from "@/features/account/api/accountApi";
import { profileCompleteness } from "@/features/account/model/profileCompleteness";
import { securityStatus } from "@/features/account/model/securityStatus";
import {
  formatRelativeTime,
  splitDeviceLabel,
} from "@/features/account/components/settingsStyles";

const profile: UserProfile = {
  id: "u-1",
  email: "learner@example.com",
  status: "active",
  email_verified_at: null,
  created_at: "2026-08-01T10:00:00Z",
  updated_at: "2026-08-10T10:00:00Z",
  profile: {
    display_name: "Nghi",
    avatar_url: null,
    country: null,
    timezone: "Asia/Ho_Chi_Minh",
    date_of_birth: null,
  },
};

const preferences: UserPreferences = {
  locale: "vi",
  theme: "system",
  daily_goal_minutes: 15,
  notification_channels: ["in_app"],
  quiet_hours: null,
  ai_processing_opt_out: false,
  practice_level: null,
  updated_at: "2026-08-10T10:00:00Z",
};

const learning: LearningProfile = {
  declared_level: "B1",
  target_level: null,
  target_exam: "none",
  weekly_minutes_goal: 90,
  motivations: [],
  created_at: "2026-08-01T10:00:00Z",
  updated_at: "2026-08-01T10:00:00Z",
};

const done = (items: ReturnType<typeof profileCompleteness>) =>
  items.filter((item) => item.done).map((item) => item.key);

describe("profileCompleteness", () => {
  it("counts nothing for a bare new account", () => {
    expect(done(profileCompleteness(profile, preferences, null))).toEqual([]);
  });

  it("counts each step only from the field that backs it", () => {
    const full: UserProfile = {
      ...profile,
      email_verified_at: "2026-08-02T10:00:00Z",
      profile: {
        ...profile.profile,
        avatar_url: "https://example.com/a.png",
        country: "VN",
        date_of_birth: "1998-03-04",
      },
    };
    expect(
      done(
        profileCompleteness(
          full,
          { ...preferences, practice_level: "B1" },
          { ...learning, target_exam: "ielts" },
        ),
      ),
    ).toEqual([
      "basicInfo",
      "photo",
      "emailVerified",
      "learningGoals",
      "learningPreferences",
    ]);
  });

  it("does not count a learning profile with no goal in it", () => {
    expect(done(profileCompleteness(profile, preferences, learning))).toEqual(
      [],
    );
    expect(
      done(
        profileCompleteness(profile, preferences, {
          ...learning,
          target_level: "B2",
        }),
      ),
    ).toEqual(["learningGoals"]);
  });
});

describe("securityStatus", () => {
  it("an unlinked account signs in with a password", () => {
    expect(securityStatus({ linked: false, can_unlink: false })).toEqual({
      hasPassword: true,
      googleLinked: false,
      suggestions: ["connectGoogle"],
    });
  });

  it("a Google-only account has no password", () => {
    expect(securityStatus({ linked: true, can_unlink: false })).toEqual({
      hasPassword: false,
      googleLinked: true,
      suggestions: ["setPassword"],
    });
  });

  it("both methods leave nothing to suggest", () => {
    expect(
      securityStatus({ linked: true, can_unlink: true }).suggestions,
    ).toEqual([]);
  });
});

describe("device label helpers", () => {
  it("splits the server's '<browser> on <os>' label", () => {
    expect(splitDeviceLabel("Chrome on Windows")).toEqual({
      browser: "Chrome",
      os: "Windows",
    });
    expect(splitDeviceLabel("Mobile Safari on iOS")).toEqual({
      browser: "Mobile Safari",
      os: "iOS",
    });
    expect(splitDeviceLabel("Unknown")).toBeNull();
    expect(splitDeviceLabel(null)).toBeNull();
  });

  it("formats relative times in the given language", () => {
    const now = Date.parse("2026-08-18T12:00:00Z");
    expect(formatRelativeTime("2026-08-18T10:00:00Z", "en", now)).toBe(
      "2 hours ago",
    );
    expect(formatRelativeTime("2026-08-17T12:00:00Z", "en", now)).toBe(
      "yesterday",
    );
    expect(formatRelativeTime("not a date", "en", now)).toBe("");
  });
});
