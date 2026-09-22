import { expect, test, type Page } from "@playwright/test";

import type { components } from "@/types/api";

import { stubAccountApi, stubAuthenticated } from "../helpers/stubs";

/**
 * Two mobile defects, guarded where the narrowest viewport lives.
 *
 * 1. The studio course editor's activity-kind select overflowed its row and was
 *    clipped by the unit card's `overflow-hidden`: at 412 px the control sat at
 *    x=345..531, off the screen, and no scroll could reach it. The row wraps
 *    now, so the control is inside the viewport on its own line.
 *
 * 2. The daily practice runner drew the fixed bottom bar over its primary
 *    action. On a 390 px phone the Check button of a four-option exercise
 *    landed behind the bar — partially visible, untappable. The runner is
 *    full-screen and distraction-free (P10.3), so the bar must not render on
 *    it while every framed screen keeps it.
 *
 * Both are geometry, not behaviour, which is the one place stubbing is right
 * (root AGENT.md §9): the payloads are typed against the generated schema, so a
 * stub that drifts fails `pnpm run typecheck`.
 */

type DailyPracticeSet = components["schemas"]["DailyPracticeSet"];
type LessonActivity = components["schemas"]["LessonActivity"];

const ACTIVITY_ID = "0199a1c2-3d4e-7f80-9abc-def01234567b";
const LESSON_ID = "0199a1c2-3d4e-7f80-9abc-def01234567a";
const CONTENT_VERSION_ID = "0199a1c2-3d4e-7f80-9abc-def01234567c";

const multipleChoice: LessonActivity = {
  id: ACTIVITY_ID,
  lesson_id: LESSON_ID,
  position: 1,
  kind: "vocab_multiple_choice",
  content_version_id: CONTENT_VERSION_ID,
  weight: 10,
  config: {
    prompt: "Choose the word that best completes the sentence.",
    options: [
      { id: "a", text: "meticulous" },
      { id: "b", text: "careless" },
      { id: "c", text: "hasty" },
      { id: "d", text: "vague" },
    ],
  } as unknown as Record<string, never>,
};

const dailySet: DailyPracticeSet = {
  id: "0199a1c2-3d4e-7f80-9abc-def0123456ff",
  local_date: "2026-09-21",
  level: "B1",
  activities: [multipleChoice],
};

async function stubDailyPractice(page: Page): Promise<void> {
  const json = (body: unknown) => ({
    status: 200,
    contentType: "application/json",
    body: JSON.stringify(body),
  });

  await page.route("**/api/v1/practice/daily**", (route) =>
    route.fulfill(json(dailySet)),
  );
  await page.route("**/api/v1/activities/*/attempts", (route) =>
    route.fulfill(
      json({
        attempt_id: "0199a1c2-3d4e-7f80-9abc-def0123456ee",
        activity_id: ACTIVITY_ID,
        status: "in_progress",
        started_at: "2026-09-21T00:00:00Z",
      }),
    ),
  );
}

test("the studio editor's activity-kind select stays inside the viewport", async ({
  page,
}) => {
  await stubAuthenticated(page);
  await stubAccountApi(page);

  await page.goto("/studio/courses/new");
  const kindSelect = page.locator("select").nth(1);
  await expect(kindSelect).toBeVisible({ timeout: 15_000 });
  await kindSelect.scrollIntoViewIfNeeded();

  const geometry = await kindSelect.evaluate((select) => {
    const rect = select.getBoundingClientRect();
    const hit = document.elementFromPoint(
      rect.left + rect.width / 2,
      rect.top + rect.height / 2,
    );
    return {
      left: Math.round(rect.left),
      right: Math.round(rect.right),
      viewportWidth: document.documentElement.clientWidth,
      isSelectHit: hit === select || select.contains(hit),
    };
  });

  expect(
    geometry.right,
    `the activity-kind select runs off the screen: right=${geometry.right} against a ${geometry.viewportWidth}px viewport`,
  ).toBeLessThanOrEqual(geometry.viewportWidth + 1);
  expect(geometry.left).toBeGreaterThanOrEqual(-1);
  expect(geometry.isSelectHit).toBe(true);
});

test("the practice runner keeps its primary action clear of the chrome", async ({
  page,
}) => {
  await stubAuthenticated(page);
  await stubAccountApi(page);
  await stubDailyPractice(page);

  await page.goto("/practice/daily?level=B1");
  const check = page.getByRole("button", { name: /Check Answer/i });
  await expect(check).toBeVisible({ timeout: 15_000 });

  // The runner is immersive: the fixed bottom bar must not be drawn over it.
  await expect(page.locator("nav.md\\:hidden")).toHaveCount(0);

  // The action is the thing that was covered, so it is hit-tested rather than
  // measured against a bar that no longer exists.
  await page.getByRole("radio", { name: /meticulous/ }).click();
  await check.scrollIntoViewIfNeeded();
  const isReachable = await check.evaluate((button) => {
    const rect = button.getBoundingClientRect();
    const hit = document.elementFromPoint(
      rect.left + rect.width / 2,
      rect.top + rect.height / 2,
    );
    return hit ? button === hit || button.contains(hit) : false;
  });
  expect(isReachable).toBe(true);
});

test("framed screens still draw the bottom bar", async ({ page }) => {
  await stubAuthenticated(page);
  await stubAccountApi(page);
  await page.route("**/api/v1/courses**", (route) =>
    route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ courses: [], total: 0 }),
    }),
  );

  await page.goto("/practice");
  await expect(page.locator("nav.md\\:hidden")).toBeVisible({
    timeout: 15_000,
  });
});
