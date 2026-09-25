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
          title:
            "Everyday English: A2–B1 Foundations — Morning Routines & Habits",
          kind: "vocab_multiple_choice",
          skill: "vocabulary",
          estimated_minutes: 15,
        },
        due_reviews_count: 12,
        skill_mastery: [
          {
            skill: "vocabulary",
            level: "B1",
            confidence: 0.85,
            updated_at: "2026-08-24T09:00:00Z",
          },
          {
            skill: "grammar",
            level: "A2",
            confidence: 0.4,
            updated_at: "2026-08-24T09:00:00Z",
          },
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
          {
            skill: "vocabulary",
            level: "B1",
            confidence: 0.85,
            updated_at: "2026-08-24T09:00:00Z",
          },
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
                example_sentence:
                  "She kept meticulous records of every transaction.",
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

/** The session a layout check of the admin screens needs. */
export const stubAdminSession: AuthSession = {
  ...stubSession,
  role: "admin",
};

/**
 * Authenticates as an administrator, so `/admin/*` renders instead of
 * redirecting to the dashboard.
 */
export async function stubAuthenticatedAdmin(page: Page): Promise<void> {
  await page.route("**/api/v1/auth/refresh", (route) =>
    route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify(stubAdminSession),
    }),
  );
}

/**
 * The WO 21 screens' API, stubbed for layout checks at 320 px.
 *
 * The payloads are the fullest reasonable state — a long file name, a long
 * slug, a fingerprint — because an empty screen has nothing to overflow with.
 * The admin payloads need `stubAuthenticatedAdmin` first; the learner ones do
 * not.
 */
export async function stubWo21Api(page: Page): Promise<void> {
  const json = (body: unknown) => ({
    status: 200,
    contentType: "application/json",
    body: JSON.stringify(body),
  });

  await page.route("**/api/v1/me/resources**", (route) =>
    route.fulfill(
      json({
        items: [
          {
            id: "018f3a5e-7b82-7d2c-80a2-bf3d6118d531",
            kind: "file",
            title:
              "Advanced Grammar Guide — Present Perfect and Past Simple (2026 edition)",
            status: "validated",
            failure_reason: "",
            original_filename:
              "advanced-grammar-guide-present-perfect-and-past-simple.pdf",
            declared_mime: "application/pdf",
            detected_mime: "application/pdf",
            byte_size: 2458120,
            checksum: null,
            source_url: null,
            download_url: "https://storage.example.com/file.pdf",
            created_at: NOW,
            updated_at: NOW,
            validated_at: NOW,
            renditions: [],
            extraction: {
              source: "pdf_text",
              char_count: 1540,
              truncated: false,
              excerpt: "Unit 1: Present Perfect Tense.",
            },
            classification: {
              cefr_estimate: "B1",
              skill: "grammar",
              nodes: [{ code: "PRESENT_PERFECT", label: "Present Perfect" }],
            },
          },
        ],
        total: 1,
        page: 1,
        page_size: 100,
      }),
    ),
  );

  await page.route("**/api/v1/exam-versions", (route) =>
    route.fulfill(
      json({
        items: [
          {
            id: "20000000-0000-0000-0000-000000000002",
            exam_family: "vstep",
            code: "VSTEP_3_5",
            title: "VSTEP (Level 3-5 / B1-C1) — Vietnamese Standardised Test",
            total_minutes: 100,
            scoring: { type: "raw" },
            source_url: "https://example.test/vstep",
            verified_at: "2026-09-20",
            is_current: true,
            notes: "",
            distinct_tests_possible: 2,
            fixed_test_count: 1,
            blueprints: [
              {
                id: "30000000-0000-0000-0010-000000000002",
                name: "vstep_default",
                cefr_distribution: { B1: 1 },
                node_distribution: {},
              },
            ],
            parts: [
              {
                part_number: 1,
                section: "listening",
                kind: "listening_comprehension",
                question_count: 35,
                group_size: 1,
              },
            ],
          },
        ],
      }),
    ),
  );

  // The version's numbered fixed tests (WO 22 Stage J): the hub's second level.
  await page.route("**/api/v1/exam-versions/*/tests", (route) =>
    route.fulfill(
      json({
        items: [
          {
            id: "30000000-0000-0000-0000-000000000001",
            number: 1,
            title: "Đề 1",
            question_count: 35,
            minutes: 100,
          },
        ],
      }),
    ),
  );

  await page.route("**/api/v1/exams", (route) => route.fulfill(json([])));
  await page.route("**/api/v1/exam-attempts**", (route) =>
    route.fulfill(json({ items: [], sittings_today: 0, daily_limit: 5 })),
  );

  await page.route("**/api/v1/me/permissions", (route) =>
    route.fulfill(
      json({
        permissions: [
          "content.review",
          "content.publish",
          "questionbank.read",
          "questionbank.create",
          "admin.dashboard",
        ],
      }),
    ),
  );

  await page.route("**/api/v1/admin/review-queue**", (route) =>
    route.fulfill(
      json({
        items: [
          {
            id: "0199a1c2-3d4e-7f80-9abc-def012345602",
            item_id: "0199a1c2-3d4e-7f80-9abc-def012345601",
            slug: "foundation-b1-grammar-tense-choice-a1b2c3d4-e5f6a7b8",
            kind: "grammar_tense_choice",
            cefr_level: "B1",
            status: "draft",
            body: {
              prompt: "She ___ lived here for three years.",
              options: [
                { id: "A", text: "has" },
                { id: "B", text: "have" },
              ],
              correct_option_id: "A",
            },
            blind_solve_answer: { selected_option_id: "A" },
            cefr_reasoning: "Present perfect corresponds to B1.",
            provenance: { model: "test-model", prompt_version: "v1" },
            node_codes: ["PRESENT_PERFECT"],
            created_at: NOW,
          },
        ],
        total: 1,
      }),
    ),
  );

  await page.route("**/api/v1/admin/questions**", (route) =>
    route.fulfill(
      json({
        items: [
          {
            id: "0199a1c2-3d4e-7f80-9abc-def01234567a",
            content_item_id: "0199a1c2-3d4e-7f80-9abc-def01234567b",
            activity_id: "0199a1c2-3d4e-7f80-9abc-def01234567c",
            exam_part_id: null,
            kind: "grammar_tense_choice",
            skill: "grammar",
            cefr_level: "B1",
            difficulty: null,
            question_count: 1,
            fingerprint:
              "7d2b45f1e8a93102efb132a0c49876543210fedcba9876543210fedcba987654",
            provenance: { model: "test-model" },
            status: "published",
            created_at: NOW,
            updated_at: NOW,
          },
        ],
        total: 1,
        limit: 20,
        offset: 0,
      }),
    ),
  );
}
