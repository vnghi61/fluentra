import type { Page } from "@playwright/test";

import type { components } from "@/types/api";

/**
 * Response stubs for the layout specs.
 *
 * Stubbing is right here and wrong in a journey. A journey asserts on system
 * behaviour, so a stub means asserting on the stub; these specs assert on
 * rendered geometry, which the API cannot influence beyond supplying a shape.
 *
 * Every payload is typed against the generated schema, so a stub that drifts
 * from `api/openapi/` fails `pnpm run typecheck` rather than silently matching
 * nothing at runtime — which is exactly how the first version of journey 8 came
 * to assert against a screen the app never rendered.
 */

type Me = components["schemas"]["Me"];
type Preferences = components["schemas"]["Preferences"];
type TrustedDeviceList = components["schemas"]["TrustedDeviceList"];
type SessionList = components["schemas"]["SessionList"];
type Challenge = components["schemas"]["Challenge"];
type AuthSession = components["schemas"]["AuthSession"];

const NOW = "2026-08-18T00:00:00Z";
const LATER = "2026-11-18T00:00:00Z";

export const stubMe: Me = {
  id: "0199a1c2-3d4e-7f80-9abc-def012345678",
  email: "layout-learner@example.com",
  status: "active",
  email_verified_at: NOW,
  created_at: NOW,
  updated_at: NOW,
  profile: {
    display_name: "Layout Learner",
    timezone: "Asia/Ho_Chi_Minh",
  },
};

export const stubPreferences: Preferences = {
  locale: "en",
  theme: "dark",
  daily_goal_minutes: 15,
  notification_channels: ["in_app", "email"],
  ai_processing_opt_out: false,
  updated_at: NOW,
};

export const stubDevices: TrustedDeviceList = {
  devices: [
    {
      id: "0199a1c2-3d4e-7f80-9abc-def0123456aa",
      current: true,
      label: "Chrome on Windows",
      trusted_at: NOW,
      last_seen_at: NOW,
      idle_expires_at: LATER,
      absolute_expires_at: LATER,
    },
  ],
};

export const stubSessions: SessionList = {
  sessions: [
    {
      id: "0199a1c2-3d4e-7f80-9abc-def0123456bb",
      current: true,
      device_label: "Chrome on Windows",
      created_at: NOW,
      last_seen_at: NOW,
    },
  ],
};

export const stubChallenge: Challenge = {
  challenge_id: "0199a1c2-3d4e-7f80-9abc-def0123456cc",
  purpose: "verify_email",
  expires_at: LATER,
  resend_after: NOW,
  attempts_remaining: 5,
};

export const stubSession: AuthSession = {
  access_token: "stub.access.token",
  token_type: "Bearer",
  expires_in: 900,
  user_id: stubMe.id,
  role: "user",
};

/**
 * Answers the boot refresh, so the app resolves to `authenticated` and the
 * guarded routes render instead of redirecting to /login.
 */
export async function stubAuthenticated(page: Page): Promise<void> {
  await page.route("**/api/v1/auth/refresh", (route) =>
    route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify(stubSession),
    }),
  );
}

/**
 * Refuses the boot refresh, so the app resolves to `unauthenticated`.
 *
 * The counterpart to stubAuthenticated, and it exists because signed-out is no
 * longer the same thing as "sees a login form": since ADR-0025 a visitor with no
 * session browses the catalogue and works through a lesson, so those screens
 * have a guest rendering that needs measuring like any other.
 */
export async function stubSignedOut(page: Page): Promise<void> {
  await page.route("**/api/v1/auth/refresh", (route) =>
    route.fulfill({
      status: 401,
      contentType: "application/json",
      body: JSON.stringify({
        type: "https://fluentra.dev/errors/TOKEN_INVALID",
        title: "Unauthorized",
        status: 401,
        code: "TOKEN_INVALID",
      }),
    }),
  );
}

/** Answers registration so the OTP screen can be reached and measured. */
export async function stubRegistration(page: Page): Promise<void> {
  await page.route("**/api/v1/auth/register", (route) =>
    route.fulfill({
      status: 202,
      contentType: "application/json",
      body: JSON.stringify(stubChallenge),
    }),
  );
}

