import { mkdir, writeFile } from "node:fs/promises";
import { resolve } from "node:path";
import { expect, test } from "@playwright/test";
import type { components } from "../../src/types/api";
import {
  stubAccountApi,
  stubAuthenticated,
  stubLearningApi,
  stubPreferences,
} from "../helpers/stubs";

// Geometry/visual fixtures only. These captures do not verify authenticated
// backend behaviour and must never be presented as real learner data.
const summary: components["schemas"]["GamificationSummary"] = {
  total_xp: 1240,
  level: 5,
  level_start_xp: 1000,
  next_level_xp: 1500,
  xp_today: 30,
  daily_goal_xp: 50,
  streak: { current: 7, longest: 12, freezes_available: 2, hours_remaining: 8 },
  badges: [],
  league: "silver",
  quests: [
    {
      code: "daily_practice",
      name: "Daily practice",
      description: "Complete three activities today.",
      progress: { complete_activities: 2 },
      steps: { complete_activities: 3 },
      reward_xp: 30,
      expires_on: "2026-10-05",
    },
  ],
};
const profile: components["schemas"]["LearningProfile"] = {
  declared_level: "B1",
  target_level: "B2",
  target_exam: "none",
  weekly_minutes_goal: 90,
  motivations: ["travel"],
  created_at: "2026-10-01T00:00:00Z",
  updated_at: "2026-10-01T00:00:00Z",
};
const plan: components["schemas"]["WeeklyPlan"] = {
  week_start: "2026-09-28",
  minutes_goal: 90,
  progress: { minutes: 30, items_done: 2, items_total: 7 },
  items: Array.from({ length: 7 }, (_, index) => ({
    kind: "lesson",
    minutes: 10,
    done: index < 2,
    focus: index === 2,
    lesson_id: `0199a1c2-3d4e-7f80-9abc-def01234560${index}`,
    title: `Lesson ${index + 1}: Everyday English — Morning Routines and Habits`,
  })),
};
const foundation: components["schemas"]["FoundationPathNode"] = {
  id: "0199a1c2-3d4e-7f80-9abc-def012345677",
  namespace: "grammar",
  code: "PRESENT_PERFECT",
  label: "Present perfect",
  cefr_level: "B1",
  attempts: 0,
  score: 0,
  mastered: false,
  next: true,
};

for (const locale of ["en", "vi"] as const) {
  test(`Dashboard geometry and captures — ${locale}`, async ({ page }) => {
    test.setTimeout(180_000);
    await stubAuthenticated(page);
    await stubAccountApi(page);
    await stubLearningApi(page);
    const payloads = {
      "/me/learning-profile": profile,
      "/me/gamification": summary,
      "/me/weekly-plan": plan,
      "/me/foundation/next": foundation,
      "/me/placement": {
        result: null,
        active_session: null,
        retake_available_at: null,
        invite_available: false,
      } satisfies components["schemas"]["PlacementOverview"],
      "/leaderboard": { entries: [], opted_in: false },
    };
    for (const [path, body] of Object.entries(payloads)) {
      await page.route(`**/api/v1${path}`, (route) =>
        route.fulfill({ json: body }),
      );
    }
    const output = resolve("../docs/ui-review/dashboard-refinement");
    await mkdir(output, { recursive: true });
    const measurements: unknown[] = [];
    for (const theme of ["light", "dark"] as const) {
      await page.route("**/api/v1/me/preferences", (route) =>
        route.fulfill({
          json: {
            ...stubPreferences,
            locale,
            theme,
            practice_level: "B1",
          } satisfies components["schemas"]["Preferences"],
        }),
      );
      for (const width of [320, 390, 768, 1440]) {
        await page.setViewportSize({
          width,
          height: width === 1440 ? 900 : 844,
        });
        await page.emulateMedia({
          colorScheme: theme,
          reducedMotion: "reduce",
        });
        await page.addInitScript(
          ({ locale, theme }) => {
            localStorage.setItem("fluentra.locale", locale);
            localStorage.setItem("fluentra.theme", theme);
          },
          { locale, theme },
        );
        await page.goto("/");
        const hero = page.getByRole("region", {
          name: locale === "vi" ? "Tiếp tục học" : "Continue Learning",
        });
        await expect(hero.getByRole("link")).toHaveAttribute(
          "href",
          "/learn/lesson/0199a1c2-3d4e-7f80-9abc-def01234567a",
        );
        await expect(hero.getByRole("progressbar")).toHaveAttribute(
          "aria-valuenow",
          "30",
        );
        const weekly = page.getByRole("complementary", {
          name: locale === "vi" ? "Kế hoạch tuần này" : "This week's plan",
        });
        // One "Show N more" expander at every width; the card no longer has a
        // separate desktop "View all".
        const expander = weekly
          .getByRole("button", { expanded: false })
          .first();
        await expect(expander).toBeVisible();
        await page.evaluate(() => document.fonts.ready);
        const metrics = await page.evaluate(() => ({
          viewport: document.documentElement.clientWidth,
          scrollWidth: document.documentElement.scrollWidth,
          mainHeight: Math.round(
            document.querySelector("main")!.getBoundingClientRect().height,
          ),
        }));
        expect(metrics.scrollWidth).toBeLessThanOrEqual(metrics.viewport + 1);
        if (width === 1440) {
          const mainBox = await hero.boundingBox();
          const weeklyBox = await weekly.boundingBox();
          expect(weeklyBox!.y).toBeGreaterThan(mainBox!.y + mainBox!.height);
        }
        await page.evaluate(() => window.scrollTo(0, 0));
        const name = `${locale}-${theme}-${width}`;
        await page.screenshot({
          path: resolve(output, `${name}.png`),
          fullPage: true,
          animations: "disabled",
        });
        measurements.push({ name, ...metrics });
        // Geometry fixture check: expanding exposes, rather than drops, all rows.
        await expander.click();
        await expect(
          page.getByRole("link", { name: /^Lesson 7:/ }),
        ).toBeVisible();
      }
    }
    await writeFile(
      resolve(output, `${locale}-measurements.json`),
      JSON.stringify(measurements, null, 2),
    );
  });
}