/** Answers the reads the account screens make, so layout can be measured. */
export async function stubAccountApi(page: Page): Promise<void> {
  const json = (body: unknown) => ({
    status: 200,
    contentType: "application/json",
    body: JSON.stringify(body),
  });

  await page.route("**/api/v1/me", (route) => route.fulfill(json(stubMe)));
  await page.route("**/api/v1/me/preferences", (route) =>
    route.fulfill(json(stubPreferences)),
  );
  await page.route("**/api/v1/auth/devices", (route) =>
    route.fulfill(json(stubDevices)),
  );
  await page.route("**/api/v1/auth/sessions", (route) =>
    route.fulfill(json(stubSessions)),
  );
}

/**
 * The learner screens' API, stubbed for layout checks.
 *
 * The 320 px suite runs without a backend, and the five learner screens are the
 * tightest layouts in the app — the four review grade buttons in Vietnamese most
 * of all. Stubbing here keeps them in the narrow-320 project rather than making
 * the layout rule wait on `make dev`.
 *
 * The payloads are the fullest reasonable state, not the emptiest: an empty
 * dashboard has nothing to overflow with.
 */
export async function stubLearningApi(page: Page): Promise<void> {
  const json = (body: unknown) => ({
    status: 200,
    contentType: "application/json",
    body: JSON.stringify(body),
  });

  await page.route("**/api/v1/me/dashboard", (route) =>
    route.fulfill(
      json({
        state: "in_progress",
        next_activity: {
          activity_id: "0199a1c2-3d4e-7f80-9abc-def01234567b",
          lesson_id: "0199a1c2-3d4e-7f80-9abc-def01234567a",
          unit_id: "0199a1c2-3d4e-7f80-9abc-def012345679",
          course_id: "0199a1c2-3d4e-7f80-9abc-def012345678",
          title: "Everyday English: A2–B1 Foundations — Morning Routines & Habits",
          kind: "vocab_multiple_choice",
          skill: "vocabulary",
          estimated_minutes: 15,
        },
        due_reviews_count: 12,
        skill_mastery: [
          { skill: "vocabulary", level: "B1", confidence: 0.85, updated_at: "2026-08-24T09:00:00Z" },
          { skill: "grammar", level: "A2", confidence: 0.4, updated_at: "2026-08-24T09:00:00Z" },
        ],
      }),
    ),
  );

  await page.route("**/api/v1/me/progress", (route) =>
    route.fulfill(
      json({
        courses: [
          {
            course_id: "0199a1c2-3d4e-7f80-9abc-def012345678",
            status: "in_progress",
            completed_activities: 12,
            total_activities: 40,
            percentage: 30,
          },
        ],
        skills: [
          { skill: "vocabulary", level: "B1", confidence: 0.85, updated_at: "2026-08-24T09:00:00Z" },
        ],
      }),
    ),
  );

  await page.route("**/api/v1/reviews/session", (route) =>
    route.fulfill(
      json({
        cards: [
          {
            id: "0199a1c2-3d4e-7f80-9abc-def01234567c",
            user_id: "0199a1c2-3d4e-7f80-9abc-def012345679",
            content_version_id: "0199a1c2-3d4e-7f80-9abc-def01234567b",
            skill: "vocabulary",
            stability: 8.42,
            difficulty: 5.1,
            due_at: "2026-09-02T10:00:00Z",
            reps: 3,
            lapses: 1,
            state: "review",
            content: {
              kind: "vocab_flashcard",
              cefr_level: "B2",
              body: {
                word: "meticulous",
                pos: "adjective",
                ipa: "/məˈtɪkjələs/",
                definition: "Showing great attention to detail.",
                definition_vi: "Tỉ mỉ, cẩn thận, kỹ lưỡng.",
                example_sentence: "She kept meticulous records of every transaction.",
              },
            },
          },
        ],
        total_due: 1,
      }),
    ),
  );

  await page.route("**/api/v1/courses**", (route) =>
    route.fulfill(
      json({
        courses: [
          {
            id: "0199a1c2-3d4e-7f80-9abc-def012345678",
            slug: "everyday-english-a2-b1",
            title: "Everyday English: A2–B1 Foundations",
            cefr_from: "A2",
            cefr_to: "B1",
            status: "published",
            estimated_hours: 24,
          },
        ],
        total: 1,
      }),
    ),
  );
}
